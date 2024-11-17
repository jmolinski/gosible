package indexed_items

import (
	"github.com/jmolinski/gosible-templates/exec"
	"reflect"
	"testing"
)

func TestIndexedItems(t *testing.T) {
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
	expected := []interface{}{
		[]interface{}{0, "abc"},
		[]interface{}{1, "a"},
		[]interface{}{2, []interface{}{1, 0}},
		[]interface{}{3, "c"},
		[]interface{}{4, 2137},
	}
	if !reflect.DeepEqual(res.Interface(), expected) {
		t.Fatal("Expected: ", expected, " Got: ", res.Interface())
	}
}
