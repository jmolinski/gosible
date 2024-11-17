package command

import (
	"github.com/mitchellh/mapstructure"
	"github.com/scylladb/gosible/modules"
	"github.com/scylladb/gosible/utils/types"
	"os"
	"strings"
	"testing"
)

func TestRemovesNotExists(t *testing.T) {
	osStat = osStatNotExists
	defer restoreOsStat()

	m := New()
	vars := types.Vars{}
	if err := mapstructure.Decode(&Params{Cmd: "id", Removes: "/usr", UsesShell: true}, &vars); err != nil {
		t.Fatal(err)
	}

	ret := m.Run(&modules.RunContext{}, vars)
	if !ret.Failed {
		t.Fatal("Expected failed return got: ", ret)
	}
	err := getErr(ret)
	if err != "'/usr' that should be removed does not exists" {
		t.Fatal("got unexpected error", err)
	}
}

func TestRemovesExists(t *testing.T) {
	osStat = osStatExists
	defer restoreOsStat()

	m := New()
	vars := types.Vars{}
	if err := mapstructure.Decode(&Params{Cmd: "id", Removes: "/usr", UsesShell: true}, &vars); err != nil {
		t.Fatal(err)
	}

	ret := m.Run(&modules.RunContext{}, vars)
	if ret.Failed {
		t.Fatal("Expected not failed return got: ", ret)
	}
}

func TestCreatesNotExists(t *testing.T) {
	osStat = osStatNotExists
	defer restoreOsStat()

	m := New()
	vars := types.Vars{}
	if err := mapstructure.Decode(&Params{Cmd: "id", Creates: "/usr", UsesShell: true}, &vars); err != nil {
		t.Fatal(err)
	}

	ret := m.Run(&modules.RunContext{}, vars)
	if ret.Failed {
		t.Fatal("Expected not failed return got: ", ret)
	}
}

func TestCreatesExists(t *testing.T) {
	osStat = osStatExists
	defer restoreOsStat()

	m := New()
	vars := types.Vars{}
	if err := mapstructure.Decode(&Params{Cmd: "id", Creates: "/usr", UsesShell: true}, &vars); err != nil {
		t.Fatal(err)
	}

	ret := m.Run(&modules.RunContext{}, vars)
	if !ret.Failed {
		t.Fatal("Expected failed return got: ", ret)
	}
	err := getErr(ret)
	if err != "'/usr' that should be created already exists" {
		t.Fatal("got unexpected error", err)
	}
}

func TestCheckCommand(t *testing.T) {
	m := New()
	vars := types.Vars{}
	if err := mapstructure.Decode(&Params{Cmd: "touch /tmp/gosible_check_command_test", UsesShell: true, Warn: true}, &vars); err != nil {
		t.Fatal(err)
	}

	ret := m.Run(&modules.RunContext{}, vars)
	if ret.Failed {
		t.Fatal("Unexpected failed return", ret.Exception)
	}
	switch len(ret.Warnings) {
	case 0:
		t.Fatal("Expected warning")
	case 1:
		if !strings.HasPrefix(ret.Warnings[0], "Consider using the file module with state=touch rather than running 'touch'.") {
			t.Fatal("Unexpected warning", ret.Warnings[0])
		}
	default:
		t.Fatal("Expected one warning got more", ret.Warnings)
	}
}

func osStatNotExists(name string) (fi os.FileInfo, err error) {
	return nil, os.ErrNotExist
}

func osStatExists(name string) (fi os.FileInfo, err error) {
	return nil, nil
}

func getErr(m *modules.Return) string {
	if m.InternalReturn == nil {
		return ""
	}
	return m.InternalReturn.Exception
}

func restoreOsStat() {
	osStat = os.Stat
}
