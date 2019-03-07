package format

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path"
	"strings"

	"github.com/nareix/joy5/format/flv"

	"github.com/nareix/joy5/av"
	"github.com/nareix/joy5/format/rtmp"
)

type dummyCloser struct{}

func (c dummyCloser) Close() error {
	return nil
}

type Reader struct {
	av.PacketReader
	io.Closer
	Rtmp *rtmp.Conn
	Flv  *flv.Demuxer
}

type Writer struct {
	av.PacketWriter
	io.Closer
	Rtmp *rtmp.Conn
	Flv  *flv.Muxer
}

func ErrUnsupported(url_ string) error {
	return fmt.Errorf("open `%s` failed: %s", url_, "unsupported format")
}

type URLOpener struct {
	OnNewRtmpConn   func(c *rtmp.Conn)
	OnNewRtmpServer func(s *rtmp.Server)
	OnNewRtmpClient func(c *rtmp.Client)
	OnNewFlvDemuxer func(r *flv.Demuxer)
	OnNewFlvMuxer   func(w *flv.Muxer)
}

func (o *URLOpener) StartRtmpServerWaitConn(host string) (c *rtmp.Conn, nc net.Conn, err error) {
	host = rtmp.HostAddDefaultPort(host)
	var lis net.Listener
	if lis, err = net.Listen("tcp", host); err != nil {
		return
	}

	s := rtmp.NewServer()
	if fn := o.OnNewRtmpServer; fn != nil {
		fn(s)
	}
	type Got struct {
		c  *rtmp.Conn
		nc net.Conn
	}
	got_ := make(chan Got, 1)
	s.HandleConn = func(c *rtmp.Conn, nc net.Conn) {
		got_ <- Got{c, nc}
	}
	go s.Serve(lis)

	got := <-got_
	c = got.c
	nc = got.nc
	if fn := o.OnNewRtmpConn; fn != nil {
		fn(c)
	}
	return
}

func (o *URLOpener) newRtmpClient() *rtmp.Client {
	c := rtmp.NewClient()
	if fn := o.OnNewRtmpClient; fn != nil {
		fn(c)
	}
	return c
}

func (o *URLOpener) Create(url_ string) (w *Writer, err error) {
	var u *url.URL
	if u, err = url.Parse(url_); err != nil {
		return
	}

	switch u.Scheme {
	case "rtmp":
		rc := o.newRtmpClient()
		var c *rtmp.Conn
		var nc net.Conn
		if c, nc, err = rc.Dial(url_, rtmp.PrepareWriting); err != nil {
			return
		}
		if fn := o.OnNewRtmpConn; fn != nil {
			fn(c)
		}
		w = &Writer{
			PacketWriter: c,
			Closer:       nc,
			Rtmp:         c,
		}
		return

	default:
		ext := path.Ext(u.Path)
		switch ext {
		case ".flv":
			var f *os.File
			if f, err = os.Create(u.Path); err != nil {
				return
			}
			c := flv.NewMuxer(f)
			if fn := o.OnNewFlvMuxer; fn != nil {
				fn(c)
			}
			w = &Writer{
				PacketWriter: flv.NewMuxer(f),
				Closer:       f,
				Flv:          c,
			}
			return

		default:
			err = ErrUnsupported(url_)
			return
		}
	}
}

func (o *URLOpener) Open(url_ string) (r *Reader, err error) {
	isServer := false
	if strings.HasPrefix(url_, "@") {
		isServer = true
		url_ = url_[1:]
	}

	var u *url.URL
	if u, err = url.Parse(url_); err != nil {
		return
	}

	switch u.Scheme {
	case "rtmp":
		if isServer {
			var c *rtmp.Conn
			var nc net.Conn
			if c, nc, err = o.StartRtmpServerWaitConn(u.Host); err != nil {
				return
			}
			r = &Reader{
				PacketReader: c,
				Closer:       nc,
				Rtmp:         c,
			}
			return
		} else {
			rc := o.newRtmpClient()
			var c *rtmp.Conn
			var nc net.Conn
			if c, nc, err = rc.Dial(url_, rtmp.PrepareReading); err != nil {
				return
			}
			if fn := o.OnNewRtmpConn; fn != nil {
				fn(c)
			}
			r = &Reader{
				PacketReader: c,
				Closer:       nc,
				Rtmp:         c,
			}
			return
		}
		return

	case "http", "https":
		ext := path.Ext(u.Path)
		switch ext {
		case ".flv":
			var hr *http.Response
			if hr, err = http.Get(url_); err != nil {
				return
			}
			c := flv.NewDemuxer(hr.Body)
			if fn := o.OnNewFlvDemuxer; fn != nil {
				fn(c)
			}
			r = &Reader{
				PacketReader: c,
				Closer:       dummyCloser{},
				Flv:          c,
			}
			return

		default:
			err = ErrUnsupported(url_)
			return
		}

	default:
		ext := path.Ext(u.Path)
		switch ext {
		case ".flv":
			var f *os.File
			if f, err = os.Open(u.Path); err != nil {
				return
			}
			c := flv.NewDemuxer(f)
			if fn := o.OnNewFlvDemuxer; fn != nil {
				fn(c)
			}
			r = &Reader{
				PacketReader: c,
				Closer:       f,
				Flv:          c,
			}
			return

		default:
			err = ErrUnsupported(url_)
			return
		}
	}
}
