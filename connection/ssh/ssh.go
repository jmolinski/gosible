// Package sshConnection is a connection that can execute command or transfer files over a ssh connection.
package sshConnection

import (
	"bytes"
	"context"
	"github.com/bramvdbogaerde/go-scp"
	"github.com/google/shlex"
	"github.com/rjeczalik/gsh"
	"github.com/rjeczalik/gsh/sshfile"
	"github.com/rjeczalik/gsh/sshutil"
	"github.com/scylladb/gosible/connection"
	"github.com/scylladb/gosible/utils/display"
	"github.com/scylladb/gosible/utils/shell"
	"github.com/scylladb/gosible/utils/types"
	"golang.org/x/crypto/ssh"
	"io"
	"strconv"
)

type ConnectionData struct {
	User         string
	HostName     string
	IdentityFile string
	Password     string
	Port         string
	Args         string
}

type Connection struct {
	Conn  gsh.Conn
	shell shell.Shell
}

// Interface compliance check.
var _ connection.Connection = &Connection{}

func (conn *Connection) Close() error {
	return conn.Conn.Close()
}

func (conn *Connection) Shell() shell.Shell {
	return conn.shell
}

func (conn *Connection) SendFile(f io.Reader, path string, mode string) error {
	var sendFileHandler gsh.SessionFunc = func(conn context.Context, session *ssh.Session) error {
		client := scp.NewConfigurer("", nil).Session(session).Create()
		defer client.Close()

		// FIXME this function breaks when file already exists. Also error is misleading.
		return client.CopyFile(f, path, mode)
	}

	return conn.Conn.Session(sendFileHandler)
}

func New(data *ConnectionData, sh shell.Shell) (*Connection, error) {
	cfg, err := getConfig(data)

	if err != nil {
		return nil, err
	}

	ctx := context.Background()

	client := &gsh.Client{
		ConfigCallback: cfg.Callback(),
		DialContext:    sshutil.DialContext,
	}

	conn, err := client.Connect(ctx, "tcp", "")
	if err != nil {
		return nil, err
	}
	return &Connection{conn, sh}, nil
}

func getConfig(data *ConnectionData) (*sshfile.Config, error) {
	words, err := shlex.Split(data.Args)
	if err != nil {
		return nil, err
	}
	opts, err := sshfile.ParseOptions(words)
	if err != nil {
		return nil, err
	}

	if data.User != "" {
		opts.User = data.User
	}
	if data.IdentityFile != "" {
		opts.IdentityFile = data.IdentityFile
	}
	if data.Port != "" {
		opts.Port, err = strconv.Atoi(data.Port)
		if err != nil {
			return nil, err
		}
	}
	if data.HostName != "" {
		opts.Hostname = data.HostName
	}

	// TODO password authentication is not supported in gsh
	return opts, nil
}

func (conn *Connection) ExecCommand(cmd string, inData *bytes.Reader, sudoable bool, becomeArgs *types.BecomeArgs) (*bytes.Buffer, *bytes.Buffer, error) {
	return connection.DefaultExecCommandImpl(conn, cmd, inData, becomeArgs)
}

type customCloser func() error

func (c customCloser) Close() error {
	return c()
}

func (conn *Connection) ExecInteractiveCommand(cmd string, becomeArgs *types.BecomeArgs) (pipes *types.ProcessPipes, closer io.Closer, err error) {
	done := make(chan bool, 1)
	sh := conn.Shell()
	var execCommandInteractiveHandler gsh.SessionFunc = func(conn context.Context, session *ssh.Session) error {
		// We want the customCloser to capture err returned from session.Wait()
		// this way we can propagate the error when closer is called -- otherwise
		// the error would be silenced.
		// Discussion: https://github.com/rjeczalik/gosible/pull/74/files#r802215807
		closer = customCloser(func() error {
			sessionCloseErr := session.Close()
			if err != nil {
				return err
			}
			return sessionCloseErr
		})
		if becomeArgs.Become {
			pipes, err = connection.StartBecome(session, cmd, becomeArgs, sh)
			if err != nil {
				return err
			}
		} else {
			pipes, err = types.NewProcessPipesFromSshSession(session)
			if err != nil {
				return err
			}

			err = session.Start(cmd)
			if err != nil {
				return err
			}
		}

		done <- true

		return session.Wait()
	}

	go func() {
		err = conn.Conn.Session(execCommandInteractiveHandler)
		if err != nil {
			display.Error(display.ErrorOptions{}, "Error during executing command over ssh: %s", err)
		}
		done <- true
	}()

	<-done
	return
}

func FromVars(vars types.Vars, sh shell.Shell) (*Connection, error) {
	var identityFile, pass, args, hostName, port, user string

	if identityFileVal, ok := vars["private_key_file"].(string); ok {
		identityFile = identityFileVal
	}
	if passVal, ok := vars["password"].(string); ok {
		pass = passVal
	}
	if argsVal, ok := vars["ssh_extra_args"].(string); ok {
		args = argsVal
	}
	if hostNameVal, ok := vars["remote_addr"].(string); ok {
		hostName = hostNameVal
	}
	if portVal, ok := vars["port"].(string); ok {
		port = portVal
	}
	if userVal, ok := vars["remote_user"].(string); ok {
		user = userVal
	}

	return New(&ConnectionData{
		HostName:     hostName,
		Port:         port,
		User:         user,
		Args:         args,
		IdentityFile: identityFile,
		Password:     pass,
	}, sh)
}
