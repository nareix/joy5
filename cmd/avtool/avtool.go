package main

import (
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"strings"

	"github.com/nareix/joy5/format/rtmp"

	"github.com/nareix/joy5/av"
	"github.com/nareix/joy5/format"
	"github.com/spf13/cobra"
)

var debugRtmpChunkData = false

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

func doConv(src, dst string) (err error) {
	var fr *format.Reader
	if fr, err = format.Open(src); err != nil {
		return
	}
	if c := fr.Rtmp; c != nil {
		handleRtmpFlags(c)
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

func main() {
	var debugRtmpOpts = []string{}
	debugRtmpOptsMap := map[string]*bool{
		"chunk": &debugRtmpChunkData,
	}

	cmdConv := &cobra.Command{
		Use:   "conv SRC [DST]",
		Short: "convert src format to dst format",
		Args:  cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			for _, o := range debugRtmpOpts {
				b := debugRtmpOptsMap[o]
				if b == nil {
					cmd.Help()
					return
				}
				*b = true
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

	{
		opts := []string{}
		for k := range debugRtmpOptsMap {
			opts = append(opts, k)
		}
		allOpts := strings.Join(opts, ",")
		cmdConv.Flags().StringSliceVar(&debugRtmpOpts, "drtmp", nil, `supported options: `+allOpts)
	}

	rootCmd := &cobra.Command{Use: "avtool"}
	rootCmd.AddCommand(cmdConv)
	rootCmd.Execute()
}
