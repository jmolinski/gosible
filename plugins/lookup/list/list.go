package list

import "github.com/jmolinski/gosible-templates/exec"

const Name = "list"

func Run(va *exec.VarArgs) *exec.Value {
	items := make([]interface{}, 0, len(va.Args))
	for _, v := range va.Args {
		items = append(items, v.Interface())
	}
	return exec.AsValue(items)
}
