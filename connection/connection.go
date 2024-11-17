// Package connection stores datatypes capable of executing command in supported by them environment.
package connection

import (
	"bytes"
	"fmt"
	"github.com/scylladb/gosible/plugins/become/repository"
	"github.com/scylladb/gosible/utils/shell"
	"github.com/scylladb/gosible/utils/types"
	"golang.org/x/crypto/ssh"
	"io"
)

// CommandExecutor interface represents type that can execute command in its context.
type CommandExecutor interface {
	// ExecCommand executes given command providing given inData to stdin.
	// Returns stdout buffer, stderr buffer and error.
	ExecCommand(cmd string, inData *bytes.Reader, sudoable bool, becomeArgs *types.BecomeArgs) (*bytes.Buffer, *bytes.Buffer, error)
	Shell() shell.Shell
	io.Closer
}

// InteractiveCommandExecutor interface represents type that can execute interactive commands in its context.
type InteractiveCommandExecutor interface {
	// ExecInteractiveCommand starts a process using the specified command.
	// Returns stdout reader, stderr reader, stdin writer, closer for both sides and error.
	ExecInteractiveCommand(cmd string, becomeArgs *types.BecomeArgs) (*types.ProcessPipes, io.Closer, error)
	Shell() shell.Shell
	io.Closer
}

type FileSender interface {
	// SendFile places file at given path with given mode at host.
	SendFile(f io.Reader, path string, mode string) error
	Shell() shell.Shell
	io.Closer
}

func DefaultExecCommandImpl(executor InteractiveCommandExecutor, cmd string, inData *bytes.Reader, becomeArgs *types.BecomeArgs) (*bytes.Buffer, *bytes.Buffer, error) {
	pipes, _, err := executor.ExecInteractiveCommand(cmd, becomeArgs)
	if err != nil {
		return nil, nil, err
	}

	if inData != nil {
		_, err = io.Copy(pipes.Stdin, inData)
		if err != nil {
			return nil, nil, err
		}
	}

	var stderrBuf, stdoutBuf bytes.Buffer
	_, err = stderrBuf.ReadFrom(pipes.Stderr)
	if err != nil {
		return nil, nil, err
	}
	_, err = stdoutBuf.ReadFrom(pipes.Stdout)
	if err != nil {
		return nil, nil, err
	}

	return &stdoutBuf, &stderrBuf, nil
}

func StartBecome(session *ssh.Session, cmd string, becomeArgs *types.BecomeArgs, sh shell.Shell) (*types.ProcessPipes, error) {
	pipes, err := types.NewProcessPipesFromSshSession(session)
	if err != nil {
		return nil, err
	}
	pluginConstructor, exists := repository.FindBecomePluginConstructor(becomeArgs.Method)
	if !exists {
		return nil, fmt.Errorf("become plugin `%s` not found", becomeArgs.Method)
	}
	plugin := pluginConstructor(becomeArgs)
	becomeCmd, saveOutputReadLen, err := plugin.BuildBecomeCommand(cmd, sh)
	if err != nil {
		return nil, err
	}
	err = session.Start(becomeCmd)
	if err != nil {
		return nil, err
	}
	if plugin.ExpectPrompt() {
		var output []byte
		buffer := make([]byte, saveOutputReadLen)
		for !plugin.CheckSuccess(output) && !plugin.CheckPasswordPrompt(output) {
			if _, err = pipes.Stderr.Read(buffer); err != nil {
				return nil, err
			}
			output = append(output, buffer...)
		}

		if !plugin.CheckSuccess(output) {
			if _, err = pipes.Stdin.Write([]byte(becomeArgs.Password + "\n")); err != nil {
				return nil, err
			}
			output = nil
			for !plugin.CheckSuccess(output) {
				if _, err = pipes.Stderr.Read(buffer); err != nil {
					return nil, err
				}
				output = append(output, buffer...)
			}
		}
	}
	return pipes, nil
}

type SendExecuteConnection interface {
	FileSender
	CommandExecutor
}

type Connection interface {
	FileSender
	CommandExecutor
	InteractiveCommandExecutor
	Shell() shell.Shell
	io.Closer
}
