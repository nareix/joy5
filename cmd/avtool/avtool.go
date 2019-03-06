package main

import (
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"strings"

	"github.com/nareix/joy5/format/flv/flvio"

	"github.com/nareix/joy5/format/flv"

	"github.com/nareix/joy5/format/rtmp"

	"github.com/nareix/joy5/av"
	"github.com/nareix/joy5/format"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

var debugRtmpChunkData = false
var debugRtmpOptsMap = map[string]*bool{
	"chunk": &debugRtmpChunkData,
}

var debugFlvHeader = false
var debugFlvOptsMap = map[string]*bool{
	"filehdr": &debugFlvHeader,
}

func handleRtmpFlags(c *rtmp.Conn) {
	if debugRtmpChunkData {
		c.LogChunkDataEvent = func(isRead bool, b []byte) {
			dir := ""
			if isRead {
				dir = "<"
			} else {
				dir = ">"
			}
			fmt.Println(dir, len(b))
			fmt.Print(hex.Dump(b))
		}
	}
}

func handleFlvDemuxerFlags(r *flv.Demuxer) {
	if debugFlvHeader {
		r.LogHeaderEvent = func(flags uint8) {
			avflags := ""
			if flags&flvio.FILE_HAS_AUDIO != 0 {
				avflags += "A"
			}
			if flags&flvio.FILE_HAS_VIDEO != 0 {
				avflags += "V"
			}
			fmt.Println("FLVHeader", "AVFlags", avflags)
		}
	}
}

func doConv(src, dst string) (err error) {
	var fr *format.Reader
	if fr, err = format.Open(src); err != nil {
		return
	}
	if c := fr.Rtmp; c != nil {
		handleRtmpFlags(c)
	}
	if c := fr.Flv; c != nil {
		handleFlvDemuxerFlags(c)
	}

	var fw *format.Writer

	for {
		var pkt av.Packet
		if pkt, err = fr.ReadPacket(); err != nil {
			if err == io.EOF {
				err = nil
			}
			return
		}

		fmt.Println(pkt.String())

		if dst != "" && fw == nil {
			if fw, err = format.Create(dst); err != nil {
				return
			}
			if c := fw.Rtmp; c != nil {
				handleRtmpFlags(c)
			}
		}

		if fw != nil {
			if err = fw.WritePacket(pkt); err != nil {
				return
			}
		}
	}
}

type debugFlags struct {
	a []*[]string
	m []map[string]*bool
}

func debugOptsString(m map[string]*bool) string {
	a := []string{}
	for k := range m {
		a = append(a, k)
	}
	return strings.Join(a, ",")
}

func (f *debugFlags) AddOpt(fs *pflag.FlagSet, name string, optmap map[string]*bool) {
	f.a = append(f.a, fs.StringSlice(name, nil, `supported options: `+debugOptsString(optmap)))
	f.m = append(f.m, optmap)
}

func (f *debugFlags) Parse() bool {
	for i := range f.a {
		a := *f.a[i]
		m := f.m[i]
		for _, o := range a {
			b := m[o]
			if b == nil {
				return false
			}
			*b = true
		}
	}
	return true
}

func main() {
	debugFlags := &debugFlags{}

	cmdConv := &cobra.Command{
		Use:   "conv SRC [DST]",
		Short: "convert src format to dst format",
		Args:  cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			if !debugFlags.Parse() {
				cmd.Help()
				return
			}

			src := args[0]
			dst := ""
			if len(args) >= 2 {
				dst = args[1]
			}
			if err := doConv(src, dst); err != nil {
				log.Println(err)
			}
		},
	}

	debugFlags.AddOpt(cmdConv.Flags(), "drtmp", debugRtmpOptsMap)
	debugFlags.AddOpt(cmdConv.Flags(), "dflv", debugFlvOptsMap)

	rootCmd := &cobra.Command{Use: "avtool"}
	rootCmd.AddCommand(cmdConv)
	rootCmd.Execute()
}
