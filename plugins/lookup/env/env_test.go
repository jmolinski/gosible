package env

import (
	"github.com/jmolinski/gosible-templates/exec"
	"strings"
	"testing"
)

func TestEnv(t *testing.T) {
	originalOsGetenv := osGetenv
	osGetenv = func(key string) string {
		return "a:/b"
	}
	defer func() { osGetenv = originalOsGetenv }()

	args := []*exec.Value{exec.AsValue("PATH")}
	res := Run(&exec.VarArgs{Args: args})

	if res.IsError() {
		t.Fatal("Expected no error")
	}
	val := res.Interface()
	if val == nil {
		t.Fatal("Expected a value")
	}
	if !res.IsList() || res.Len() != 1 || !res.Index(0).IsString() {
		t.Fatal("Expected list with 1 element of type string")
	}
	if !strings.Contains(res.Index(0).String(), ":/") {
		t.Fatal("PATH env var should contain \":/\", instead got ", res.Index(0).String())
	}
}
