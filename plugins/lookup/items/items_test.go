package items

import (
	"github.com/jmolinski/gosible-templates/exec"
	"reflect"
	"testing"
)

func TestItems(t *testing.T) {
	args := []*exec.Value{
		exec.AsValue("abc"),
		exec.AsValue([]interface{}{"a", []interface{}{1, 0}, "c"}),
		exec.AsValue(2137),
	}

	res := Run(&exec.VarArgs{Args: args})

	if res.IsError() {
		t.Fatal("Expected no error")
	}
	if res.Len() != 5 {
		t.Fatal("Expected 5 items")
	}
	if _, ok := res.Interface().([]interface{}); !ok {
		t.Fatal("Expected []interface{}")
	}

	// Asserts that only one level is flattened.
	if !reflect.DeepEqual(res.Interface(), []interface{}{"abc", "a", []interface{}{1, 0}, "c", 2137}) {
		t.Fatal("Expected: ", []interface{}{"abc", "a", []interface{}{1, 0}, "c", 2137}, " Got: ", res.Interface())
	}
}
