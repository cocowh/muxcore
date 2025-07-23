// Copyright (c) 2025 cocowh. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package iface

import (
	"context"
	"net"
)

type Connection interface {
	ID() string
	RemoteAddr() net.Addr
	LocalAddr() net.Addr
	Read(b []byte) (int, error)
	Write(b []byte) (int, error)
	Close() error
	Context() context.Context
	SetHandler(handler Handler)
}
