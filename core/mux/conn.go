// Copyright (c) 2025 cocowh. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package mux

import (
	// 移除未使用的 "bufio" 导入
	"context"
	"net"

	// 移除未使用的 "runtime/debug" 导入
	"sync"

	"github.com/cocowh/muxcore/core/iface"
	"github.com/google/uuid"
)

type conn struct {
	mu           sync.RWMutex
	id           string
	rawConn      iface.Connection
	ctx          context.Context
	cancel       context.CancelFunc
	ioHandler    iface.IOHandler
	eventHandler iface.EventHandler
	protocol     string
}

func NewConnection(rawConn iface.Connection, protocol string) iface.Connection {
	ctx, cancel := context.WithCancel(context.Background())

	conn := &conn{
		id:       uuid.New().String(),
		rawConn:  rawConn,
		ctx:      ctx,
		cancel:   cancel,
		protocol: protocol,
	}

	return conn
}

func (c *conn) ID() string {
	return c.id
}

func (c *conn) RemoteAddr() net.Addr {
	return c.rawConn.RemoteAddr()
}

func (c *conn) LocalAddr() net.Addr {
	return c.rawConn.LocalAddr()
}

func (c *conn) Read(b []byte) (int, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.rawConn.Read(b)
}

func (c *conn) Write(b []byte) (int, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.rawConn.Write(b)
}

func (c *conn) Close() error {
	c.cancel()
	return c.rawConn.Close()
}

func (c *conn) Context() context.Context {
	return c.ctx
}

func (c *conn) SetIOHandler(handler iface.IOHandler) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.ioHandler = handler
}

func (c *conn) SetEventHandler(handler iface.EventHandler) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.eventHandler = handler

	if handler != nil {
		handler.OnConnect(c)
	}
}
