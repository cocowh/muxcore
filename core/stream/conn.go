// Copyright (c) 2025 cocowh. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package stream

import (
	"bufio"
	"context"
	"net"
	"runtime/debug"
	"sync"
	"time"

	"github.com/cocowh/muxcore/core/common"
	"github.com/cocowh/muxcore/core/iface"
	"github.com/cocowh/muxcore/core/utils"
	"github.com/cocowh/muxcore/pkg/logger"
	"github.com/google/uuid"
)

type conn struct {
	mu           sync.RWMutex
	id           string
	rawConn      net.Conn
	ctx          context.Context
	cancel       context.CancelFunc
	ioHandler    iface.IOHandler
	eventHandler iface.EventHandler
	reader       *bufio.Reader
	writer       *bufio.Writer
}

func NewConnection(rawConn net.Conn) iface.Connection {
	ctx, cancel := context.WithCancel(context.Background())

	conn := &conn{
		id:      uuid.New().String(),
		rawConn: rawConn,
		ctx:     ctx,
		cancel:  cancel,
		reader:  bufio.NewReader(rawConn),
		writer:  bufio.NewWriter(rawConn),
	}

	go conn.recvLoop()

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

	if c.ioHandler != nil {
		return c.ioHandler.Read(c.reader, b)
	}

	return c.reader.Read(b)
}

func (c *conn) Write(b []byte) (int, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.ioHandler != nil {
		n, err := c.ioHandler.Write(c.writer, b)
		if err != nil {
			return n, err
		}
		return n, c.writer.Flush()
	}

	n, err := c.writer.Write(b)
	if err != nil {
		return n, err
	}
	return n, c.writer.Flush()
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

func (c *conn) recvLoop() {
	defer utils.PanicHandler(func() {
		logger.Errorf("Stream connection recv loop panic, id: %s, stack: %s", c.id, string(debug.Stack()))
		c.Close()
	})

	defer func() {
		c.mu.RLock()
		handler := c.eventHandler
		c.mu.RUnlock()

		if handler != nil {
			handler.OnClose(c)
		}
	}()

	buf := make([]byte, 4096)

	for {
		select {
		case <-c.ctx.Done():
			return
		default:
		}

		n, err := c.Read(buf)
		if err != nil {
			c.mu.RLock()
			handler := c.eventHandler
			c.mu.RUnlock()

			if handler != nil {
				handler.OnError(c, err)
			}

			// 如果是临时错误，继续尝试
			if common.IsTemporaryError(err) {
				time.Sleep(100 * time.Millisecond)
				continue
			}

			return
		}

		if n > 0 {
			c.mu.RLock()
			handler := c.eventHandler
			c.mu.RUnlock()

			if handler != nil {
				data := make([]byte, n)
				copy(data, buf[:n])
				handler.OnMessage(c, data)
			}
		}
	}
}
