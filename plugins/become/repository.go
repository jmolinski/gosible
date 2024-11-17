package become

import (
	"bytes"
	"fmt"
	"github.com/alessio/shellescape"
	"github.com/chai2010/gettext-go"
	"github.com/scylladb/gosible/utils/shell"
	"github.com/scylladb/gosible/utils/types"
	"math/rand"
)

type Base struct {
	args       *types.BecomeArgs
	id         string
	name       string
	fail       []string // Message to detect prompted password being wrong.
	missing    []string // Message to detect prompted password missing.
	RequireTty bool
	prompt     string
	success    string
}

func (b *Base) GetOption(name string) interface{} {
	// TODO Improve it once VarsManager is properly implemented.
	switch name {
	case "become_user":
		return b.args.User
	case "become_pass":
		return b.args.Password
	case "become_flags":
		return b.args.Flags
	case "become_exe":
		return nil
	default:
		return nil
	}
}

func (b *Base) ExpectPrompt() bool {
	return b.prompt != "" && b.GetOption("become_pass") != nil
}

func (b *Base) buildSuccessCommand(cmd string, sh shell.Shell, noExe bool) string {
	if cmd == "" || sh == nil || b.success == "" {
		return cmd
	}
	cmd = shellescape.Quote(fmt.Sprintf("%s %s >&2 %s %s", sh.Echo(), b.success, sh.CommandSep(), cmd))
	exe := sh.Executable()
	if exe != "" && !noExe {
		cmd = fmt.Sprintf("%s -c %s", exe, cmd)
	}
	return cmd
}

func (b *Base) CheckSuccess(output []byte) bool {
	bSuccess := []byte(b.success)
	for _, line := range bytes.Split(output, []byte("\n")) {
		if bytes.Contains(line, bSuccess) {
			return true
		}
	}
	return false
}

func (b *Base) CheckPasswordPrompt(output []byte) bool {
	bPrompt := []byte(b.prompt)
	if len(b.prompt) == 0 {
		return false
	}

	for _, line := range bytes.Split(output, []byte("\n")) {
		if bytes.Contains(line, bPrompt) {
			return true
		}
	}
	return false
}

func (b *Base) checkPasswordError(output []byte, msg string) bool {
	bFail := []byte(gettext.DGettext(b.name, msg))
	return len(bFail) != 0 && bytes.Contains(output, bFail)
}

func (b *Base) CheckIncorrectPassword(output []byte) bool {
	for _, errstring := range b.fail {
		if b.checkPasswordError(output, errstring) {
			return true
		}
	}
	return false
}

func (b *Base) CheckMissingPassword(output []byte) bool {
	for _, errstring := range b.missing {
		if b.checkPasswordError(output, errstring) {
			return true
		}
	}
	return false
}

func (b *Base) BuildBecomeCmdHook() {
	b.id = genId()
	b.success = fmt.Sprintf("BECOME-SUCCESS-%s", b.id)
}

func genId() string {
	lower := "abcdefghijklmnopqrstuvwxyz"
	length := 32
	res := make([]byte, length)
	for i := range res {
		res[i] = lower[rand.Intn(len(lower))]
	}
	return string(res)
}
