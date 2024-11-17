package pip

import (
	"fmt"
	"github.com/scylladb/gosible/module_utils/gosibleModule"
	"strings"
)

// isPy3 checks if provided path is python3 interpreter.
func (m *Module) isPy3(pythonPath string) (bool, error) {
	res, err := m.RunCommand([]string{pythonPath, "--version"}, gosibleModule.RunCommandDefaultKwargs())
	if err != nil {
		return false, err
	}
	pythonVersionSplit := strings.Split(string(res.Stdout), " ")
	if len(pythonVersionSplit) != 2 {
		return false, fmt.Errorf("unexpected python version output: %s", string(res.Stdout))
	}
	versionSplit := strings.Split(pythonVersionSplit[1], ".")
	return versionSplit[0] == "3", nil
}

// hasPipModule checks if provided python interpreter has pip.
func (m *Module) hasPipModule(pythonPath string) (bool, error) {
	res, err := m.RunCommand([]string{pythonPath, "-m", "pip"}, gosibleModule.RunCommandDefaultKwargs())
	if err != nil {
		return false, err
	}
	return res.Rc == 0, nil
}
