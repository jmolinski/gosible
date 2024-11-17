package hostname

import (
	"bufio"
	"errors"
	"fmt"
	"github.com/Showmax/go-fqdn"
	"github.com/scylladb/gosible/module_utils/gosibleModule"
	"github.com/scylladb/gosible/modules"
	pathUtils "github.com/scylladb/gosible/utils/path"
	"github.com/scylladb/gosible/utils/sysInfo"
	"github.com/scylladb/gosible/utils/types"
	"io/ioutil"
	"os"
	"strconv"
	"strings"
)

type Module struct {
	*gosibleModule.GosibleModule[*Params]
}

type Return struct {
	Name string
}

type strategy interface {
	updateCurrentAndPermanentHostname() (bool, error)
	updateCurrentHostname() error
	updatePermanentHostname() error
	getCurrentHostname() (string, error)
	getPermanentHostname() (string, error)
	setCurrentHostname(string) error
	setPermanentHostname(string) error
}

type baseStrategyDeriver interface {
	getCurrentHostname() (string, error)
	getPermanentHostname() (string, error)
	setPermanentHostname(string) error
	setCurrentHostname(string) error
}

type baseStrategy struct {
	module  *gosibleModule.GosibleModule[*Params]
	changed bool
	baseStrategyDeriver
}

func (b *baseStrategy) updateCurrentAndPermanentHostname() (bool, error) {
	if err := b.updateCurrentHostname(); err != nil {
		return false, nil
	}
	if err := b.updatePermanentHostname(); err != nil {
		return false, nil
	}
	return b.changed, nil
}

func (b *baseStrategy) updateCurrentHostname() error {
	currentName, err := b.getCurrentHostname()
	if err != nil {
		return err
	}
	if currentName != b.module.Params.Name {
		if err != b.setCurrentHostname(b.module.Params.Name) {
			return err
		}
		b.changed = true
	}
	return nil
}

func (b *baseStrategy) updatePermanentHostname() error {
	currentName, err := b.getPermanentHostname()
	if err != nil {
		return err
	}
	if currentName != b.module.Params.Name {
		if err != b.setPermanentHostname(b.module.Params.Name) {
			return err
		}
		b.changed = true
	}
	return nil
}

type fileStrategy struct {
	file string
}

func wrapUpdateErr(err error) error {
	return fmt.Errorf("failed to update hostname: %w", err)
}

func wrapGetErr(err error) error {
	return fmt.Errorf("failed to read hostname: %w", err)
}

func (f *fileStrategy) getCurrentHostname() (string, error) {
	return f.getPermanentHostname()
}

func (f *fileStrategy) setCurrentHostname(string) error {
	return nil
}

func (f *fileStrategy) getPermanentHostname() (string, error) {
	regular, err := pathUtils.IsRegular(f.file)
	if err != nil {
		return "", err
	}
	if !regular {
		return "", nil
	}
	content, err := ioutil.ReadFile(f.file)
	if err != nil {
		return "", wrapGetErr(err)
	}
	return strings.TrimSpace(string(content)), nil
}

func (f *fileStrategy) setPermanentHostname(name string) error {
	if err := os.WriteFile(f.file, []byte(name+"\n"), 0644); err != nil {
		return wrapUpdateErr(err)
	}
	return nil
}

type redHatStrategy struct {
	networkFile string
}

func (r *redHatStrategy) getCurrentHostname() (string, error) {
	return r.getPermanentHostname()
}

func (r *redHatStrategy) getPermanentHostname() (string, error) {
	file, err := os.Open(r.networkFile)
	if err != nil {
		return "", fmt.Errorf("failed to read hostname: %w", err)
	}
	sc := bufio.NewScanner(file)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if strings.HasPrefix(line, "HOSTNAME") {
			for i, v := range strings.Split(line, "=") {
				if i == 1 {
					return strings.TrimSpace(v), nil
				}
			}
		}
	}
	return "", fmt.Errorf("unable to locate HOSTNAME entry in %s", r.networkFile)
}

func (r *redHatStrategy) setPermanentHostname(name string) error {
	file, err := os.OpenFile(r.networkFile, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		return wrapUpdateErr(err)
	}
	defer file.Close()
	sc := bufio.NewScanner(file)
	lines := make([]string, 0, 16)
	found := false
	for sc.Scan() {
		line := sc.Text()
		if strings.HasPrefix(strings.TrimSpace(line), "HOSTNAME") {
			found = true
			lines = append(lines, fmt.Sprintf("HOSTNAME=%s", name))
		} else {
			lines = append(lines, line)
		}
	}
	if !found {
		lines = append(lines, fmt.Sprintf("HOSTNAME=%s", name))
	}
	data := []byte(strings.Join(lines, "\n") + "\n")
	if err = file.Truncate(0); err != nil {
		return wrapUpdateErr(err)
	}
	if _, err = file.Seek(0, 0); err != nil {
		return wrapUpdateErr(err)
	}
	n, err := file.Write(data)
	if err != nil {
		return wrapUpdateErr(err)
	}
	if n != len(data) {
		return wrapUpdateErr(fmt.Errorf("couldn't write everything to %s", r.networkFile))
	}
	return nil
}

func (r *redHatStrategy) setCurrentHostname(string) error {
	return nil
}

type openRcStrategy struct {
	file string
}

func (o *openRcStrategy) getCurrentHostname() (string, error) {
	return o.getPermanentHostname()
}

func (o *openRcStrategy) getPermanentHostname() (string, error) {
	regular, err := pathUtils.IsRegular(o.file)
	if err != nil {
		return "", wrapGetErr(err)
	}
	if !regular {
		return "", nil
	}
	file, err := os.Open(o.file)
	if err != nil {
		return "", wrapGetErr(err)
	}
	defer file.Close()
	sc := bufio.NewScanner(file)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if strings.HasPrefix(strings.TrimSpace(line), "hostname=") {
			return strings.Trim(line[10:], "\""), nil
		}
	}
	return "", wrapGetErr(fmt.Errorf("couldn't find hostname line in %s", o.file))
}

func (o *openRcStrategy) setPermanentHostname(name string) error {
	file, err := os.OpenFile(o.file, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		return wrapUpdateErr(err)
	}
	defer file.Close()
	sc := bufio.NewScanner(file)
	lines := make([]string, 0, 16)
	found := false
	for sc.Scan() {
		line := sc.Text()
		if strings.HasPrefix(strings.TrimSpace(line), "hostname=") {
			found = true
			lines = append(lines, fmt.Sprintf("hostname=\"%s\"", name))
		} else {
			lines = append(lines, line)
		}
	}
	if !found {
		lines = append(lines, fmt.Sprintf("hostname=\"%s\"", name))
	}
	data := []byte(strings.Join(lines, "\n") + "\n")
	if err = file.Truncate(0); err != nil {
		return wrapUpdateErr(err)
	}
	if _, err = file.Seek(0, 0); err != nil {
		return wrapUpdateErr(err)
	}
	n, err := file.Write(data)
	if err != nil {
		return wrapUpdateErr(err)
	}
	if n != len(data) {
		return wrapUpdateErr(fmt.Errorf("couldn't write everything to %s", o.file))
	}
	return nil
}

func (o *openRcStrategy) setCurrentHostname(string) error {
	return nil
}

type alpineStrategy struct {
	*fileStrategy
	command string
	module  *gosibleModule.GosibleModule[*Params]
}

func (a *alpineStrategy) setCurrentHostname(s string) error {
	if err := a.fileStrategy.setCurrentHostname(s); err != nil {
		return err
	}
	hostnameCmd, err := a.module.GetBinPath(a.command, nil, true)
	if err != nil {
		return wrapUpdateErr(err)
	}
	cmd := []string{hostnameCmd, "-F", a.file}
	res, err := a.module.RunCommand(cmd, gosibleModule.RunCommandDefaultKwargs())
	if err != nil {
		return wrapUpdateErr(err)
	}
	if err = res.Validate(); err != nil {
		return wrapUpdateErr(err)
	}
	return nil
}

type freeBSDStrategy struct {
	file        string
	hostnameCmd string
	module      *gosibleModule.GosibleModule[*Params]
}

func (f *freeBSDStrategy) getCurrentHostname() (string, error) {
	cmd := []string{f.hostnameCmd}
	res, err := f.module.RunCommand(cmd, gosibleModule.RunCommandDefaultKwargs())
	if err != nil {
		return "", wrapGetErr(err)
	}
	if err = res.Validate(); err != nil {
		return "", wrapGetErr(err)
	}
	return strings.TrimSpace(string(res.Stdout)), nil
}

func (f *freeBSDStrategy) getPermanentHostname() (string, error) {
	regular, err := pathUtils.IsRegular(f.file)
	if err != nil {
		return "", wrapGetErr(err)
	}
	if !regular {
		return "", nil
	}
	file, err := os.Open(f.file)
	if err != nil {
		return "", wrapGetErr(err)
	}
	defer file.Close()
	sc := bufio.NewScanner(file)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if strings.HasPrefix(strings.TrimSpace(line), "hostname=") {
			return strings.Trim(line[10:], "\""), nil
		}
	}
	return "", wrapGetErr(fmt.Errorf("couldn't find hostname line in %s", f.file))
}

func (f *freeBSDStrategy) setPermanentHostname(name string) error {
	lines := make([]string, 0, 16)
	found := false
	regular, err := pathUtils.IsRegular(f.file)
	var file *os.File
	if err != nil {
		return wrapUpdateErr(err)
	}
	if regular {
		file, err = os.OpenFile(f.file, os.O_RDWR|os.O_CREATE, 0644)
		if err != nil {
			return wrapUpdateErr(err)
		}
		defer file.Close()
		sc := bufio.NewScanner(file)
		for sc.Scan() {
			line := strings.TrimSpace(sc.Text())
			if strings.HasPrefix(line, "hostname=") {
				found = true
				lines = append(lines, fmt.Sprintf("hostname=\"%s\"", name))
			} else {
				lines = append(lines, line)
			}
		}
	} else {
		file, err = os.Create(f.file)
		if err != nil {
			return wrapUpdateErr(err)
		}
		defer file.Close()
	}
	if !found {
		lines = append(lines, fmt.Sprintf("hostname=\"%s\"", name))
	}
	data := []byte(strings.Join(lines, "\n") + "\n")
	if err = file.Truncate(0); err != nil {
		return wrapUpdateErr(err)
	}
	if _, err = file.Seek(0, 0); err != nil {
		return wrapUpdateErr(err)
	}
	n, err := file.Write(data)
	if err != nil {
		return wrapUpdateErr(err)
	}
	if n != len(data) {
		return wrapUpdateErr(fmt.Errorf("couldn't write everything to %s", f.file))
	}
	return nil
}

func (f *freeBSDStrategy) setCurrentHostname(name string) error {
	cmd := []string{f.hostnameCmd, name}
	res, err := f.module.RunCommand(cmd, gosibleModule.RunCommandDefaultKwargs())
	if err != nil {
		return wrapUpdateErr(err)
	}
	if err = res.Validate(); err != nil {
		return wrapUpdateErr(err)
	}
	return nil
}

type solarisStrategy struct {
	hostnameCmd string
	module      *gosibleModule.GosibleModule[*Params]
}

func (s *solarisStrategy) getCurrentHostname() (string, error) {
	return s.getPermanentHostname()
}

func (s *solarisStrategy) getPermanentHostname() (string, error) {
	fmri := "svc:/system/identity:node"
	pattern := "config/nodename"
	cmd := fmt.Sprintf("/usr/sbin/svccfg -s %s listprop -o value %s", fmri, pattern)
	kwargs := gosibleModule.RunCommandDefaultKwargs()
	kwargs.UseUnsafeShell = true
	res, err := s.module.RunCommand(cmd, kwargs)
	if err != nil {
		return "", wrapGetErr(err)
	}
	if err = res.Validate(); err != nil {
		return "", wrapGetErr(err)
	}
	return strings.TrimSpace(string(res.Stdout)), nil
}

func (s *solarisStrategy) setHostname(name string, permanent bool) error {
	cmd := []string{s.hostnameCmd, "-t", name}
	if permanent {
		cmd = []string{s.hostnameCmd, name}
	}
	res, err := s.module.RunCommand(cmd, gosibleModule.RunCommandDefaultKwargs())
	if err != nil {
		return wrapUpdateErr(err)
	}
	if err = res.Validate(); err != nil {
		return wrapUpdateErr(err)
	}
	return nil
}

func (s *solarisStrategy) setPermanentHostname(name string) error {
	return s.setHostname(name, true)
}

func (s *solarisStrategy) setCurrentHostname(name string) error {
	return s.setHostname(name, false)
}

type systemdStrategy struct {
	hostnamectlCmd string
	module         *gosibleModule.GosibleModule[*Params]
}

func (s *systemdStrategy) getCurrentHostname() (string, error) {
	return s.getHostname(false)
}

func (s *systemdStrategy) getPermanentHostname() (string, error) {
	return s.getHostname(true)
}

func (s *systemdStrategy) getHostname(permanent bool) (string, error) {
	flag := "--transient"
	if permanent {
		flag = "--static"
	}
	cmd := []string{s.hostnamectlCmd, flag, "status"}
	res, err := s.module.RunCommand(cmd, gosibleModule.RunCommandDefaultKwargs())
	if err != nil {
		return "", wrapGetErr(err)
	}
	if err = res.Validate(); err != nil {
		return "", wrapGetErr(err)
	}
	return strings.TrimSpace(string(res.Stdout)), nil
}

func (s *systemdStrategy) setHostname(name string, flag string) error {
	if len(name) > 64 {
		return wrapUpdateErr(fmt.Errorf("name cannot be longer than 64 characters on systemd servers, try a shorter name"))
	}
	cmd := []string{s.hostnamectlCmd, flag, "set-hostname", name}
	res, err := s.module.RunCommand(cmd, gosibleModule.RunCommandDefaultKwargs())
	if err != nil {
		return wrapUpdateErr(err)
	}
	if err = res.Validate(); err != nil {
		return wrapUpdateErr(err)
	}
	return nil
}

func (s *systemdStrategy) setPermanentHostname(name string) error {
	if err := s.setHostname(name, "--pretty"); err != nil {
		return err
	}
	return s.setHostname(name, "--static")
}

func (s *systemdStrategy) setCurrentHostname(name string) error {
	return s.setHostname(name, "--transient")
}

type baseDeriverStrategy struct{}

func (b *baseDeriverStrategy) getCurrentHostname() (string, error) {
	return b.getPermanentHostname()
}

func (b *baseDeriverStrategy) getPermanentHostname() (string, error) {
	return "", wrapGetErr(errors.New("getting hostname is not implemented"))
}

func (b *baseDeriverStrategy) setPermanentHostname(s string) error {
	return wrapUpdateErr(errors.New("setting hostname is not implemented"))
}

func (b *baseDeriverStrategy) setCurrentHostname(s string) error {
	return nil
}

func newBaseStrategy(module *gosibleModule.GosibleModule[*Params], deriver baseStrategyDeriver) strategy {
	return &baseStrategy{module: module, baseStrategyDeriver: deriver}
}

func newRedHatStrategy(module *gosibleModule.GosibleModule[*Params]) (strategy, error) {
	return newBaseStrategy(module, &redHatStrategy{networkFile: "/etc/sysconfig/network"}), nil
}

func newAlpineStrategy(module *gosibleModule.GosibleModule[*Params]) (strategy, error) {
	return newBaseStrategy(module, &alpineStrategy{
		fileStrategy: &fileStrategy{file: "/etc/hostname"},
		command:      "hostname",
		module:       module,
	}), nil
}

func newOpenBSDStrategy(module *gosibleModule.GosibleModule[*Params]) (strategy, error) {
	return newBaseStrategy(module, &fileStrategy{
		file: "/etc/myname",
	}), nil
}

func newFreeBSDStrategy(module *gosibleModule.GosibleModule[*Params]) (strategy, error) {
	path, err := module.GetBinPath("hostname", nil, true)
	if err != nil {
		return nil, err
	}
	return newBaseStrategy(module, &freeBSDStrategy{
		file:        "/etc/rc.conf.d/hostname",
		hostnameCmd: path,
		module:      module,
	}), nil
}

func newDarwinStrategy(module *gosibleModule.GosibleModule[*Params]) (strategy, error) {
	// TODO As we do not support MacOS rn and the implementation is long implementation was skipped as of now.
	return nil, errors.New("darwin strategy is not implemented")
}

func newOpenRcStrategy(module *gosibleModule.GosibleModule[*Params]) (strategy, error) {
	return newBaseStrategy(module, &openRcStrategy{
		file: "/etc/conf.d/hostname",
	}), nil
}

func newSolarisStrategy(module *gosibleModule.GosibleModule[*Params]) (strategy, error) {
	path, err := module.GetBinPath("hostname", nil, true)
	if err != nil {
		return nil, err
	}
	return newBaseStrategy(module, &solarisStrategy{
		module:      module,
		hostnameCmd: path,
	}), nil
}

func newFileStrategy(module *gosibleModule.GosibleModule[*Params]) (strategy, error) {
	return newBaseStrategy(module, &fileStrategy{file: "/etc/hostname"}), nil
}

func newSLESStrategy(module *gosibleModule.GosibleModule[*Params]) (strategy, error) {
	distroVersion := sysInfo.DistributionVersion()
	dF, err := strconv.ParseFloat(distroVersion, 32)
	if err != nil {
		return nil, err
	}
	if 10. <= dF && dF <= 12. {
		return newBaseStrategy(module, &fileStrategy{file: "/etc/HOSTNAME"}), nil
	}
	return nil, getUnimplementedStrategyErr()
}

func newSystemdStrategy(module *gosibleModule.GosibleModule[*Params]) (strategy, error) {
	path, err := module.GetBinPath("hostnamectl", nil, true)
	if err != nil {
		return nil, err
	}
	return newBaseStrategy(module, &systemdStrategy{
		hostnamectlCmd: path,
		module:         module,
	}), nil
}

func newBaseDeriverStrategy(module *gosibleModule.GosibleModule[*Params]) (strategy, error) {
	return newBaseStrategy(module, &baseDeriverStrategy{}), nil
}

var platformToStrategy = map[string]map[string]func(module *gosibleModule.GosibleModule[*Params]) (strategy, error){
	"Linux": {
		"Sles":             newSLESStrategy,
		"Redhat":           newRedHatStrategy,
		"Anolis":           newRedHatStrategy,
		"Cloudlinuxserver": newRedHatStrategy,
		"Cloudlinux":       newRedHatStrategy,
		"Alinux":           newRedHatStrategy,
		"Scientific":       newRedHatStrategy,
		"Oracle":           newRedHatStrategy,
		"Virtuozzo":        newRedHatStrategy,
		"Amazon":           newRedHatStrategy,
		"Altlinux":         newRedHatStrategy,
		"Eurolinux":        newRedHatStrategy,
		"Debian":           newFileStrategy,
		"Kylin":            newFileStrategy,
		"Cumulus-linux":    newFileStrategy,
		"Kali":             newFileStrategy,
		"Parrot":           newFileStrategy,
		"Ubuntu":           newFileStrategy,
		"Linuxmint":        newFileStrategy,
		"Linaro":           newFileStrategy,
		"Devuan":           newFileStrategy,
		"Raspbian":         newFileStrategy,
		"Neon":             newFileStrategy,
		"Void":             newFileStrategy,
		"Pop":              newFileStrategy,
		"Gentoo":           newOpenRcStrategy,
		"Alpine":           newAlpineStrategy,
	},
	"OpenBSD": {
		"": newOpenBSDStrategy,
	},
	"SunOS": {
		"": newSolarisStrategy,
	},
	"FreeBSD": {
		"": newFreeBSDStrategy,
	},
	"NetBSD": {
		"": newFreeBSDStrategy,
	},
	"Darwin": {
		"": newDarwinStrategy,
	},
}

var strats = map[string]func(module *gosibleModule.GosibleModule[*Params]) (strategy, error){
	"alpine":  newAlpineStrategy,
	"debian":  newSystemdStrategy,
	"freebsd": newFreeBSDStrategy,
	"generic": newBaseDeriverStrategy,
	"macos":   newDarwinStrategy,
	"macosx":  newDarwinStrategy,
	"darwin":  newDarwinStrategy,
	"openbsd": newOpenBSDStrategy,
	"openrc":  newOpenRcStrategy,
	"redhat":  newRedHatStrategy,
	"sles":    newSLESStrategy,
	"solaris": newSolarisStrategy,
	"systemd": newSystemdStrategy,
}

func (m *Module) Run(ctx *modules.RunContext, vars types.Vars) *modules.Return {
	err := m.ParseParams(ctx, vars)
	if err != nil {
		return m.MarkReturnFailed(err)
	}

	strat, err := m.getStrategy()
	if err != nil {
		return m.MarkReturnFailed(err)
	}

	currentHostname, err := strat.getCurrentHostname()
	if err != nil {
		return m.MarkReturnFailed(err)
	}
	permanentHostname, err := strat.getPermanentHostname()
	if err != nil {
		return m.MarkReturnFailed(err)
	}
	changed, err := strat.updateCurrentAndPermanentHostname()
	if err != nil {
		return m.MarkReturnFailed(err)
	}

	nameBefore := permanentHostname
	if m.Params.Name != currentHostname {
		nameBefore = currentHostname
	}

	diff := modules.Diff{}
	if changed {
		diff.After = fmt.Sprintf("hostname = %s\n", m.Params.Name)
		diff.Before = fmt.Sprintf("hostname = %s\n", nameBefore)
	}

	facts := map[string]interface{}{
		"ansible_hostname": strings.Split(m.Params.Name, ".")[0],
		"ansible_nodename": m.Params.Name,
	}

	hostname, fqdnErr := fqdn.FqdnHostname()
	fqdnErrStr := ""
	if fqdnErr == nil {
		facts["ansible_fqdn"] = hostname
		facts["ansible_domain"] = strings.Join(strings.Split(hostname, ".")[1:], ".")
	} else {
		fqdnErrStr = fqdnErr.Error()
	}

	if err = m.Close(); err != nil {
		return m.MarkReturnFailed(err)
	}
	return m.UpdateReturn(&modules.Return{
		Changed:              changed,
		Diff:                 &diff,
		ModuleSpecificReturn: &Return{Name: m.Params.Name},
		InternalReturn:       &modules.InternalReturn{AnsibleFacts: facts, Exception: fqdnErrStr},
	})
}

func (m *Module) getStrategy() (strategy, error) {
	if m.Params.Use != "" {
		return strats[m.Params.Use](m.GosibleModule)
	}
	isSystemdManaged, err := sysInfo.IsSystemdManaged(m.GosibleModule)
	if err != nil {
		return nil, err
	}
	if sysInfo.Platform() == "Linux" && isSystemdManaged {
		return newSystemdStrategy(m.GosibleModule)
	}
	strat := sysInfo.GetPlatformSpecificValue(platformToStrategy)
	if strat != nil {
		return strat(m.GosibleModule)
	}

	return nil, getUnimplementedStrategyErr()
}

func getUnimplementedStrategyErr() error {
	system := sysInfo.Platform()
	distro := sysInfo.Distribution()
	msgPlatform := system
	if distro != "" {
		msgPlatform = fmt.Sprintf("%s (%s)", msgPlatform, distro)
	}
	return fmt.Errorf("hostname module cannot be used on platform %s", msgPlatform)
}

func (p *Params) Validate() error {
	if p.Name == "" {
		return errors.New("parameter `name` for module hostname must be provided")
	}
	if _, ok := strats[p.Use]; p.Use != "" && !ok {
		return fmt.Errorf("unknown stategy `%s` for module hostname", p.Use)
	}
	return nil
}

var _ modules.Module = &Module{}

type Params struct {
	Name string
	Use  string
}

func New() *Module {
	return &Module{}
}

func (m *Module) Name() string {
	return "hostname"
}
