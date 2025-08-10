package pool

import (
	"net"
	"sync"

	"github.com/cocowh/muxcore/pkg/buffer"
	"github.com/cocowh/muxcore/pkg/logger"
	"github.com/google/uuid"
)

// Connection 表示一个连接及其状态
type Connection struct {
	ID       string
	Conn     net.Conn
	Buffer   *buffer.BytesBuffer
	Protocol string
	IsActive bool
}

// ConnectionPool 连接池
type ConnectionPool struct {
	connections map[string]*Connection
	mutex       sync.RWMutex
}

// New 创建一个新的ConnectionPool
func New() *ConnectionPool {
	return &ConnectionPool{
		connections: make(map[string]*Connection),
	}
}

// AddConnection 添加一个新连接到池中
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

// GetConnection 从池中获取连接
func (p *ConnectionPool) GetConnection(id string) (*Connection, bool) {
	p.mutex.RLock()
	defer p.mutex.RUnlock()

	conn, exists := p.connections[id]
	return conn, exists
}

// RemoveConnection 从池中移除连接
func (p *ConnectionPool) RemoveConnection(id string) {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	if conn, exists := p.connections[id]; exists {
		conn.Conn.Close()
		delete(p.connections, id)
		logger.Debug("Removed connection from pool: ", id)
	}
}

// UpdateConnectionProtocol 更新连接的协议信息
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

// GetActiveConnections 获取所有活跃连接
func (p *ConnectionPool) GetActiveConnections() []*Connection {
	p.mutex.RLock()
	defer p.mutex.RUnlock()

	activeConns := []*Connection{}
	for _, conn := range p.connections {
		if conn.IsActive {
			activeConns = append(activeConns, conn)
		}
	}
	return activeConns
}

// GetActiveConnectionCount 获取特定协议的活跃连接数
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

// Close 关闭连接池
func (p *ConnectionPool) Close() {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	for _, conn := range p.connections {
		conn.Conn.Close()
	}
	p.connections = make(map[string]*Connection)
	logger.Info("Connection pool closed")
}
