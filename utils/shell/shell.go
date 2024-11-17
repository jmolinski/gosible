package shell

import (
	"github.com/scylladb/gosible/utils/types"
)

type Shell interface {
	Echo() string
	CommandSep() string
	Executable() string
	Exists(string) string
}

func Get(_ types.Vars) (Shell, error) {
	// TODO implement
	return Default(), nil
}
