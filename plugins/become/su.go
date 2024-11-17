package become

import (
	"fmt"
	"github.com/alessio/shellescape"
	"github.com/scylladb/gosible/utils/shell"
	"github.com/scylladb/gosible/utils/types"
	"regexp"
	"strings"
)

type Su struct {
	Base
}

var suPromptLocalizations = []string{
	"Password",
	"암호",
	"パスワード",
	"Adgangskode",
	"Contraseña",
	"Contrasenya",
	"Hasło",
	"Heslo",
	"Jelszó",
	"Lösenord",
	"Mật khẩu",
	"Mot de passe",
	"Parola",
	"Parool",
	"Pasahitza",
	"Passord",
	"Passwort",
	"Salasana",
	"Sandi",
	"Senha",
	"Wachtwoord",
	"ססמה",
	"Лозинка",
	"Парола",
	"Пароль",
	"गुप्तशब्द",
	"शब्दकूट",
	"సంకేతపదము",
	"හස්පදය",
	"密码",
	"密碼",
	"口令",
}

func (s *Su) CheckPasswordPrompt(output []byte) bool {
	prompts, ok := s.GetOption("prompt_l10n").([]string)
	if !ok || len(prompts) == 0 {
		prompts = suPromptLocalizations
	}
	promptsWithPrefix := make([]string, len(prompts))
	for i, prompt := range prompts {
		promptsWithPrefix[i] = fmt.Sprintf(`(\w+\'s )?` + prompt)
	}
	passwordReRaw := strings.Join(promptsWithPrefix, "|") + " ?(:|：) ?"
	suPromptLocalizationsRe := "(?i)" + passwordReRaw
	matched, _ := regexp.Match(suPromptLocalizationsRe, output)
	return matched
}

func (s *Su) BuildBecomeCommand(cmd string, shell shell.Shell) (string, int, error) {
	s.BuildBecomeCmdHook()
	s.prompt = "Prompt handling for ``su`` is more complicated, this is used to satisfy the connection plugin"

	if cmd == "" {
		return cmd, 0, nil
	}

	exe, ok := s.GetOption("become_exe").(string)
	if !ok || exe == "" {
		exe = s.name
	}
	flags, ok := s.GetOption("become_flags").(string)
	user, ok := s.GetOption("become_user").(string)
	successCmd := s.buildSuccessCommand(cmd, shell, false)

	return fmt.Sprintf("%s %s %s -c %s", exe, flags, user, shellescape.Quote(successCmd)), 1, nil
}

func NewSu(args *types.BecomeArgs) *Su {
	return &Su{
		Base{
			args: args,
			name: "su",
			fail: []string{"Authentication failure"},
		},
	}
}
