package remote

import (
	"os"
	"path"
)

func ReadPythonRuntimeData() ([]byte, error) {
	runtimePath, err := getPythonRuntimePath()
	if err != nil {
		return nil, err
	}
	return os.ReadFile(runtimePath)
}

func getPythonRuntimePath() (string, error) {
	gosiblePath, err := getGosiblePath()
	if err != nil {
		return "", err
	}
	return path.Join(gosiblePath, "remote", "py_runtime.zip"), nil
}
