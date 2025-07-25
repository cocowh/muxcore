// Copyright (c) 2025 cocowh. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package iface

import "bufio"

type IOHandler interface {
	Read(reader *bufio.Reader, b []byte) (int, error)
	ReadData(reader *bufio.Reader) (int, any, error)
	Write(writer *bufio.Writer, b []byte) (int, error)
}

type EventHandler interface {
	OnConnect(conn Connection)
	OnMessage(conn Connection, data any)
	OnClose(conn Connection)
	OnError(conn Connection, err error)
}
