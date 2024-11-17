package pythonModule

import (
	"errors"
	"github.com/scylladb/gosible/modules"
	"github.com/scylladb/gosible/utils/types"
)

type PythonExecutorGetter interface {
	getExecutor(vars types.Vars) (*PythonExecutor, error)
}

type PythonModule struct {
	executorGetter PythonExecutorGetter
	name           string
	moduleName     string
}

var _ modules.Module = &PythonModule{}

func NewModule(executorGetter PythonExecutorGetter, name string, moduleName string) *PythonModule {
	return &PythonModule{
		executorGetter: executorGetter,
		name:           name,
		moduleName:     moduleName,
	}
}

func NewStandardModule(executorGetter PythonExecutorGetter, name string, prefix string) *PythonModule {
	return NewModule(executorGetter, prefix+name, "ansible.modules."+name)
}

func (p PythonModule) Name() string {
	return p.name
}

func (p PythonModule) Run(ctx *modules.RunContext, vars types.Vars) *modules.Return {
	if len(ctx.MetaArgs.PyRuntimeZipData) > 0 {
		if err := SaveRuntimeData(ctx.MetaArgs.PyRuntimeZipData); err != nil {
			return makeErrorReturn(err)
		}
	}

	req := executeModuleRequest{
		ModuleName: p.moduleName,
		Args:       vars,
	}
	rsp := executeModuleResponse{}
	executor, err := p.executorGetter.getExecutor(vars)
	if err != nil {
		return makeErrorReturn(err)
	}
	if err = executor.executeCommand(cmdExecuteModule, req, &rsp); err != nil {
		return makeErrorReturn(err)
	}
	if rsp.Exception != nil {
		return makeErrorReturn(errors.New(*rsp.Exception))
	}
	return convertResult(rsp.Result)
}

func makeErrorReturn(err error) *modules.Return {
	return &modules.Return{
		Changed: false,
		Failed:  true,
		InternalReturn: &modules.InternalReturn{
			Exception:          err.Error(),
			NeedsPythonRuntime: errors.Is(err, ErrNoExecutorRuntime),
		},
	}
}

func convertResult(pyRes map[string]interface{}) *modules.Return {
	res := &modules.Return{}
	if x, found := pyRes["changed"].(bool); found {
		res.Changed = x
		delete(pyRes, "changed")
	}
	if x, found := pyRes["failed"].(bool); found {
		res.Failed = x
		delete(pyRes, "failed")
	}
	if x, found := pyRes["msg"].(string); found {
		res.Msg = x
		delete(pyRes, "msg")
	}

	// TODO: Remaining properties, such as warnings, deprecations, debug logs.

	res.ModuleSpecificReturn = pyRes
	return res
}
