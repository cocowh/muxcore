// Copyright (c) 2025 cocowh. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package httpx

import (
	"context"
	"crypto/tls"
	"net/http"
	"sync"
	"time"

	"github.com/cocowh/muxcore/core/common"
	"github.com/cocowh/muxcore/core/iface"
	"github.com/cocowh/muxcore/pkg/logger"
	"github.com/spf13/viper"
)

type Server struct {
	mu           sync.RWMutex
	httpServer   *http.Server
	addr         string
	eventHandler iface.EventHandler
	ioHandler    iface.IOHandler
	ctx          context.Context
	cancel       context.CancelFunc
	opts         *ServerOptions
	handlers     map[string]http.HandlerFunc
}

type ServerOptions struct {
	ReadTimeout    time.Duration
	WriteTimeout   time.Duration
	MaxConnections int
	TLSConfig      any // *tls.Config
}

func NewServerOptions() *ServerOptions {
	return &ServerOptions{
		ReadTimeout:    viper.GetDuration("httpx.read_timeout"),
		WriteTimeout:   viper.GetDuration("httpx.write_timeout"),
		MaxConnections: viper.GetInt("httpx.max_connections"),
	}
}

func NewServer(addr string, opts *ServerOptions) iface.Server {
	if opts == nil {
		opts = NewServerOptions()
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &Server{
		addr:     addr,
		handlers: make(map[string]http.HandlerFunc),
		opts:     opts,
		ctx:      ctx,
		cancel:   cancel,
	}
}

func (s *Server) AddRoute(path string, handler http.HandlerFunc) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.handlers[path] = handler
}

func (s *Server) SetEventHandler(handler iface.EventHandler) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.eventHandler = handler
}

func (s *Server) SetIOHandler(handler iface.IOHandler) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ioHandler = handler
}

func (s *Server) Start() error {
	// 创建HTTP处理器
	mux := http.NewServeMux()

	// 注册路由处理器
	s.mu.RLock()
	for path, handler := range s.handlers {
		mux.HandleFunc(path, handler)
	}
	s.mu.RUnlock()

	s.httpServer = &http.Server{
		Addr:    s.addr,
		Handler: mux,
	}

	// 启动服务器
	go func() {
		var err error
		if s.opts.TLSConfig != nil {
			// 启动HTTPS服务器
			if tlsConfig, ok := s.opts.TLSConfig.(*tls.Config); ok {
				s.httpServer.TLSConfig = tlsConfig
				err = s.httpServer.ListenAndServeTLS("", "")
			} else {
				err = common.ErrInvalidTLSConfig
				logger.Errorf("Invalid TLS config: %v", err)
			}
		} else {
			// 启动HTTP服务器
			err = s.httpServer.ListenAndServe()
		}

		if err != nil && err != http.ErrServerClosed {
			logger.Errorf("HTTP server error: %v", err)
		}
	}()

	logger.Infof("HTTP server started on %s", s.addr)
	return nil
}

func (s *Server) Stop() error {
	s.cancel()

	if s.httpServer != nil {
		// stop http server
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		s.httpServer.Shutdown(ctx)
	}

	logger.Infof("HTTP server stopped")
	return nil
}

func (s *Server) GetConnection(id string) (iface.Connection, bool) {
	// HTTP服务器不直接管理连接，返回nil
	return nil, false
}

func (s *Server) RemoveConnection(id string) {
	// HTTP服务器不直接管理连接，空实现
}
