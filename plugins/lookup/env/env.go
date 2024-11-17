package env

import (
	"github.com/jmolinski/gosible-templates/exec"
	"github.com/scylladb/gosible/utils/display"
	"os"
	"strings"
)

const Name = "env"

var osGetenv = os.Getenv

func Run(va *exec.VarArgs) *exec.Value {
	results := make([]interface{}, 0, len(va.Args))
	for _, v := range va.Args {
		envVarValue := ""
		if v.IsString() {
			tokens := strings.Fields(strings.TrimSpace(v.String()))
			if len(tokens) > 0 {
				varName := tokens[0]
				// Value is an empty string if the variable is not set
				envVarValue = osGetenv(varName)
			}
		} else {
			display.Debug(nil, "env: %s is not a string", v.String())
		}
		results = append(results, envVarValue)
	}
	return exec.AsValue(results)
}
