// Copyright (c) 2025 cocowh. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package iface

import "context"

type Client interface {
	Connect() error
	Send(data []byte) (int, error)
	Close() error
	SetEventHandler(eventHandler EventHandler)
	SetIOHandler(ioHandler IOHandler)
	Context() context.Context
}
