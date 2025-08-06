// Copyright (c) 2025 cocowh. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package ws

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
	"golang.org/x/net/websocket"
)

type conn struct {
	id           string
	rawConn      *websocket.Conn
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

func NewConnection(wsConn *websocket.Conn) iface.Connection {
	ctx, cancel := context.WithCancel(context.Background())
	return &conn{
		id:           uuid.NewString(),
		rawConn:      wsConn,
		ctx:          ctx,
		cancel:       cancel,
		readTimeout:  viper.GetDuration("ws.read_timeout"),
		writeTimeout: viper.GetDuration("ws.write_timeout"),
		bufReader:    bufio.NewReader(wsConn),
		bufWriter:    bufio.NewWriter(wsConn),
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

	return c.rawConn.Read(b)
}

func (c *conn) Write(b []byte) (int, error) {
	if c.closed {
		return 0, common.ErrConnectionClosed
	}

	// 设置写超时
	if c.writeTimeout > 0 {
		c.rawConn.SetWriteDeadline(time.Now().Add(c.writeTimeout))
	}

	return c.rawConn.Write(b)
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

	return c.rawConn.Close()
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
			n, data, err = c.ioHandler.ReadData(c.bufReader)
		} else {
			data = make([]byte, 4096)
			n, err = c.rawConn.Read(data.([]byte))
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
