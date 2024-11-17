package repository

import (
	"github.com/scylladb/gosible/utils/shell"
	"github.com/scylladb/gosible/utils/types"
)

type BecomeBuildBecomeCommand func(cmd string, shell shell.Shell) (string, error)

type BecomePlugin interface {
	BuildBecomeCommand(cmd string, shell shell.Shell) (string, int, error)
	ExpectPrompt() bool
	CheckSuccess(output []byte) bool
	CheckPasswordPrompt(output []byte) bool
	CheckIncorrectPassword(output []byte) bool
	CheckMissingPassword(output []byte) bool
}

type BecomePluginConstructor func(args *types.BecomeArgs) BecomePlugin

var becomePlugins = map[string]BecomePluginConstructor{}

func RegisterBecomePlugin(name string, bec BecomePluginConstructor) {
	becomePlugins[name] = bec
}

func FindBecomePluginConstructor(name string) (BecomePluginConstructor, bool) {
	becomePlugin, ok := becomePlugins[name]
	return becomePlugin, ok
}
