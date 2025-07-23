// Copyright (c) 2025 cocowh. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package tcp

import (
	"context"
	"github.com/cocowh/muxcore/core/iface"
	"github.com/google/uuid"
	"net"
	"sync"
	"time"
)

type conn struct {
	id          string
	rawConn     net.Conn
	ctx         context.Context
	cancel      context.CancelFunc
	mu          sync.Mutex
	closed      bool
	handler     iface.Handler
	readTimeout time.Duration
}

func NewConnection(c net.Conn) iface.Connection {
	ctx, cancel := context.WithCancel(context.Background())
	return &conn{
		id:          uuid.NewString(),
		rawConn:     c,
		ctx:         ctx,
		cancel:      cancel,
		readTimeout: time.Second * 10, //todo:: from config
	}
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
	return c.rawConn.Read(b)
}

func (c *conn) Write(b []byte) (int, error) {
	return c.rawConn.Write(b)
}

func (c *conn) Close() error {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return nil
	}
	c.closed = true
	c.cancel()
	c.mu.Unlock()

	if c.handler != nil {
		c.handler.OnClose(c)
	}
	return c.rawConn.Close()
}

func (c *conn) Context() context.Context {
	return c.ctx
}

func (c *conn) SetHandler(handler iface.Handler) {
	c.mu.Lock()
	c.handler = handler
	c.mu.Unlock()

	if c.handler != nil {
		c.handler.OnConnect(c)
	}

	go c.readLoop()
}

func (c *conn) readLoop() {
	buf := make([]byte, 4096)
	for {
		select {
		case <-c.ctx.Done():
			return
		default:
		}
		err := c.rawConn.SetReadDeadline(time.Now().Add(c.readTimeout))
		if err != nil {
			c.notifyError(err)
			c.Close()
			return
		}
		n, err := c.rawConn.Read(buf)
		if err != nil {
			c.notifyError(err)
			c.Close()
			return
		}
		c.notifyMessage()
	}
}

func (c *conn) notifyMessage(data []byte) {

}

func (c *conn) notifyError(err error) {
	if c.handler != nil {
		c.handler.OnError(c, err)
	}
}
