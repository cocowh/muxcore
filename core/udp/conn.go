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
	"github.com/cocowh/muxcore/core/utils"
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
	writeTimeout time.Duration
	bufReader    *bufio.Reader
	bufWriter    *bufio.Writer
	eventHandler iface.EventHandler
	ioHandler    iface.IOHandler
	once         sync.Once
}

func NewConnection(c *net.UDPConn, remoteAddr *net.UDPAddr) iface.Connection {
	ctx, cancel := context.WithCancel(context.Background())
	return &conn{
		id:           uuid.NewString(),
		rawConn:      c,
		remoteAddr:   remoteAddr,
		ctx:          ctx,
		cancel:       cancel,
		readTimeout:  viper.GetDuration("udp.read_timeout"),
		writeTimeout: viper.GetDuration("udp.write_timeout"),
		bufReader:    bufio.NewReader(c),
		bufWriter:    bufio.NewWriter(c),
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

	if c.eventHandler != nil {
		return 0, common.ErrConflictWithEventHandler
	}

	// 设置读超时
	if c.readTimeout > 0 {
		c.rawConn.SetReadDeadline(time.Now().Add(c.readTimeout))
	}

	if c.ioHandler != nil {
		return c.ioHandler.Read(c.bufReader, b)
	}

	n, _, err := c.rawConn.ReadFromUDP(b)
	return n, err
}

func (c *conn) Write(b []byte) (int, error) {
	if c.closed {
		return 0, common.ErrConnectionClosed
	}

	// 设置写超时
	if c.writeTimeout > 0 {
		c.rawConn.SetWriteDeadline(time.Now().Add(c.writeTimeout))
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
		c.once.Do(func() {
			go c.recv()
		})
	}
}

func (c *conn) recv() {
	defer utils.PanicHandler(func() {
		c.closeOnError(common.ErrPanic)
	})

	if err := c.recvLoop(); err != nil {
		c.closeOnError(err)
	}
}

func (c *conn) recvLoop() error {
	for {
		select {
		case <-c.ctx.Done():
			return nil
		default:
		}

		if c.readTimeout > 0 {
			c.rawConn.SetReadDeadline(time.Now().Add(c.readTimeout))
		}

		var data any
		var err error
		var n int

		if c.ioHandler != nil {
			_, data, err = c.ioHandler.ReadData(c.bufReader)
		} else {
			data = make([]byte, 65536)
			n, _, err = c.rawConn.ReadFromUDP(data.([]byte))
			if n > 0 {
				if bytesData, ok := data.([]byte); ok {
					data = bytesData[:n]
				}
			}
		}

		if err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				continue
			}
			c.notifyError(err)
			return err
		}

		c.notifyMessage(data)
	}
}

func (c *conn) notifyMessage(data any) {
	if c.eventHandler != nil {
		c.eventHandler.OnMessage(c, data)
	}
}

func (c *conn) notifyError(err error) {
	if c.eventHandler != nil {
		c.eventHandler.OnError(c, err)
	}
}

func (c *conn) closeOnError(err error) {
	if err != nil {
		c.notifyError(err)
	}
	c.Close()
}
