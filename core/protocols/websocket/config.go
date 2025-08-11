// Copyright (c) 2025 cocowh. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package websocket

import "time"

// WebSocketConfig WebSocket配置
type WebSocketConfig struct {
	MaxConnections    int           `yaml:"max_connections"`
	PingInterval      time.Duration `yaml:"ping_interval"`
	PongTimeout       time.Duration `yaml:"pong_timeout"`
	MaxMessageSize    int64         `yaml:"max_message_size"`
	EnableCompression bool          `yaml:"enable_compression"`
	ReadBufferSize    int           `yaml:"read_buffer_size"`
	WriteBufferSize   int           `yaml:"write_buffer_size"`
	HandshakeTimeout  time.Duration `yaml:"handshake_timeout"`
}

// DefaultWebSocketConfig 默认WebSocket配置
func DefaultWebSocketConfig() *WebSocketConfig {
	return &WebSocketConfig{
		MaxConnections:    1000,
		PingInterval:      30 * time.Second,
		PongTimeout:       10 * time.Second,
		MaxMessageSize:    1024 * 1024, // 1MB
		EnableCompression: true,
		ReadBufferSize:    4096,
		WriteBufferSize:   4096,
		HandshakeTimeout:  10 * time.Second,
	}
}