package pathUtils

import (
	"os"
	"strings"
	"testing"
)

func TestGetBinPath(t *testing.T) {
	resp := []bool{false, true}
	getPathEnv = func() string { return "/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin" }
	pathListSep = ':'
	exist = func(string) (bool, error) { return false, nil }
	isExecutable = func(string) (bool, error) {
		r := resp[0]
		resp = resp[1:]
		return r, nil
	}

	path, err := GetBinPath("notacommand", nil, true)
	if err != nil {
		t.Fatal("err is not nil", err)
	}
	if path != "/usr/local/bin/notacommand" {
		t.Fatal("expected:", "/usr/local/bin/notacommand", "got:", path)
	}
}

func TestGetBinPathRaiseValueError(t *testing.T) {
	getPathEnv = func() string { return "" }
	isExecutable = func(path string) (bool, error) { return strings.IndexByte(path, os.PathSeparator) != -1, nil }
	exist = func(string) (bool, error) { return false, nil }

	path, err := GetBinPath("notacommand", nil, true)
	if err == nil || err.Error() != "failed to find required executable \"notacommand\"" {
		t.Fatal("expected error `failed to find required executable \"notacommand\"` got ", err, "and path", path)
	}
}
