package example

import (
	"context"
	"fmt"
	"github.com/scylladb/gosible/plugins"
)

const Name = "example"

func New() *Action {
	return &Action{}
}

type Action struct{}

func (a *Action) Run(_ context.Context, actionCtx *plugins.ActionContext) *plugins.Return {
	msg := "Hello world"
	if customMessage, ok := actionCtx.GetStringArg("message"); ok {
		msg += ": " + customMessage
	}
	msg = fmt.Sprintf("<%s> %s", actionCtx.Connection.Host.Name, msg)
	return &plugins.Return{Msg: msg}
}
