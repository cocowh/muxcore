// Copyright (c) 2025 cocowh. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package httpx

import (
	"bytes"
	"context"
	"crypto/tls"
	"io/ioutil"
	"net/http"
	"sync"
	"time"

	"github.com/cocowh/muxcore/core/iface"
	"github.com/cocowh/muxcore/pkg/logger"
	"github.com/spf13/viper"
)

type Client struct {
	mu           sync.RWMutex
	client       *http.Client
	baseURL      string
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
	TLSConfig      any // *tls.Config
}

func NewClientOptions() *ClientOptions {
	return &ClientOptions{
		ConnectTimeout: viper.GetDuration("httpx.client.connect_timeout"),
		ReadTimeout:    viper.GetDuration("httpx.client.read_timeout"),
		WriteTimeout:   viper.GetDuration("httpx.client.write_timeout"),
	}
}

func NewClient(baseURL string, opts *ClientOptions) iface.Client {
	if opts == nil {
		opts = NewClientOptions()
	}

	// 创建HTTP客户端
	client := &http.Client{
		Timeout: opts.ConnectTimeout + opts.ReadTimeout + opts.WriteTimeout,
	}

	// 配置TLS
	if opts.TLSConfig != nil {
		if tlsConfig, ok := opts.TLSConfig.(*tls.Config); ok {
			client.Transport = &http.Transport{
				TLSClientConfig: tlsConfig,
			}
		} else {
			logger.Errorf("Invalid TLS config for HTTP client")
		}
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &Client{
		client:  client,
		baseURL: baseURL,
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
	// HTTP客户端不需要提前连接
	return nil
}

func (c *Client) Send(data []byte) (int, error) {
	c.mu.RLock()
	client := c.client
	baseURL := c.baseURL
	header := c.header
	c.mu.RUnlock()

	// 创建请求
	req, err := http.NewRequest("POST", baseURL, bytes.NewReader(data))
	if err != nil {
		return 0, err
	}

	// 设置请求头
	for key, value := range header {
		req.Header.Set(key, value)
	}

	// 发送请求
	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	// 读取响应体
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return 0, err
	}

	// 触发事件
	c.mu.RLock()
	handler := c.eventHandler
	c.mu.RUnlock()

	if handler != nil {
		// 这里简化处理，实际应创建一个HTTP连接对象
		handler.OnMessage(nil, body)
	}

	return len(data), nil
}

func (c *Client) Close() error {
	c.cancel()
	return nil
}

func (c *Client) SetEventHandler(eventHandler iface.EventHandler) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.eventHandler = eventHandler
}

func (c *Client) SetIOHandler(ioHandler iface.IOHandler) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.ioHandler = ioHandler
}

func (c *Client) Context() context.Context {
	return c.ctx
}
