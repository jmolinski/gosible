package stdIoConn

import "net"

/**
A stub net.Addr implementation, returned by FileConn and StdIoConn.
*/
type stdIoAddr struct {
}

var _ net.Addr = &stdIoAddr{}

func (s stdIoAddr) Network() string {
	return "stdio"
}

func (s stdIoAddr) String() string {
	return "stdio"
}
