package rtmp

import (
	"bufio"
	"io"
)

type wrapReadWriter struct {
	conn *Conn
	br   *bufio.Reader
	rw   ReadWriteFlusher
}

func (r *wrapReadWriter) Peek(n int) (b []byte, err error) {
	return r.br.Peek(n)
}

func (r *wrapReadWriter) Read(b []byte) (n int, err error) {
	if n, err = io.ReadFull(r.br, b); err != nil {
		return 0, err
	}

	r.conn.ackn += uint32(n)

	if fn := r.conn.LogChunkDataEvent; fn != nil {
		fn(true, b)
	}

	return
}

func (r *wrapReadWriter) write(b []byte) (err error) {
	_, err = r.Write(b)
	return
}

func (r *wrapReadWriter) Write(b []byte) (n int, err error) {
	if n, err = r.rw.Write(b); err != nil {
		return
	}

	if fn := r.conn.LogChunkDataEvent; fn != nil {
		fn(false, b)
	}

	return
}

func (r *wrapReadWriter) Flush() (err error) {
	return r.rw.Flush()
}

func newWrapReadWriter(conn *Conn, rw ReadWriteFlusher) *wrapReadWriter {
	return &wrapReadWriter{
		conn: conn,
		br:   bufio.NewReaderSize(rw, 4),
		rw:   rw,
	}
}
