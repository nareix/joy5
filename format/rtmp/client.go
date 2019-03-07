package rtmp

import (
	"bufio"
	"fmt"
	"net"
	"net/url"
	"time"
)

func HostAddDefaultPort(host string) string {
	if _, _, err := net.SplitHostPort(host); err != nil {
		host = net.JoinHostPort(host, "1935")
	}
	return host
}

type Client struct {
	LogEvent func(c *Conn, nc net.Conn, e int)
}

func NewClient() *Client {
	return &Client{}
}

func (t *Client) Dial(url_ string, flags int) (c *Conn, nc net.Conn, err error) {
	var u *url.URL
	if u, err = url.Parse(url_); err != nil {
		return
	}

	host := HostAddDefaultPort(u.Host)

	if nc, err = net.Dial("tcp", host); err != nil {
		err = fmt.Errorf("dial `%s` failed: %s", host, err)
		return
	}

	rw := &bufReadWriter{
		Reader: bufio.NewReaderSize(nc, BufioSize),
		Writer: bufio.NewWriterSize(nc, BufioSize),
	}
	c_ := NewConn(rw)
	c_.URL = u

	nc.SetDeadline(time.Now().Add(time.Second * 15))
	if err = c_.Prepare(StageGotPublishOrPlayCommand, flags); err != nil {
		if fn := t.LogEvent; fn != nil {
			fn(c_, nc, EventHandshakeFailed)
		}
		nc.Close()
		return
	}
	nc.SetDeadline(time.Time{})

	c = c_

	if fn := t.LogEvent; fn != nil {
		fn(c, nc, EventConnConnected)
	}
	return
}
