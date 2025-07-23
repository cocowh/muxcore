// Copyright (c) 2025 cocowh. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package iface

type Handler interface {
	OnConnect(conn Connection)
	OnMessage(conn Connection, data []byte)
	OnClose(conn Connection)
	OnError(conn Connection, err error)
}
