package service

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/scylladb/gosible/module_utils/gosibleModule"
	"github.com/scylladb/gosible/modules"
	"github.com/scylladb/gosible/utils/slices"
	"github.com/scylladb/gosible/utils/sysInfo"
	"github.com/scylladb/gosible/utils/types"
)

var _ modules.Module = New()

type Module struct {
	*gosibleModule.GosibleModule[*Params]
}

type Params struct {
	Arguments string `mapstructrure:"arguments"`
	Enabled   *bool  `mapstructrure:"enabled"`
	Name      string `mapstructrure:"name"`
	Pattern   string `mapstructrure:"pattern"`
	Runlevel  string `mapstructrure:"runlevel"`
	Sleep     int    `mapstructrure:"sleep"`
	State     string `mapstructrure:"state"`
	Use       string `mapstructrure:"use"`
}

type Return struct {
	Name    string
	Changed bool
	State   string
	Enabled bool
}

func New() *Module {
	return &Module{GosibleModule: gosibleModule.New(&Params{Runlevel: "default"})}
}

func (m *Module) Name() string {
	return "service"
}

func (m *Module) Run(ctx *modules.RunContext, vars types.Vars) *modules.Return {
	if err := m.ParseParams(ctx, prepareVars(vars)); err != nil {
		return m.MarkReturnFailed(err)
	}
	svc, err := m.getService()
	if err != nil {
		return m.MarkReturnFailed(err)
	}
	m.addInstantiationDebugMsg()
	if err = svc.getServiceTools(); err != nil {
		return m.MarkReturnFailed(err)
	}
	ret := Return{}

	if err = m.handleServiceEnabled(svc, &ret); err != nil {
		return m.MarkReturnFailed(err)
	}
	if m.Params.State == "" {
		ret.Changed = svc.hasChanged()
		return m.GetReturn()
	}
	ret.State = m.Params.State
	if m.Params.Pattern != "" {
		if err = svc.checkPs(); err != nil {
			return m.MarkReturnFailed(err)
		}
	} else {
		if _, err = svc.getServiceStatus(); err != nil {
			return m.MarkReturnFailed(err)
		}
	}
	svcChanged, err := svc.checkServiceChanged()
	if err != nil {
		return m.MarkReturnFailed(err)
	}
	if svcChanged {
		modRet, err := svc.modifyServiceState()
		if err = checkRc(modRet); err != nil {
			return m.MarkReturnFailed(err)
		}
	}
	ret.Changed = svc.hasChanged() || svcChanged
	if err = m.addStateToReturn(svc, &ret); err != nil {
		return m.MarkReturnFailed(err)
	}

	if err = m.Close(); err != nil {
		return m.MarkReturnFailed(err)
	}
	return m.GetReturn()
}

type service interface {
	checkPs() error
	checkServiceChanged() (bool, error)
	hasChanged() bool
	getServiceTools() error
	modifyServiceState() (*gosibleModule.RunCommandResult, error)
	serviceEnable(enable bool) (*gosibleModule.RunCommandResult, error)
	getServiceStatus() (*bool, error)
	serviceControl(action string) (*gosibleModule.RunCommandResult, error)
}

func checkRc(modRet *gosibleModule.RunCommandResult) error {
	if modRet.Rc != 0 {
		if len(modRet.Stderr) != 0 {
			if bytes.Contains(modRet.Stderr, []byte("Job is already running")) {
				// upstart got confused, one such possibility is MySQL on Ubuntu 12.04
				// where status may report it has no start/stop links and we could
				// not get accurate status
			} else {
				return errors.New(string(modRet.Stderr))
			}
		} else {
			return errors.New(string(modRet.Stdout))
		}
	}
	return nil
}

func getUnimplementedServiceErr() error {
	system := sysInfo.Platform()
	distro := sysInfo.Distribution()
	msgPlatform := system
	if distro != "" {
		msgPlatform = fmt.Sprintf("%s (%s)", msgPlatform, distro)
	}
	return fmt.Errorf("service module cannot be used on platform %s", msgPlatform)
}

// TODO implement services for other platforms.
var platformToService = map[string]map[string]func(module *gosibleModule.GosibleModule[*Params]) (service, error){
	"Linux": {
		"": newLinuxService,
	},
}

func (m *Module) getService() (service, error) {
	strat := sysInfo.GetPlatformSpecificValue(platformToService)
	if strat != nil {
		return strat(m.GosibleModule)
	}

	return nil, getUnimplementedServiceErr()
}

func (m *Module) addInstantiationDebugMsg() {
	distro := sysInfo.Distribution()
	platform := sysInfo.Platform()
	m.Debug(fmt.Sprintf("Service instantiated - platform %s", platform))
	if distro != "" {
		if _, ok := platformToService[platform][distro]; ok {
			m.Debug(fmt.Sprintf("Service instantiated - platform %s", platform))
		}
	}
}

func (m *Module) handleServiceEnabled(svc service, ret *Return) error {
	if m.Params.Enabled != nil {
		_, err := svc.serviceEnable(*m.Params.Enabled)
		if err != nil {
			return err
		}
		ret.Enabled = *m.Params.Enabled
	}
	return nil
}

func (m *Module) addStateToReturn(svc service, ret *Return) error {
	if m.Params.State == "" {
		status, err := svc.getServiceStatus()
		if err != nil {
			return err
		}
		if status == nil {
			ret.State = "absent"
		} else if *status {
			ret.State = "stopped"
		} else {
			ret.State = "started"
		}
	} else {
		if slices.Contains([]string{"reloaded", "restarted", "started"}, m.Params.State) {
			ret.State = "started"
		} else {
			ret.State = "stopped"
		}
	}

	return nil
}

func prepareVars(vars types.Vars) types.Vars {
	if v, ok := vars["args"].(string); ok {
		vars["arguments"] = v
	}

	return vars
}

func (p *Params) Validate() error {
	if p.Name == "" {
		return errors.New("name must be provided")
	}
	if p.State == "" && p.Enabled == nil {
		return errors.New("state or enabled must be set")
	}
	if p.State != "" {
		stateOpts := []string{"started", "stopped", "reloaded", "restarted"}
		if !slices.Contains(stateOpts, p.State) {
			return fmt.Errorf("state must be one of %v got %s", stateOpts, p.State)
		}
	}

	return nil
}
