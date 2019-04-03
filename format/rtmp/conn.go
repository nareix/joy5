package rtmp

import (
	"io"
	"net/url"

	"github.com/nareix/joy5/format/flv/flvio"
)

type ReadWriteFlusher interface {
	io.ReadWriter
	Flush() error
}

type Conn struct {
	LogStageEvent       func(event string, url string)
	LogChunkDataEvent   func(isRead bool, b []byte)
	LogChunkHeaderEvent func(isRead bool, m message)
	LogTagEvent         func(isRead bool, t flvio.Tag)
	LogMsgEvent         func(isRead bool, m message)

	HandleEvent func(msgtypeid uint8, msgdata []byte) (handled bool, err error)

	URL      *url.URL
	PageUrl  string
	TcUrl    string
	FlashVer string

	PubPlayErr            error
	PubPlayOnStatusParams flvio.AMFMap

	closeNotify chan bool

	wrapRW *wrapReadWriter

	peekread chan *message

	avmsgsid            uint32
	writebuf, writebuf2 []byte
	readbuf, readbuf2   []byte
	lastackn, ackn      uint32
	writeMaxChunkSize   int
	ReadMaxChunkSize    int
	readAckSize         uint32
	readcsmap           map[uint32]*message
	aggmsg              *message

	lastcmd *command

	isserver   bool
	Publishing bool
	Stage      Stage

	SendSampleAccess bool
	BypassMsgtypeid  []uint8
}

func NewConn(rw ReadWriteFlusher) *Conn {
	c := &Conn{}
	c.closeNotify = make(chan bool, 1)
	c.wrapRW = newWrapReadWriter(c, rw)
	c.readcsmap = make(map[uint32]*message)
	c.ReadMaxChunkSize = 128
	c.writeMaxChunkSize = 128
	c.writebuf = make([]byte, 256)
	c.writebuf2 = make([]byte, 256)
	c.readbuf = make([]byte, 256)
	c.readbuf2 = make([]byte, 256)
	c.readAckSize = 2500000
	return c
}

func (c *Conn) writing() bool {
	if c.isserver {
		return !c.Publishing
	} else {
		return c.Publishing
	}
}

func (c *Conn) writePubPlayErrBeforeClose() {
	if c.PubPlayErr == nil {
		return
	}
	if c.Stage < StageGotPublishOrPlayCommand {
		return
	}
	c.Prepare(StageCommandDone, PrepareWriting)
}

func (c *Conn) flushWrite() error {
	return c.wrapRW.Flush()
}
