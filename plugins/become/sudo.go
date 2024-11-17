package become

import (
	"fmt"
	"github.com/alessio/shellescape"
	"github.com/google/shlex"
	"github.com/scylladb/gosible/utils/shell"
	"github.com/scylladb/gosible/utils/types"
	"regexp"
	"strings"
)

type Sudo struct {
	Base
}

func (s *Sudo) BuildBecomeCommand(cmd string, shell shell.Shell) (string, int, error) {
	s.BuildBecomeCmdHook()
	if cmd == "" {
		return cmd, 0, nil
	}
	becomeCmd, ok := s.GetOption("become_exe").(string)
	if !ok || becomeCmd == "" {
		becomeCmd = s.name
	}
	flags, ok := s.GetOption("become_flags").(string)
	prompt := ""
	pass, ok := s.GetOption("become_pass").(string)
	if ok && pass != "" {
		s.prompt = fmt.Sprintf("[sudo via ansible, key=%s] password:", s.id)
		splitFlags, err := shlex.Split(flags)
		if err != nil {
			return "", 0, err
		}
		reflag := make([]string, len(splitFlags))
		re := regexp.MustCompile(`^(-\w*)n(\w*.*)`)
		for _, flag := range splitFlags {
			if flag == "-n" || flag == "--non-interactive" {
				continue
			}
			if strings.HasPrefix(flag, "--") {
				re.ReplaceAllString(flag, "$1$2")
			}
			reflag = append(reflag, flag)
		}
		flags = shellescape.QuoteCommand(reflag)
		prompt = fmt.Sprintf("-p \"%s\"", s.prompt)
	}
	user, ok := s.GetOption("become_user").(string)
	if ok && user != "" {
		user = fmt.Sprintf("-u %s", user)
	}
	flags = fmt.Sprintf("-S %s", flags)
	r := strings.Join([]string{becomeCmd, flags, prompt, user, s.buildSuccessCommand(cmd, shell, false)}, " ")
	return r, len(s.success) + 1, nil
}

func NewSudo(args *types.BecomeArgs) *Sudo {
	return &Sudo{
		Base{
			args: args,
			name: "sudo",

			fail:    []string{"Sorry, try again."},
			missing: []string{"Sorry, a password is required to run sudo", "sudo: a password is required"},
		},
	}
}
