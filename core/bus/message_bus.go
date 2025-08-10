package bus

import (
	"fmt"
	"sync"

	"github.com/cocowh/muxcore/pkg/logger"
	"golang.org/x/net/websocket"
)

// MessageBus WebSocket消息总线
type MessageBus struct {
	connections map[string]*websocket.Conn
	mutex       sync.RWMutex
}

// NewMessageBus 创建消息总线
func NewMessageBus() *MessageBus {
	return &MessageBus{
		connections: make(map[string]*websocket.Conn),
	}
}

// RegisterConnection 注册WebSocket连接
func (b *MessageBus) RegisterConnection(connID string, conn *websocket.Conn) {
	b.mutex.Lock()
	defer b.mutex.Unlock()

	b.connections[connID] = conn
	logger.Info("Registered WebSocket connection: ", connID)
}

// UnregisterConnection 注销WebSocket连接
func (b *MessageBus) UnregisterConnection(connID string) {
	b.mutex.Lock()
	defer b.mutex.Unlock()

	if _, exists := b.connections[connID]; exists {
		delete(b.connections, connID)
		logger.Info("Unregistered WebSocket connection: ", connID)
	}
}

// BroadcastMessage 广播消息到所有连接
func (b *MessageBus) BroadcastMessage(excludeConnID string, message string) {
	b.mutex.RLock()
	defer b.mutex.RUnlock()

	for connID, conn := range b.connections {
		if connID != excludeConnID {
			err := websocket.Message.Send(conn, message)
			if err != nil {
				logger.Error("Failed to send message to ", connID, ": ", err)
				// 这里可以选择移除无法发送消息的连接
			}
		}
	}

	logger.Debug("Broadcast message from ", excludeConnID)
}

// SendMessage 发送消息到特定连接
func (b *MessageBus) SendMessage(connID string, message string) error {
	b.mutex.RLock()
	conn, exists := b.connections[connID]
	b.mutex.RUnlock()

	if !exists {
		logger.Error("Connection not found: ", connID)
		return fmt.Errorf("connection not found: %s", connID)
	}

	err := websocket.Message.Send(conn, message)
	if err != nil {
		logger.Error("Failed to send message to ", connID, ": ", err)
		return err
	}

	return nil
}
