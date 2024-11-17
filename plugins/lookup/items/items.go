package items

import "github.com/jmolinski/gosible-templates/exec"

const Name = "items"

func Run(va *exec.VarArgs) *exec.Value {
	items := make([]interface{}, 0)
	for _, v := range va.Args {
		val := v.Interface()
		if lst, ok := val.([]interface{}); ok {
			items = append(items, lst...)
		} else {
			items = append(items, val)
		}
	}
	return exec.AsValue(items)
}
