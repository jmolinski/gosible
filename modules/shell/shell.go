package shell

import (
	"github.com/scylladb/gosible/modules"
	"github.com/scylladb/gosible/modules/command"
	"github.com/scylladb/gosible/utils/types"
)

type Return struct {
	Cmd   string // The command executed by the task.
	Delta string // The command execution delta time.
	End   string // The command execution end time.
	Msg   bool   // changed
	Start string // The command execution start time.
}

type Module struct{}

var _ modules.Module = &Module{}

func New() *Module {
	return &Module{}
}

func (m *Module) Name() string {
	return "shell"
}

func (m *Module) Run(ctx *modules.RunContext, p types.Vars) *modules.Return {
	p = preprocessVars(p)

	return command.New().Run(ctx, p)
}

func preprocessVars(vars types.Vars) types.Vars {
	vars["_uses_shell"] = true

	return vars
}
