// Copyright (c) 2025 cocowh. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package stream

import (
	"context"
	"net"
	"sync"
	"time"

	"github.com/cocowh/muxcore/core/common"
	"github.com/cocowh/muxcore/core/iface"
	"github.com/cocowh/muxcore/pkg/logger"
	"github.com/spf13/viper"
)

type Client struct {
	mu           sync.RWMutex
	conn         iface.Connection
	network      string
	addr         string
	header       map[string]string
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
	Reconnect      struct {
		Enable     bool
		MaxRetries int
		Delay      time.Duration
	}
}

func NewClientOptions() *ClientOptions {
	opts := &ClientOptions{
		ConnectTimeout: viper.GetDuration("stream.client.connect_timeout"),
		ReadTimeout:    viper.GetDuration("stream.client.read_timeout"),
		WriteTimeout:   viper.GetDuration("stream.client.write_timeout"),
	}

	// 单独设置嵌套结构体字段
	opts.Reconnect.Enable = viper.GetBool("stream.client.reconnect.enable")
	opts.Reconnect.MaxRetries = viper.GetInt("stream.client.reconnect.max_retries")
	opts.Reconnect.Delay = viper.GetDuration("stream.client.reconnect.delay")

	return opts
}

func NewClient(network, addr string, opts *ClientOptions) iface.Client {
	if opts == nil {
		opts = NewClientOptions()
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &Client{
		network: network,
		addr:    addr,
		header:  make(map[string]string),
		opts:    opts,
		ctx:     ctx,
		cancel:  cancel,
	}
}

func (c *Client) SetHeader(key, value string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.header[key] = value
}

func (c *Client) Connect() error {
	c.mu.RLock()
	network := c.network
	addr := c.addr
	opts := c.opts
	c.mu.RUnlock()

	var conn net.Conn
	var err error

	// 设置连接超时
	ctx, cancel := context.WithTimeout(context.Background(), opts.ConnectTimeout)
	defer cancel()

	// 使用 net.DialContext 建立连接
	conn, err = net.DialContext(ctx, network, addr)
	if err != nil {
		logger.Errorf("Failed to connect to %s: %v", addr, err)
		return common.ErrConnectionClosed
	}

	// 设置读写超时
	conn.SetReadDeadline(time.Now().Add(opts.ReadTimeout))
	conn.SetWriteDeadline(time.Now().Add(opts.WriteTimeout))

	// 创建连接对象
	streamConn := NewConnection(conn)
	streamConn.SetEventHandler(c.eventHandler)
	streamConn.SetIOHandler(c.ioHandler)

	c.mu.Lock()
	c.conn = streamConn
	c.mu.Unlock()

	logger.Infof("Stream client connected to %s", addr)
	return nil
}

func (c *Client) Send(data []byte) (int, error) {
	c.mu.RLock()
	conn := c.conn
	c.mu.RUnlock()

	if conn == nil {
		return 0, common.ErrClientClosed
	}

	return conn.Write(data)
}

func (c *Client) Close() error {
	c.cancel()

	c.mu.RLock()
	conn := c.conn
	c.mu.RUnlock()

	if conn != nil {
		return conn.Close()
	}

	return nil
}

func (c *Client) SetEventHandler(eventHandler iface.EventHandler) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.eventHandler = eventHandler

	if c.conn != nil {
		c.conn.SetEventHandler(eventHandler)
	}
}

func (c *Client) SetIOHandler(ioHandler iface.IOHandler) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.ioHandler = ioHandler

	if c.conn != nil {
		c.conn.SetIOHandler(ioHandler)
	}
}

func (c *Client) Context() context.Context {
	return c.ctx
}
