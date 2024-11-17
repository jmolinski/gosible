package plugins

import (
	"context"
	"github.com/scylladb/gosible/utils/fqcn"
)

type Action interface {
	Run(context.Context, *ActionContext) *Return
}

type ActionFn func() Action

var actions = map[string]ActionFn{}

func RegisterAction(name string, actionFn ActionFn) {
	for _, fqcn := range fqcn.ToInternalFcqns(name) {
		actions[fqcn] = actionFn
	}
}

func FindAction(name string) (Action, bool) {
	fn, ok := actions[name]
	if !ok {
		return nil, false
	}
	return fn(), true
}
