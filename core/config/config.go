// Copyright (c) 2025 cocowh. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package config

import (
	"github.com/cocowh/muxcore/core/observability"
	"sync"
	"time"

	"github.com/cocowh/muxcore/pkg/errors"
	"github.com/spf13/viper"
)

// Manager is a manager for application configurations.
type Manager struct {
	viper   *viper.Viper
	configs map[string]interface{}
	mutex   sync.RWMutex
}

// NewConfigManager creates a new Manager instance.
func NewConfigManager(configPath string) (*Manager, error) {
	// init viper
	v := viper.New()
	v.SetConfigFile(configPath)

	// read config file
	if err := v.ReadInConfig(); err != nil {
		muxErr := errors.ConfigError(errors.ErrCodeConfigNotFound, "failed to read config file").WithCause(err).WithContext("config_path", configPath)
		return nil, muxErr
	}

	// create Manager instance
	cm := &Manager{
		viper:   v,
		configs: make(map[string]interface{}),
	}

	// validate config
	if err := cm.validateConfig(); err != nil {
		muxErr := errors.Convert(err)
		return nil, muxErr.WithContext("operation", "validate_config")
	}

	return cm, nil
}

// validateConfig validates the configuration and sets default values if necessary.
func (cm *Manager) validateConfig() error {
	// validate server config
	if !cm.viper.IsSet("server.address") {
		cm.viper.Set("server.address", ":8080")
	}

	// validate monitor config
	if !cm.viper.IsSet("monitor.interval") {
		cm.viper.Set("monitor.interval", 10)
	}
	if !cm.viper.IsSet("monitor.metrics.enabled") {
		cm.viper.Set("monitor.metrics.enabled", true)
	}
	if !cm.viper.IsSet("monitor.metrics.address") {
		cm.viper.Set("monitor.metrics.address", ":9090")
	}

	// 验证日志配置
	if !cm.viper.IsSet("logger.level") {
		cm.viper.Set("logger.level", "info")
	}
	if !cm.viper.IsSet("logger.format") {
		cm.viper.Set("logger.format", "text")
	}
	if !cm.viper.IsSet("logger.log_dir") {
		cm.viper.Set("logger.log_dir", "./logs")
	}
	if !cm.viper.IsSet("logger.base_name") {
		cm.viper.Set("logger.base_name", "muxcore")
	}
	if !cm.viper.IsSet("logger.max_size") {
		cm.viper.Set("logger.max_size", 100)
	}
	if !cm.viper.IsSet("logger.max_age") {
		cm.viper.Set("logger.max_age", 7)
	}
	if !cm.viper.IsSet("logger.max_backups") {
		cm.viper.Set("logger.max_backups", 3)
	}

	// 验证协议配置
	if !cm.viper.IsSet("protocols.http.enabled") {
		cm.viper.Set("protocols.http.enabled", true)
	}
	if !cm.viper.IsSet("protocols.websocket.enabled") {
		cm.viper.Set("protocols.websocket.enabled", true)
	}
	if !cm.viper.IsSet("protocols.grpc.enabled") {
		cm.viper.Set("protocols.grpc.enabled", true)
	}
	if !cm.viper.IsSet("protocols.custom.enabled") {
		cm.viper.Set("protocols.custom.enabled", false)
	}

	// 验证池配置
	if !cm.viper.IsSet("pools.connection.max_size") {
		cm.viper.Set("pools.connection.max_size", 1000)
	}
	if !cm.viper.IsSet("pools.connection.max_idle_time") {
		cm.viper.Set("pools.connection.max_idle_time", 300)
	}
	if !cm.viper.IsSet("pools.goroutine.workers") {
		cm.viper.Set("pools.goroutine.workers", 0)
	}
	if !cm.viper.IsSet("pools.goroutine.queue_size") {
		cm.viper.Set("pools.goroutine.queue_size", 1000)
	}

	// 验证安全配置
	if !cm.viper.IsSet("security.enabled") {
		cm.viper.Set("security.enabled", true)
	}
	if !cm.viper.IsSet("security.security_level") {
		cm.viper.Set("security.security_level", 1)
	}
	if !cm.viper.IsSet("security.tls.enabled") {
		cm.viper.Set("security.tls.enabled", false)
	}
	if !cm.viper.IsSet("security.auth.enabled") {
		cm.viper.Set("security.auth.enabled", false)
	}

	// 验证检测器配置
	if !cm.viper.IsSet("detector.buffer_size") {
		cm.viper.Set("detector.buffer_size", 4096)
	}
	if !cm.viper.IsSet("detector.read_timeout") {
		cm.viper.Set("detector.read_timeout", 5)
	}
	if !cm.viper.IsSet("detector.max_detect_size") {
		cm.viper.Set("detector.max_detect_size", 1024)
	}

	// 验证可观测性配置
	if !cm.viper.IsSet("observability.enabled") {
		cm.viper.Set("observability.enabled", true)
	}
	if !cm.viper.IsSet("observability.sampling_rate") {
		cm.viper.Set("observability.sampling_rate", 0.1)
	}

	// 验证可靠性配置
	if !cm.viper.IsSet("reliability.circuit_breaker.enabled") {
		cm.viper.Set("reliability.circuit_breaker.enabled", true)
	}
	if !cm.viper.IsSet("reliability.circuit_breaker.failure_threshold") {
		cm.viper.Set("reliability.circuit_breaker.failure_threshold", 0.1)
	}
	if !cm.viper.IsSet("reliability.circuit_breaker.reset_timeout") {
		cm.viper.Set("reliability.circuit_breaker.reset_timeout", 30)
	}

	// 验证处理器配置
	if !cm.viper.IsSet("processor.max_concurrency") {
		cm.viper.Set("processor.max_concurrency", 1000)
	}
	if !cm.viper.IsSet("processor.queue_size") {
		cm.viper.Set("processor.queue_size", 10000)
	}
	if !cm.viper.IsSet("processor.timeout") {
		cm.viper.Set("processor.timeout", 30)
	}

	// 验证消息总线配置
	if !cm.viper.IsSet("message_bus.buffer_size") {
		cm.viper.Set("message_bus.buffer_size", 1000)
	}
	if !cm.viper.IsSet("message_bus.max_subscribers") {
		cm.viper.Set("message_bus.max_subscribers", 100)
	}

	// 补充buffer pool默认值
	if !cm.viper.IsSet("pools.buffer.default_size") {
		cm.viper.Set("pools.buffer.default_size", 4096)
	}
	if !cm.viper.IsSet("pools.buffer.max_pool_size") {
		cm.viper.Set("pools.buffer.max_pool_size", 1000)
	}

	// 补充security配置默认值
	if !cm.viper.IsSet("security.dos_threshold") {
		cm.viper.Set("security.dos_threshold", 1000)
	}
	if !cm.viper.IsSet("security.dos_window_size") {
		cm.viper.Set("security.dos_window_size", 60)
	}
	if !cm.viper.IsSet("security.dos_cleanup_interval") {
		cm.viper.Set("security.dos_cleanup_interval", 300)
	}
	if !cm.viper.IsSet("security.dos_block_duration") {
		cm.viper.Set("security.dos_block_duration", 600)
	}
	if !cm.viper.IsSet("security.syn_cookie_cleanup_interval") {
		cm.viper.Set("security.syn_cookie_cleanup_interval", 120)
	}
	if !cm.viper.IsSet("security.dpi_max_packet_size") {
		cm.viper.Set("security.dpi_max_packet_size", 8192)
	}
	if !cm.viper.IsSet("security.default_auth_policy") {
		cm.viper.Set("security.default_auth_policy", "allow")
	}
	if !cm.viper.IsSet("security.event_buffer_size") {
		cm.viper.Set("security.event_buffer_size", 10000)
	}
	if !cm.viper.IsSet("security.metrics_interval") {
		cm.viper.Set("security.metrics_interval", 30)
	}

	// 补充HTTP协议配置默认值
	if !cm.viper.IsSet("protocols.http.enable_http2") {
		cm.viper.Set("protocols.http.enable_http2", true)
	}
	if !cm.viper.IsSet("protocols.http.max_header_size") {
		cm.viper.Set("protocols.http.max_header_size", 1048576) // 1MB
	}
	if !cm.viper.IsSet("protocols.http.enable_gzip") {
		cm.viper.Set("protocols.http.enable_gzip", true)
	}
	if !cm.viper.IsSet("protocols.http.max_request_size") {
		cm.viper.Set("protocols.http.max_request_size", 10485760) // 10MB
	}
	if !cm.viper.IsSet("protocols.http.read_timeout") {
		cm.viper.Set("protocols.http.read_timeout", 30)
	}
	if !cm.viper.IsSet("protocols.http.write_timeout") {
		cm.viper.Set("protocols.http.write_timeout", 30)
	}
	if !cm.viper.IsSet("protocols.http.idle_timeout") {
		cm.viper.Set("protocols.http.idle_timeout", 120)
	}

	// 补充gRPC协议配置默认值
	if !cm.viper.IsSet("protocols.grpc.max_recv_msg_size") {
		cm.viper.Set("protocols.grpc.max_recv_msg_size", 4194304) // 4MB
	}
	if !cm.viper.IsSet("protocols.grpc.max_send_msg_size") {
		cm.viper.Set("protocols.grpc.max_send_msg_size", 4194304) // 4MB
	}
	if !cm.viper.IsSet("protocols.grpc.connection_timeout") {
		cm.viper.Set("protocols.grpc.connection_timeout", 10)
	}
	if !cm.viper.IsSet("protocols.grpc.keepalive_time") {
		cm.viper.Set("protocols.grpc.keepalive_time", 30)
	}
	if !cm.viper.IsSet("protocols.grpc.keepalive_timeout") {
		cm.viper.Set("protocols.grpc.keepalive_timeout", 5)
	}
	if !cm.viper.IsSet("protocols.grpc.enable_reflection") {
		cm.viper.Set("protocols.grpc.enable_reflection", false)
	}
	if !cm.viper.IsSet("protocols.grpc.enable_health_check") {
		cm.viper.Set("protocols.grpc.enable_health_check", true)
	}
	if !cm.viper.IsSet("protocols.grpc.max_concurrent_streams") {
		cm.viper.Set("protocols.grpc.max_concurrent_streams", 1000)
	}

	// 补充WebSocket协议配置默认值
	if !cm.viper.IsSet("protocols.websocket.max_connections") {
		cm.viper.Set("protocols.websocket.max_connections", 5000)
	}
	if !cm.viper.IsSet("protocols.websocket.ping_interval") {
		cm.viper.Set("protocols.websocket.ping_interval", 30)
	}
	if !cm.viper.IsSet("protocols.websocket.pong_timeout") {
		cm.viper.Set("protocols.websocket.pong_timeout", 10)
	}
	if !cm.viper.IsSet("protocols.websocket.max_message_size") {
		cm.viper.Set("protocols.websocket.max_message_size", 1048576) // 1MB
	}
	if !cm.viper.IsSet("protocols.websocket.enable_compression") {
		cm.viper.Set("protocols.websocket.enable_compression", true)
	}
	if !cm.viper.IsSet("protocols.websocket.read_buffer_size") {
		cm.viper.Set("protocols.websocket.read_buffer_size", 4096)
	}
	if !cm.viper.IsSet("protocols.websocket.write_buffer_size") {
		cm.viper.Set("protocols.websocket.write_buffer_size", 4096)
	}
	if !cm.viper.IsSet("protocols.websocket.handshake_timeout") {
		cm.viper.Set("protocols.websocket.handshake_timeout", 10)
	}

	// 补充logger配置的缺失字段默认值
	if !cm.viper.IsSet("logger.enable_warn_file") {
		cm.viper.Set("logger.enable_warn_file", true)
	}
	if !cm.viper.IsSet("logger.enable_error_file") {
		cm.viper.Set("logger.enable_error_file", true)
	}

	// 补充可靠性配置默认值
	if !cm.viper.IsSet("reliability.degradation.enabled") {
		cm.viper.Set("reliability.degradation.enabled", false)
	}
	if !cm.viper.IsSet("reliability.degradation.monitoring_interval") {
		cm.viper.Set("reliability.degradation.monitoring_interval", 60)
	}
	if !cm.viper.IsSet("reliability.degradation.thresholds.simplified") {
		cm.viper.Set("reliability.degradation.thresholds.simplified", 0.7)
	}
	if !cm.viper.IsSet("reliability.degradation.thresholds.critical_only") {
		cm.viper.Set("reliability.degradation.thresholds.critical_only", 0.5)
	}
	if !cm.viper.IsSet("reliability.degradation.thresholds.maintain_only") {
		cm.viper.Set("reliability.degradation.thresholds.maintain_only", 0.3)
	}

	// 补充熔断器协议特定阈值默认值
	if !cm.viper.IsSet("reliability.circuit_breaker.protocols.http.protocol_failure_threshold") {
		cm.viper.Set("reliability.circuit_breaker.protocols.http.protocol_failure_threshold", 0.1)
	}
	if !cm.viper.IsSet("reliability.circuit_breaker.protocols.http.service_failure_threshold") {
		cm.viper.Set("reliability.circuit_breaker.protocols.http.service_failure_threshold", 0.15)
	}
	if !cm.viper.IsSet("reliability.circuit_breaker.protocols.grpc.protocol_failure_threshold") {
		cm.viper.Set("reliability.circuit_breaker.protocols.grpc.protocol_failure_threshold", 0.1)
	}
	if !cm.viper.IsSet("reliability.circuit_breaker.protocols.grpc.service_failure_threshold") {
		cm.viper.Set("reliability.circuit_breaker.protocols.grpc.service_failure_threshold", 0.15)
	}
	if !cm.viper.IsSet("reliability.circuit_breaker.protocols.websocket.protocol_failure_threshold") {
		cm.viper.Set("reliability.circuit_breaker.protocols.websocket.protocol_failure_threshold", 0.1)
	}
	if !cm.viper.IsSet("reliability.circuit_breaker.protocols.websocket.service_failure_threshold") {
		cm.viper.Set("reliability.circuit_breaker.protocols.websocket.service_failure_threshold", 0.15)
	}

	// 验证路由器配置
	if !cm.viper.IsSet("router.max_routes") {
		cm.viper.Set("router.max_routes", 10000)
	}
	if !cm.viper.IsSet("router.compression_enabled") {
		cm.viper.Set("router.compression_enabled", true)
	}

	return nil
}

// GetServerConfig get server config
func (cm *Manager) GetServerConfig() ServerConfig {
	cm.mutex.RLock()
	defer cm.mutex.RUnlock()

	return ServerConfig{
		Address: cm.viper.GetString("server.address"),
	}
}

// GetMonitorConfig get monitor config
func (cm *Manager) GetMonitorConfig() MonitorConfig {
	cm.mutex.RLock()
	defer cm.mutex.RUnlock()

	return MonitorConfig{
		Interval:       cm.viper.GetInt("monitor.interval"),
		MetricsEnabled: cm.viper.GetBool("monitor.metrics.enabled"),
		MetricsAddress: cm.viper.GetString("monitor.metrics.address"),
	}
}

// GetLoggerConfig 获取日志配置
func (cm *Manager) GetLoggerConfig() LoggerConfig {
	cm.mutex.RLock()
	defer cm.mutex.RUnlock()

	return LoggerConfig{
		Level:           cm.viper.GetString("logger.level"),
		Format:          cm.viper.GetString("logger.format"),
		LogDir:          cm.viper.GetString("logger.log_dir"),
		BaseName:        cm.viper.GetString("logger.base_name"),
		TimeFormat:      cm.viper.GetString("logger.time_format"),
		MaxSizeMB:       cm.viper.GetInt("logger.max_size"),
		MaxAgeDays:      cm.viper.GetInt("logger.max_age"),
		MaxBackups:      cm.viper.GetInt("logger.max_backups"),
		EnableWarnFile:  cm.viper.GetBool("logger.enable_warn_file"),
		EnableErrorFile: cm.viper.GetBool("logger.enable_error_file"),
	}
}

// GetProtocolConfig 获取基础协议配置
func (cm *Manager) GetProtocolConfig() ProtocolConfig {
	cm.mutex.RLock()
	defer cm.mutex.RUnlock()

	return ProtocolConfig{
		HTTP: HTTPConfig{
			Enabled:        cm.viper.GetBool("protocols.http.enabled"),
			MaxConnections: cm.viper.GetInt("protocols.http.max_connections"),
		},
		WebSocket: WebSocketConfig{
			Enabled:        cm.viper.GetBool("protocols.websocket.enabled"),
			MaxConnections: cm.viper.GetInt("protocols.websocket.max_connections"),
		},
		GRPC: GRPCConfig{
			Enabled:        cm.viper.GetBool("protocols.grpc.enabled"),
			MaxConnections: cm.viper.GetInt("protocols.grpc.max_connections"),
		},
		Custom: CustomConfig{
			Enabled: cm.viper.GetBool("protocols.custom.enabled"),
		},
	}
}

// GetHTTPProtocolConfig 获取完整的HTTP协议配置
func (cm *Manager) GetHTTPProtocolConfig() *HTTPProtocolConfig {
	cm.mutex.RLock()
	defer cm.mutex.RUnlock()

	// 构建HTTP协议配置
	config := &HTTPProtocolConfig{
		Enabled:        cm.viper.GetBool("protocols.http.enabled"),
		EnableHTTP2:    cm.viper.GetBool("protocols.http.enable_http2"),
		MaxHeaderSize:  cm.viper.GetInt("protocols.http.max_header_size"),
		EnableGzip:     cm.viper.GetBool("protocols.http.enable_gzip"),
		MaxRequestSize: cm.viper.GetInt64("protocols.http.max_request_size"),
	}

	if cm.viper.IsSet("protocols.http.read_timeout") {
		config.ReadTimeout = time.Duration(cm.viper.GetInt("protocols.http.read_timeout")) * time.Second
	}
	if cm.viper.IsSet("protocols.http.write_timeout") {
		config.WriteTimeout = time.Duration(cm.viper.GetInt("protocols.http.write_timeout")) * time.Second
	}
	if cm.viper.IsSet("protocols.http.idle_timeout") {
		config.IdleTimeout = time.Duration(cm.viper.GetInt("protocols.http.idle_timeout")) * time.Second
	}

	return config
}

// GetGRPCProtocolConfig 获取完整的gRPC协议配置
func (cm *Manager) GetGRPCProtocolConfig() *GRPCProtocolConfig {
	cm.mutex.RLock()
	defer cm.mutex.RUnlock()

	// 构建gRPC协议配置
	config := &GRPCProtocolConfig{
		Enabled:              cm.viper.GetBool("protocols.grpc.enabled"),
		MaxRecvMsgSize:       cm.viper.GetInt("protocols.grpc.max_recv_msg_size"),
		MaxSendMsgSize:       cm.viper.GetInt("protocols.grpc.max_send_msg_size"),
		EnableReflection:     cm.viper.GetBool("protocols.grpc.enable_reflection"),
		EnableHealthCheck:    cm.viper.GetBool("protocols.grpc.enable_health_check"),
		MaxConcurrentStreams: uint32(cm.viper.GetInt("protocols.grpc.max_concurrent_streams")),
	}

	if cm.viper.IsSet("protocols.grpc.connection_timeout") {
		config.ConnectionTimeout = time.Duration(cm.viper.GetInt("protocols.grpc.connection_timeout")) * time.Second
	}
	if cm.viper.IsSet("protocols.grpc.keepalive_time") {
		config.KeepaliveTime = time.Duration(cm.viper.GetInt("protocols.grpc.keepalive_time")) * time.Second
	}
	if cm.viper.IsSet("protocols.grpc.keepalive_timeout") {
		config.KeepaliveTimeout = time.Duration(cm.viper.GetInt("protocols.grpc.keepalive_timeout")) * time.Second
	}

	return config
}

// GetWebSocketProtocolConfig 获取完整的WebSocket协议配置
func (cm *Manager) GetWebSocketProtocolConfig() *WebSocketProtocolConfig {
	cm.mutex.RLock()
	defer cm.mutex.RUnlock()

	// 构建WebSocket协议配置
	config := &WebSocketProtocolConfig{
		Enabled:           cm.viper.GetBool("protocols.websocket.enabled"),
		MaxConnections:    cm.viper.GetInt("protocols.websocket.max_connections"),
		MaxMessageSize:    cm.viper.GetInt64("protocols.websocket.max_message_size"),
		EnableCompression: cm.viper.GetBool("protocols.websocket.enable_compression"),
		ReadBufferSize:    cm.viper.GetInt("protocols.websocket.read_buffer_size"),
		WriteBufferSize:   cm.viper.GetInt("protocols.websocket.write_buffer_size"),
	}

	if cm.viper.IsSet("protocols.websocket.ping_interval") {
		config.PingInterval = time.Duration(cm.viper.GetInt("protocols.websocket.ping_interval")) * time.Second
	}
	if cm.viper.IsSet("protocols.websocket.pong_timeout") {
		config.PongTimeout = time.Duration(cm.viper.GetInt("protocols.websocket.pong_timeout")) * time.Second
	}
	if cm.viper.IsSet("protocols.websocket.handshake_timeout") {
		config.HandshakeTimeout = time.Duration(cm.viper.GetInt("protocols.websocket.handshake_timeout")) * time.Second
	}

	return config
}

// GetOptimizedConnectionPoolConfig 获取优化连接池配置
func (cm *Manager) GetOptimizedConnectionPoolConfig() *OptimizedConnectionPoolConfig {
	cm.mutex.RLock()
	defer cm.mutex.RUnlock()

	// 从默认配置开始
	config := &OptimizedConnectionPoolConfig{
		MaxConnections:      1000,
		MinConnections:      10,
		MaxIdleTime:         300 * time.Second,
		HealthCheckInterval: 30 * time.Second,
		ConnectionTimeout:   10 * time.Second,
		EnableLoadBalancing: true,
		EnableMetrics:       true,
		BufferSize:          4096,
	}

	// 从配置文件覆盖
	if cm.viper.IsSet("pools.optimized_connection.max_connections") {
		config.MaxConnections = cm.viper.GetInt("pools.optimized_connection.max_connections")
	}
	if cm.viper.IsSet("pools.optimized_connection.min_connections") {
		config.MinConnections = cm.viper.GetInt("pools.optimized_connection.min_connections")
	}
	if cm.viper.IsSet("pools.optimized_connection.max_idle_time") {
		config.MaxIdleTime = time.Duration(cm.viper.GetInt("pools.optimized_connection.max_idle_time")) * time.Second
	}
	if cm.viper.IsSet("pools.optimized_connection.health_check_interval") {
		config.HealthCheckInterval = time.Duration(cm.viper.GetInt("pools.optimized_connection.health_check_interval")) * time.Second
	}
	if cm.viper.IsSet("pools.optimized_connection.connection_timeout") {
		config.ConnectionTimeout = time.Duration(cm.viper.GetInt("pools.optimized_connection.connection_timeout")) * time.Second
	}
	if cm.viper.IsSet("pools.optimized_connection.enable_load_balancing") {
		config.EnableLoadBalancing = cm.viper.GetBool("pools.optimized_connection.enable_load_balancing")
	}
	if cm.viper.IsSet("pools.optimized_connection.enable_metrics") {
		config.EnableMetrics = cm.viper.GetBool("pools.optimized_connection.enable_metrics")
	}
	if cm.viper.IsSet("pools.optimized_connection.buffer_size") {
		config.BufferSize = cm.viper.GetInt("pools.optimized_connection.buffer_size")
	}

	return config
}

// GetOptimizedGoroutinePoolConfig 获取优化协程池配置
func (cm *Manager) GetOptimizedGoroutinePoolConfig() *OptimizedGoroutinePoolConfig {
	cm.mutex.RLock()
	defer cm.mutex.RUnlock()

	// 从默认配置开始
	config := &OptimizedGoroutinePoolConfig{
		MinWorkers:         10,
		MaxWorkers:         1000,
		QueueSize:          1000,
		ScaleUpThreshold:   0.8,
		ScaleDownThreshold: 0.2,
		ScaleInterval:      10 * time.Second,
		WorkerTimeout:      30 * time.Second,
		EnableMetrics:      true,
	}

	// 从配置文件覆盖
	if cm.viper.IsSet("pools.optimized_goroutine.min_workers") {
		config.MinWorkers = cm.viper.GetInt("pools.optimized_goroutine.min_workers")
	}
	if cm.viper.IsSet("pools.optimized_goroutine.max_workers") {
		config.MaxWorkers = cm.viper.GetInt("pools.optimized_goroutine.max_workers")
	}
	if cm.viper.IsSet("pools.optimized_goroutine.queue_size") {
		config.QueueSize = cm.viper.GetInt("pools.optimized_goroutine.queue_size")
	}
	if cm.viper.IsSet("pools.optimized_goroutine.scale_up_threshold") {
		config.ScaleUpThreshold = cm.viper.GetFloat64("pools.optimized_goroutine.scale_up_threshold")
	}
	if cm.viper.IsSet("pools.optimized_goroutine.scale_down_threshold") {
		config.ScaleDownThreshold = cm.viper.GetFloat64("pools.optimized_goroutine.scale_down_threshold")
	}
	if cm.viper.IsSet("pools.optimized_goroutine.scale_interval") {
		config.ScaleInterval = time.Duration(cm.viper.GetInt("pools.optimized_goroutine.scale_interval")) * time.Second
	}
	if cm.viper.IsSet("pools.optimized_goroutine.worker_timeout") {
		config.WorkerTimeout = time.Duration(cm.viper.GetInt("pools.optimized_goroutine.worker_timeout")) * time.Second
	}
	if cm.viper.IsSet("pools.optimized_goroutine.enable_metrics") {
		config.EnableMetrics = cm.viper.GetBool("pools.optimized_goroutine.enable_metrics")
	}

	return config
}

// GetOptimizedRouterConfig 获取优化路由器配置
func (cm *Manager) GetOptimizedRouterConfig() *OptimizedRouterConfig {
	cm.mutex.RLock()
	defer cm.mutex.RUnlock()

	// 默认负载均衡策略
	strategy := LoadBalanceStrategyRoundRobin
	if cm.viper.IsSet("router.optimized.load_balance_strategy") {
		strategyStr := cm.viper.GetString("router.optimized.load_balance_strategy")
		switch strategyStr {
		case "round_robin":
			strategy = LoadBalanceStrategyRoundRobin
		case "weighted_round_robin":
			strategy = LoadBalanceStrategyWeightedRoundRobin
		case "least_connections":
			strategy = LoadBalanceStrategyLeastConnections
		case "least_response_time":
			strategy = LoadBalanceStrategyLeastResponseTime
		case "consistent_hash":
			strategy = LoadBalanceStrategyConsistentHash
		}
	}

	// 从默认配置开始
	config := &OptimizedRouterConfig{
		MaxConcurrency:      1000,
		RequestTimeout:      30 * time.Second,
		EnableLoadBalancing: true,
		EnableCaching:       true,
		EnableFailover:      true,
		EnableMetrics:       true,
		HealthCheckInterval: 10 * time.Second,
		CacheSize:           10000,
		CacheTTL:            5 * time.Minute,
		LoadBalanceStrategy: strategy,
		FailoverThreshold:   3,
		MetricsInterval:     30 * time.Second,
	}

	// 从配置文件覆盖
	if cm.viper.IsSet("router.optimized.max_concurrency") {
		config.MaxConcurrency = cm.viper.GetInt("router.optimized.max_concurrency")
	}
	if cm.viper.IsSet("router.optimized.request_timeout") {
		config.RequestTimeout = time.Duration(cm.viper.GetInt("router.optimized.request_timeout")) * time.Second
	}
	if cm.viper.IsSet("router.optimized.enable_load_balancing") {
		config.EnableLoadBalancing = cm.viper.GetBool("router.optimized.enable_load_balancing")
	}
	if cm.viper.IsSet("router.optimized.enable_caching") {
		config.EnableCaching = cm.viper.GetBool("router.optimized.enable_caching")
	}
	if cm.viper.IsSet("router.optimized.enable_failover") {
		config.EnableFailover = cm.viper.GetBool("router.optimized.enable_failover")
	}
	if cm.viper.IsSet("router.optimized.enable_metrics") {
		config.EnableMetrics = cm.viper.GetBool("router.optimized.enable_metrics")
	}
	if cm.viper.IsSet("router.optimized.health_check_interval") {
		config.HealthCheckInterval = time.Duration(cm.viper.GetInt("router.optimized.health_check_interval")) * time.Second
	}
	if cm.viper.IsSet("router.optimized.cache_size") {
		config.CacheSize = cm.viper.GetInt("router.optimized.cache_size")
	}
	if cm.viper.IsSet("router.optimized.cache_ttl") {
		config.CacheTTL = time.Duration(cm.viper.GetInt("router.optimized.cache_ttl")) * time.Second
	}
	if cm.viper.IsSet("router.optimized.failover_threshold") {
		config.FailoverThreshold = cm.viper.GetInt("router.optimized.failover_threshold")
	}
	if cm.viper.IsSet("router.optimized.metrics_interval") {
		config.MetricsInterval = time.Duration(cm.viper.GetInt("router.optimized.metrics_interval")) * time.Second
	}

	return config
}

// GetObservabilityConfig get observability config
func (cm *Manager) GetObservabilityConfig() *observability.Config {
	cm.mutex.RLock()
	defer cm.mutex.RUnlock()
	alertThresholds := make(map[string]float64)
	for k, v := range cm.viper.GetStringMap("observability.alerting_thresholds") {
		alertThresholds[k] = v.(float64)
	}
	c := &observability.Config{
		MetricsEnabled:      cm.viper.GetBool("observability.optimized.metrics_enabled"),
		MetricsInterval:     cm.viper.GetDuration("observability.optimized.metrics_interval"),
		MetricsRetention:    cm.viper.GetDuration("observability.optimized.metrics_retention"),
		MetricsBufferSize:   cm.viper.GetInt("observability.optimized.metrics_buffer_size"),
		TracingEnabled:      cm.viper.GetBool("observability.enabled"),
		SamplingRate:        cm.viper.GetFloat64("observability.sampling_rate"),
		MaxSpansPerTrace:    cm.viper.GetInt("observability.max_spans_per_trace"),
		TraceTimeout:        cm.viper.GetDuration("observability.trace_timeout"),
		PerformanceEnabled:  cm.viper.GetBool("observability.performance_enabled"),
		ResourceMonitoring:  cm.viper.GetBool("observability.resource_monitoring"),
		HealthCheckEnabled:  cm.viper.GetBool("observability.health_check_enabled"),
		HealthCheckInterval: cm.viper.GetDuration("observability.health_check_interval"),
		AlertingEnabled:     cm.viper.GetBool("observability.alerting_enabled"),
		AlertThresholds:     alertThresholds,
		AlertCooldown:       cm.viper.GetDuration("observability.alert_cooldown"),
	}

	return c
}

// GetPoolConfig gets the pool configuration
func (cm *Manager) GetPoolConfig() *PoolConfig {
	cm.mutex.RLock()
	defer cm.mutex.RUnlock()

	return &PoolConfig{
		Connection: ConnectionPoolConfig{
			MaxSize:     cm.viper.GetInt("pools.connection.max_size"),
			MaxIdleTime: cm.viper.GetInt("pools.connection.max_idle_time"),
		},
		Goroutine: GoroutinePoolConfig{
			Workers:   cm.viper.GetInt("pools.goroutine.workers"),
			QueueSize: cm.viper.GetInt("pools.goroutine.queue_size"),
		},
		Buffer: BufferPoolConfig{
			DefaultSize: cm.viper.GetInt("pools.buffer.default_size"),
			MaxPoolSize: cm.viper.GetInt("pools.buffer.max_pool_size"),
		},
	}
}

// GetReliabilityConfig 获取可靠性配置
func (cm *Manager) GetReliabilityConfig() ReliabilityConfig {
	cm.mutex.RLock()
	defer cm.mutex.RUnlock()

	return ReliabilityConfig{
		CircuitBreaker: CircuitBreakerConfig{
			Enabled:          cm.viper.GetBool("reliability.circuit_breaker.enabled"),
			FailureThreshold: cm.viper.GetFloat64("reliability.circuit_breaker.failure_threshold"),
			ResetTimeout:     cm.viper.GetInt("reliability.circuit_breaker.reset_timeout"),
			Protocols: ProtocolThresholds{
				HTTP: ProtocolConfigThreshold{
					ProtocolFailureThreshold: cm.viper.GetFloat64("reliability.circuit_breaker.protocols.http.protocol_failure_threshold"),
					ServiceFailureThreshold:  cm.viper.GetFloat64("reliability.circuit_breaker.protocols.http.service_failure_threshold"),
				},
				GRPC: ProtocolConfigThreshold{
					ProtocolFailureThreshold: cm.viper.GetFloat64("reliability.circuit_breaker.protocols.grpc.protocol_failure_threshold"),
					ServiceFailureThreshold:  cm.viper.GetFloat64("reliability.circuit_breaker.protocols.grpc.service_failure_threshold"),
				},
				WebSocket: ProtocolConfigThreshold{
					ProtocolFailureThreshold: cm.viper.GetFloat64("reliability.circuit_breaker.protocols.websocket.protocol_failure_threshold"),
					ServiceFailureThreshold:  cm.viper.GetFloat64("reliability.circuit_breaker.protocols.websocket.service_failure_threshold"),
				},
			},
		},
		Degradation: DegradationConfig{
			Enabled:            cm.viper.GetBool("reliability.degradation.enabled"),
			MonitoringInterval: cm.viper.GetInt("reliability.degradation.monitoring_interval"),
			Thresholds: DegradationThresholds{
				Simplified:   cm.viper.GetFloat64("reliability.degradation.thresholds.simplified"),
				CriticalOnly: cm.viper.GetFloat64("reliability.degradation.thresholds.critical_only"),
				MaintainOnly: cm.viper.GetFloat64("reliability.degradation.thresholds.maintain_only"),
			},
		},
	}
}

// GetRouterConfig 获取路由器配置
func (cm *Manager) GetRouterConfig() RouterConfig {
	cm.mutex.RLock()
	defer cm.mutex.RUnlock()

	return RouterConfig{
		MaxRoutes:          cm.viper.GetInt("router.max_routes"),
		CompressionEnabled: cm.viper.GetBool("router.compression_enabled"),
	}
}

// GetSecurityConfig 获取安全配置
func (cm *Manager) GetSecurityConfig() SecurityConfig {
	cm.mutex.RLock()
	defer cm.mutex.RUnlock()

	return SecurityConfig{
		Enabled:                  cm.viper.GetBool("security.enabled"),
		SecurityLevel:            cm.viper.GetInt("security.security_level"),
		DOSThreshold:             cm.viper.GetInt("security.dos_threshold"),
		DOSWindowSize:            cm.viper.GetInt("security.dos_window_size"),
		DOSCleanupInterval:       cm.viper.GetInt("security.dos_cleanup_interval"),
		DOSBlockDuration:         cm.viper.GetInt("security.dos_block_duration"),
		SYNCookieCleanupInterval: cm.viper.GetInt("security.syn_cookie_cleanup_interval"),
		DPIMaxPacketSize:         cm.viper.GetInt("security.dpi_max_packet_size"),
		DefaultAuthPolicy:        cm.viper.GetString("security.default_auth_policy"),
		EventBufferSize:          cm.viper.GetInt("security.event_buffer_size"),
		MetricsInterval:          cm.viper.GetInt("security.metrics_interval"),
		TLS: TLSConfig{
			Enabled:  cm.viper.GetBool("security.tls.enabled"),
			CertFile: cm.viper.GetString("security.tls.cert_file"),
			KeyFile:  cm.viper.GetString("security.tls.key_file"),
		},
		Auth: AuthConfig{
			Enabled:   cm.viper.GetBool("security.auth.enabled"),
			JWTSecret: cm.viper.GetString("security.auth.jwt_secret"),
		},
	}
}

// GetDetectorConfig 获取检测器配置
func (cm *Manager) GetDetectorConfig() DetectorConfig {
	cm.mutex.RLock()
	defer cm.mutex.RUnlock()

	return DetectorConfig{
		BufferSize:    cm.viper.GetInt("detector.buffer_size"),
		ReadTimeout:   cm.viper.GetInt("detector.read_timeout"),
		MaxDetectSize: cm.viper.GetInt("detector.max_detect_size"),
	}
}

// GetProcessorConfig 获取处理器配置
func (cm *Manager) GetProcessorConfig() ProcessorConfig {
	cm.mutex.RLock()
	defer cm.mutex.RUnlock()

	return ProcessorConfig{
		MaxConcurrency: cm.viper.GetInt("processor.max_concurrency"),
		QueueSize:      cm.viper.GetInt("processor.queue_size"),
		Timeout:        cm.viper.GetInt("processor.timeout"),
	}
}

// GetMessageBusConfig 获取消息总线配置
func (cm *Manager) GetMessageBusConfig() MessageBusConfig {
	cm.mutex.RLock()
	defer cm.mutex.RUnlock()

	return MessageBusConfig{
		BufferSize:      cm.viper.GetInt("message_bus.buffer_size"),
		PublishStrategy: cm.viper.GetString("message_bus.publish_strategy"),
		MaxSubscribers:  cm.viper.GetInt("message_bus.max_subscribers"),
	}
}

// Reload 重新加载配置
func (cm *Manager) Reload() error {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()

	if err := cm.viper.ReadInConfig(); err != nil {
		muxErr := errors.ConfigError(errors.ErrCodeConfigParseError, "failed to reload config").WithCause(err)
		return muxErr
	}

	if err := cm.validateConfig(); err != nil {
		return err
	}

	return nil
}
