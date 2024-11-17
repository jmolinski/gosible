package varnames

import (
	"github.com/jmolinski/gosible-templates/exec"
	"github.com/scylladb/gosible/utils/slices"
	"sort"
	"testing"
)

func TestVarnames(t *testing.T) {
	env := map[string]interface{}{
		"qz_1": "v1",
		"qz_2": "v2",
		"qa_1": "v3",
		"qz_":  "v4",
	}

	res := Run(&exec.VarArgs{
		Args: []*exec.Value{exec.AsValue("[")},
		Env:  env,
	})
	if !res.IsError() {
		t.Fatal("Expected an error - failed to compule regex")
	}

	testCases := []struct {
		args     []*exec.Value
		expected []string
	}{
		{[]*exec.Value{exec.AsValue("^qz_.+")}, []string{"qz_1", "qz_2"}},
		{[]*exec.Value{exec.AsValue("^qz_.+"), exec.AsValue("^q.+1")}, []string{"qz_1", "qz_2", "qz_1", "qa_1"}},
	}

	for _, tc := range testCases {
		res = Run(&exec.VarArgs{
			Args: tc.args,
			Env:  env,
		})
		actual := res.Interface().([]string)
		sort.Strings(tc.expected)
		sort.Strings(actual)
		if !slices.Equal(actual, tc.expected) {
			t.Fatal("Expected: ", tc.expected, " Got: ", actual)
		}
	}
}
