package vars

import (
	"github.com/jmolinski/gosible-templates/exec"
	"reflect"
	"testing"
)

func TestVars(t *testing.T) {
	res := Run(&exec.VarArgs{
		Args: []*exec.Value{exec.AsValue("nameUndefined")},
		Env:  make(map[string]interface{}),
	})
	if !res.IsError() {
		t.Fatal("Expected an error - name not defined")
	}

	res = Run(&exec.VarArgs{
		Args: []*exec.Value{exec.AsValue("nameUndefined")},
		KwArgs: map[string]*exec.Value{
			"default": exec.AsValue("aaa"),
		},
		Env: make(map[string]interface{}),
	})
	if res.IsError() {
		t.Fatal("Expected no error")
	}
	if res.Len() != 1 {
		t.Fatal("Expected 1 result")
	}
	if res.Index(0).String() != "aaa" {
		t.Fatal("Expected the default value to be returned")
	}

	res = Run(&exec.VarArgs{
		Args: []*exec.Value{exec.AsValue("a"), exec.AsValue("b"), exec.AsValue("c")},
		KwArgs: map[string]*exec.Value{
			"default": exec.AsValue("aaa"),
		},
		Env: map[string]interface{}{
			"a": "v1",
			"b": "v2",
			"c": "v3",
		},
	})

	expected := []interface{}{"v1", "v2", "v3"}
	if !reflect.DeepEqual(res.Interface(), expected) {
		t.Fatal("Expected: ", expected, " Got: ", res.Interface())
	}
}
