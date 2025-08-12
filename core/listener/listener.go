// Copyright (c) 2025 cocowh. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package listener

import (
	"context"
	"net"
	"sync"

	"github.com/cocowh/muxcore/core/detector"
	coreNet "github.com/cocowh/muxcore/core/network"
	"github.com/cocowh/muxcore/core/performance"
	"github.com/cocowh/muxcore/core/pool"
	"github.com/cocowh/muxcore/pkg/errors"
	"github.com/cocowh/muxcore/pkg/logger"
)

// Listener listens for incoming network connections and manages them using a connection pool.
type Listener struct {
	addr          string
	listener      net.Listener
	detector      *detector.ProtocolDetector
	pool          *pool.ConnectionPool
	goroutinePool *pool.GoroutinePool
	bufferPool    *performance.BufferPool
	wg            sync.WaitGroup
	ctx           context.Context
	cancel        context.CancelFunc
}

// New creates a new Listener instance.
func New(addr string, detector *detector.ProtocolDetector, pool *pool.ConnectionPool, goroutinePool *pool.GoroutinePool, bufferPool *performance.BufferPool) *Listener {
	ctx, cancel := context.WithCancel(context.Background())
	return &Listener{
		addr:          addr,
		detector:      detector,
		pool:          pool,
		goroutinePool: goroutinePool,
		bufferPool:    bufferPool,
		ctx:           ctx,
		cancel:        cancel,
	}
}

// Start starts the listener.
func (l *Listener) Start() error {
	var err error
	l.listener, err = net.Listen("tcp", l.addr)
	if err != nil {
		muxErr := errors.NetworkError(errors.ErrCodeNetworkRefused, "failed to start listener").WithCause(err).WithContext("address", l.addr)
		errors.Handle(l.ctx, muxErr)
		return muxErr
	}

	logger.Info("Started listening on ", l.addr)

	l.wg.Add(1)
	go l.acceptLoop()

	return nil
}

// Stop stops the listener.
func (l *Listener) Stop() {
	l.cancel()
	if l.listener != nil {
		l.listener.Close()
	}
	l.wg.Wait()
	logger.Info("Stopped listening")
}

// acceptLoop accepts connections and adds them to the connection pool
func (l *Listener) acceptLoop() {
	defer l.wg.Done()

	for {
		select {
		case <-l.ctx.Done():
			return
		default:
			conn, err := l.listener.Accept()
			if err != nil {
				select {
				case <-l.ctx.Done():
					return
				default:
					muxErr := errors.Convert(err).WithContext("operation", "accept_connection")
					errors.Handle(l.ctx, muxErr)
					continue
				}
			}

			// 获取缓冲区大小
			bufferSize := 4096 // 默认缓冲区大小
			if l.bufferPool != nil {
				// 从缓冲区池获取一个缓冲区以确定大小
				buffer := l.bufferPool.Get()
				bufferSize = len(buffer.Bytes())
				l.bufferPool.Put(buffer)
			}

			// 创建带缓冲的连接
			bufConn := coreNet.NewBufferedConn(conn, bufferSize)

			// 将连接放入池中
			connID := l.pool.AddConnection(bufConn)
			logger.Debug("Accepted new connection: ", connID)

			// 使用goroutine池处理协议检测
			connIDCopy := connID
			bufConnCopy := bufConn
			l.goroutinePool.Submit(func() {
				l.detector.DetectProtocol(connIDCopy, bufConnCopy)
			})
		}
	}
}
