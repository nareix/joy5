package format

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path"

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

func Create(url_ string) (w *Writer, err error) {
	var u *url.URL
	if u, err = url.Parse(url_); err != nil {
		return
	}

	switch u.Scheme {
	case "rtmp":
		var c *rtmp.Conn
		var nc net.Conn
		if c, nc, err = rtmp.Dial(url_, rtmp.PrepareWriting); err != nil {
			return
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
			w = &Writer{
				PacketWriter: flv.NewMuxer(f),
				Closer:       f,
				Flv:          c,
			}
			return

		default:
			err = fmt.Errorf("open `%s` failed: %s", url_, "unsupported format")
			return
		}
	}
}

func Open(url_ string) (r *Reader, err error) {
	var u *url.URL
	if u, err = url.Parse(url_); err != nil {
		return
	}

	errUnsupported := fmt.Errorf("open `%s` failed: %s", url_, "unsupported format")

	switch u.Scheme {
	case "rtmp":
		var c *rtmp.Conn
		var nc net.Conn
		if c, nc, err = rtmp.Dial(url_, rtmp.PrepareReading); err != nil {
			return
		}
		r = &Reader{
			PacketReader: c,
			Closer:       nc,
			Rtmp:         c,
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
			r = &Reader{
				PacketReader: c,
				Closer:       dummyCloser{},
				Flv:          c,
			}
			return

		default:
			err = errUnsupported
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
			r = &Reader{
				PacketReader: c,
				Closer:       f,
				Flv:          c,
			}
			return

		default:
			err = errUnsupported
			return
		}
	}
}
