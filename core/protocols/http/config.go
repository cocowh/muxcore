// Copyright (c) 2025 cocowh. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package http

import "time"

// HTTPConfig HTTP配置
type HTTPConfig struct {
	EnableHTTP2    bool          `yaml:"enable_http2"`
	MaxHeaderSize  int           `yaml:"max_header_size"`
	ReadTimeout    time.Duration `yaml:"read_timeout"`
	WriteTimeout   time.Duration `yaml:"write_timeout"`
	IdleTimeout    time.Duration `yaml:"idle_timeout"`
	EnableGzip     bool          `yaml:"enable_gzip"`
	MaxRequestSize int64         `yaml:"max_request_size"`
}

// DefaultHTTPConfig 默认HTTP配置
func DefaultHTTPConfig() *HTTPConfig {
	return &HTTPConfig{
		EnableHTTP2:    true,
		MaxHeaderSize:  1 << 20, // 1MB
		ReadTimeout:    30 * time.Second,
		WriteTimeout:   30 * time.Second,
		IdleTimeout:    120 * time.Second,
		EnableGzip:     true,
		MaxRequestSize: 32 << 20, // 32MB
	}
}
