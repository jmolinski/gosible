package pip

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/google/shlex"
	"github.com/scylladb/gosible/module_utils/gosibleModule"
	"github.com/scylladb/gosible/module_utils/locale"
	"github.com/scylladb/gosible/modules"
	pathUtils "github.com/scylladb/gosible/utils/path"
	"github.com/scylladb/gosible/utils/types"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"syscall"
)

var _ modules.Module = New()

type Module struct {
	*gosibleModule.GosibleModule[*Params]
	Params Params
}

type Params struct {
	Chdir                  string   `mapstructure:"chdir"`
	Editable               bool     `mapstructure:"editable"`
	Executable             string   `mapstructure:"executable"`
	ExtraArgs              string   `mapstructure:"extra_args"`
	Name                   []string `mapstructure:"name"`
	Requirements           string   `mapstructure:"requirements"`
	State                  string   `mapstructure:"state"`
	Umask                  string   `mapstructure:"umask"`
	Version                string   `mapstructure:"version"`
	Virtualenv             string   `mapstructure:"virtualenv"`
	VirtualenvCommand      string   `mapstructure:"virtualenv_command"`
	VirtualenvPython       string   `mapstructure:"virtualenv_python"`
	VirtualenvSitePackages bool     `mapstructure:"virtualenv_site_packages"`
}

type Return struct {
	Cmd          []string
	Name         []string
	Requirements string
	Version      string
	Virtualenv   string
}

func (m *Module) Name() string {
	return "pip"
}

type requirement struct {
	HasVersionSpecifier bool
	Str                 string
}

type requirementData struct {
	name    string
	version string
}

func (m *Module) Run(ctx *modules.RunContext, vars types.Vars) *modules.Return {
	err := m.ParseParams(ctx, vars)
	if err != nil {
		return m.MarkReturnFailed(err)
	}
	venvCreated := false
	oldUmask, err := m.setUmask()
	if err != nil {
		return m.MarkReturnFailed(err)
	}
	defer func() {
		if oldUmask != nil {
			syscall.Umask(*oldUmask)
		}
	}()
	pythonPath := ctx.MetaArgs.PythonInterpreter

	var stdout, stderr []byte
	env := m.getVirtualenv()
	if env != "" {
		exists, err := pathUtils.Exists(filepath.Join(env, "bin", "activate"))
		if err != nil {
			return m.MarkReturnFailed(err)
		}
		if exists {
			venvCreated = true
			res, err := m.setupVirtualenv(pythonPath)
			if err != nil {
				return m.MarkReturnFailed(err)
			}
			stdout = append(stdout, res.Stdout...)
			stderr = append(stderr, res.Stderr...)
		}
	}
	pip, err := m.getPip()
	state := stateMap[m.Params.State]
	cmd := make([]string, 0, len(pip)+len(state))
	copy(cmd, pip)
	cmd = append(cmd, state...)

	// If there's a virtualenv we want things we install to be able to use other
	// installations that exist as binaries within this virtualenv. Example: we
	// install cython and then gevent -- gevent needs to use the cython binary,
	// not just a python package that will be found by calling the right python.
	// So if there's a virtualenv, we add that bin/ to the beginning of the PATH
	// in run_command by setting path_prefix here.
	pathPrefix := ""
	if env != "" {
		pathPrefix = filepath.Join(env, "bin")
	}

	// Automatically apply -e option to extra_args when source is a VCS url. VCS
	// includes those beginning with svn+, git+, hg+ or bzr+
	hasVcs := false
	var packages []requirement
	if len(m.Params.Name) != 0 {
		hasVcs = checkHasVcs(m.Params.Name)
		packages, err = m.getPipRequirements(pythonPath)
		if err != nil {
			return m.MarkReturnFailed(err)
		}

		if err = m.validateVersionNameCombo(packages); err != nil {
			return m.MarkReturnFailed(err)
		}
		// if the version specifier is provided by version, append that into the package
		packages, err = m.convertToPipRequirements(pythonPath, []requirementData{{name: m.Params.Name[0], version: m.Params.Version}})
		if err != nil {
			return m.MarkReturnFailed(err)
		}
	}
	extraArgs := m.getExtraArgs()
	if extraArgs != "" {
		parts, err := shlex.Split(extraArgs)
		if err != nil {
			return m.MarkReturnFailed(err)
		}
		cmd = append(cmd, parts...)
	}
	if len(m.Params.Name) != 0 {
		for _, p := range packages {
			cmd = append(cmd, p.Str)
		}
	} else if m.Params.Requirements != "" {
		cmd = append(cmd, "-r", m.Params.Requirements)
	}

	var outFreezeBefore []byte
	if m.Params.Requirements != "" || hasVcs {
		pipCmd, pipRes, err := m.getPackages(pip)
		if err != nil {
			return m.fail(pipCmd, stdout, stderr)
		}
		outFreezeBefore = pipRes.Stdout
	}

	kwargs := gosibleModule.RunCommandDefaultKwargs()
	kwargs.PathPrefix = pathPrefix
	kwargs.Cwd = m.getChdir()
	res, err := m.RunCommand(cmd, kwargs)
	stdout = append(stdout, res.Stdout...)
	stderr = append(stderr, res.Stderr...)
	if m.pipCommandFailed(res, outFreezeBefore) {
		return m.fail(cmd, stdout, stdout)
	}

	changed := venvCreated
	if !changed {
		if m.Params.State == "absent" {
			changed = bytes.Contains(res.Stdout, []byte("Successfully uninstalled"))
		} else {
			pipCmd, pipRes, err := m.getPackages(pip)
			if err != nil {
				return m.fail(pipCmd, stdout, stderr)
			}
			outFreezeAfter := pipRes.Stdout
			changed = bytes.Equal(outFreezeAfter, outFreezeBefore)
		}
	}

	if err = m.Close(); err != nil {
		return m.MarkReturnFailed(err)
	}
	return m.UpdateReturn(&modules.Return{
		Stderr:  stderr,
		Stdout:  stdout,
		Changed: changed,
		ModuleSpecificReturn: &Return{
			Cmd:          cmd,
			Name:         m.Params.Name,
			Requirements: m.Params.Requirements,
			Virtualenv:   env,
			Version:      m.Params.Version,
		},
	})
}

func (m *Module) pipCommandFailed(res *gosibleModule.RunCommandResult, outFreezeBefore []byte) bool {
	return (res.Rc != 0) &&
		!(res.Rc == 1 && m.Params.State == "absent") &&
		!(bytes.Contains(outFreezeBefore, []byte("not installed")) || bytes.Contains(res.Stderr, []byte("not installed")))
}

func (m *Module) fail(cmd []string, out, err []byte) *modules.Return {
	return &modules.Return{
		Failed: true,
		Stderr: err,
		Stdout: out,
		ModuleSpecificReturn: &Return{
			Cmd: cmd,
		},
	}
}

func (m *Module) getExtraArgs() string {
	extraArgs := m.Params.ExtraArgs
	if m.Params.Editable {
		var argsList []string
		if m.Params.ExtraArgs != "" {
			argsList = strings.Split(m.Params.ExtraArgs, " ")
		}
		for _, arg := range argsList {
			if arg == "-e" {
				return extraArgs
			}
		}
		argsList = append(argsList, "-e")
		return strings.Join(argsList, " ")
	}
	return extraArgs
}
func checkHasVcs(names []string) bool {
	for _, pkg := range names {
		if pkg != "" && isVcsUrl(pkg) {
			return true
		}
	}
	return false
}

func (m *Module) getPipRequirements(pythonPath string) ([]requirement, error) {
	requirementsData := make([]requirementData, 0, len(m.Params.Name))
	for _, name := range recoverPackageName(m.Params.Name) {
		requirementsData = append(requirementsData, requirementData{name: name})
	}
	return m.convertToPipRequirements(pythonPath, requirementsData)
}

func recoverPackageName(names []string) []string {
	// rebuild input name to a flat list so we can tolerate any combination of input
	splitNames := make([]string, 0, len(names))
	for _, name := range names {
		splitNames = append(splitNames, strings.Split(name, ",")...)
	}

	// reconstruct the names
	var nameParts, packageNames []string
	inBrackets := false
	for _, name := range splitNames {
		if isPackageName(name) && !inBrackets {
			if len(nameParts) != 0 {
				packageNames = append(packageNames, strings.Join(nameParts, ","))
			}
			nameParts = nil
		}
		if strings.Contains(name, "[") {
			inBrackets = true
		}
		if inBrackets && strings.Contains(name, "]") {
			inBrackets = false
		}
		nameParts = append(nameParts, name)
	}
	packageNames = append(packageNames, strings.Join(nameParts, ","))
	return packageNames
}

func isPackageName(name string) bool {
	trimmed := strings.TrimSpace(name)
	for _, k := range []string{">=", "<=", ">", "<", "==", "!=", "~="} {
		if strings.HasPrefix(trimmed, k) {
			return false
		}
	}
	return true
}

func (m *Module) getVirtualenv() string {
	env := m.Params.Virtualenv
	if env != "" && m.Params.Chdir != "" {
		return filepath.Join(m.Params.Chdir, env)
	}
	return env
}

func (m *Module) getChdir() string {
	chdir := m.Params.Chdir
	if chdir == "" {
		return os.TempDir()
	}
	return chdir
}

func (m *Module) setUmask() (oldUmask *int, err error) {
	var umask *int
	if m.Params.Umask != "" {
		tmp, err := strconv.ParseInt(m.Params.Umask, 8, 32)
		if err != nil {
			return nil, errors.New("umask must be an octal integer")
		}
		*umask = int(tmp)
	}
	if umask != nil {
		*oldUmask = syscall.Umask(*umask)
	}
	return oldUmask, nil
}

var vcsRe = regexp.MustCompile(`(svn|git|hg|bzr)\+`)

func isVcsUrl(pkg string) bool {
	return vcsRe.MatchString(pkg)
}

func (m *Module) setupVirtualenv(pyPath string) (*gosibleModule.RunCommandResult, error) {
	cmd, err := shlex.Split(m.Params.VirtualenvCommand)
	if err != nil {
		return nil, err
	}

	// Find the binary for the command in the PATH
	// and switch the command for the explicit path.
	if filepath.Base(cmd[0]) == cmd[0] {
		cmd[0], err = m.GetBinPath(cmd[0], nil, true)
		if err != nil {
			return nil, err
		}
	}

	// Add the system-site-packages option if that
	// is enabled, otherwise explicitly set the option
	// to not use system-site-packages if that is an
	// option provided by the command's help function.
	if m.Params.VirtualenvSitePackages {
		cmd = append(cmd, "--system-site-packages")
	} else {
		opts, err := m.getCmdOpts(cmd[0])
		if err != nil {
			return nil, err
		}
		for _, opt := range opts {
			if opt == "--no-site-packages" {
				cmd = append(cmd, "--no-site-packages")
				break
			}
		}
	}

	// -p is a virtualenv option, not compatible with pyenv or venv
	// this conditional validates if the command being used is not any of them
	if !strings.Contains(m.Params.VirtualenvCommand, "pyenv") && !strings.Contains(m.Params.VirtualenvCommand, "-m venv") {
		if m.Params.VirtualenvPython != "" {
			cmd = append(cmd, fmt.Sprintf("-p%s", m.Params.VirtualenvPython))
		} else {
			py3, err := m.isPy3(pyPath)
			if err != nil {
				return nil, err
			}
			if py3 {
				cmd = append(cmd, fmt.Sprintf("-p%s", pyPath))
			}
		}
	} else {
		return nil, errors.New("virtualenv_python should not be used when using the venv module or pyvenv as virtualenv_command")
	}

	cmd = append(cmd, m.getVirtualenv())
	return m.RunCommand(cmd, gosibleModule.RunCommandDefaultKwargs())
}

func (m *Module) getPip() (pip []string, err error) {
	candidatePipBaseName := "pip3"

	if m.Params.Executable != "" {
		if filepath.IsAbs(m.Params.Executable) {
			pip = []string{m.Params.Executable}
		} else {
			// If you define your own executable that executable should be the only candidate.
			// As noted in the docs, executable doesn't work with virtualenvs.
			candidatePipBaseName = m.Params.Executable
		}
	} else if m.getVirtualenv() == "" {
		py := m.MetaArgs.PythonInterpreter
		hasPip, err := m.hasPipModule(py)
		if err != nil {
			return nil, err
		}
		if hasPip {
			// If no executable or virtualenv were specified, use the pip module for the current Python interpreter if available.
			// Use of `__main__` is required to support Python 2.6 since support for executing packages with `runpy` was added in Python 2.7.
			// Without it Python 2.6 gives the following error: pip is a package and cannot be directly executed
			pip = []string{py, "-m", "pip.__main__"}
		}
	}

	if pip == nil {
		if m.getVirtualenv() == "" {
			path, err := m.GetBinPath(candidatePipBaseName, nil, true)
			if err != nil {
				return nil, fmt.Errorf("Unable to find any of %s to use.  pip needs to be installed.", candidatePipBaseName)
			}
			pip = []string{path}
		} else {
			// If we're using a virtualenv we must use the pip from the virtualenv.
			venvDir := filepath.Join(m.getVirtualenv(), "bin")
			candidates := []string{candidatePipBaseName, "pip"}
			for _, basename := range candidates {
				candidate := filepath.Join(venvDir, basename)
				exists, err := pathUtils.Exists(candidate)
				if err != nil {
					return nil, err
				}
				if exists {
					exe, err := pathUtils.IsExecutable(candidate)
					if err != nil {
						return nil, err
					}
					if exe {
						return []string{candidate}, nil
					}
				}
			}
			return nil, fmt.Errorf("unable to find pip in the virtualenv, %s, under any of these names: %s. Make sure pip is present in the virtualenv", m.getVirtualenv(), strings.Join(candidates, ", "))
		}
	}

	return
}

func (m *Module) validateVersionNameCombo(packages []requirement) error {
	if m.Params.Version != "" {
		if packages[0].HasVersionSpecifier {
			return errors.New("The 'version' argument conflicts with any version specifier provided along with a package name.\nPlease keep the version specifier, but remove the 'version' argument.")
		}
	}
	return nil
}

func (m *Module) getPackages(pip []string) ([]string, *gosibleModule.RunCommandResult, error) {
	cmd := make([]string, 0, len(pip)+2)
	copy(cmd, pip)
	cmd = append(cmd, "list", "--format=freeze")
	loc, err := locale.GetBestParsableLocale(m.GosibleModule, nil, false)
	if err != nil {
		return nil, nil, err
	}
	langEnv := map[string]string{
		"LANG":        loc,
		"LC_ALL":      loc,
		"LC_MESSAGES": loc,
	}

	kwargs := gosibleModule.RunCommandDefaultKwargs()
	kwargs.Cwd = m.getChdir()
	kwargs.EnvironUpdate = langEnv
	res, err := m.RunCommand(cmd, kwargs)
	if err != nil {
		return cmd, res, err
	}
	// If there was an error (pip version too old) then use 'pip freeze'
	if res.Rc != 0 {
		cmd = append(cmd[:len(cmd)-2], "freeze")
		res, err = m.RunCommand(cmd, kwargs)
		if err != nil {
			return cmd, res, err
		}
	}

	return cmd, res, nil
}

func (m *Module) getCmdOpts(cmd string) ([]string, error) {
	help := cmd + " --help"
	res, err := m.RunCommand(help, gosibleModule.RunCommandDefaultKwargs())
	if err != nil {
		return nil, err
	}
	if res.Rc != 0 {
		return nil, fmt.Errorf("could not get output from %s: %s%s", help, string(res.Stdout), string(res.Stderr))
	}
	words := strings.Fields(strings.TrimSpace(string(res.Stdout)))
	cmdOptions := make([]string, 0, len(words))
	for _, word := range words {
		if strings.HasPrefix(word, "--") {
			cmdOptions = append(cmdOptions, word)
		}
	}
	return cmdOptions, nil
}

func New() *Module {
	return &Module{GosibleModule: gosibleModule.New(&Params{
		VirtualenvCommand: "virtualenv",
		State:             "present",
	})}
}

var stateMap = map[string][]string{
	"present":        {"install"},
	"absent":         {"uninstall", "-y"},
	"latest":         {"install", "-U"},
	"forcereinstall": {"install", "-U", "--force-reinstall"}}

func (p *Params) Validate() error {
	if p.State != "" {
		if _, ok := stateMap[p.State]; !ok {
			return fmt.Errorf("unexpected value for 'state': %s", p.State)
		}
	}
	if len(p.Name) == 0 && p.Requirements == "" {
		return errors.New("one of 'name', 'requirements' is required")
	}
	if len(p.Name) != 0 && p.Requirements != "" {
		return errors.New("only one of 'name', 'requirements' can be set")
	}
	if p.Executable != "" && p.Virtualenv != "" {
		return errors.New("only one of 'executable', 'virtualenv' can be set")
	}
	if p.State == "latest" && p.Version != "" {
		return errors.New("'version' is incompatible with 'state'=latest")
	}
	if p.Version != "" && len(p.Name) > 1 {
		return errors.New("'version' argument is ambiguous when installing multiple package distributions.\nPlease specify version restrictions next to each package in 'name' argument.")
	}

	return nil
}
