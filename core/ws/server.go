// Copyright (c) 2025 cocowh. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package ws

import (
	"context"
	"crypto/tls"
	"net/http"
	"runtime/debug"
	"sync"
	"time"

	"github.com/cocowh/muxcore/core/common"
	"github.com/cocowh/muxcore/core/iface"
	"github.com/cocowh/muxcore/core/utils"
	"github.com/cocowh/muxcore/pkg/logger"
	"github.com/spf13/viper"
	"golang.org/x/net/websocket"
)

type Server struct {
	mu           sync.RWMutex
	httpServer   *http.Server
	network      string
	addr         string
	path         string
	eventHandler iface.EventHandler
	ioHandler    iface.IOHandler
	ctx          context.Context
	cancel       context.CancelFunc
	opts         *ServerOptions
	conns        map[string]iface.Connection
}

type ServerOptions struct {
	ReadTimeout    time.Duration
	WriteTimeout   time.Duration
	MaxConnections int
	TLSConfig      any // *tls.Config
}

func NewServerOptions() *ServerOptions {
	return &ServerOptions{
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   5 * time.Second,
		MaxConnections: 1000,
	}
}

func NewServer(network, addr, path string, opts *ServerOptions) iface.Server {
	if opts == nil {
		opts = NewServerOptions()
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &Server{
		network: network,
		addr:    addr,
		path:    path,
		opts:    opts,
		ctx:     ctx,
		cancel:  cancel,
		conns:   make(map[string]iface.Connection),
	}
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
	// 创建WebSocket处理器
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != s.path {
			http.NotFound(w, r)
			return
		}
		// 升级HTTP连接到WebSocket
		websocket.Handler(s.handleWebSocket).ServeHTTP(w, r)
	})

	s.httpServer = &http.Server{
		Addr:    s.addr,
		Handler: handler,
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
			logger.Errorf("WebSocket server error: %v", err)
		}
	}()

	// 启动心跳检查
	go s.heartbeatCheck()

	logger.Infof("WebSocket server started on %s%s", s.addr, s.path)
	return nil
}

func (s *Server) handleWebSocket(wsConn *websocket.Conn) {
	defer utils.PanicHandler(func() {
		logger.Errorf("WebSocket handler panic, stack: %s", string(debug.Stack()))
	})

	// check max connections
	s.mu.RLock()
	if len(s.conns) >= s.opts.MaxConnections {
		s.mu.RUnlock()
		wsConn.Close()
		logger.Warnf("Too many WebSocket connections, rejecting new connection")
		return
	}
	s.mu.RUnlock()

	// create connection
	conn := NewConnection(wsConn)
	conn.SetEventHandler(s.eventHandler)
	conn.SetIOHandler(s.ioHandler)

	// add connection to map
	s.mu.Lock()
	s.conns[conn.ID()] = conn
	s.mu.Unlock()

	logger.Infof("New WebSocket connection established from %s", wsConn.RemoteAddr().String())

	// wait for connection close
	<-conn.Context().Done()

	// remove connection from map
	s.RemoveConnection(conn.ID())
}

func (s *Server) Stop() error {
	s.cancel()

	if s.httpServer != nil {
		// stop http server
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		s.httpServer.Shutdown(ctx)
	}

	s.mu.Lock()
	for _, conn := range s.conns {
		conn.Close()
	}
	s.conns = make(map[string]iface.Connection)
	s.mu.Unlock()

	logger.Infof("WebSocket server stopped")
	return nil
}

func (s *Server) GetConnection(id string) (iface.Connection, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	conn, exists := s.conns[id]
	return conn, exists
}

func (s *Server) RemoveConnection(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if conn, exists := s.conns[id]; exists {
		conn.Close()
		delete(s.conns, id)
		logger.Infof("WebSocket connection removed and closed, id: %s", id)
	} else {
		logger.Warnf("Attempted to remove non-existent WebSocket connection, id: %s", id)
	}
}

func (s *Server) heartbeatCheck() {
	// 从配置中读取心跳间隔，默认为30秒
	heartbeatInterval := viper.GetDuration("ws.heartbeat_interval")
	if heartbeatInterval <= 0 {
		heartbeatInterval = 30 * time.Second
	}

	ticker := time.NewTicker(heartbeatInterval)
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			s.mu.RLock()
			for _, conn := range s.conns {
				go func(c iface.Connection) {
					// 发送心跳
					if err := s.sendHeartbeat(c); err != nil {
						// 判断是否为临时错误
						if common.IsTemporaryError(err) {
							logger.Warnf("Temporary error sending heartbeat: %v", err)
							return
						}
						logger.Warnf("Failed to send heartbeat, closing connection: %v", err)
						s.RemoveConnection(c.ID())
					}
				}(conn)
			}
			s.mu.RUnlock()
		}
	}
}

func (s *Server) sendHeartbeat(conn iface.Connection) error {
	// 发送心跳帧
	heartbeat := []byte("ping")
	_, err := conn.Write(heartbeat)
	return err
}
