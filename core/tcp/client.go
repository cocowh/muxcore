// Copyright (c) 2025 cocowh. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package tcp

import (
	"context"
	"net"
	"sync"
	"time"

	"github.com/cocowh/muxcore/core/constant"
	"github.com/cocowh/muxcore/core/iface"
	"github.com/cocowh/muxcore/pkg/logger"
)

type Client struct {
	mu           sync.Mutex
	conn         iface.Connection
	addr         string
	network      string
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

func NewClient(network, addr string, opts *ClientOptions) *Client {
	if opts == nil {
		opts = NewClientOptions()
	}
	ctx, cancel := context.WithCancel(context.Background())
	return &Client{
		network: network,
		addr:    addr,
		opts:    opts,
		ctx:     ctx,
		cancel:  cancel,
	}
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

func (c *Client) Connect() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.conn != nil && !c.isConnClosed() {
		return nil
	}
	return c.connect()
}

func (c *Client) connect() error {
	var conn net.Conn
	var err error
	var retries int

	for {
		conn, err = net.DialTimeout(c.network, c.addr, c.opts.ConnectTimeout)
		if err == nil {
			break
		}
		logger.Errorf("Failed to connect to %s: %v", c.addr, err)

		if !c.opts.Reconnect || (c.opts.MaxRetries > 0 && retries >= c.opts.MaxRetries) {
			return err
		}

		retries++
		select {
		case <-time.After(c.opts.ReconnectDelay):
		case <-c.ctx.Done():
			return constant.ErrClientClosed
		}
	}

	c.conn = NewConnection(conn)
	c.conn.SetEventHandler(c.eventHandler)
	c.conn.SetIOHandler(c.ioHandler)

	conn.SetReadDeadline(time.Now().Add(c.opts.ReadTimeout))
	conn.SetWriteDeadline(time.Now().Add(c.opts.WriteTimeout))

	logger.Infof("Connected to %s successfully", c.addr)
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
	return nil
}

func (c *Client) isConnClosed() bool {
	// todo
	return c.conn == nil
}
