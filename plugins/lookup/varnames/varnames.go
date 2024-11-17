package varnames

import (
	"fmt"
	"github.com/jmolinski/gosible-templates/exec"
	"regexp"
)

const Name = "varnames"

func Run(va *exec.VarArgs) *exec.Value {
	variableNames := make([]string, 0)

	for _, pattern := range va.Args {
		if !pattern.IsString() {
			return exec.AsValue(fmt.Sprintf("%s: argument must be a string", pattern.String()))
		}
		r, err := regexp.Compile(pattern.String())
		if err != nil {
			return exec.AsValue(fmt.Errorf("failed to compile regexp `%s`", pattern))
		}

		for key := range va.Env {
			if r.FindStringIndex(key) != nil {
				variableNames = append(variableNames, key)
			}
		}
	}

	return exec.AsValue(variableNames)
}
