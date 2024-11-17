package stdIoConn

import (
	"github.com/scylladb/gosible/utils/display"
	"io"
	"net"
	"time"
)

type StdIoConn struct {
	in     io.Reader
	out    io.Writer
	closer io.Closer
}

var _ net.Conn = &StdIoConn{}

func NewStdIoConn(in io.Reader, out io.Writer, closer io.Closer) *StdIoConn {
	return &StdIoConn{in: in, out: out, closer: closer}
}

func (c *StdIoConn) Read(b []byte) (n int, err error) {
	return c.in.Read(b)
}

func (c *StdIoConn) Write(b []byte) (n int, err error) {
	return c.out.Write(b)
}

func (c *StdIoConn) Close() error {
	display.Debug(nil, "called stdIoConn.closer.Close()")
	if c.closer == nil {
		display.Fatal(display.ErrorOptions{}, "stdIoConn.closer.Close() called on nil pointer")
	}
	return c.closer.Close()
}

func (c *StdIoConn) LocalAddr() net.Addr {
	return stdIoAddr{}
}

func (c *StdIoConn) RemoteAddr() net.Addr {
	return stdIoAddr{}
}

func (c *StdIoConn) SetDeadline(time.Time) error {
	return nil
}

func (c *StdIoConn) SetReadDeadline(time.Time) error {
	// Non-trivial to implement with a Reader and doesn't seem to be actually required for grpc to function.
	return nil
}

func (c *StdIoConn) SetWriteDeadline(time.Time) error {
	// Non-trivial to implement with a Writer and doesn't seem to be actually required for grpc to function.
	return nil
}
