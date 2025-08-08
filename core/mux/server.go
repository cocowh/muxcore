// Copyright (c) 2025 cocowh. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package mux

import (
	"context"
	"sync"
	"time"

	"github.com/cocowh/muxcore/core/iface"
	"github.com/cocowh/muxcore/pkg/logger"
	"github.com/spf13/viper"
)

type Server struct {
	mu           sync.RWMutex
	servers      map[string]iface.Server
	routers      map[string]string
	eventHandler iface.EventHandler
	ioHandler    iface.IOHandler
	ctx          context.Context
	cancel       context.CancelFunc
	opts         *ServerOptions
}

type ServerOptions struct {
	ReadTimeout    time.Duration
	WriteTimeout   time.Duration
	MaxConnections int
}

func NewServerOptions() *ServerOptions {
	return &ServerOptions{
		ReadTimeout:    viper.GetDuration("mux.read_timeout"),
		WriteTimeout:   viper.GetDuration("mux.write_timeout"),
		MaxConnections: viper.GetInt("mux.max_connections"),
	}
}

func NewServer(opts *ServerOptions) iface.Server {
	if opts == nil {
		opts = NewServerOptions()
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &Server{
		servers: make(map[string]iface.Server),
		routers: make(map[string]string),
		opts:    opts,
		ctx:     ctx,
		cancel:  cancel,
	}
}

func (s *Server) RegisterProtocol(protocol string, server iface.Server) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.servers[protocol] = server
}

func (s *Server) AddRoute(path string, protocol string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.routers[path] = protocol
}

func (s *Server) SetEventHandler(handler iface.EventHandler) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.eventHandler = handler

	// 为所有注册的服务器设置事件处理器
	for _, server := range s.servers {
		server.SetEventHandler(handler)
	}
}

func (s *Server) SetIOHandler(handler iface.IOHandler) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ioHandler = handler

	// 为所有注册的服务器设置IO处理器
	for _, server := range s.servers {
		server.SetIOHandler(handler)
	}
}

func (s *Server) Start() error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// 启动所有注册的服务器
	for protocol, server := range s.servers {
		if err := server.Start(); err != nil {
			logger.Errorf("Failed to start %s server: %v", protocol, err)
			return err
		}
	}

	logger.Infof("Mux server started with %d protocols", len(s.servers))
	return nil
}

func (s *Server) Stop() error {
	s.cancel()

	s.mu.RLock()
	defer s.mu.RUnlock()

	// 停止所有注册的服务器
	for protocol, server := range s.servers {
		if err := server.Stop(); err != nil {
			logger.Errorf("Failed to stop %s server: %v", protocol, err)
		}
	}

	logger.Infof("Mux server stopped")
	return nil
}

func (s *Server) GetConnection(id string) (iface.Connection, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// 在所有服务器中查找连接
	for _, server := range s.servers {
		if conn, exists := server.GetConnection(id); exists {
			return conn, true
		}
	}

	return nil, false
}

func (s *Server) RemoveConnection(id string) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// 在所有服务器中移除连接
	for _, server := range s.servers {
		server.RemoveConnection(id)
	}
}
