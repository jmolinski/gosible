package shell

import (
	"fmt"
	"github.com/alessio/shellescape"
	"github.com/scylladb/gosible/config"
)

func Default() Shell {
	return &def{}
}

type def struct{}

func (d def) Exists(path string) string {
	return fmt.Sprintf("test -e %s", shellescape.Quote(path))
}

func (d def) Echo() string {
	return "echo"
}

func (d def) CommandSep() string {
	return ";"
}

func (d def) Executable() string {
	return config.Manager().Settings.DEFAULT_EXECUTABLE
}
