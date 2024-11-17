package modules

import (
	"github.com/scylladb/gosible/utils/fqcn"
	"github.com/scylladb/gosible/utils/types"
)

type ExecuteCallback func(vars types.Vars) *Return

type ModuleRegistry struct {
	modules map[string]func() Module
}

func NewRegistry() *ModuleRegistry {
	return &ModuleRegistry{
		modules: map[string]func() Module{},
	}
}

func (r *ModuleRegistry) RegisterModuleFn(moduleFn func() Module) {
	name := moduleFn().Name()
	for _, internalName := range fqcn.ToInternalFcqns(name) {
		r.modules[internalName] = moduleFn
	}
}

func (r *ModuleRegistry) FindModule(name string) (Module, bool) {
	module, ok := r.modules[name]
	if ok {
		return module(), ok
	}
	return nil, ok
}
