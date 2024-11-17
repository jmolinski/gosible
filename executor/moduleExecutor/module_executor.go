package moduleExecutor

import (
	"context"
	"encoding/json"
	"github.com/davecgh/go-spew/spew"
	"github.com/scylladb/gosible/modules"
	playbookTypes "github.com/scylladb/gosible/playbook/types"
	"github.com/scylladb/gosible/plugins"
	"github.com/scylladb/gosible/remote"
	pb "github.com/scylladb/gosible/remote/proto"
	"github.com/scylladb/gosible/utils/display"

	"github.com/scylladb/gosible/utils/types"
	varsPkg "github.com/scylladb/gosible/vars"
	"time"
)

// TimeoutRemoteTaskExecution is the timeout for task executed on a remote server.
// TODO this should be removed or configurable per task, 10s is not enough for many tasks
const TimeoutRemoteTaskExecution = 10 * time.Second

func ExecuteRemoteModuleTask(task *playbookTypes.Task, play *playbookTypes.Play, conn *plugins.ConnectionContext, varsEnv types.Vars) (*modules.Return, error) {
	return executeRemoteModuleTask(task, play, conn, varsEnv, false)
}

func executeRemoteModuleTask(task *playbookTypes.Task, play *playbookTypes.Play, conn *plugins.ConnectionContext, varsEnv types.Vars, uploadPyRuntime bool) (*modules.Return, error) {
	preparedArgs, err := prepareArgs(task, varsEnv)
	if err != nil {
		return nil, err
	}
	argsJson, err := json.Marshal(preparedArgs)
	if err != nil {
		return nil, err
	}
	metaArgs, err := prepareMetaArgs(task, uploadPyRuntime)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), TimeoutRemoteTaskExecution)
	defer cancel()
	req := &pb.ExecuteModuleRequest{
		ModuleName: task.Action.Name,
		VarsJson:   argsJson,
		MetaArgs:   metaArgs,
	}
	rsp, err := conn.RemoteExecutorClient.ExecuteModule(ctx, req)
	if err != nil {
		return nil, err
	}
	display.Display(display.Options{}, "Remote module execution result: %s", rsp.ReturnValueJson)
	var ret modules.Return
	if err = json.Unmarshal(rsp.ReturnValueJson, &ret); err != nil {
		return nil, err
	}
	if ret.InternalReturn != nil && ret.NeedsPythonRuntime && !uploadPyRuntime {
		return executeRemoteModuleTask(task, play, conn, varsEnv, true)
	}
	display.Debug(&conn.Host.Name, spew.Sdump(ret))
	// TODO do something meaningful with the execution result
	return &ret, nil
}

func prepareArgs(task *playbookTypes.Task, varsEnv types.Vars) (types.Vars, error) {
	templatedArgs, err := varsPkg.TemplateActionArgs(task.Action.Args, varsEnv)
	if err != nil {
		return nil, err
	}

	prepared := make(types.Vars)
	for k, v := range templatedArgs {
		prepared[k] = v
	}
	return prepared, nil
}

func prepareMetaArgs(task *playbookTypes.Task, uploadPyRuntime bool) (*pb.MetaArgs, error) {
	ret := &pb.MetaArgs{}

	ret.PythonInterpreter = getPythonInterpreter(task)

	if uploadPyRuntime {
		display.Display(display.Options{}, "Uploading Python runtime to remote host")
		runtimeData, err := remote.ReadPythonRuntimeData()
		if err != nil {
			return nil, err
		}
		ret.PyRuntimeZipData = runtimeData
	}

	return ret, nil
}

func getPythonInterpreter(task *playbookTypes.Task) string {
	// TODO in some edge cases ansible uses different python executable.
	return "/usr/bin/python3"
}
