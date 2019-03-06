package rtmp

import (
	"bufio"
	"net"
	"time"
)

const (
	EventServerConnected = 1
	EventHandshakeFailed = 2
	EventHandshakeDone   = 3
	EventConnFinish      = 4
)

type Server struct {
	HandleConn func(c *Conn, nc net.Conn)

	HandshakeTimeout time.Duration

	LogEvent func(c *Conn, e int)
}

func NewServer() *Server {
	return &Server{
		HandshakeTimeout: time.Second * 10,
	}
}

type bufReadWriter struct {
	*bufio.Reader
	*bufio.Writer
}

var BufioSize = 4096

func (s *Server) handleAcceptConn(nc net.Conn) {
	defer nc.Close()

	rw := &bufReadWriter{
		Reader: bufio.NewReaderSize(nc, BufioSize),
		Writer: bufio.NewWriterSize(nc, BufioSize),
	}
	c := NewConn(rw)
	c.isserver = true

	if s.LogEvent != nil {
		s.LogEvent(c, EventServerConnected)
	}

	nc.SetDeadline(time.Now().Add(time.Second * 15))
	if err := c.Prepare(StageGotPublishOrPlayCommand, 0); err != nil {
		if s.LogEvent != nil {
			s.LogEvent(c, EventHandshakeFailed)
		}
		return
	}
	nc.SetDeadline(time.Time{})

	s.HandleConn(c, nc)

	if s.LogEvent != nil {
		s.LogEvent(c, EventConnFinish)
	}
}

func (s *Server) Serve(lis net.Listener) (err error) {
	for {
		var nc net.Conn
		if nc, err = lis.Accept(); err != nil {
			return
		}
		go s.handleAcceptConn(nc)
	}
}
