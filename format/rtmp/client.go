package rtmp

import (
	"bufio"
	"context"
	"crypto/tls"
	"net"
	"net/url"
	"time"
)

func (t *Client) FromNetConn(nc net.Conn, u *url.URL, flags int) (c *Conn, err error) {
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
		return
	}
	nc.SetDeadline(time.Time{})

	c = c_
	return
}

func UrlGetHost(u *url.URL) string {
	host := u.Host
	defaultPort := ""
	switch u.Scheme {
	case "rtmp":
		defaultPort = "1935"
	case "rtmps":
		defaultPort = "443"
	}
	if _, _, err := net.SplitHostPort(host); err != nil {
		host = net.JoinHostPort(host, defaultPort)
	}
	return host
}

type Client struct {
	LogEvent       func(c *Conn, nc net.Conn, e int)
	ReplaceRawConn func(nc net.Conn) net.Conn
	NewDialFunc    func() func(ctx context.Context, network, address string) (net.Conn, error)
}

func NewClient() *Client {
	return &Client{}
}

func (t *Client) doDial(host string) (nc net.Conn, err error) {
	dialer := &net.Dialer{}
	dial := dialer.DialContext
	if fn := t.NewDialFunc; fn != nil {
		dial = fn()
	}
	ctx, _ := context.WithTimeout(context.Background(), time.Second*15)
	if nc, err = dial(ctx, "tcp", host); err != nil {
		return
	}
	if t.ReplaceRawConn != nil {
		nc = t.ReplaceRawConn(nc)
	}
	return
}

func (t *Client) Dial(url_ string, flags int) (c *Conn, nc net.Conn, err error) {
	var u *url.URL
	if u, err = url.Parse(url_); err != nil {
		return
	}
	host := UrlGetHost(u)

	var nc_ net.Conn
	switch u.Scheme {
	case "rtmp":
		if nc_, err = t.doDial(host); err != nil {
			return
		}
	case "rtmps":
		if nc_, err = t.doDial(host); err != nil {
			return
		}
		nc_ = tls.Client(nc_, &tls.Config{InsecureSkipVerify: true})
	}

	var c_ *Conn
	if c_, err = t.FromNetConn(nc_, u, flags); err != nil {
		nc_.Close()
		return
	}

	c = c_
	nc = nc_
	return
}
