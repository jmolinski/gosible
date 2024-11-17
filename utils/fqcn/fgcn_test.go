package fqcn

import (
	"reflect"
	"sort"
	"testing"
)

type setup struct {
	name string
	res  []string
}

func TestFqcn(t *testing.T) {
	testCases := []setup{
		{name: "get_url", res: []string{"get_url", "ansible.builtin.get_url", "ansible.legacy.get_url"}},
		{name: "pip", res: []string{"pip", "ansible.builtin.pip", "ansible.legacy.pip"}},
		{name: "ansible.builtin.get_url", res: []string{"ansible.builtin.get_url"}},
		{name: "foo.bar", res: []string{"foo.bar"}},
	}

	for _, testCase := range testCases {
		res := ToInternalFcqns(testCase.name)
		sort.Strings(testCase.res)
		sort.Strings(res)
		if !reflect.DeepEqual(testCase.res, res) {
			t.Fatal("expected", testCase.res, "got", res)
		}
	}
}
