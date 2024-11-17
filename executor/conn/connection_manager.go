package conn

import (
	"fmt"
	"github.com/scylladb/gosible/connection/factory"
	"github.com/scylladb/gosible/inventory"
	playbookTypes "github.com/scylladb/gosible/playbook/types"
	"github.com/scylladb/gosible/plugins"
	"github.com/scylladb/gosible/remote"
	pb "github.com/scylladb/gosible/remote/proto"
	"github.com/scylladb/gosible/utils/display"
	"github.com/scylladb/gosible/utils/maps"
	"github.com/scylladb/gosible/utils/parallel"
	"github.com/scylladb/gosible/utils/shell"
	"github.com/scylladb/gosible/utils/types"
)

type Manager struct {
	defaultConn *plugins.ConnectionContext
	becomeConns map[string]*plugins.ConnectionContext
	Host        *inventory.Host
	vars        types.Vars
	passwords   types.Passwords
}

const argBecome = "become"
const argBecomeUser = "become_user"
const argBecomeMethod = "become_method"
const argBecomeFlags = "become_flags"

const varBecomePassword = "become_pass"

func NewManager(host *inventory.Host, vars types.Vars, passwords types.Passwords) *Manager {
	return &Manager{Host: host, vars: vars, becomeConns: make(map[string]*plugins.ConnectionContext), passwords: passwords}
}

func (cm *Manager) UpdateOpts(vars types.Vars) {
	cm.vars = vars
}

func (cm *Manager) Close() error {
	f := func(c *plugins.ConnectionContext) error { return c.Close() }
	if errors := parallel.ForAll(maps.Values(cm.becomeConns), f); errors.IsError() {
		return fmt.Errorf("while closing connection to host %s, %w", cm.Host.Name, errors.Combine())
	}
	return nil
}

func createConnection(host *inventory.Host, vars types.Vars, becomeArgs *types.BecomeArgs) (*plugins.ConnectionContext, error) {
	display.Debug(&host.Name, "Creating a connection for host")

	sh, err := shell.Get(vars)
	if err != nil {
		return nil, err
	}

	conn, err := factory.CreateConnection(vars, sh)
	if err != nil {
		return nil, err
	}
	display.Debug(&host.Name, "Starting the gosible executable on host and creating a grpc connection to it")
	grpcConn, err := remote.Execute(conn, becomeArgs)
	if err != nil {
		return nil, err
	}

	return &plugins.ConnectionContext{
		Connection:           conn,
		RemoteExecutorConn:   grpcConn,
		RemoteExecutorClient: pb.NewGosibleClientClient(grpcConn),
		Host:                 host,
	}, nil
}

func (cm *Manager) GetConnForTask(task *playbookTypes.Task) (*plugins.ConnectionContext, error) {
	becomeArgs := cm.getBecomeArgs(task)
	if becomeArgs.Become {
		return cm.getBecomeConn(becomeArgs)
	}
	return cm.getDefaultConn()
}

func (cm *Manager) getDefaultConn() (*plugins.ConnectionContext, error) {
	if cm.defaultConn != nil {
		return cm.defaultConn, nil
	}
	conn, err := createConnection(cm.Host, cm.vars, &types.BecomeArgs{})
	if err != nil {
		return nil, err
	}
	cm.defaultConn = conn
	return conn, nil
}

func (cm *Manager) getBecomeConn(args *types.BecomeArgs) (*plugins.ConnectionContext, error) {
	if conn, ok := cm.becomeConns[args.User]; ok {
		return conn, nil
	}
	conn, err := createConnection(cm.Host, cm.vars, args)
	if err != nil {
		return nil, err
	}
	cm.becomeConns[args.User] = conn
	return conn, nil
}

func (cm *Manager) getBecomeArgs(task *playbookTypes.Task) *types.BecomeArgs {
	args := types.NewBecomeArgs()
	if pass, ok := cm.vars[varBecomePassword].(string); ok {
		args.Password = pass
	}
	if v, ok := task.Keywords[argBecome].(bool); ok {
		args.Become = v
	}
	if v, ok := task.Keywords[argBecomeUser].(string); ok {
		args.User = v
	}
	if v, ok := task.Keywords[argBecomeMethod].(string); ok {
		args.Method = v
	}
	if v, ok := task.Keywords[argBecomeFlags].(string); ok {
		args.Flags = v
	}

	return args
}

func CloseConnMgrs(connMgrs map[string]*Manager) error {
	f := func(cm *Manager) error { return cm.Close() }
	if errors := parallel.ForAll(maps.Values(connMgrs), f); errors.IsError() {
		return fmt.Errorf("while closing connection managers, %w", errors.Combine())
	}
	return nil
}
