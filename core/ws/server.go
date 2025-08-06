// Copyright (c) 2025 cocowh. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package ws

import (
	"context"
	"net/http"
	"runtime/debug"
	"sync"
	"time"

	"github.com/cocowh/muxcore/core/iface"
	"github.com/cocowh/muxcore/core/utils"
	"github.com/cocowh/muxcore/pkg/logger"
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
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != s.path {
			http.NotFound(w, r)
			return
		}
		// upgrade http to websocket
		websocket.Handler(s.handleWebSocket).ServeHTTP(w, r)
	})

	s.httpServer = &http.Server{
		Addr:    s.addr,
		Handler: handler,
	}

	// start http server
	go func() {
		var err error
		if s.opts.TLSConfig != nil {
			// start https server
			// tlsConfig := s.opts.TLSConfig.(*tls.Config)
			// err = s.httpServer.ListenAndServeTLS("", "")
		} else {
			// 启动HTTP服务器
			err = s.httpServer.ListenAndServe()
		}

		if err != nil && err != http.ErrServerClosed {
			logger.Errorf("WebSocket server error: %v", err)
		}
	}()

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
