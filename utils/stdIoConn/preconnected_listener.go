package stdIoConn

import (
	"net"
)

type PreconnectedListener struct {
	conn net.Conn
}

var _ net.Listener = &PreconnectedListener{}

func NewPreconnectedListener(conn net.Conn) *PreconnectedListener {
	return &PreconnectedListener{
		conn: conn,
	}
}

func (s *PreconnectedListener) Accept() (net.Conn, error) {
	if s.conn == nil {
		// hang up
		select {}
	}
	conn := s.conn
	s.conn = nil
	return conn, nil
}

func (s *PreconnectedListener) Close() error {
	return nil
}

func (s *PreconnectedListener) Addr() net.Addr {
	return stdIoAddr{}
}
