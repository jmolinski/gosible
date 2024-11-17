package sequence

import (
	"github.com/jmolinski/gosible-templates/exec"
	"reflect"
	"testing"
)

func TestSequence(t *testing.T) {
	args := []*exec.Value{exec.AsValue("start=1 end=7 stride=2")}
	res := Run(&exec.VarArgs{Args: args})

	if res.IsError() {
		t.Fatal("Expected no error")
	}

	actual := res.Interface()
	expected := []string{"1", "3", "5", "7"}
	if !reflect.DeepEqual(actual, expected) {
		t.Fatal("Expected: ", expected, " Got: ", actual)
	}
}
