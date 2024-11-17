package shell

import (
	"bytes"
	"fmt"
	"github.com/scylladb/gosible/modules"
	"github.com/scylladb/gosible/utils/types"
	"testing"
)

type setup struct {
	chdir                 string
	stdin, stdout, stderr []byte
}

// TODO write more and harder tests.
var commandOutput = map[string]setup{
	"echo foobar": {stdout: []byte("foobar\n")},
	"cat":         {stdin: []byte("42"), stdout: []byte("42\n")},
	"pwd":         {chdir: "/", stdout: []byte("/\n")},
}

func TestRun(t *testing.T) {
	for c, o := range commandOutput {
		mod := &Module{}

		vars := types.Vars{
			"cmd":   c,
			"stdin": o.stdin,
			"chdir": o.chdir,
		}

		r := mod.Run(&modules.RunContext{}, vars)
		checkReturnCorrect(r, &o, t)

		vars = types.Vars{
			"_raw_params": c,
			"stdin":       o.stdin,
			"chdir":       o.chdir,
		}

		r = mod.Run(&modules.RunContext{}, vars)
		checkReturnCorrect(r, &o, t)
	}
}

func checkReturnCorrect(r *modules.Return, o *setup, t *testing.T) {
	if r.Failed {
		t.Fatal("command execution failed", o.String(), r.Exception, string(r.Stdout), string(r.Stderr))
	} else if !bytes.Equal(r.Stdout, o.stdout) || !bytes.Equal(r.Stderr, o.stderr) {
		t.Fatal("incorrect result", r, o)
	}
}

func (s *setup) String() string {
	return fmt.Sprint(s.chdir, string(s.stdin), string(s.stdout), string(s.stderr))
}
