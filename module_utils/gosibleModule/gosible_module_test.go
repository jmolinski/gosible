package gosibleModule

import (
	"bytes"
	"github.com/davecgh/go-spew/spew"
	"io"
	"os/exec"
	"testing"
)

type setup struct {
	cwd            string
	data           []byte
	stdout, stderr []byte
}

// TODO write more and harder tests.
var testRunData = map[string]setup{
	"echo foobar": {stdout: []byte("foobar\n")},
	"cat":         {data: []byte("42"), stdout: []byte("42")},
	"pwd":         {cwd: "/", stdout: []byte("/\n")},
}

var testRunDataBeforeCommunicate = map[string]setup{
	"cat": {data: []byte("42"), stdout: []byte("6 * 9 = 42")},
}

func TestRun(t *testing.T) {
	for c, o := range testRunData {
		mod := New[Validatable](nil)

		kwargs := *RunCommandDefaultKwargs()
		kwargs.Cwd = o.cwd
		kwargs.Data = o.data
		kwargs.UseUnsafeShell = true
		r, err := mod.RunCommand(c, &kwargs)
		if err != nil {
			t.Fatal("function threw error", c, spew.Sprint(o), err)
		}
		checkReturnCorrect(r, &o, c, t)
	}
}

func TestRunBeforeCommunicate(t *testing.T) {
	beforeCommunicationFunc := func(cmd *exec.Cmd, stdinPipe io.Writer, stdoutPipe io.Reader, stderr *bytes.Buffer) error {
		_, err := stdinPipe.Write([]byte("6 * 9 = "))
		return err
	}

	for c, o := range testRunDataBeforeCommunicate {
		mod := New[Validatable](nil)

		kwargs := *RunCommandDefaultKwargs()
		kwargs.Cwd = o.cwd
		kwargs.Data = o.data
		kwargs.BeforeCommunicateCallback = beforeCommunicationFunc
		kwargs.UseUnsafeShell = true

		r, err := mod.RunCommand(c, &kwargs)
		if err != nil {
			t.Fatal("function threw error", c, spew.Sprint(o), err)
		}
		checkReturnCorrect(r, &o, c, t)
	}
}

func checkReturnCorrect(r *RunCommandResult, o *setup, c string, t *testing.T) {
	if r.Rc != 0 || bytes.Compare(r.Stdout, o.stdout) != 0 || bytes.Compare(r.Stderr, o.stderr) != 0 {
		t.Fatal("incorrect result", r, o, c)
	}
}
