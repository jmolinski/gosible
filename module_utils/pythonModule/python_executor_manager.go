package pythonModule

import (
	"errors"
	"github.com/scylladb/gosible/utils/osUtils"
	"github.com/scylladb/gosible/utils/types"
	"os"
	"path"
)

var ErrNoExecutorRuntime = errors.New("executor runtime is not available")

type PythonExecutorManager struct {
	current *PythonExecutor
}

func NewExecutorManager() *PythonExecutorManager {
	return &PythonExecutorManager{
		current: nil,
	}
}

func getRuntimePath() (string, error) {
	dir, err := osUtils.GetBinaryDir()
	if err != nil {
		return "", err
	}
	return path.Join(dir, "py_runtime.zip"), nil
}

func (m *PythonExecutorManager) getExecutor(_ types.Vars) (*PythonExecutor, error) {
	if m.current != nil && !m.current.invalid {
		return m.current, nil
	}

	runtimePath, err := getRuntimePath()
	if err != nil {
		return nil, err
	}

	if _, err := os.Stat(runtimePath); errors.Is(err, os.ErrNotExist) {
		return nil, ErrNoExecutorRuntime
	}

	executor, err := newExecutor(runtimePath)
	if err != nil {
		return nil, err
	}
	m.current = executor
	return m.current, nil
}
