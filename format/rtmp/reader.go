package rtmp

import (
	"bufio"
	"io"
	"sync/atomic"
)

type wrapReadWriter struct {
	conn *Conn
	br   *bufio.Reader
}

func (r *wrapReadWriter) Peek(n int) (b []byte, err error) {
	return r.br.Peek(n)
}

func (r *wrapReadWriter) Read(b []byte) (n int, err error) {
	if n, err = io.ReadFull(r.br, b); err != nil {
		return 0, err
	}

	atomic.AddInt64(&r.conn.rawbytes, int64(len(b)))
	r.conn.ackn += uint32(n)

	if r.conn.HandleRead != nil {
		r.conn.HandleRead(b)
	}
	if DebugChunkData {
		r.conn.Logger.HexdumpInfo("Read", b)
	}

	if r.conn.HookOnRead != nil {
		r.conn.HookOnRead(b)
	}

	return
}

func (r *wrapReadWriter) write(b []byte) (err error) {
	_, err = r.Write(b)
	return
}

func (r *wrapReadWriter) Write(b []byte) (n int, err error) {
	if n, err = r.conn.RW.Write(b); err != nil {
		return
	}
	atomic.AddInt64(&r.conn.rawbytes, int64(len(b)))
	atomic.AddInt64(&r.conn.BytesSent, int64(len(b)))
	if DebugChunkData {
		r.conn.Logger.HexdumpInfo("Write", b)
	}
	return
}

func newWrapReadWriter(conn *Conn) *wrapReadWriter {
	return &wrapReadWriter{
		conn: conn,
		br:   bufio.NewReaderSize(conn.RW, 4),
	}
}
