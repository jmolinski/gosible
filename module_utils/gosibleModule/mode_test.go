package gosibleModule

import (
	"github.com/davecgh/go-spew/spew"
	"github.com/scylladb/gosible/utils/osUtils"
	"os"
	"testing"
	"time"
)

type modeTestData struct {
	statInfo os.FileMode
	mode     string
	expected os.FileMode
}

var data = []modeTestData{
	// Going from no permissions to setting all for user, group, and/or other
	{040000, "a+rwx", 0777},
	{040000, "u+rwx,g+rwx,o+rwx", 0777},
	{040000, "o+rwx", 0007},
	{040000, "g+rwx", 0070},
	{040000, "u+rwx", 0700},

	// Going from all permissions to none for user, group, and/or other
	{040777, "a-rwx", 0000},
	{040777, "u-rwx,g-rwx,o-rwx", 0000},
	{040777, "o-rwx", 0770},
	{040777, "g-rwx", 0707},
	{040777, "u-rwx", 0077},

	// now using absolute assignment from None to a set of perms
	{040000, "a=rwx", 0777},
	{040000, "u=rwx,g=rwx,o=rwx", 0777},
	{040000, "o=rwx", 0007},
	{040000, "g=rwx", 0070},
	{040000, "u=rwx", 0700},

	// X effect on files and dirs
	{040000, "a+X", 0111},
	{0100000, "a+X", 0},
	{040000, "a=X", 0111},
	{0100000, "a=X", 0},
	{040777, "a-X", 0666},
	// Same as chmod but is it a bug?
	// chmod a-X statfile <== removes execute from statfile
	{0100777, "a-X", 0666},

	// Multiple permissions
	{040000, "u=rw-x+X,g=r-x+X,o=r-x+X", 0755},
	{0100000, "u=rw-x+X,g=r-x+X,o=r-x+X", 0644},
}

var umaskData = []modeTestData{
	{0100000, "+rwx", 0770},
	{0100777, "-rwx", 007},
}

type modeTestDataErr struct {
	statInfo os.FileMode
	mode     string
	err      string
}

var invalidData = []modeTestDataErr{
	{040000, "a=foo", "bad symbolic permission for mode: a=foo"},
	{040000, "f=rwx", "bad symbolic permission for mode: f=rwx"},
}

type fMode struct {
	os.FileMode
}

func (f fMode) Mode() os.FileMode {
	return f.FileMode
}

func (f fMode) IsDir() bool {
	// Python styl dir check
	return f.FileMode&040000 != 0
}

func (f fMode) Name() string {
	return "foobar"
}

func (f fMode) Size() int64 {
	return 0
}

func (f fMode) Sys() any {
	return nil
}

func (f fMode) ModTime() time.Time {
	return time.Now()
}

func toFileInfo(mode os.FileMode) os.FileInfo {
	return fMode{mode}
}

func TestGoodSymbolicModes(t *testing.T) {
	for _, d := range data {
		mode, err := symbolicModeToOctal(toFileInfo(d.statInfo), d.mode)
		if err != nil {
			t.Error("on", spew.Sprint(d), err)
		} else if mode != d.expected {
			t.Error("on", spew.Sprint(d), "expected", d.expected, "got", mode)
		}
	}
}

func TestUmaskWithSymbolicModes(t *testing.T) {
	getUmask = func() int { return 07 }
	defer func() {
		getUmask = osUtils.GetUmask
	}()

	for _, d := range umaskData {
		mode, err := symbolicModeToOctal(toFileInfo(d.statInfo), d.mode)
		if err != nil {
			t.Error("on", spew.Sprint(d), err)
		} else if mode != d.expected {
			t.Error("on", spew.Sprint(d), "expected", d.expected, "got", mode)
		}
	}
}

func TestInvalidSymbolicModes(t *testing.T) {
	for _, d := range invalidData {
		_, err := symbolicModeToOctal(toFileInfo(d.statInfo), d.mode)
		if err == nil {
			t.Error("on", spew.Sprint(d), "expected err", d.err)
		} else if err.Error() != d.err {
			t.Error("on", spew.Sprint(d), "expected", d.err, "got", err)
		}
	}
}
