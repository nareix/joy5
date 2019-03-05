package rtmp

import (
	"fmt"
	"io"
	"net"
	"net/url"
	"strings"
	"sync/atomic"
	"time"

	"github.com/hashicorp/errwrap"

	"github.com/nareix/joy5/format/flv"
	"github.com/nareix/joy5/format/flv/flvio"
	"github.com/nareix/joy5/utils"
)

func IsRtmpError(err error) bool {
	return false
}

var Timeout = time.Second * 30
var DialTimeout = time.Second * 5
var DataTimeout = time.Second * 10

var DebugConn = false
var DebugCmd = false
var DebugStage = false
var DebugMsgHeader = false
var DebugMsgData = false
var DebugChunkHeader = false
var DebugChunkData = false

func splitPath(u *url.URL) (app, stream string) {
	nu := *u
	nu.ForceQuery = false

	pathsegs := strings.SplitN(nu.RequestURI(), "/", -1)
	if len(pathsegs) == 2 {
		app = pathsegs[1]
	}
	if len(pathsegs) == 3 {
		app = pathsegs[1]
		stream = pathsegs[2]
	}
	if len(pathsegs) > 3 {
		app = strings.Join(pathsegs[1:3], "/")
		stream = strings.Join(pathsegs[3:], "/")
	}
	return
}

func getTcUrl(u *url.URL) string {
	app, _ := splitPath(u)
	nu := *u
	nu.RawQuery = ""
	nu.Path = "/"
	return nu.String() + app
}

func createURL(tcurl, app, play string) (u *url.URL, err error) {
	if u, err = url.ParseRequestURI("/" + app + "/" + play); err != nil {
		return
	}

	var tu *url.URL
	if tu, err = url.Parse(tcurl); err != nil {
		return
	}

	if tu.Host == "" {
		err = fmt.Errorf("TcUrlHostEmpty")
		return
	}
	u.Host = tu.Host

	if tu.Scheme == "" {
		err = fmt.Errorf("TcUrlSchemeEmpty")
		return
	}
	u.Scheme = tu.Scheme

	return
}

type Server struct {
	HandleConn func(*Conn)
}

func NewServer() *Server {
	return &Server{}
}

func (s *Server) handleConn(conn *Conn) {
	var err error

	if DebugConn {
		conn.Logger.Info("RtmpServerConnected", "LocalAddr", conn.ConnLocalAddr, "RemoteAddr", conn.ConnRemoteAddr)
	}

	if err = conn.SetDeadline(time.Now().Add(Timeout)); err != nil {
		goto fail
	}

	if err = conn.handshakeServer(); err != nil {
		err = errwrap.Wrapf("HandshakeServerFailed", err)
		goto fail
	}

	if DebugConn {
		conn.Logger.Info("RtmpServerHandshakeDone", "FastRtmp", conn.isFastRtmp)
	}

	go s.HandleConn(conn)
	return

fail:
	if DebugConn {
		conn.Logger.Error("RtmpServerHandleConnFailed", "Err", err)
	}
	conn.Close()
	return
}

func (s *Server) Serve(listener net.Listener) (err error) {
	for {
		var netconn net.Conn
		if netconn, err = listener.Accept(); err != nil {
			return
		}

		c := NewConn(netconn)
		c.LocalAddr = netconn.LocalAddr().String()
		c.RemoteAddr = netconn.RemoteAddr().String()
		c.isserver = true

		go s.handleConn(c)
	}
}

type Stage int

var stagestrs = map[Stage]string{
	StageHandshakeDone:           "StageHandshakeDone",
	StageGotPublishOrPlayCommand: "StageGotPublishOrPlayCommand",
	StageCommandDone:             "StageCommandDone",
	StageDataStart:               "StageDataStart",
}

func (s Stage) String() string {
	return stagestrs[s]
}

const (
	StageHandshakeDone = iota
	StageGotPublishOrPlayCommand
	StageCommandDone
	StageDataStart
)

const (
	PrepareReading = iota + 1
	PrepareWriting
)

type Conn struct {
	LocalAddr  string
	RemoteAddr string

	Logger *utils.Logger

	URL      *url.URL
	PageUrl  string
	TcUrl    string
	FlashVer string

	PubPlayErr            error
	PubPlayOnStatusParams flvio.AMFMap

	closeNotify chan bool

	TagToPkt *flv.TagToPacket
	PktToTag *flv.PacketToTag

	RW     io.ReadWriter
	wrapRW *wrapReadWriter

	rawbytes  int64
	BytesSent int64

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

func NewConn(RW io.ReadWriter) *Conn {
	c := &Conn{}
	c.closeNotify = make(chan bool, 1)
	c.TagToPkt = flv.NewTagToPacket()
	c.PktToTag = flv.NewPacketToTag()
	c.RW = RW
	c.wrapRW = newWrapReadWriter(conn)
	c.readcsmap = make(map[uint32]*message)
	c.ReadMaxChunkSize = 128
	c.writeMaxChunkSize = 128
	c.writebuf = make([]byte, 256)
	c.writebuf2 = make([]byte, 256)
	c.readbuf = make([]byte, 256)
	c.readbuf2 = make([]byte, 256)
	c.Logger = utils.NewLogger()
	c.Logger.ShowElapsed = true
	c.TagToPkt.Logger = c.Logger
	c.PktToTag.Logger = c.Logger
	c.readAckSize = 2500000
	return conn
}

func (c *Conn) CloseNotify() <-chan bool {
	return c.closeNotify
}

func (c *Conn) writing() bool {
	if c.isserver {
		return !c.Publishing
	} else {
		return c.Publishing
	}
}

func (c *Conn) IsServer() bool {
	return c.isserver
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

func (c *Conn) RawBytes() int64 {
	return atomic.SwapInt64(&c.rawbytes, 0)
}
