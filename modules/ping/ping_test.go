package ping

import (
	"github.com/scylladb/gosible/modules"
	"github.com/scylladb/gosible/utils/types"
	"testing"
)

func TestReturnDataProvided(t *testing.T) {
	for _, d := range []string{"", "6", "*", "9", "=", "42"} {
		r := New().Run(&modules.RunContext{}, types.Vars{
			"data": d,
		})

		if r.Failed {
			t.Fatal("ping execution failed", r.Exception)
		}
		ret := r.ModuleSpecificReturn.(*Return)
		if ret.Ping != d {
			t.Fatal("expected", d, "got", ret.Ping)
		}
	}
}

func TestDefaultValue(t *testing.T) {
	r := New().Run(&modules.RunContext{}, types.Vars{})

	if r.Failed {
		t.Fatal("ping execution failed", r.Exception)
	}
	ret := r.ModuleSpecificReturn.(*Return)
	if ret.Ping != DefaultData {
		t.Fatal("expected default data got", ret.Ping)
	}
}

func TestCrash(t *testing.T) {
	r := New().Run(&modules.RunContext{}, types.Vars{"data": "crash"})

	if !r.Failed {
		t.Fatal("ping execution should fail")
	}
	if r.Exception != errorStr {
		t.Fatal("expected", errorStr, "got", r.Exception)
	}
}
