package command

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/google/shlex"
	"github.com/hhkbp2/go-strftime"
	"github.com/scylladb/gosible/module_utils/gosibleModule"
	"github.com/scylladb/gosible/modules"
	"github.com/scylladb/gosible/utils/slices"
	"github.com/scylladb/gosible/utils/types"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const newline = "\n"

var osStat = os.Stat

type Params struct {
	Argv []string `mapstructure:"argv"` // Passes the command as a list rather than a string.
	//Use argv to avoid quoting values that would otherwise be interpreted incorrectly (for example "user name").
	//Only the string (free form) or the list (argv) form can be provided, not both. One or the other must be provided.
	Chdir    string `mapstructure:"chdir"`       // Change into this directory before running the command.
	Cmd      string `mapstructure:"cmd"`         // Change into this directory before running the command.
	Creates  string `mapstructure:"creates"`     // The command to run.
	FreeForm string `mapstructure:"_raw_params"` // The command module takes a free form string as a command to run.
	//There is no actual parameter named 'free form'.
	Removes string `mapstructure:"removes"` // A filename or (since 2.0) glob pattern. If a matching file exists, this step will be run.
	//This is checked after creates is checked.
	Stdin           []byte `mapstructure:"stdin"`             // Set the stdin of the command directly to the specified value.
	StdinAddNewLine bool   `mapstructure:"stdin_add_newline"` // If set to yes, append a newline to stdin data.
	StripEmptyEnds  bool   `mapstructure:"stdin_empty_ends"`  // Strip empty lines from the end of stdout/stderr in result.
	Warn            bool   `mapstructure:"warn"`              // (deprecated) Enable or disable task warnings.
	UsesShell       bool   `mapstructure:"_uses_shell"`
	Executable      string `mapstructure:"executable"`
}

func (p *Params) Validate() error {
	return nil
}

type Return struct {
	Cmd   string // The command executed by the task.
	Delta string // The command execution delta time.
	End   string // The command execution end time.
	Msg   bool   // changed
	Start string // The command execution start time.
}

type Module struct {
	*gosibleModule.GosibleModule[*Params]
}

var _ modules.Module = New()

func New() *Module {
	return &Module{GosibleModule: gosibleModule.New[*Params](&Params{
		StdinAddNewLine: true,
		StripEmptyEnds:  true,
	})}
}

func (m *Module) Name() string {
	return "command"
}

func (m *Module) Run(ctx *modules.RunContext, vars types.Vars) *modules.Return {
	if err := m.ParseParams(ctx, prepareVars(vars)); err != nil {
		return m.MarkReturnFailed(err)
	}
	if err := m.runCommand(); err != nil {
		m.MarkReturnFailed(err)

		if e, ok := err.(*exec.ExitError); ok {
			m.UpdateReturn(&modules.Return{Rc: e.ExitCode(), Stderr: e.Stderr})
		}
	}

	return m.GetReturn()
}

func prepareVars(vars types.Vars) types.Vars {
	if v, ok := vars["stdin"]; ok {
		switch v.(type) {
		case string:
			vars["stdin"] = []byte(v.(string))
		}
	}

	return vars
}

func (m *Module) runCommand() error {
	err := m.checkPrerequisites()
	if err != nil {
		return err
	}

	stdin := m.Params.Stdin
	if stdin != nil && m.Params.StdinAddNewLine {
		stdin = append(stdin, []byte(newline)...)
	}

	startTime := time.Now()

	args := m.getArgs()

	kwargs := gosibleModule.RunCommandDefaultKwargs()
	kwargs.Data = stdin
	kwargs.UseUnsafeShell = m.Params.UsesShell
	kwargs.Executable = m.Params.Executable
	kwargs.Cwd = m.Params.Chdir

	res, err := m.RunCommand(args, kwargs)
	if err != nil {
		return err
	}

	endTime := time.Now()
	elapsedTime := endTime.Sub(startTime)

	serr := res.Stderr
	sout := res.Stdout
	if m.Params.StripEmptyEnds {
		serr = stripEmptyEnds(serr)
		sout = stripEmptyEnds(sout)
	}

	// TODO check if and how command changed environment.
	// TODO Fill all fields.
	m.UpdateReturn(&modules.Return{
		Rc:     res.Rc,
		Stderr: serr,
		Stdout: sout,
		ModuleSpecificReturn: &Return{
			Cmd:   getReturnCmd(args),
			Delta: formatDelta(elapsedTime),
			End:   formatTime(endTime),
			Start: formatTime(startTime),
		}})

	return nil
}

func (m *Module) getArgs() interface{} {
	var args interface{} = m.getCommand()
	if args == "" {
		args = m.Params.Argv
	}

	return args
}

func getReturnCmd(args interface{}) string {
	switch args.(type) {
	case string:
		return args.(string)
	case []string:
		return strings.Join(args.([]string), " ")
	default:
		panic("getReturnCmd expects only string or slice of strings as an input")
	}
}

func (m *Module) getCommand() string {
	command := m.Params.Cmd
	if command == "" {
		command = m.Params.FreeForm
	}
	return command
}

func (m *Module) checkPrerequisites() error {
	if m.Params.Warn {
		if err := m.checkCommand(m.getArgs()); err != nil {
			return err
		}
	}

	if m.Params.Removes != "" {
		if _, err := osStat(m.Params.Removes); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return fmt.Errorf("'%s' that should be removed does not exists", m.Params.Removes)
			} else {
				return fmt.Errorf("couldn't determine if file exists: %w", err)
			}
		}
	}

	if m.Params.Creates != "" {
		if _, err := osStat(m.Params.Creates); err != nil {
			if !errors.Is(err, os.ErrNotExist) {
				return fmt.Errorf("couldn't determine if file exists: %w", err)
			}
		} else {
			return fmt.Errorf("'%s' that should be created already exists", m.Params.Creates)
		}
	}

	isCmdSet := m.getCommand() != ""
	areArgvSet := m.Params.Argv != nil
	if isCmdSet == areArgvSet {
		return errors.New("either command or argv must be provided, not both")
	}

	return nil
}

// checkCommand checks if modules can be used instead of cmd provided
func (m *Module) checkCommand(cmdline interface{}) error {
	arguments := map[string]string{"chown": "owner", "chmod": "mode", "chgrp": "group",
		"ln": "state=link", "mkdir": "state=directory",
		"rmdir": "state=absent", "rm": "state=absent", "touch": "state=touch"}
	commands := map[string]string{"curl": "get_url or uri", "wget": "get_url or uri",
		"svn": "subversion", "service": "service",
		"mount": "mount", "rpm": "yum, dnf or zypper", "yum": "yum", "apt-get": "apt",
		"tar": "unarchive", "unzip": "unarchive", "sed": "replace, lineinfile or template",
		"dnf": "dnf", "zypper": "zypper"}
	become := []string{"sudo", "su", "pbrun", "pfexec", "runas", "pmrun", "machinectl"}
	var cmd string
	switch cmdline.(type) {
	case string:
		splitCmd, err := shlex.Split(cmdline.(string))
		if err != nil {
			return err
		}
		cmd = splitCmd[0]
	case []string:
		cmd = cmdline.([]string)[0]
	default:
		return errors.New("cmdline can only be string or []string")
	}
	baseCmd := filepath.Base(cmd)
	disableSuffix := fmt.Sprintf("If you need to use '%s' because the %%s module is insufficient you can add 'warn: false' to this command task or set 'command_warnings=False' in the defaults section of ansible.cfg to get rid of this message.", baseCmd)
	if arg, ok := arguments[baseCmd]; ok {
		m.Warn(fmt.Sprintf("Consider using the file module with %s rather than running '%s'.  "+disableSuffix, arg, baseCmd, "file"))
	}
	if comm, ok := commands[baseCmd]; ok {
		m.Warn(fmt.Sprintf("Consider using the %s module rather than running '%s'.  "+disableSuffix, comm, baseCmd, comm))
	}
	if slices.Contains(become, baseCmd) {
		m.Warn(fmt.Sprintf("Consider using 'become', 'become_method', and 'become_user' rather than running %s", baseCmd))
	}
	return nil
}

func stripEmptyEnds(s []byte) []byte {
	return bytes.TrimLeft(s, newline)
}

func formatTime(time time.Time) string {
	return strftime.Format("%Y-%m-%d %H:%M:%S.%6n", time)
}

func formatDelta(delta time.Duration) string {
	return fmt.Sprintf("%d:%d:%d.%d", int(delta.Hours()), int(delta.Minutes()), int(delta.Seconds()), delta.Microseconds())
}
