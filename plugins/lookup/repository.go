package lookup

import (
	"github.com/jmolinski/gosible-templates/exec"
)

type Fn func(*exec.VarArgs) *exec.Value

var plugins = make(map[string]Fn)

func RegisterLookupPlugin(name string, fn Fn) {
	plugins[name] = fn
}

func ResetLookupPlugins() {
	plugins = make(map[string]Fn)
	RegisterDefaultPlugins()
}

func FindLookupPlugin(name string) Fn {
	return plugins[name]
}
