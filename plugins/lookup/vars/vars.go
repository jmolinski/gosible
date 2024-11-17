package vars

import (
	"errors"
	"github.com/jmolinski/gosible-templates/exec"
)

const Name = "vars"

func Run(va *exec.VarArgs) *exec.Value {
	varsEnv := va.Env

	var defaultValue interface{}
	isDefaultDefined := false
	if defaultArg, ok := va.KwArgs["default"]; ok {
		defaultValue = defaultArg.Interface()
		isDefaultDefined = true
	}

	resolvedVars := make([]interface{}, 0, len(va.Args))
	for _, varName := range va.Args {
		if varName.IsString() {
			if varValue, ok := varsEnv[varName.String()]; ok {
				resolvedVars = append(resolvedVars, varValue)
			} else if isDefaultDefined {
				resolvedVars = append(resolvedVars, defaultValue)
			} else {
				return exec.AsValue(errors.New("Variable not found: " + varName.String()))
			}
		} else {
			return exec.AsValue(errors.New("vars: variable name must be a string"))
		}
	}

	return exec.AsValue(resolvedVars)
}
