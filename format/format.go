package format

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path"
	"strings"
	"time"

	"github.com/nareix/joy5/format/flv"
	"github.com/nareix/joy5/format/mp4"
	"github.com/nareix/joy5/format/rtsp"

	"github.com/nareix/joy5/av"
	"github.com/nareix/joy5/format/rtmp"
)

type dummyCloser struct{}

func (c dummyCloser) Close() error {
	return nil
}

type Mp4WriterCloser struct {
	io.Closer
	*mp4.Muxer
}

func (c Mp4WriterCloser) Close() error {
	if err := c.Muxer.WriteTrailer(); err != nil {
		return err
	}
	if err := c.Closer.Close(); err != nil {
		return err
	}

	return nil
}

type Reader struct {
	av.PacketReader
	io.Closer
	NetConn  net.Conn
	Rtmp     *rtmp.Conn
	Flv      *flv.Demuxer
	IsRemote bool
}

type Writer struct {
	av.PacketWriter
	io.Closer
	NetConn  net.Conn
	Rtmp     *rtmp.Conn
	IsRemote bool
}

func ErrUnsupported(url_ string) error {
	return fmt.Errorf("open `%s` failed: %s", url_, "unsupported format")
}

type URLOpener struct {
	NewDialFunc     func() func(ctx context.Context, network, address string) (net.Conn, error)
	ReplaceRawConn  func(nc net.Conn) net.Conn
	ReplaceRawRW    func(rw io.ReadWriter) io.ReadWriter
	OnNewRtmpConn   func(c *rtmp.Conn)
	OnNewRtmpServer func(s *rtmp.Server)
	OnNewRtmpClient func(c *rtmp.Client)
	OnNewFlvDemuxer func(r *flv.Demuxer)
	OnNewFlvMuxer   func(w *flv.Muxer)
	av.Streamer
}

func (o *URLOpener) StartRtmpServerWaitConn(u *url.URL) (c *rtmp.Conn, nc net.Conn, err error) {
	host := rtmp.UrlGetHost(u)
	var lis net.Listener
	if lis, err = net.Listen("tcp", host); err != nil {
		return
	}

	s := rtmp.NewServer()
	s.ReplaceRawConn = o.ReplaceRawConn
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
	go func() {
		for {
			nc, err := lis.Accept()
			if err != nil {
				time.Sleep(time.Second)
				continue
			}
			go s.HandleNetConn(nc)
		}
	}()

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
	c.ReplaceRawConn = o.ReplaceRawConn
	c.NewDialFunc = o.NewDialFunc
	if fn := o.OnNewRtmpClient; fn != nil {
		fn(c)
	}
	return c
}

func (o *URLOpener) Create(url_ string, streamer av.Streamer) (w *Writer, err error) {
	var u *url.URL
	if u, err = url.Parse(url_); err != nil {
		return
	}

	switch u.Scheme {
	case "rtmp", "rtmps":
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
			IsRemote:     true,
			PacketWriter: c,
			Closer:       nc,
			Rtmp:         c,
			NetConn:      nc,
		}
		return

	default:
		ext := path.Ext(u.Path)
		var f *os.File
		if f, err = os.Create(u.Path); err != nil {
			return
		}
		switch ext {
		case ".flv":
			c := flv.NewMuxer(NewStreamsWriteSeeker(f, streamer))
			if fn := o.OnNewFlvMuxer; fn != nil {
				fn(c)
			}
			w = &Writer{
				PacketWriter: c,
				Closer:       f,
			}
			return
		case ".mp4":
			var m *mp4.Muxer
			m, err = mp4.NewMuxer(NewStreamsWriteSeeker(f, streamer))
			if err != nil {
				return nil, err
			}
			w = &Writer{
				PacketWriter: m,
				Closer:       Mp4WriterCloser{f, m},
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
			if c, nc, err = o.StartRtmpServerWaitConn(u); err != nil {
				return
			}
			r = &Reader{
				PacketReader: c,
				Closer:       nc,
				Rtmp:         c,
				NetConn:      nc,
				IsRemote:     true,
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
				NetConn:      nc,
				IsRemote:     true,
			}
			return
		}
	case "rtsp":
		var clt *rtsp.Client
		clt, err = rtsp.DialTimeout(url_, 3*time.Second)
		if err != nil {
			return nil, err
		}
		r = &Reader{
			PacketReader: clt,
			Closer:       clt,
			IsRemote:     true,
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
				IsRemote:     true,
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

type StreamsWriteSeeker struct {
	io.WriteSeeker
	av.Streamer
}

func NewStreamsWriteSeeker(w io.WriteSeeker, s av.Streamer) *StreamsWriteSeeker {
	return &StreamsWriteSeeker{
		w, s,
	}
}
