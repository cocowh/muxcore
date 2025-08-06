// Copyright (c) 2025 cocowh. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package udp

import (
	"bufio"
	"context"
	"net"
	"sync"
	"time"

	"github.com/cocowh/muxcore/core/common"
	"github.com/cocowh/muxcore/core/iface"
	"github.com/google/uuid"
	"github.com/spf13/viper"
)

type conn struct {
	id           string
	rawConn      *net.UDPConn
	remoteAddr   *net.UDPAddr
	ctx          context.Context
	cancel       context.CancelFunc
	mu           sync.Mutex
	closed       bool
	readTimeout  time.Duration
	bufReader    *bufio.Reader
	bufWriter    *bufio.Writer
	eventHandler iface.EventHandler
	ioHandler    iface.IOHandler
}

func NewConnection(c *net.UDPConn, remoteAddr *net.UDPAddr) iface.Connection {
	ctx, cancel := context.WithCancel(context.Background())
	return &conn{
		id:          uuid.NewString(),
		rawConn:     c,
		remoteAddr:  remoteAddr,
		ctx:         ctx,
		cancel:      cancel,
		readTimeout: viper.GetDuration("udp.read_timeout"),
		bufReader:   bufio.NewReader(c),
		bufWriter:   bufio.NewWriter(c),
	}
}

func (c *conn) ID() string {
	return c.id
}

func (c *conn) RemoteAddr() net.Addr {
	return c.remoteAddr
}

func (c *conn) LocalAddr() net.Addr {
	return c.rawConn.LocalAddr()
}

func (c *conn) Read(b []byte) (int, error) {
	if c.closed {
		return 0, common.ErrConnectionClosed
	}

	if c.ioHandler != nil {
		return c.ioHandler.Read(c.bufReader, b)
	}

	// 设置读超时
	if c.readTimeout > 0 {
		c.rawConn.SetReadDeadline(time.Now().Add(c.readTimeout))
	}

	n, _, err := c.rawConn.ReadFromUDP(b)
	return n, err
}

func (c *conn) Write(b []byte) (int, error) {
	if c.closed {
		return 0, common.ErrConnectionClosed
	}

	if c.ioHandler != nil {
		return c.ioHandler.Write(c.bufWriter, b)
	}

	return c.rawConn.WriteToUDP(b, c.remoteAddr)
}

func (c *conn) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return nil
	}

	c.closed = true
	c.cancel()

	if c.eventHandler != nil {
		c.eventHandler.OnClose(c)
	}

	// UDP连接不需要关闭底层连接，由客户端/服务器管理
	return nil
}

func (c *conn) Context() context.Context {
	return c.ctx
}

func (c *conn) SetIOHandler(handler iface.IOHandler) {
	c.mu.Lock()
	c.ioHandler = handler
	c.mu.Unlock()
}

func (c *conn) SetEventHandler(handler iface.EventHandler) {
	c.mu.Lock()
	c.eventHandler = handler
	c.mu.Unlock()

	if c.eventHandler != nil {
		c.eventHandler.OnConnect(c)
	}
}
