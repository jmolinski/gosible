package lookup

import (
	"github.com/davecgh/go-spew/spew"
	"github.com/jmolinski/gosible-templates/exec"
	"testing"
)

func four(*exec.VarArgs) *exec.Value {
	return exec.AsSafeValue(4)
}

func TestRepository(t *testing.T) {
	name := "four"

	RegisterLookupPlugin(name, four)
	f := FindLookupPlugin(name)
	res := f(nil)
	if !res.Safe || res.Val.Int() != 4 {
		t.Fatal(res, "is not", spew.Sdump(exec.AsSafeValue(4)))
	}
}
