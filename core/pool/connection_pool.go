// Copyright (c) 2025 cocowh. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package pool

import (
	"net"
	"sync"

	"github.com/cocowh/muxcore/pkg/buffer"
	"github.com/cocowh/muxcore/pkg/logger"
	"github.com/google/uuid"
)

// Connection a connection
type Connection struct {
	ID       string
	Conn     net.Conn
	Buffer   *buffer.BytesBuffer
	Protocol string
	IsActive bool
}

// ConnectionPool connection pool
type ConnectionPool struct {
	connections map[string]*Connection
	mutex       sync.RWMutex
}

// New create a new connection pool
func New() *ConnectionPool {
	return &ConnectionPool{
		connections: make(map[string]*Connection),
	}
}

// AddConnection add connection to pool
func (p *ConnectionPool) AddConnection(conn net.Conn) string {
	id := uuid.New().String()
	connection := &Connection{
		ID:       id,
		Conn:     conn,
		Buffer:   buffer.NewBytesBuffer(1024),
		IsActive: true,
	}

	p.mutex.Lock()
	p.connections[id] = connection
	p.mutex.Unlock()

	logger.Debug("Added connection to pool: ", id)
	return id
}

// GetConnection get connection from pool
func (p *ConnectionPool) GetConnection(id string) (*Connection, bool) {
	p.mutex.RLock()
	defer p.mutex.RUnlock()

	conn, exists := p.connections[id]
	return conn, exists
}

// RemoveConnection remove connection from pool
func (p *ConnectionPool) RemoveConnection(id string) {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	if conn, exists := p.connections[id]; exists {
		err := conn.Conn.Close()
		if err != nil {
			logger.Error("Failed to close connection: ", id)
		}
		delete(p.connections, id)
		logger.Debug("Removed connection from pool: ", id)
	}
}

// UpdateConnectionProtocol update connection protocol
func (p *ConnectionPool) UpdateConnectionProtocol(id string, protocol string) bool {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	if conn, exists := p.connections[id]; exists {
		conn.Protocol = protocol
		logger.Debug("Updated connection protocol: ", id, " to ", protocol)
		return true
	}
	return false
}

// GetActiveConnections get active connections
func (p *ConnectionPool) GetActiveConnections() []*Connection {
	p.mutex.RLock()
	defer p.mutex.RUnlock()

	var activeConns []*Connection
	for _, conn := range p.connections {
		if conn.IsActive {
			activeConns = append(activeConns, conn)
		}
	}
	return activeConns
}

// GetActiveConnectionCount get active connection count
func (p *ConnectionPool) GetActiveConnectionCount(protocol string) int {
	p.mutex.RLock()
	defer p.mutex.RUnlock()

	count := 0
	for _, conn := range p.connections {
		if conn.IsActive && conn.Protocol == protocol {
			count++
		}
	}
	return count
}

// Close the connection pool
func (p *ConnectionPool) Close() {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	for _, conn := range p.connections {
		conn.Conn.Close()
	}
	p.connections = make(map[string]*Connection)
	logger.Info("Connection pool closed")
}
