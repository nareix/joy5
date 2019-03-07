package main

import (
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"strings"
	"sync/atomic"
	"time"

	"github.com/nareix/joy5/format/rtmp"

	"github.com/nareix/joy5/av"
	"github.com/nareix/joy5/format"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

func doConv(src, dst string) (err error) {
	fo := newFormatOpener()

	var fr *format.Reader
	if fr, err = fo.Open(src); err != nil {
		return
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
			if fw, err = fo.Create(dst); err != nil {
				return
			}
		}

		if fw != nil {
			if err = fw.WritePacket(pkt); err != nil {
				return
			}
		}
	}
}

func doServeRtmp(listenAddr, file string) (err error) {
	fo := newFormatOpener()

	filePkts := []av.Packet{}

	if file != "" {
		var r *format.Reader
		if r, err = fo.Open(file); err != nil {
			return
		}
		for {
			var pkt av.Packet
			if pkt, err = r.ReadPacket(); err != nil {
				if err != io.EOF {
					return
				}
				break
			}
			filePkts = append(filePkts, pkt)
		}
		r.Close()
	}

	var lis net.Listener
	if lis, err = net.Listen("tcp", listenAddr); err != nil {
		return
	}

	s := rtmp.NewServer()
	s.OnNewConn = func(c *rtmp.Conn) {
		handleRtmpConnFlags(c)
	}
	handleRtmpServerFlags(s)

	var pubN, subN int64

	s.HandleConn = func(c *rtmp.Conn, nc net.Conn) {
		defer func() {
			nc.Close()
		}()

		var p *int64
		if c.Publishing {
			p = &pubN
		} else {
			p = &subN
		}
		atomic.AddInt64(p, 1)
		defer atomic.AddInt64(p, -1)

		if c.Publishing {
			for {
				if _, err := c.ReadPacket(); err != nil {
					return
				}
			}
		} else {
			start := time.Now()
			for _, pkt := range filePkts {
				if err := c.WritePacket(pkt); err != nil {
					return
				}
				time.Sleep(pkt.Time - time.Now().Sub(start))
			}
		}
	}

	go func() {
		for range time.Tick(time.Second) {
			fmt.Println("sub", atomic.LoadInt64(&subN), "pub", atomic.LoadInt64(&pubN))
		}
	}()

	s.Serve(lis)
	return
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

	run := func(fn func(cmd *cobra.Command, args []string) error) func(cmd *cobra.Command, args []string) {
		return func(cmd *cobra.Command, args []string) {
			if !debugFlags.Parse() {
				cmd.Help()
				os.Exit(1)
			}
			if err := fn(cmd, args); err != nil {
				log.Println(err)
				os.Exit(1)
			}
		}
	}

	cmdServeRtmp := &cobra.Command{
		Use:   "servertmp LISTEN_ADDR [FILE]",
		Short: "start rtmp server for benchmark",
		Args:  cobra.MinimumNArgs(1),
		Run: run(func(cmd *cobra.Command, args []string) error {
			listenAddr := args[0]
			file := ""
			if len(args) >= 2 {
				file = args[1]
			}
			return doServeRtmp(listenAddr, file)
		}),
	}

	cmdConv := &cobra.Command{
		Use:   "conv SRC [DST]",
		Short: "convert src format to dst format",
		Args:  cobra.MinimumNArgs(1),
		Run: run(func(cmd *cobra.Command, args []string) error {
			src := args[0]
			dst := ""
			if len(args) >= 2 {
				dst = args[1]
			}
			return doConv(src, dst)
		}),
	}

	addDebugFlags := func(fs *pflag.FlagSet) {
		debugFlags.AddOpt(fs, "drtmp", debugRtmpOptsMap)
		debugFlags.AddOpt(fs, "dflv", debugFlvOptsMap)
	}
	addDebugFlags(cmdConv.Flags())
	addDebugFlags(cmdServeRtmp.Flags())

	rootCmd := &cobra.Command{Use: "avtool"}
	rootCmd.AddCommand(cmdConv)
	rootCmd.AddCommand(cmdServeRtmp)
	rootCmd.Execute()
}
