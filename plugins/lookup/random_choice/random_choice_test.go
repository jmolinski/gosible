package random_choice

import (
	"github.com/jmolinski/gosible-templates/exec"
	"testing"
)

func TestRandomChoice(t *testing.T) {
	res := Run(&exec.VarArgs{Args: []*exec.Value{}})

	if res.IsError() {
		t.Fatal("Expected no error")
	}
	if !res.IsList() || res.Len() != 0 {
		t.Fatal("Expected empty list")
	}

	args := []*exec.Value{
		exec.AsValue("a"),
		exec.AsValue("b"),
		exec.AsValue("c"),
	}
	res = Run(&exec.VarArgs{Args: args})

	if res.IsError() {
		t.Fatal("Expected no error")
	}
	if !res.IsList() || res.Len() != 1 {
		t.Fatal("Expected a list with one item")
	}

	drawnArg := res.Index(0)
	foundInArgsList := false
	for _, arg := range args {
		if drawnArg.String() == arg.String() {
			foundInArgsList = true
		}
	}
	if !foundInArgsList {
		t.Fatal("Expected to find the item in the list of arguments")
	}
}
