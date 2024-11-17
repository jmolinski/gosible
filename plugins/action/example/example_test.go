package example

import (
	"context"
	"github.com/scylladb/gosible/inventory"
	"github.com/scylladb/gosible/plugins"
	"github.com/scylladb/gosible/utils/types"
	"testing"
)

func runTestOnHandler(t *testing.T, vars types.Vars, expectedMsg string) {
	r := New().Run(context.Background(), &plugins.ActionContext{
		Connection: &plugins.ConnectionContext{Host: &inventory.Host{
			Name: "host",
		}},
		Args: vars,
	})
	if r.Failed || r.Skipped {
		t.Error(r)
	}
	if r.Msg != expectedMsg {
		t.Error(r)
	}
}

func TestHelloWorld(t *testing.T) {
	runTestOnHandler(t, types.Vars{}, "<host> Hello world")
	runTestOnHandler(t, types.Vars{"message": "2137"}, "<host> Hello world: 2137")
}
