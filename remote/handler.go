package remote

import (
	"context"
	"crypto/sha256"
	"fmt"
	"github.com/scylladb/gosible/connection"
	"github.com/scylladb/gosible/utils/display"
	"github.com/scylladb/gosible/utils/osUtils"
	"github.com/scylladb/gosible/utils/stdIoConn"
	"github.com/scylladb/gosible/utils/types"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"io"
	"log"
	"net"
	"os"
	"path"
	"strings"
	"time"
)

var grpcDialTimeout = time.Second * 3

// Execute executes remote gosible subprogram over provided connection.
// Should be run as a goroutine as it waits for end of execution.
func Execute(conn connection.Connection, becomeArgs *types.BecomeArgs) (*grpc.ClientConn, error) {
	binaryPath, err := getBinaryPath()
	if err != nil {
		return nil, fmt.Errorf("failed to get binary path: %v", err)
	}

	remotePath, err := sendToRemote(binaryPath, conn, becomeArgs)
	if err != nil {
		return nil, fmt.Errorf("failed to send binary to remote: %v", err)
	}
	return execute(remotePath, conn, becomeArgs)
}

func execute(path string, conn connection.InteractiveCommandExecutor, becomeArgs *types.BecomeArgs) (*grpc.ClientConn, error) {
	display.Debug(nil, "Executing the remote gosible subprogram on host")

	pipes, closer, err := conn.ExecInteractiveCommand(path, becomeArgs)
	if err != nil {
		return nil, err
	}

	bConn := stdIoConn.NewStdIoConn(pipes.Stdout, pipes.Stdin, closer)

	sshDialer := func(ctx context.Context, addr string) (net.Conn, error) {
		return bConn, nil
	}
	grpcCtx, cancel := context.WithTimeout(context.Background(), grpcDialTimeout)
	defer cancel()
	display.Debug(nil, "Running grpc.DialContext")
	return grpc.DialContext(grpcCtx, "", grpc.WithBlock(), grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithContextDialer(sshDialer))
}

func sendToRemote(path string, conn connection.SendExecuteConnection, becomeArgs *types.BecomeArgs) (string, error) {
	if becomeArgs.User != "" {
		return sendToRemoteLegacy(path, conn)
	}

	display.Debug(nil, "Fetching minimum required information about the remote host")

	f, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("failed to open binary: %v", err)
	}
	defer f.Close()

	si, err := gatherSystemInfo(path, conn)
	if err != nil {
		return "", err
	}

	if si.gosibleBinExists {
		return si.gosibleBinPath, nil
	}

	display.Debug(nil, "Sending the gosible subprogram binary to host")
	return si.gosibleTmpBinPath, conn.SendFile(f, si.gosibleTmpBinPath, "0555")
}

func sendToRemoteLegacy(path string, conn connection.SendExecuteConnection) (string, error) {
	// TODO this function should check if binary already exists on host.
	display.Debug(nil, "Sending the gosible subprogram binary to host")

	f, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("failed to open binary: %v", err)
	}
	defer f.Close()

	dir, err := getDirPathLegacy(conn)
	if err != nil {
		return "", err
	}

	remotePath := dir + "/" + ClientFileName

	return remotePath, conn.SendFile(f, remotePath, "0555")
}

func getDirPathLegacy(conn connection.CommandExecutor) (string, error) {
	// TODO we should use same directory. Should be changed when overriding gets fixed in ssh connection.
	stdout, _, err := conn.ExecCommand("DIR=$(mktemp -d); echo \"$DIR\"; chmod 1775 \"$DIR\"", nil, false, &types.BecomeArgs{})
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(stdout.String()), nil
}

func getGosiblePath() (string, error) {
	return osUtils.GetBinaryDir()
}

func getBinaryPath() (string, error) {
	gosiblePath, err := getGosiblePath()
	if err != nil {
		return "", err
	}
	return path.Join(gosiblePath, "remote", "gosible_client"), nil
}

func getHashStr(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		log.Fatal(err)
	}
	return fmt.Sprintf("%x", h.Sum(nil)), nil
}
