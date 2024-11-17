package defaultModules

import (
	"github.com/scylladb/gosible/module_utils/pythonModule"
	"github.com/scylladb/gosible/modules"
)

func RegisterPython(reg *modules.ModuleRegistry) {
	executorManager := pythonModule.NewExecutorManager()
	reg.RegisterModuleFn(toPyModFn(executorManager, "get_url"))
	reg.RegisterModuleFn(toPyModFn(executorManager, "setup"))
	reg.RegisterModuleFn(toPyModFn(executorManager, "apt"))
	reg.RegisterModuleFn(toPyModFn(executorManager, "apt_repository"))
	reg.RegisterModuleFn(toPyModFn(executorManager, "apt_key"))
}

func toPyModFn(executorManager *pythonModule.PythonExecutorManager, name string) func() modules.Module {
	return func() modules.Module { return pythonModule.NewStandardModule(executorManager, name, "py_") } // TODO: remove prefix
}
