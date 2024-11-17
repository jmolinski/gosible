package defaultPlugins

import (
	"github.com/scylladb/gosible/plugins"
	"github.com/scylladb/gosible/plugins/action/debug"
	"github.com/scylladb/gosible/plugins/action/example"
	"github.com/scylladb/gosible/plugins/action/setFact"
	"github.com/scylladb/gosible/plugins/action/waitForConnection"
	"github.com/scylladb/gosible/plugins/become"
	"github.com/scylladb/gosible/plugins/become/repository"
	"github.com/scylladb/gosible/plugins/lookup"
	"github.com/scylladb/gosible/utils/types"
)

func Register() {
	plugins.RegisterAction(example.Name, toActionFn(example.New))
	plugins.RegisterAction(debug.Name, toActionFn(debug.New))
	plugins.RegisterAction(waitForConnection.Name, toActionFn(waitForConnection.New))
	plugins.RegisterAction(setFact.Name, toActionFn(setFact.New))

	RegisterBecomePlugins()

	lookup.RegisterDefaultPlugins()
}

func toActionFn[T plugins.Action](fn func() T) func() plugins.Action {
	return func() plugins.Action { return fn() }
}

func RegisterBecomePlugins() {
	repository.RegisterBecomePlugin("su", toPluginFn(become.NewSu))
	repository.RegisterBecomePlugin("sudo", toPluginFn(become.NewSudo))
}

func toPluginFn[T repository.BecomePlugin](fn func(*types.BecomeArgs) T) repository.BecomePluginConstructor {
	return func(vars *types.BecomeArgs) repository.BecomePlugin { return fn(vars) }
}
