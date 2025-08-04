// Copyright (c) 2025 cocowh. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package tcp

import (
	"context"
	"go/constant"
	"net"
	"runtime/debug"
	"sync"
	"time"

	"github.com/cocowh/muxcore/core/iface"
	"github.com/cocowh/muxcore/core/utils"
	"github.com/cocowh/muxcore/pkg/logger"
)

type Server struct {
	mu           sync.RWMutex
	listener     net.Listener
	network      string
	addr         string
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
	ReusePort      bool
	TLSConfig      any // *tls.Config
}

func NewServerOptions() *ServerOptions {
	return &ServerOptions{
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   5 * time.Second,
		MaxConnections: 1000,
		ReusePort:      false,
	}
}

func NewServer(network, addr string, opts *ServerOptions) iface.Server {
	if opts == nil {
		opts = NewServerOptions()
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &Server{
		network: network,
		addr:    addr,
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
	var err error

	s.listener, err = net.Listen(s.network, s.addr)
	if err != nil {
		return err
	}

	logger.Infof("TCP server started on %s", s.addr)

	go s.acceptLoop()

	return nil
}

func (s *Server) Stop() error {
	s.cancel()

	if s.listener != nil {
		s.listener.Close()
	}

	s.mu.Lock()
	for _, conn := range s.conns {
		conn.Close()
	}
	s.conns = make(map[string]iface.Connection)
	s.mu.Unlock()

	logger.Infof("TCP server stopped")
	return nil
}

func (s *Server) acceptLoop() {
	defer utils.PanicHandler(func() {
		logger.Errorf("TCP server accept loop panic, stack: %s", string(debug.Stack()))
		s.Stop()
	})

	for {
		select {
		case <-s.ctx.Done():
			return
		default:
		}

		conn, err := s.listener.Accept()
		if err != nil {
			if netErr, ok := err.(net.Error); ok {
				if netErr.Timeout() {
					logger.Warnf("Accept timeout error: %v", err)
					time.Sleep(100 * time.Millisecond)
					continue
				}
				if constant.IsTemporaryError(err) {
					logger.Warnf("Temporary accept error: %v", err)
					time.Sleep(100 * time.Millisecond)
					continue
				}
			}
			logger.Errorf("Failed to accept connection: %v", err)
			return
		}

		s.mu.RLock()
		if len(s.conns) >= s.opts.MaxConnections {
			s.mu.RUnlock()
			conn.Close()
			logger.Warnf("Too many connections, rejecting new connection")
			continue
		}
		s.mu.RUnlock()

		tcpConn := NewConnection(conn)
		tcpConn.SetEventHandler(s.eventHandler)
		tcpConn.SetIOHandler(s.ioHandler)

		conn.SetReadDeadline(time.Now().Add(s.opts.ReadTimeout))
		conn.SetWriteDeadline(time.Now().Add(s.opts.WriteTimeout))

		s.mu.Lock()
		s.conns[tcpConn.ID()] = tcpConn
		s.mu.Unlock()

		logger.Infof("New connection established from %s", conn.RemoteAddr().String())
	}
}

func (s *Server) GetConnection(id string) (iface.Connection, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	conn, exists := s.conns[id]
	return conn, exists
}

func (s *Server) RemoveConnection(id string) {
	conn, exists := s.GetConnection(id)
	if !exists {
		logger.Warnf("Attempted to remove non-existent connection, id: %s", id)
		return
	}

	s.mu.Lock()
	delete(s.conns, id)
	s.mu.Unlock()

	conn.Close()
	logger.Infof("Connection removed and closed, id: %s", id)
}
