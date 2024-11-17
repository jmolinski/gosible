package plugins

import (
	"fmt"
	"github.com/scylladb/gosible/connection"
	"github.com/scylladb/gosible/inventory"
	pb "github.com/scylladb/gosible/remote/proto"
	"github.com/scylladb/gosible/utils/shell"
	"google.golang.org/grpc"
)

// ConnectionContext is responsible for keeping all connection specific data, such as the actual connection (eg. SSH),
// connected host information, as well as instantiated plugins for the given host.
// In our architecture, we want to keep a connection to each host therefore the name.
type ConnectionContext struct {
	Connection           connection.CommandExecutor
	RemoteExecutorConn   *grpc.ClientConn
	RemoteExecutorClient pb.GosibleClientClient
	Host                 *inventory.Host
}

func (c *ConnectionContext) Shell() shell.Shell {
	return c.Connection.Shell()
}

func (c *ConnectionContext) Close() error {
	reErr := c.RemoteExecutorConn.Close()
	cErr := c.Connection.Close()

	if reErr == nil {
		return cErr
	}
	if cErr == nil {
		return reErr
	}
	return fmt.Errorf("%s\n%w", reErr, cErr)
}
