package main

import (
	"log"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

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

	cmdBenchRtmp := &cobra.Command{
		Use:   "benchrtmp LISTEN_ADDR [FILE]",
		Short: "start rtmp server for benchmark",
		Args:  cobra.MinimumNArgs(1),
		Run: run(func(cmd *cobra.Command, args []string) error {
			listenAddr := args[0]
			file := ""
			if len(args) >= 2 {
				file = args[1]
			}
			return doBenchRtmp(listenAddr, file)
		}),
	}

	cmdForwardRtmp := &cobra.Command{
		Use:   "forwardrtmp LISTEN_ADDR",
		Short: "start rtmp forward server",
		Args:  cobra.MinimumNArgs(1),
		Run: run(func(cmd *cobra.Command, args []string) error {
			listenAddr := args[0]
			return doForwardRtmp(listenAddr)
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

	cmdPubsubRtmp := &cobra.Command{
		Use:   "pubsubrtmp LISTEN_ADDR",
		Short: "simple pub sub rtmp server",
		Args:  cobra.MinimumNArgs(1),
		Run: run(func(cmd *cobra.Command, args []string) error {
			listenAddr := args[0]
			return doPubsubRtmp(listenAddr)
		}),
	}

	cmdAvcc2Annexb := &cobra.Command{
		Use:   "avcc2annexb src dst",
		Short: "convert avcc flv to annexb flv",
		Args:  cobra.MinimumNArgs(2),
		Run: run(func(cmd *cobra.Command, args []string) error {
			return doAvcc2Annexb(args[0], args[1])
		}),
	}

	cmdMoveH264SeqhdrToKeyFrame := &cobra.Command{
		Use:   "moveh264seqhdrtokeyframe src dst",
		Short: "move h264 seqhdr to keyframe",
		Args:  cobra.MinimumNArgs(2),
		Run: run(func(cmd *cobra.Command, args []string) error {
			return doMoveH264SeqhdrToKeyFrame(args[0], args[1])
		}),
	}

	addDebugFlags := func(fs *pflag.FlagSet) {
		debugFlags.AddOpt(fs, "drtmp", debugRtmpOptsMap)
		debugFlags.AddOpt(fs, "dflv", debugFlvOptsMap)
	}
	addDebugFlags(cmdConv.Flags())
	addDebugFlags(cmdBenchRtmp.Flags())
	addDebugFlags(cmdForwardRtmp.Flags())
	addDebugFlags(cmdPubsubRtmp.Flags())
	cmdConv.Flags().BoolVar(&optPrintStatSec, "statsec", false, "print stat per second")
	cmdConv.Flags().BoolVar(&optNativeRate, "re", false, "native rate")
	cmdConv.Flags().BoolVar(&optDontPrintPkt, "qpkt", false, "don't print pkt")

	rootCmd := &cobra.Command{Use: "avtool"}
	rootCmd.AddCommand(cmdConv)
	rootCmd.AddCommand(cmdBenchRtmp)
	rootCmd.AddCommand(cmdForwardRtmp)
	rootCmd.AddCommand(cmdPubsubRtmp)
	rootCmd.AddCommand(cmdAvcc2Annexb)
	rootCmd.AddCommand(cmdMoveH264SeqhdrToKeyFrame)
	rootCmd.Execute()
}
