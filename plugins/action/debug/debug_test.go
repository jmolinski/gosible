package debug

import (
	"context"
	"fmt"
	"github.com/scylladb/gosible/inventory"
	"github.com/scylladb/gosible/plugins"
	"github.com/scylladb/gosible/utils/types"
	"testing"
)

func assertNotFailed(t *testing.T, r *plugins.Return) {
	if r.Failed {
		t.Fatal("r.Failed is true")
	}
}

func assertMsgEqual(t *testing.T, r *plugins.Return, expectedMsg string) {
	if r.Msg != expectedMsg {
		t.Fatal(fmt.Sprintf("\"%s\" != \"%s\"", r.Msg, expectedMsg))
	}
}

func assertTrue(t *testing.T, b bool, msg string) {
	if !b {
		t.Fatal(msg)
	}
}

func runTestOnHandler(t *testing.T, vars types.Vars) *plugins.Return {
	r := New().Run(context.Background(), &plugins.ActionContext{
		Connection: &plugins.ConnectionContext{Host: &inventory.Host{
			Name: "host",
		}},
		Args: vars,
		VarsEnv: types.Vars{
			"test": 2137,
		},
	})
	return r
}

func TestNoArgs(t *testing.T) {
	r := runTestOnHandler(t, types.Vars{})
	assertNotFailed(t, r)
	assertMsgEqual(t, r, "Hello world!")
	assertTrue(t, r.Skipped == false, "r.Skipped should not be true")
}

func TestMsgArg(t *testing.T) {
	r := runTestOnHandler(t, types.Vars{"msg": "test"})
	assertNotFailed(t, r)
	assertMsgEqual(t, r, "test")
}

func TestVarArg(t *testing.T) {
	r := runTestOnHandler(t, types.Vars{"var": "test"})
	assertNotFailed(t, r)
	assertMsgEqual(t, r, "2137")
}

func TestMsgVarArgsAreMutuallyExclusive(t *testing.T) {
	r := runTestOnHandler(t, types.Vars{"msg": "test", "var": "test"})
	assertTrue(t, r.Failed, "r.Failed should be true")
	assertMsgEqual(t, r, "'msg' and 'var' are incompatible options")
}

func TestVerbosityThreshold(t *testing.T) {
	r := runTestOnHandler(t, types.Vars{"verbosity": 2137})
	assertNotFailed(t, r)
	assertTrue(t, r.Skipped, "r.Skipped should be true")
}
