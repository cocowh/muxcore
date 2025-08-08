// Copyright (c) 2025 cocowh. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package ws

import (
	"context"
	"net/http"
	"sync"
	"time"

	"github.com/cocowh/muxcore/core/common"
	"github.com/cocowh/muxcore/core/iface"
	"github.com/cocowh/muxcore/pkg/logger"
	"golang.org/x/net/websocket"
)

type Client struct {
	mu           sync.Mutex
	conn         iface.Connection
	url          string
	header       http.Header
	eventHandler iface.EventHandler
	ioHandler    iface.IOHandler
	ctx          context.Context
	cancel       context.CancelFunc
	opts         *ClientOptions
}

type ClientOptions struct {
	ConnectTimeout time.Duration
	ReadTimeout    time.Duration
	WriteTimeout   time.Duration
	Reconnect      bool
	ReconnectDelay time.Duration
	MaxRetries     int
}

func NewClientOptions() *ClientOptions {
	return &ClientOptions{
		ConnectTimeout: 5 * time.Second,
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   5 * time.Second,
		Reconnect:      true,
		ReconnectDelay: 2 * time.Second,
		MaxRetries:     3,
	}
}

func NewClient(url string, opts *ClientOptions) *Client {
	if opts == nil {
		opts = NewClientOptions()
	}

	ctx, cancel := context.WithCancel(context.Background())

	c := &Client{
		url:    url,
		header: make(http.Header),
		opts:   opts,
		ctx:    ctx,
		cancel: cancel,
	}

	// 启动客户端心跳检查
	go c.heartbeatCheck()

	return c
}

func (c *Client) SetHeader(key, value string) {
	c.mu.Lock()
	c.header.Set(key, value)
	c.mu.Unlock()
}

func (c *Client) Connect() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conn != nil && !c.isConnClosed() {
		return nil
	}

	return c.connect()
}

func (c *Client) connect() error {
	var wsConn *websocket.Conn
	var err error
	var retries int

	config, err := websocket.NewConfig(c.url, "http://localhost/")
	if err != nil {
		return err
	}

	config.Header = c.header

	for {
		// 设置连接超时
		ctx, cancel := context.WithTimeout(context.Background(), c.opts.ConnectTimeout)
		defer cancel()

		// 创建WebSocket连接
		wsConn, err = config.DialContext(ctx)
		if err == nil {
			break
		}

		logger.Errorf("Failed to connect to %s: %v", c.url, err)

		if !c.opts.Reconnect || (c.opts.MaxRetries > 0 && retries >= c.opts.MaxRetries) {
			return err
		}

		retries++
		select {
		case <-time.After(c.opts.ReconnectDelay):
		case <-c.ctx.Done():
			return common.ErrClientClosed
		}
	}

	c.conn = NewConnection(wsConn)
	c.conn.SetEventHandler(c.eventHandler)
	c.conn.SetIOHandler(c.ioHandler)

	logger.Infof("WebSocket client connected to %s successfully", c.url)
	return nil
}

func (c *Client) Send(data []byte) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conn == nil || c.isConnClosed() {
		if err := c.connect(); err != nil {
			return 0, err
		}
	}

	return c.conn.Write(data)
}

func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.cancel()

	if c.conn != nil {
		c.conn.Close()
		c.conn = nil
	}

	logger.Infof("WebSocket client closed")
	return nil
}

func (c *Client) SetEventHandler(handler iface.EventHandler) {
	c.mu.Lock()
	c.eventHandler = handler
	if c.conn != nil {
		c.conn.SetEventHandler(handler)
	}
	c.mu.Unlock()
}

func (c *Client) SetIOHandler(handler iface.IOHandler) {
	c.mu.Lock()
	c.ioHandler = handler
	if c.conn != nil {
		c.conn.SetIOHandler(handler)
	}
	c.mu.Unlock()
}

func (c *Client) Context() context.Context {
	return c.ctx
}

func (c *Client) isConnClosed() bool {
	if c.conn == nil {
		return true
	}

	// 检查连接上下文是否已完成
	select {
	case <-c.conn.Context().Done():
		return true
	default:
		return false
	}
}

// 添加客户端心跳检查
func (c *Client) heartbeatCheck() {
	// 每30秒发送一次心跳
	heartbeatInterval := 30 * time.Second
	ticker := time.NewTicker(heartbeatInterval)
	defer ticker.Stop()

	for {
		select {
		case <-c.ctx.Done():
			return
		case <-ticker.C:
			c.mu.Lock()
			if c.conn != nil && !c.isConnClosed() {
				// 发送心跳
				go func(conn iface.Connection) {
					if _, err := conn.Write([]byte("ping")); err != nil {
						logger.Warnf("Failed to send client heartbeat: %v", err)
						// 不是临时错误则关闭连接
						if !common.IsTemporaryError(err) {
							c.mu.Lock()
							conn.Close()
							c.conn = nil
							c.mu.Unlock()
						}
					}
				}(c.conn)
			}
			c.mu.Unlock()
		}
	}
}
