package factory

import (
	"github.com/scylladb/gosible/connection"
	sshConnection "github.com/scylladb/gosible/connection/ssh"
	"github.com/scylladb/gosible/utils/shell"
	"github.com/scylladb/gosible/utils/types"
)

func CreateConnection(vars types.Vars, sh shell.Shell) (connection.Connection, error) {
	conn, err := sshConnection.FromVars(vars, sh)
	return conn, err
}
