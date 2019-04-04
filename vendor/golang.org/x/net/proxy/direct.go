// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package proxy

import (
	"context"
	"net"
)

type direct struct{}

// Direct is a direct proxy: one that makes network connections directly.
var Direct = direct{}

func (direct) Dial(network, addr string) (net.Conn, error) {
	return net.Dial(network, addr)
}

func (direct) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	dialer := net.Dialer{}
	return dialer.DialContext(ctx, network, address)
}
