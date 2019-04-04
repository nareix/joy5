package main

import (
	"log"
	"net"
	"os"

	"github.com/armon/go-socks5"
)

func doSocks5Server(listenAddr string) (err error) {
	conf := &socks5.Config{
		Logger: log.New(os.Stderr, "", log.LstdFlags),
	}
	var s *socks5.Server
	if s, err = socks5.New(conf); err != nil {
		return
	}
	var l net.Listener
	if l, err = net.Listen("tcp", listenAddr); err != nil {
		return
	}
	s.Serve(l)
	return
}
