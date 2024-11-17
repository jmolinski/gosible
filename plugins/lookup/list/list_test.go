package list

import (
	"github.com/jmolinski/gosible-templates/exec"
	"reflect"
	"testing"
)

func TestList(t *testing.T) {
	args := []*exec.Value{
		exec.AsValue("abc"),
		exec.AsValue(2137),
		exec.AsValue([]string{"a", "b", "c"}),
	}

	res := Run(&exec.VarArgs{Args: args})

	if res.IsError() {
		t.Fatal("Expected no error")
	}
	if res.Len() != 3 {
		t.Fatal("Expected 3 items")
	}

	if arr, ok := res.Interface().([]interface{}); ok {
		for i, v := range arr {
			if !reflect.DeepEqual(v, args[i].Interface()) {
				t.Fatal("Expected", args[i].Interface(), "got", v)
			}
		}
	} else {
		t.Fatal("Expected []interface{}")
	}
}
