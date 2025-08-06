// Copyright (c) 2025 cocowh. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package udp

import (
	"context"
	"net"
	"runtime/debug"
	"sync"
	"time"

	"github.com/cocowh/muxcore/core/common"
	"github.com/cocowh/muxcore/core/iface"
	"github.com/cocowh/muxcore/core/utils"
	"github.com/cocowh/muxcore/pkg/logger"
)

type Client struct {
	mu           sync.Mutex
	conn         *net.UDPConn
	remoteAddr   *net.UDPAddr
	localAddr    *net.UDPAddr
	eventHandler iface.EventHandler
	ioHandler    iface.IOHandler
	ctx          context.Context
	cancel       context.CancelFunc
	opts         *ClientOptions
}

type ClientOptions struct {
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
}

func NewClientOptions() *ClientOptions {
	return &ClientOptions{
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 5 * time.Second,
	}
}

func NewClient(network, localAddr, remoteAddr string, opts *ClientOptions) (*Client, error) {
	if opts == nil {
		opts = NewClientOptions()
	}

	// 解析远程地址
	udpRemoteAddr, err := net.ResolveUDPAddr(network, remoteAddr)
	if err != nil {
		return nil, err
	}

	// 解析本地地址
	udpLocalAddr, err := net.ResolveUDPAddr(network, localAddr)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &Client{
		remoteAddr: udpRemoteAddr,
		localAddr:  udpLocalAddr,
		opts:       opts,
		ctx:        ctx,
		cancel:     cancel,
	}, nil
}

func (c *Client) Connect() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conn != nil {
		return nil
	}

	// 创建UDP连接
	conn, err := net.ListenUDP("udp", c.localAddr)
	if err != nil {
		return err
	}

	c.conn = conn

	// 启动接收goroutine
	go c.recvLoop()

	logger.Infof("UDP client started on %s", c.localAddr.String())
	return nil
}

func (c *Client) Send(data []byte) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conn == nil {
		if err := c.Connect(); err != nil {
			return 0, err
		}
	}

	// 设置写超时
	if c.opts.WriteTimeout > 0 {
		c.conn.SetWriteDeadline(time.Now().Add(c.opts.WriteTimeout))
	}

	// 发送数据
	n, err := c.conn.WriteToUDP(data, c.remoteAddr)
	if err != nil {
		logger.Errorf("Failed to send data: %v", err)
		return n, err
	}

	return n, nil
}

func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.cancel()

	if c.conn != nil {
		c.conn.Close()
		c.conn = nil
	}

	logger.Infof("UDP client closed")
	return nil
}

func (c *Client) SetEventHandler(handler iface.EventHandler) {
	c.mu.Lock()
	c.eventHandler = handler
	c.mu.Unlock()
}

func (c *Client) SetIOHandler(handler iface.IOHandler) {
	c.mu.Lock()
	c.ioHandler = handler
	c.mu.Unlock()
}

func (c *Client) Context() context.Context {
	return c.ctx
}

func (c *Client) recvLoop() {
	defer utils.PanicHandler(func() {
		logger.Errorf("UDP client recv loop panic, stack: %s", string(debug.Stack()))
	})

	buffer := make([]byte, 65536) // 最大UDP包大小

	for {
		select {
		case <-c.ctx.Done():
			return
		default:
		}

		// 设置读超时
		if c.opts.ReadTimeout > 0 {
			c.conn.SetReadDeadline(time.Now().Add(c.opts.ReadTimeout))
		}

		// 接收数据
		n, remoteAddr, err := c.conn.ReadFromUDP(buffer)
		if err != nil {
			// 处理超时错误
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				continue
			}

			logger.Errorf("Failed to read data: %v", err)
			if !common.IsTemporaryError(err) {
				break
			}
			continue
		}

		if n > 0 {
			data := make([]byte, n)
			copy(data, buffer[:n])

			// 创建一个连接对象用于事件处理
			conn := NewConnection(c.conn, remoteAddr)
			conn.SetEventHandler(c.eventHandler)
			conn.SetIOHandler(c.ioHandler)

			// 触发消息事件
			if c.eventHandler != nil {
				c.eventHandler.OnMessage(conn, data)
			}
		}
	}
}
