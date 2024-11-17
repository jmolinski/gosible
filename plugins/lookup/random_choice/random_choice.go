package random_choice

import (
	"github.com/jmolinski/gosible-templates/exec"
	"math/rand"
)

const Name = "random_choice"

func Run(va *exec.VarArgs) *exec.Value {
	if len(va.Args) == 0 {
		return exec.AsValue([]interface{}{})
	}

	randomTerm := va.Args[rand.Intn(len(va.Args))]
	return exec.AsValue([]interface{}{randomTerm.Interface()})
}
