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
	"github.com/spf13/viper"
)

type Server struct {
	mu           sync.RWMutex
	conn         *net.UDPConn
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
}

func NewServerOptions() *ServerOptions {
	return &ServerOptions{
		ReadTimeout:    viper.GetDuration("udp.read_timeout"),
		WriteTimeout:   viper.GetDuration("udp.write_timeout"),
		MaxConnections: 1000,
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
	// 解析地址
	udpAddr, err := net.ResolveUDPAddr(s.network, s.addr)
	if err != nil {
		return err
	}

	// 创建UDP连接
	conn, err := net.ListenUDP(s.network, udpAddr)
	if err != nil {
		return err
	}

	s.conn = conn

	logger.Infof("UDP server started on %s", s.addr)

	// 启动接收goroutine
	go s.recvLoop()

	return nil
}

func (s *Server) Stop() error {
	s.cancel()

	if s.conn != nil {
		s.conn.Close()
		s.conn = nil
	}

	s.mu.Lock()
	// 关闭所有连接
	for _, conn := range s.conns {
		conn.Close()
	}
	s.conns = make(map[string]iface.Connection)
	s.mu.Unlock()

	logger.Infof("UDP server stopped")
	return nil
}

func (s *Server) recvLoop() {
	defer utils.PanicHandler(func() {
		logger.Errorf("UDP server recv loop panic, stack: %s", string(debug.Stack()))
		s.Stop()
	})

	buffer := make([]byte, 65536) // 最大UDP包大小

	for {
		select {
		case <-s.ctx.Done():
			return
		default:
		}

		// 设置读超时
		if s.opts.ReadTimeout > 0 {
			s.conn.SetReadDeadline(time.Now().Add(s.opts.ReadTimeout))
		}

		// 接收数据
		n, remoteAddr, err := s.conn.ReadFromUDP(buffer)
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

			// 检查连接数限制
			s.mu.RLock()
			if len(s.conns) >= s.opts.MaxConnections {
				s.mu.RUnlock()
				logger.Warnf("Too many connections, dropping packet from %s", remoteAddr.String())
				continue
			}
			s.mu.RUnlock()

			// 创建或获取连接对象
			connID := remoteAddr.String()
			conn, exists := s.GetConnection(connID)

			if !exists {
				// 创建新连接
				conn = NewConnection(s.conn, remoteAddr)
				conn.SetEventHandler(s.eventHandler)
				conn.SetIOHandler(s.ioHandler)

				s.mu.Lock()
				s.conns[connID] = conn
				s.mu.Unlock()

				logger.Infof("New UDP connection from %s", remoteAddr.String())
			}

			// 触发消息事件
			if s.eventHandler != nil {
				s.eventHandler.OnMessage(conn, data)
			}
		}
	}
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
		logger.Infof("UDP connection removed: %s", id)
	} else {
		logger.Warnf("Attempted to remove non-existent UDP connection: %s", id)
	}
}
