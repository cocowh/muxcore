// Copyright (c) 2025 cocowh. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package network

import (
	"io"
	"net"
	"sync"
	"time"

	"github.com/cocowh/muxcore/pkg/buffer"
	"github.com/cocowh/muxcore/pkg/errors"
)

// BufferedConn net.Conn with buffer
type BufferedConn struct {
	net.Conn
	buffer      *buffer.BytesBuffer
	mutex       sync.Mutex
	readTimeout time.Duration
	closed      bool
}

// NewBufferedConn 创建带缓冲的连接
func NewBufferedConn(conn net.Conn, bufferSize int) *BufferedConn {
	if bufferSize <= 0 {
		bufferSize = 4096
	}
	return &BufferedConn{
		Conn:   conn,
		buffer: buffer.NewBytesBuffer(bufferSize),
	}
}

// Read 读取数据
func (bc *BufferedConn) Read(b []byte) (int, error) {
	bc.mutex.Lock()
	defer bc.mutex.Unlock()

	if bc.closed {
		return 0, io.EOF
	}

	// 先从缓冲区读取数据
	if bc.buffer.Remain() > 0 {
		n, err := bc.buffer.Read(b)
		if n > 0 {
			return n, err
		}
	}

	// 缓冲区为空，直接从连接读取
	if len(b) >= bc.buffer.Cap() {
		// 如果目标缓冲区足够大，直接读取
		return bc.Conn.Read(b)
	}

	// 否则，填充内部缓冲区
	bc.buffer.Reset()
	tempBuf := make([]byte, bc.buffer.Cap())
	n, err := bc.Conn.Read(tempBuf)
	if err != nil {
		return 0, err
	}

	// 将数据写入缓冲区
	bc.buffer.Write(tempBuf[:n])

	// 从缓冲区读取到目标缓冲区
	return bc.buffer.Read(b)
}

// Peek 预读数据但不从流中移除
func (bc *BufferedConn) Peek(n int) ([]byte, error) {
	bc.mutex.Lock()
	defer bc.mutex.Unlock()

	if bc.closed {
		return nil, io.EOF
	}

	// 确保缓冲区有足够数据
	for bc.buffer.Remain() < n {
		// 需要读取更多数据
		tempBuf := make([]byte, bc.buffer.Cap())

		// 设置读超时
		if bc.readTimeout > 0 {
			bc.Conn.SetReadDeadline(time.Now().Add(bc.readTimeout))
		}

		nRead, err := bc.Conn.Read(tempBuf)
		if err != nil {
			if bc.buffer.Remain() >= n {
				break // 有足够数据可以返回
			}
			return nil, err
		}

		// 将新数据追加到缓冲区
		bc.buffer.Write(tempBuf[:nRead])
	}

	// 返回请求的数据但不移除
	if bc.buffer.Remain() < n {
		return bc.buffer.Bytes()[bc.buffer.GetRPos():], nil
	}

	startPos := bc.buffer.GetRPos()
	return bc.buffer.Bytes()[startPos : startPos+n], nil
}

// SetReadTimeout 设置读取超时
func (bc *BufferedConn) SetReadTimeout(timeout time.Duration) {
	bc.mutex.Lock()
	defer bc.mutex.Unlock()
	bc.readTimeout = timeout
}

// Close 关闭连接
func (bc *BufferedConn) Close() error {
	bc.mutex.Lock()
	defer bc.mutex.Unlock()

	if bc.closed {
		return nil
	}

	bc.closed = true

	// 归还缓冲区到池中
	if bc.buffer != nil {
		buffer.ReleaseBytesBuffer(bc.buffer)
		bc.buffer = nil
	}

	return bc.Conn.Close()
}

// Write 写入数据
func (bc *BufferedConn) Write(b []byte) (int, error) {
	bc.mutex.Lock()
	defer bc.mutex.Unlock()

	if bc.closed {
		return 0, errors.NetworkError(errors.ErrCodeNetworkConnectionLost, "connection closed")
	}

	return bc.Conn.Write(b)
}

// Available 返回缓冲区中可读的字节数
func (bc *BufferedConn) Available() int {
	bc.mutex.Lock()
	defer bc.mutex.Unlock()
	return bc.buffer.Remain()
}

// Reset 重置缓冲区
func (bc *BufferedConn) Reset() {
	bc.mutex.Lock()
	defer bc.mutex.Unlock()
	bc.buffer.Reset()
}
