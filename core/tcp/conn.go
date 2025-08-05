// Copyright (c) 2025 cocowh. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package tcp

import (
	"bufio"
	"context"
	"net"
	"sync"
	"time"

	"github.com/cocowh/muxcore/core/common"
	"github.com/cocowh/muxcore/core/iface"
	"github.com/cocowh/muxcore/core/utils"
	"github.com/cocowh/muxcore/pkg/logger"
	"github.com/google/uuid"
)

type conn struct {
	id           string
	rawConn      net.Conn
	ctx          context.Context
	cancel       context.CancelFunc
	mu           sync.Mutex
	closed       bool
	readTimeout  time.Duration
	bufReader    *bufio.Reader
	bufWriter    *bufio.Writer
	eventHandler iface.EventHandler
	ioHandler    iface.IOHandler
	once         sync.Once
}

func NewConnectionByAddr(addr net.Addr) (iface.Connection, error) {
	c, err := net.Dial(addr.Network(), addr.String())
	if err != nil {
		return nil, err
	}
	return NewConnection(c), nil
}

func NewConnection(c net.Conn) iface.Connection {
	ctx, cancel := context.WithCancel(context.Background())
	return &conn{
		id:          uuid.NewString(),
		rawConn:     c,
		ctx:         ctx,
		cancel:      cancel,
		readTimeout: time.Second * 10, //todo:: from config
		bufReader:   bufio.NewReader(c),
		bufWriter:   bufio.NewWriter(c),
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
	if c.ioHandler != nil {
		return c.ioHandler.Read(c.bufReader, b)
	}
	return c.bufReader.Read(b)
}

func (c *conn) Write(b []byte) (int, error) {
	if c.ioHandler != nil {
		return c.ioHandler.Write(c.bufWriter, b)
	}
	return c.bufWriter.Write(b)
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

	if c.eventHandler != nil {
		c.eventHandler.OnClose(c)
	}
	return c.rawConn.Close()
}

func (c *conn) closeOnError(err error) {
	if err != nil {
		if err.Error() != "EOF" {
			logger.Warnf("conn will close. id: %s,addr:%s, err: %s\n", c.id, c.rawConn.RemoteAddr(), err.Error())
		}
		c.notifyError(err)
	}
	c.Close()
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
		var data any
		var err error
		if c.ioHandler != nil {
			_, data, err = c.ioHandler.ReadData(c.bufReader)
			if err != nil {
				c.notifyError(err)
				return err
			}
		} else {
			data = make([]byte, 1024)
			_, err = c.bufReader.Read(data.([]byte))
		}
		if err != nil {
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
