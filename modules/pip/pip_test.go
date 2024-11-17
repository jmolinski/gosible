package pip

import (
	"reflect"
	"testing"
)

type testRecoverPackageNameData struct {
	input    []string
	expected []string
}

func TestRecoverPackageName(t *testing.T) {
	var testData = []testRecoverPackageNameData{
		{input: []string{"django>1.11.1", "<1.11.2", "ipaddress", "simpleproject<2.0.0", ">1.1.0"}, expected: []string{"django>1.11.1,<1.11.2", "ipaddress", "simpleproject<2.0.0,>1.1.0"}},
		{input: []string{"django>1.11.1,<1.11.2,ipaddress", "simpleproject<2.0.0,>1.1.0"}, expected: []string{"django>1.11.1,<1.11.2", "ipaddress", "simpleproject<2.0.0,>1.1.0"}},
		{input: []string{"django>1.11.1", "<1.11.2", "ipaddress,simpleproject<2.0.0,>1.1.0"}, expected: []string{"django>1.11.1,<1.11.2", "ipaddress", "simpleproject<2.0.0,>1.1.0"}},
	}

	for _, data := range testData {
		got := recoverPackageName(data.input)
		if !reflect.DeepEqual(data.expected, got) {
			t.Error("on input", data.input, "wrong output, expected:", data.expected, "got", got)
		}
	}
}

type testConvertToPipRequirementsData struct {
	input    []requirementData
	expected []requirement
}

// Requires python with setuptools installed to be installed.
func TestConvertToPipRequirements(t *testing.T) {
	var testData = []testConvertToPipRequirementsData{
		{input: []requirementData{{name: "django", version: "3.2"}, {name: "numpy"}}, expected: []requirement{{Str: "django==3.2", HasVersionSpecifier: true}, {Str: "numpy", HasVersionSpecifier: false}}},
	}

	for _, data := range testData {
		m := New()

		got, err := m.convertToPipRequirements("/usr/bin/python3", data.input)
		if err != nil {
			t.Error(err)
		}
		if !reflect.DeepEqual(data.expected, got) {
			t.Error("on input", data.input, "wrong output, expected:", data.expected, "got", got)
		}
	}
}
