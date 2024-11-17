package types

import (
	"github.com/scylladb/gosible/config"
	"golang.org/x/crypto/ssh"
	"io"
)

type Passwords struct {
	Ssh    []byte
	Become []byte
}

type Vars map[string]interface{}

type Facts = Vars

type BecomeArgs struct {
	User     string
	Become   bool
	Method   string
	Flags    string
	Password string
}

type ProcessPipes struct {
	Stdin  io.Writer
	Stdout io.Reader
	Stderr io.Reader
}

func NewProcessPipesFromSshSession(session *ssh.Session) (pipes *ProcessPipes, err error) {
	pipes = &ProcessPipes{}
	pipes.Stdout, err = session.StdoutPipe()
	if err != nil {
		return nil, err
	}
	pipes.Stderr, err = session.StderrPipe()
	if err != nil {
		return nil, err
	}
	pipes.Stdin, err = session.StdinPipe()
	if err != nil {
		return nil, err
	}
	return
}

func NewBecomeArgs() *BecomeArgs {
	settings := config.Manager().Settings
	return &BecomeArgs{User: settings.DEFAULT_BECOME_USER, Method: settings.DEFAULT_BECOME_METHOD}
}

func (v Vars) GetOrDefault(key string, defaultValue interface{}) interface{} {
	if val, ok := v[key]; ok {
		return val
	}
	return defaultValue
}
