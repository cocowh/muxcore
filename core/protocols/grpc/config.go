// Copyright (c) 2025 cocowh. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package grpc

import "time"

// GRPCConfig gRPC配置
type GRPCConfig struct {
	MaxRecvMsgSize       int           `yaml:"max_recv_msg_size"`
	MaxSendMsgSize       int           `yaml:"max_send_msg_size"`
	ConnectionTimeout    time.Duration `yaml:"connection_timeout"`
	KeepaliveTime        time.Duration `yaml:"keepalive_time"`
	KeepaliveTimeout     time.Duration `yaml:"keepalive_timeout"`
	EnableReflection     bool          `yaml:"enable_reflection"`
	EnableHealthCheck    bool          `yaml:"enable_health_check"`
	MaxConcurrentStreams uint32        `yaml:"max_concurrent_streams"`
}

// DefaultGRPCConfig 默认gRPC配置
func DefaultGRPCConfig() *GRPCConfig {
	return &GRPCConfig{
		MaxRecvMsgSize:       4 << 20, // 4MB
		MaxSendMsgSize:       4 << 20, // 4MB
		ConnectionTimeout:    30 * time.Second,
		KeepaliveTime:        30 * time.Second,
		KeepaliveTimeout:     5 * time.Second,
		EnableReflection:     true,
		EnableHealthCheck:    true,
		MaxConcurrentStreams: 1000,
	}
}
