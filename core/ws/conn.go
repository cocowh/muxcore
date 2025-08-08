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
	"github.com/cocowh/muxcore/pkg/logger"
	"github.com/google/uuid"
	"github.com/spf13/viper"
	"golang.org/x/net/websocket"
)

type conn struct {
	id                string
	rawConn           *websocket.Conn
	ctx               context.Context
	cancel            context.CancelFunc
	mu                sync.Mutex
	closed            bool
	readTimeout       time.Duration
	writeTimeout      time.Duration
	bufReader         *bufio.Reader
	bufWriter         *bufio.Writer
	heartbeatInterval time.Duration
	lastActivity      time.Time
	eventHandler      iface.EventHandler
	ioHandler         iface.IOHandler
	once              sync.Once
}

func NewConnection(wsConn *websocket.Conn) iface.Connection {
	ctx, cancel := context.WithCancel(context.Background())
	c := &conn{
		id:                uuid.NewString(),
		rawConn:           wsConn,
		ctx:               ctx,
		cancel:            cancel,
		readTimeout:       viper.GetDuration("ws.read_timeout"),
		writeTimeout:      viper.GetDuration("ws.write_timeout"),
		bufReader:         bufio.NewReader(wsConn),
		bufWriter:         bufio.NewWriter(wsConn),
		heartbeatInterval: viper.GetDuration("ws.heartbeat_interval"),
		lastActivity:      time.Now(),
	}

	// 设置默认的心跳间隔
	if c.heartbeatInterval <= 0 {
		c.heartbeatInterval = 30 * time.Second
	}

	return c
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

		// 更新最后活动时间
		c.lastActivity = time.Now()

		// 处理心跳响应
		if bytesData, ok := data.([]byte); n > 0 && ok {
			if string(bytesData) == "ping" {
				// 回复pong
				if _, err = c.Write([]byte("pong")); err != nil {
					logger.Warnf("Failed to send pong: %v", err)
					c.notifyError(err)
				}
				continue
			} else if string(bytesData) == "pong" {
				// 收到pong，更新活动时间
				c.lastActivity = time.Now()
				continue
			}
		}

		if err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				// 检查是否超过心跳间隔
				if time.Since(c.lastActivity) > c.heartbeatInterval*2 {
					logger.Warnf("Heartbeat timeout for connection: %s", c.RemoteAddr().String())
					c.notifyError(common.ErrHeartbeatTimeout)
					return common.ErrHeartbeatTimeout
				}
				continue
			}
			// 判断是否为临时错误
			if common.IsTemporaryError(err) {
				logger.Warnf("Temporary error on connection: %v", err)
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
