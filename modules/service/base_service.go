package service

import (
	"errors"
	"fmt"
	"github.com/scylladb/gosible/module_utils/gosibleModule"
	"github.com/scylladb/gosible/module_utils/locale"
	"github.com/scylladb/gosible/utils/slices"
	"github.com/scylladb/gosible/utils/sysInfo"
	"strings"
)

func newBasicServiceSharedState(m *gosibleModule.GosibleModule[*Params]) *basicServiceSharedState {
	return &basicServiceSharedState{
		module: m,
	}
}

func newBaseService(state *basicServiceSharedState, deriver baseServiceDeriver) service {
	return &baseService{basicServiceSharedState: state, baseServiceDeriver: deriver}
}

type basicServiceSharedState struct {
	module  *gosibleModule.GosibleModule[*Params]
	changed bool
	running *bool
}

type baseServiceDeriver interface {
	getServiceTools() error
	serviceEnable(bool) (*gosibleModule.RunCommandResult, error)
	getServiceStatus() (*bool, error)
	serviceControl(action string) (*gosibleModule.RunCommandResult, error)
}

type baseService struct {
	changed bool
	baseServiceDeriver
	*basicServiceSharedState
}

const newline = "\n"

func (b *baseService) checkPs() error {
	psFlags := "auxww"
	if sysInfo.Platform() == "SunOs" {
		psFlags = "-ef"
	}
	// Find ps binary
	psBin, err := b.module.GetBinPath("ps", nil, true)
	if err != nil {
		return err
	}
	ret, err := b.module.RunCommand(fmt.Sprintf("%s %s", psBin, psFlags), gosibleModule.RunCommandDefaultKwargs())
	// If rc is 0, set running as appropriate
	if ret.Rc == 0 {
		b.setRunning(false)
		if pat := b.module.Params.Pattern; pat != "" {
			lines := strings.Split(string(ret.Stdout), newline)
			for _, line := range lines {
				if strings.Contains(line, b.module.Params.Pattern) && !strings.Contains(line, "pattern=") {
					*b.running = true
				}
			}
		}
	}

	return nil
}

func (b *baseService) checkServiceChanged() (bool, error) {
	state := b.module.Params.State
	if state == "" && b.running == nil {
		return false, errors.New("failed determining service state, possible typo of service name?")
	}

	hasStarted := !isTrue(b.running) && slices.Contains([]string{"reloaded", "started"}, state)
	hasStopped := isTrue(b.running) && slices.Contains([]string{"reloaded", "stopped"}, state)
	hasRestarted := state == "restarted"

	svcChanged := hasStarted || hasStopped || hasRestarted
	return svcChanged, nil
}

func isTrue(b *bool) bool {
	return b != nil && *b
}

func (b *baseService) hasChanged() bool {
	return b.changed
}

func (b *baseService) modifyServiceState() (*gosibleModule.RunCommandResult, error) {
	action := ""
	if !isTrue(b.running) && b.module.Params.State == "reloaded" {
		action = "start"
	} else {
		switch b.module.Params.State {
		case "started":
			action = "start"
		case "stopped":
			action = "stop"
		case "reloaded":
			action = "reload"
		case "restarted":
			action = "restart"
		}
	}

	return b.serviceControl(action)
}

func (ss *basicServiceSharedState) executeCommand(cmd string) (*gosibleModule.RunCommandResult, error) {
	kwargs, err := ss.getExecuteCommandKwargs()
	if err != nil {
		return nil, err
	}

	// chkconfig localizes messages and we're screen scraping so make
	// sure we use the C locale
	return ss.module.RunCommand(cmd, kwargs)
}

func (ss *basicServiceSharedState) executeCommandDaemonized(cmd string) (*gosibleModule.RunCommandResult, error) {
	// Ansible executes this command in a daemon. But there seems to be no clear reason why.
	// TODO dig further why ansible daemonizes it.
	return ss.executeCommand(cmd)
}

func (ss *basicServiceSharedState) getLangEnv() (map[string]string, error) {
	loc, err := locale.GetBestParsableLocale(ss.module, nil, false)
	if err != nil {
		return nil, err
	}
	return map[string]string{
		"LANG":       loc,
		"LC_ALL":     loc,
		"LC_MESSAGE": loc,
	}, nil
}

func (ss *basicServiceSharedState) getExecuteCommandKwargs() (*gosibleModule.RunCommandKwargs, error) {
	langEnv, err := ss.getLangEnv()
	if err != nil {
		return nil, err
	}
	kwargs := gosibleModule.RunCommandDefaultKwargs()
	kwargs.EnvironUpdate = langEnv

	return kwargs, nil
}

func (ss *basicServiceSharedState) setRunning(b bool) {
	ss.running = &b
}
