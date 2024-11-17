package plugins

import (
	"github.com/scylladb/gosible/utils/types"
)

// ActionContext holds all information a plugin may need in an action handler.
type ActionContext struct {
	Connection *ConnectionContext
	Args       types.Vars
	VarsEnv    types.Vars
}

func CreateActionContext(connection *ConnectionContext, args, varsEnv types.Vars) ActionContext {
	return ActionContext{
		Connection: connection,
		Args:       args,
		VarsEnv:    varsEnv,
	}
}

func (ctx *ActionContext) GetStringArg(name string) (string, bool) {
	val, ok := ctx.Args[name]
	if !ok {
		return "", ok
	}
	v, ok := val.(string)
	return v, ok
}
