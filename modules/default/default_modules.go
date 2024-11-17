package defaultModules

import (
	"github.com/scylladb/gosible/modules"
	"github.com/scylladb/gosible/modules/command"
	getUrl "github.com/scylladb/gosible/modules/get_url"
	"github.com/scylladb/gosible/modules/group"
	"github.com/scylladb/gosible/modules/hostname"
	"github.com/scylladb/gosible/modules/ping"
	"github.com/scylladb/gosible/modules/pip"
	"github.com/scylladb/gosible/modules/service"
	"github.com/scylladb/gosible/modules/shell"
	"github.com/scylladb/gosible/modules/waitFor"
)

func Register(reg *modules.ModuleRegistry) {
	reg.RegisterModuleFn(toModFn(command.New))
	reg.RegisterModuleFn(toModFn(shell.New))
	reg.RegisterModuleFn(toModFn(getUrl.New))
	reg.RegisterModuleFn(toModFn(group.New))
	reg.RegisterModuleFn(toModFn(hostname.New))
	reg.RegisterModuleFn(toModFn(pip.New))
	reg.RegisterModuleFn(toModFn(ping.New))
	reg.RegisterModuleFn(toModFn(waitFor.New))
	reg.RegisterModuleFn(toModFn(service.New))
}

func toModFn[T modules.Module](fn func() T) func() modules.Module {
	return func() modules.Module { return fn() }
}
