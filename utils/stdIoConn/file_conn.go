package stdIoConn

import (
	"fmt"
	"net"
	"os"
	"time"
)

type FileConn struct {
	in      *os.File
	out     *os.File
	closeFn func()
}

var _ net.Conn = &FileConn{}

func NewFileConn(in *os.File, out *os.File, closeFn func()) *FileConn {
	return &FileConn{in: in, out: out, closeFn: closeFn}
}

func (c *FileConn) Read(b []byte) (n int, err error) {
	return c.in.Read(b)
}

func (c *FileConn) Write(b []byte) (n int, err error) {
	return c.out.Write(b)
}

func (c *FileConn) Close() error {
	c.closeFn()
	errIn := c.in.Close()
	errOut := c.out.Close()
	if errIn != nil && errOut != nil {
		return fmt.Errorf("closing both in and out failed; in: %w; out: %v", errIn, errOut)
	}
	if errIn != nil {
		return errIn
	}
	return errOut
}

func (c *FileConn) LocalAddr() net.Addr {
	return stdIoAddr{}
}

func (c *FileConn) RemoteAddr() net.Addr {
	return stdIoAddr{}
}

func (c *FileConn) SetDeadline(t time.Time) error {
	if err := c.SetReadDeadline(t); err != nil {
		return err
	}
	if err := c.SetWriteDeadline(t); err != nil {
		return err
	}
	return nil
}

func (c *FileConn) SetReadDeadline(t time.Time) error {
	return c.in.SetDeadline(t)
}

func (c *FileConn) SetWriteDeadline(t time.Time) error {
	return c.out.SetDeadline(t)
}
