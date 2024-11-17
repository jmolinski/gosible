package service

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/scylladb/gosible/module_utils/gosibleModule"
	"github.com/scylladb/gosible/module_utils/serviceUtils"
	pathUtils "github.com/scylladb/gosible/utils/path"
	"github.com/scylladb/gosible/utils/slices"
	"github.com/scylladb/gosible/utils/version"
	"golang.org/x/sys/unix"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
)

type linuxService struct {
	*basicServiceSharedState
	crashed        bool
	svcCmd         string
	systemdUnit    string
	svcInitScript  string
	svcInitCtl     string
	enableCmd      string
	upstartVersion version.Loose
}

func (l *linuxService) setSharedState(state *basicServiceSharedState) {
	l.basicServiceSharedState = state
}

var versionRe = regexp.MustCompile(`\(upstart (.*)\)`)

func (l *linuxService) getServiceTools() (err error) {
	paths := []string{"/sbin", "/usr/sbin", "/bin", "/usr/bin"}
	binaries := []string{"service", "chkconfig", "update-rc.d", "rc-service", "rc-update", "systemctl", "initctl", "start", "stop", "restart", "insserv"}
	initPaths := []string{"/etc/init.d"}
	location := make(map[string]string)
	for _, binary := range binaries {
		location[binary], err = l.module.GetBinPath(binary, paths, false)
		if err != nil {
			return
		}
	}

	for _, initDir := range initPaths {
		initScript := fmt.Sprintf("%s/%s", initDir, l.module.Params.Name)
		regular, err := pathUtils.IsRegular(initScript)
		if err != nil {
			return err
		}
		if regular {
			l.svcInitScript = initScript
		}
	}
	if err = l.getEnableAndSvcCmd(location); err != nil {
		return err
	}

	if l.enableCmd == "" {
		return serviceUtils.ErrorIfMissing(true, l.module.Params.Name, "host")
	}
	// If no service control tool selected yet, try to see if 'service' is available
	if l.svcCmd == "" && location["service"] != "" {
		l.svcCmd = location["service"]
	}
	// couldn't find anything yet
	if l.svcCmd == "" && l.svcInitScript == "" {
		return errors.New("cannot find 'service' binary or init script for service,  possible typo in service name?, aborting")
	}
	l.svcInitCtl = location["initctl"]

	return nil
}

func (l *linuxService) getSystemdServiceEnabled() (bool, error) {
	serviceName := l.systemdUnit
	ret, err := l.executeCommand(fmt.Sprintf("%s is-enabled %s", l.enableCmd, serviceName))
	if err != nil {
		return false, err
	}
	if ret.Rc == 0 {
		return true, nil
	}
	if bytes.HasPrefix(ret.Stdout, []byte("disabled")) {
		return false, nil
	}
	exists, err := sysvExists(serviceName)
	if err != nil {
		return false, err
	}
	if exists {
		return sysvIsEnabled(serviceName)
	}
	return false, nil
}

func (l *linuxService) getSystemdStatusDict() (map[string]string, error) {
	// Check status first as show will not fail if service does not exist
	ret, err := l.executeCommand(fmt.Sprintf("%s show %s", l.enableCmd, l.systemdUnit))
	if err == nil {
		return nil, err
	}
	if ret.Rc != 0 {
		return nil, fmt.Errorf("failure %d running systemctl show for %s: %s", ret.Rc, l.systemdUnit, string(ret.Stderr))
	}
	if bytes.Contains(ret.Stdout, []byte("LoadState=not-found")) {
		return nil, fmt.Errorf("systemd could not find the requested service \"%s\": %s", l.systemdUnit, string(ret.Stderr))
	}
	var key, value string
	var valueBuffer []string
	statusDict := make(map[string]string)
	for _, line := range strings.Split(string(ret.Stdout), newline) {
		if strings.Contains(line, "=") {
			if key == "" {
				// systemd fields that are shell commands can be multi-line
				// We take a value that begins with a "{" as the start of
				// a shell command and a line that ends with "}" as the end of
				// the command
				split := strings.SplitN(line, "=", 2)
				key = split[0]
				value = split[1]
				trimmedValue := strings.TrimSpace(value)
				if trimmedValue[0] == '{' {
					if trimmedValue[len(trimmedValue)-1] == '}' {
						statusDict[key] = value
						key = ""
					} else {
						valueBuffer = append(valueBuffer, value)
					}
				} else {
					statusDict[key] = value
				}
			} else {
				trimmedLine := strings.TrimSpace(line)
				if trimmedLine[len(trimmedLine)-1] == '}' {
					statusDict[key] = strings.Join(valueBuffer, "\n")
					key = ""
					valueBuffer = nil
				} else {
					valueBuffer = append(valueBuffer, value)
				}
			}
		} else {
			valueBuffer = append(valueBuffer, value)
		}
	}

	return statusDict, nil
}

func (l *linuxService) getSystemdServiceStatus() (*bool, error) {
	d, err := l.getSystemdStatusDict()
	if err != nil {
		return nil, err
	}
	if d["ActiveState"] == "active" {
		// run-once services (for which a single successful exit indicates
		// that they are running as designed) should not be restarted here.
		// Thus, we are not checking d['SubState'].
		l.setRunning(true)
		l.crashed = false
	} else if d["ActiveState"] == "failed" {
		l.setRunning(false)
		l.crashed = true
	} else if d["ActiveState"] == "" {
		return nil, fmt.Errorf("no ActiveState value in systemctl show output for %s", l.systemdUnit)
	} else {
		l.setRunning(false)
		l.crashed = false
	}
	return l.running, nil
}

func sysvIsEnabled(name string) (bool, error) {
	glob, err := filepath.Glob(fmt.Sprintf("/etc/rc?.d/S??%s", name))
	if err != nil {
		return false, err
	}
	return glob == nil, nil
}

func sysvExists(name string) (bool, error) {
	err := syscall.Access(name, unix.X_OK)
	switch err {
	case nil:
		return true, nil
	case syscall.EACCES:
		return false, nil
	default:
		return false, err
	}
}

func checkSystemd(location map[string]string) (bool, error) {
	if location["systemctl"] == "" {
		return false, nil
	}
	for _, canary := range []string{"/run/systemd/system/", "/dev/.run/systemd/", "/dev/.systemd/"} {
		exists, err := pathUtils.Exists(canary)
		if err != nil {
			return false, err
		}
		if exists {
			return true, nil
		}
	}
	// If all else fails, check if init is the systemd command, using comm as cmdline could be symlink
	data, err := os.ReadFile("/proc/1/comm")
	if err != nil {
		if os.IsNotExist(err) {
			// If comm doesn't exist, old kernel, no systemd
			return false, nil
		}
		return false, err
	}
	if bytes.Contains(data, []byte("systemd")) {
		return true, nil
	}

	return false, nil
}

func (l *linuxService) serviceEnable(enable bool) (ret *gosibleModule.RunCommandResult, err error) {
	if l.enableCmd == "" {
		return nil, fmt.Errorf("cannot detect command to enable service %s, typo or init system potentially unknown", l.module.Params.Name)
	}
	l.changed = true
	methodBySuffix := map[string]func(enable bool) (string, *gosibleModule.RunCommandResult, error){
		"initctl":     l.initctlServiceEnable,
		"chkconfig":   l.sysvServiceEnable,
		"systemctl":   l.systemdServiceEnable,
		"rc-update":   l.openRCServiceEnable,
		"update-rc.d": l.updateRcDStyleServiceEnable,
		"insserv":     l.insservServiceEnable,
	}
	var action string
	for k, v := range methodBySuffix {
		if strings.HasSuffix(l.enableCmd, k) {
			action, ret, err = v(enable)
			if err != nil {
				return
			}
			break
		}
	}
	if action == "" {
		return
	}
	// If we've gotten to the end, the service needs to be updated
	l.changed = true
	// we change argument order depending on real binary used:
	// rc-update and systemctl need the argument order reversed
	var cmd string
	if strings.HasSuffix(l.enableCmd, "rc-update") {
		cmd = fmt.Sprintf("%s %s %s %s", l.enableCmd, action, l.module.Params.Name, l.module.Params.Runlevel)
	} else if strings.HasSuffix(l.enableCmd, "systemctl") {
		cmd = fmt.Sprintf("%s %s %s", l.enableCmd, action, l.systemdUnit)
	} else {
		cmd = fmt.Sprintf("%s %s %s", l.enableCmd, l.module.Params.Name, action)
	}
	ret, err = l.executeCommand(cmd)
	if err != nil {
		return
	}
	if ret.Rc != 0 {
		if len(ret.Stderr) == 0 {
			err = fmt.Errorf("failure for %s %s: rc=%d %s", action, l.module.Params.Name, ret.Rc, string(ret.Stdout))
		} else {
			err = fmt.Errorf("error when trying to %s %s: rc=%d %s", action, l.module.Params.Name, ret.Rc, string(ret.Stderr))
		}
	}
	return
}

func (l *linuxService) getServiceStatus() (*bool, error) {
	if strings.HasSuffix(l.svcCmd, "systemctl") {
		return l.getSystemdServiceStatus()
	}

	ret, err := l.serviceControl("status")
	if err != nil {
		return nil, err
	}
	// if we have decided the service is managed by upstart, we check for some additional output...
	if l.svcInitCtl != "" && l.running == nil {
		// check the job status by upstart response
		initctlRet, err := l.executeCommand(fmt.Sprintf("%s status %s %s", l.svcInitCtl, l.module.Params.Name, l.module.Params.Arguments))
		if err != nil {
			return nil, err
		}
		if bytes.Contains(initctlRet.Stdout, []byte("stop/waiting")) {
			l.setRunning(false)
		} else if bytes.Contains(initctlRet.Stdout, []byte("stop/running")) {
			l.setRunning(true)
		}
	}

	if l.running == nil && strings.HasSuffix(l.svcCmd, "rc-service") {
		openrcRet, err := l.executeCommand(fmt.Sprintf("%s %s status", l.svcCmd, l.module.Params.Name))
		if err != nil {
			return nil, err
		}
		l.setRunning(bytes.Contains(openrcRet.Stdout, []byte("started")))
		l.crashed = bytes.Contains(openrcRet.Stdout, []byte("crashed"))
	}

	// Prefer a non-zero return code. For reference, see:
	// http://refspecs.linuxbase.org/LSB_4.1.0/LSB-Core-generic/LSB-Core-generic/iniscrptact.html
	if l.running == nil && slices.Contains([]int{1, 2, 3, 4, 69}, ret.Rc) {
		l.setRunning(false)
	}
	// if the job status is still not known check it by status output keywords
	// Only check keywords if there's only one line of output (some init
	// scripts will output verbosely in case of error and those can emit
	// keywords that are picked up as false positives
	if l.running == nil && bytes.Count(ret.Stdout, []byte("\n")) <= 1 {
		// first transform the status output that could irritate keyword matching
		cleanOut := bytes.ReplaceAll(ret.Stdout, []byte(strings.ToLower(l.module.Params.Name)), []byte(""))
		if bytes.Contains(cleanOut, []byte("stop")) {
			l.setRunning(false)
		} else if bytes.Contains(cleanOut, []byte("run")) {
			l.setRunning(!bytes.Contains(cleanOut, []byte("not ")))
		} else if bytes.Contains(cleanOut, []byte("start")) && !bytes.Contains(cleanOut, []byte("not ")) {
			l.setRunning(true)
		} else if bytes.Contains(cleanOut, []byte("could not access pid file")) {
			l.setRunning(false)
		} else if bytes.Contains(cleanOut, []byte("is dead and pid file exists")) {
			l.setRunning(false)
		} else if bytes.Contains(cleanOut, []byte("dead but subsys locked")) {
			l.setRunning(false)
		} else if bytes.Contains(cleanOut, []byte("dead but pid file exists")) {
			l.setRunning(false)
		}
	}

	// if the job status is still not known and we got a zero for the
	// return code, assume here that the service is running
	if l.running == nil && ret.Rc == 0 {
		l.setRunning(true)
	}
	// if the job status is still not known check it by special conditions
	if l.running == nil {
		if l.module.Params.Name == "iptables" && bytes.Contains(ret.Stdout, []byte("ACCEPT")) {
			// iptables status command output is lame
			// TODO: lookup if we can use a return code for this instead?
			l.setRunning(true)
		}
	}

	return l.running, nil
}

func (l *linuxService) serviceControl(action string) (*gosibleModule.RunCommandResult, error) {
	var svcCmd string
	args := l.module.Params.Arguments
	if l.svcCmd != "" {
		if strings.HasSuffix(l.svcCmd, "systemctl") {
			// systemd commands take the form <cmd> <action> <name>
			svcCmd = l.svcCmd
			args = fmt.Sprintf("%s %s", l.systemdUnit, args)
		} else if strings.HasSuffix(l.svcCmd, "initctl") {
			// initctl commands take the form <cmd> <action> <name>
			svcCmd = l.svcCmd
			args = fmt.Sprintf("%s %s", l.module.Params.Name, args)
		} else {
			// SysV and OpenRC take the form <cmd> <name> <action>
			svcCmd = fmt.Sprintf("%s %s", l.svcCmd, l.module.Params.Name)
		}
	} else {
		// upstart
		svcCmd = l.svcInitScript
	}
	// In OpenRC, if a service crashed, we need to reset its status to
	// stopped with the zap command, before we can start it back.
	if strings.HasSuffix(l.svcCmd, "rc-service") && action == "start" && l.crashed {
		_, err := l.executeCommandDaemonized(fmt.Sprintf("%s zap", l.svcCmd))
		if err != nil {
			return nil, err
		}
	}

	if action == "restart" {
		return l.handleRestart(svcCmd, args)
	}
	return l.handleNonRestart(action, svcCmd, args)
}

func (l *linuxService) handleRestart(svcCmd, args string) (*gosibleModule.RunCommandResult, error) {
	var retStop, retStart *gosibleModule.RunCommandResult
	var errStop, errStart error
	if strings.HasSuffix(l.svcCmd, "rc-service") {
		//All services in OpenRC support restart.
		return l.executeCommandDaemonized(fmt.Sprintf("%s %s %s", svcCmd, "restart", args))
	}
	// In other systems, not all services support restart. Do it the hard way.
	if svcCmd != "" {
		// upstart or systemd
		retStop, errStop = l.executeCommandDaemonized(fmt.Sprintf("%s %s %s", svcCmd, "stop", args))
	} else {
		// SysV
		retStop, errStop = l.executeCommandDaemonized(fmt.Sprintf("%s %s %s", "stop", l.module.Params.Name, args))
	}
	if errStop != nil {
		return nil, errStop
	}
	if svcCmd != "" {
		// upstart or systemd
		retStart, errStart = l.executeCommandDaemonized(fmt.Sprintf("%s %s %s", svcCmd, "start", args))
	} else {
		// SysV
		retStart, errStart = l.executeCommandDaemonized(fmt.Sprintf("%s %s %s", "start", l.module.Params.Name, args))
	}
	if errStart != nil {
		return nil, errStop
	}

	// merge return information
	if retStop.Rc != 0 && retStart.Rc == 0 {
		return retStart, nil
	}
	return &gosibleModule.RunCommandResult{
		Rc:     retStart.Rc + retStop.Rc,
		Stdout: append(retStop.Stdout, retStart.Stdout...),
		Stderr: append(retStop.Stderr, retStart.Stderr...),
	}, nil
}

func (l *linuxService) getEnableAndSvcCmd(location map[string]string) error {
	isSystemd, err := checkSystemd(location)
	if err != nil {
		return err
	}
	if isSystemd {
		l.systemdUnit = l.module.Params.Name
		l.svcCmd = location["systemctl"]
		l.enableCmd = location["systemctl"]
	} else {
		exists, err := pathUtils.Exists(fmt.Sprintf("/etc/init/%s.conf", l.module.Params.Name))
		if err != nil {
			return err
		}
		if location["initctl"] != "" && exists {
			// service is managed by upstart
			l.enableCmd = location["initctl"]
			// set the upstart version based on the output of 'initctl version'
			l.upstartVersion = version.Loose{0, 0, 0}
			ret, err := l.module.RunCommand(fmt.Sprintf("%s version", location["initctl"]), gosibleModule.RunCommandDefaultKwargs())
			if err == nil && ret.Rc == 0 {
				res := versionRe.Find(ret.Stdout)
				if res != nil {
					if v, err := version.NewLoose(string(res)); err == nil {
						l.upstartVersion = v
					}
				}
			}
		} else if location["rc-service"] != "" {
			// service is managed by OpenRC
			l.svcCmd = location["rc-service"]
			l.enableCmd = location["rc-service"]
		} else if l.svcInitScript != "" {
			// service is managed by with SysV init scripts
			l.enableCmd = location["update-rc.d"]
			if l.enableCmd == "" {
				l.enableCmd = location["insserv"]
			}
			if l.enableCmd == "" {
				l.enableCmd = location["chkconfig"]
			}
		}
	}
	return nil
}

func (l *linuxService) handleNonRestart(action, svcCmd, args string) (*gosibleModule.RunCommandResult, error) {
	if svcCmd != "" {
		// upstart or systemd or OpenRC
		return l.executeCommandDaemonized(fmt.Sprintf("%s %s %s", svcCmd, action, args))
	} else {
		// SysV
		return l.executeCommandDaemonized(fmt.Sprintf("%s %s %s", action, l.module.Params.Name, args))
	}
}

func (l *linuxService) initctlServiceEnable(enable bool) (string, *gosibleModule.RunCommandResult, error) {
	var manRe *regexp.Regexp
	var configLine, overrideState []byte
	if l.upstartVersion.Compare(version.Loose{0, 6, 7}) >= 0 {
		manRe = regexp.MustCompile(`(?im)^manual\s*$`)
		configLine = []byte("manual\n")
	} else {
		manRe = regexp.MustCompile(`(?im)^start on manual\s*$`)
		configLine = []byte("start on manual\n")
	}
	initpath := "/etc/init"
	confFileName := fmt.Sprintf("%s/%s.conf", initpath, l.module.Params.Name)
	overrideFileName := fmt.Sprintf("%s/%s.override", initpath, l.module.Params.Name)
	// Check to see if files contain the manual line in .conf and fail if True
	confFileContent, err := os.ReadFile(confFileName)
	if err != nil {
		return "", nil, err
	}
	if manRe.Match(confFileContent) {
		return "", nil, errors.New("manual stanza not supported in a .conf file")
	}

	l.changed = false
	overrideFileContent, err := os.ReadFile(overrideFileName)
	if err == nil {
		if enable && manRe.Match(overrideFileContent) {
			// Remove manual stanza if present and service enabled
			l.changed = true
			overrideState = manRe.ReplaceAll(overrideFileContent, []byte(""))
		} else if !enable && !manRe.Match(overrideFileContent) {
			// Add manual stanza if not present and service disabled
			l.changed = true
			overrideState = bytes.Join([][]byte{overrideFileContent, configLine}, []byte("\n"))
		} else {
			// service already in desired state
		}
	} else if errors.Is(err, os.ErrNotExist) {
		if enable {
			// service already in desired state
		} else {
			l.changed = true
			overrideState = configLine
		}
	} else {
		return "", nil, err
	}
	// The initctl method of enabling and disabling services is much
	// different than for the other service methods.  So actually
	// committing the change is done in this conditional and then we
	// skip the boilerplate at the bottom of the method
	if l.changed {
		err := os.WriteFile(overrideFileName, overrideState, 0)
		if err != nil {
			return "", nil, fmt.Errorf("could not modify override file: %v", err)
		}
	}
	return "", nil, nil
}

func (l *linuxService) sysvServiceEnable(enable bool) (string, *gosibleModule.RunCommandResult, error) {
	var action string
	if enable {
		action = "enable"
	} else {
		action = "disable"
	}

	ret, err := l.executeCommand(fmt.Sprintf("%s --list %s", l.enableCmd, l.module.Params.Name))
	if err != nil {
		return "", nil, err
	}
	if bytes.Contains(ret.Stderr, []byte(fmt.Sprintf("chkconfig --add %s", l.module.Params.Name))) {
		_, err = l.executeCommand(fmt.Sprintf("%s --add %s", l.enableCmd, l.module.Params.Name))
		if err != nil {
			return "", nil, err
		}
		ret, err = l.executeCommand(fmt.Sprintf("%s --list %s", l.enableCmd, l.module.Params.Name))
		if err != nil {
			return "", nil, err
		}
	}
	if bytes.Contains(ret.Stdout, []byte(l.module.Params.Name)) {
		return "", nil, fmt.Errorf("service %s does not support chkconfig", l.module.Params.Name)
	}
	// Check if we're already in the correct state
	if bytes.Contains(ret.Stdout, []byte(fmt.Sprintf("3:%s", action))) &&
		bytes.Contains(ret.Stdout, []byte(fmt.Sprintf("5:%s", action))) {
		l.changed = false
		return "", nil, nil
	}
	return action, nil, nil
}

func (l *linuxService) updateRcDStyleServiceEnable(enable bool) (string, *gosibleModule.RunCommandResult, error) {
	slinks, err := filepath.Glob("/etc/rc?.d/S??" + l.module.Params.Name)
	if err != nil {
		return "", nil, err
	}
	isEnabled := slinks != nil
	if enable == isEnabled {
		l.changed = false
		return "", nil, nil
	}
	l.changed = true
	var action string
	if enable {
		action = "enable"
		klinks, err := filepath.Glob("/etc/rc?.d/K??" + l.module.Params.Name)
		if err != nil {
			return "", nil, err
		}
		if klinks == nil {
			ret, err := l.executeCommand(fmt.Sprintf("%s %s defaults", l.enableCmd, l.module.Params.Name))
			if err != nil {
				return "", nil, err
			}
			if ret.Rc != 0 {
				if len(ret.Stderr) == 0 {
					return "", nil, fmt.Errorf("%s %s %s %s", string(ret.Stdout), l.enableCmd, l.module.Params.Name, action)
				}
				return "", nil, errors.New(string(ret.Stderr))
			}
		}
	} else {
		action = "disable"
	}

	ret, err := l.executeCommand(fmt.Sprintf("%s %s %s", l.enableCmd, l.module.Params.Name, action))
	if err != nil {
		return "", nil, err
	}
	if ret.Rc != 0 {
		if len(ret.Stderr) == 0 {
			return "", nil, fmt.Errorf("%s %s %s %s", string(ret.Stdout), l.enableCmd, l.module.Params.Name, action)
		}
		return "", nil, errors.New(string(ret.Stderr))
	}
	return "", nil, nil
}

func (l *linuxService) systemdServiceEnable(enable bool) (string, *gosibleModule.RunCommandResult, error) {
	var action string
	if enable {
		action = "enable"
	} else {
		action = "disable"
	}
	// Check if we're already in the correct state
	serviceEnabled, err := l.getSystemdServiceEnabled()
	if err != nil {
		return "", nil, err
	}
	if enable == serviceEnabled {
		l.changed = false
		return "", nil, nil
	}
	return action, nil, nil
}

func (l *linuxService) insservServiceEnable(enable bool) (string, *gosibleModule.RunCommandResult, error) {
	var cmd string
	if enable {
		cmd = fmt.Sprintf("%s -n -v %s", l.enableCmd, l.module.Params.Name)
	} else {
		cmd = fmt.Sprintf("%s -n -r -v %s", l.enableCmd, l.module.Params.Name)
	}
	ret, err := l.executeCommand(cmd)
	if err != nil {
		return "", nil, err
	}
	l.changed = false
	for _, line := range bytes.Split(ret.Stderr, []byte(newline)) {
		if (enable && bytes.Contains(line, []byte("enable service"))) ||
			(!enable && bytes.Contains(line, []byte("remove service"))) {
			l.changed = true
			break
		}
	}
	if !l.changed {
		return "", nil, nil
	}

	if enable {
		ret, err = l.executeCommand(fmt.Sprintf("%s %s", l.enableCmd, l.module.Params.Name))
		if err != nil {
			return "", nil, err
		}
		if ret.Rc != 0 || len(ret.Stderr) != 0 {
			return "", nil, fmt.Errorf("failed to install service. rc: %d, out: %s, err: %s", ret.Rc, string(ret.Stdout), string(ret.Stderr))
		}

		return "", ret, err
	}
	ret, err = l.executeCommand(fmt.Sprintf("%s -r %s", l.enableCmd, l.module.Params.Name))
	if err != nil {
		return "", nil, err
	}
	if ret.Rc != 0 || len(ret.Stderr) != 0 {
		return "", nil, fmt.Errorf("failed to remove service. rc: %d, out: %s, err: %s", ret.Rc, string(ret.Stdout), string(ret.Stderr))
	}

	return "", ret, err

}

var spaceRe = regexp.MustCompile(`\s+`)

func (l *linuxService) openRCServiceEnable(enable bool) (string, *gosibleModule.RunCommandResult, error) {
	var action string
	if enable {
		action = "add"
	} else {
		action = "delete"
	}

	ret, err := l.executeCommand(fmt.Sprintf("%s show", l.enableCmd))
	if err != nil {
		return "", nil, err
	}
	broke := false
	for _, line := range strings.Split(string(ret.Stdout), "\n") {
		splitLine := strings.SplitN(line, "|", 2)
		serviceName := splitLine[0]
		runLevelsStr := splitLine[1]
		if serviceName != l.module.Params.Name {
			continue
		}
		runLevels := spaceRe.Split(runLevelsStr, -1)
		if enable == slices.Contains(runLevels, l.module.Params.Runlevel) {
			// service already enabled for the runlevel or service already disabled for the runlevel
			l.changed = false
		}
		broke = true
		break
	}
	if !broke && enable {
		// service already disabled altogether
		l.changed = false
	}
	if l.changed {
		return "", nil, nil
	}
	return action, nil, nil
}

func newLinuxService(m *gosibleModule.GosibleModule[*Params]) (service, error) {
	sharedState := newBasicServiceSharedState(m)
	return newBaseService(sharedState, &linuxService{basicServiceSharedState: sharedState}), nil
}
