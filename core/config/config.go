// Copyright (c) 2025 cocowh. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package config

import (
	"sync"

	"github.com/cocowh/muxcore/pkg/errors"
	"github.com/spf13/viper"
)

// ConfigManager 配置管理器
type ConfigManager struct {
	viper   *viper.Viper
	configs map[string]interface{}
	mutex   sync.RWMutex
}

// NewConfigManager 创建配置管理器
func NewConfigManager(configPath string) (*ConfigManager, error) {
	// 初始化viper
	v := viper.New()
	v.SetConfigFile(configPath)

	// 读取配置文件
	if err := v.ReadInConfig(); err != nil {
		muxErr := errors.ConfigError(errors.ErrCodeConfigNotFound, "failed to read config file").WithCause(err).WithContext("config_path", configPath)
		return nil, muxErr
	}

	// 创建配置管理器
	cm := &ConfigManager{
		viper:   v,
		configs: make(map[string]interface{}),
	}

	// 验证配置
	if err := cm.validateConfig(); err != nil {
		muxErr := errors.Convert(err)
		return nil, muxErr.WithContext("operation", "validate_config")
	}

	return cm, nil
}

// validateConfig 验证配置
func (cm *ConfigManager) validateConfig() error {
	// 验证服务器配置
	if !cm.viper.IsSet("server.address") {
		cm.viper.Set("server.address", ":8080")
	}

	// 验证监控配置
	if !cm.viper.IsSet("monitor.interval") {
		cm.viper.Set("monitor.interval", 10)
	}
	// 监控指标配置默认值
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
	if !cm.viper.IsSet("logger.output") {
		cm.viper.Set("logger.output", "console")
	}
	if !cm.viper.IsSet("logger.base_name") {
		cm.viper.Set("logger.base_name", "muxcore")
	}
	if !cm.viper.IsSet("logger.log_dir") {
		cm.viper.Set("logger.log_dir", "./logs")
	}
	if !cm.viper.IsSet("logger.max_size_mb") {
		cm.viper.Set("logger.max_size_mb", 100)
	}
	if !cm.viper.IsSet("logger.max_age_days") {
		cm.viper.Set("logger.max_age_days", 7)
	}
	if !cm.viper.IsSet("logger.max_backups") {
		cm.viper.Set("logger.max_backups", 3)
	}
	if !cm.viper.IsSet("logger.enable_warn_file") {
		cm.viper.Set("logger.enable_warn_file", false)
	}
	if !cm.viper.IsSet("logger.enable_error_file") {
		cm.viper.Set("logger.enable_error_file", true)
	}

	// 验证池配置
	if !cm.viper.IsSet("pools.connection.max_size") {
		cm.viper.Set("pools.connection.max_size", 1000)
	}
	if !cm.viper.IsSet("pools.connection.max_idle_time") {
		cm.viper.Set("pools.connection.max_idle_time", 300)
	}
	if !cm.viper.IsSet("pools.goroutine.workers") {
		cm.viper.Set("pools.goroutine.workers", 0) // 0 means use CPU count
	}
	if !cm.viper.IsSet("pools.goroutine.queue_size") {
		cm.viper.Set("pools.goroutine.queue_size", 1000)
	}
	if !cm.viper.IsSet("pools.buffer.default_size") {
		cm.viper.Set("pools.buffer.default_size", 4096)
	}
	if !cm.viper.IsSet("pools.buffer.max_pool_size") {
		cm.viper.Set("pools.buffer.max_pool_size", 1000)
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

	// 协议特定熔断器配置
	if !cm.viper.IsSet("reliability.circuit_breaker.protocols.http.protocol_failure_threshold") {
		cm.viper.Set("reliability.circuit_breaker.protocols.http.protocol_failure_threshold", 100)
	}
	if !cm.viper.IsSet("reliability.circuit_breaker.protocols.http.service_failure_threshold") {
		cm.viper.Set("reliability.circuit_breaker.protocols.http.service_failure_threshold", 50)
	}
	if !cm.viper.IsSet("reliability.circuit_breaker.protocols.grpc.protocol_failure_threshold") {
		cm.viper.Set("reliability.circuit_breaker.protocols.grpc.protocol_failure_threshold", 80)
	}
	if !cm.viper.IsSet("reliability.circuit_breaker.protocols.grpc.service_failure_threshold") {
		cm.viper.Set("reliability.circuit_breaker.protocols.grpc.service_failure_threshold", 40)
	}
	if !cm.viper.IsSet("reliability.circuit_breaker.protocols.websocket.protocol_failure_threshold") {
		cm.viper.Set("reliability.circuit_breaker.protocols.websocket.protocol_failure_threshold", 50)
	}
	if !cm.viper.IsSet("reliability.circuit_breaker.protocols.websocket.service_failure_threshold") {
		cm.viper.Set("reliability.circuit_breaker.protocols.websocket.service_failure_threshold", 30)
	}

	// 降级配置
	if !cm.viper.IsSet("reliability.degradation.enabled") {
		cm.viper.Set("reliability.degradation.enabled", true)
	}
	if !cm.viper.IsSet("reliability.degradation.monitoring_interval") {
		cm.viper.Set("reliability.degradation.monitoring_interval", 10)
	}
	if !cm.viper.IsSet("reliability.degradation.thresholds.simplified") {
		cm.viper.Set("reliability.degradation.thresholds.simplified", 0.05)
	}
	if !cm.viper.IsSet("reliability.degradation.thresholds.critical_only") {
		cm.viper.Set("reliability.degradation.thresholds.critical_only", 0.15)
	}
	if !cm.viper.IsSet("reliability.degradation.thresholds.maintain_only") {
		cm.viper.Set("reliability.degradation.thresholds.maintain_only", 0.30)
	}

	// 验证路由器配置
	if !cm.viper.IsSet("router.max_routes") {
		cm.viper.Set("router.max_routes", 10000)
	}
	if !cm.viper.IsSet("router.compression_enabled") {
		cm.viper.Set("router.compression_enabled", true)
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

	// 验证安全配置
	if !cm.viper.IsSet("security.enabled") {
		cm.viper.Set("security.enabled", true)
	}
	if !cm.viper.IsSet("security.security_level") {
		cm.viper.Set("security.security_level", 1) // Medium
	}
	if !cm.viper.IsSet("security.dos_threshold") {
		cm.viper.Set("security.dos_threshold", 100)
	}
	if !cm.viper.IsSet("security.dos_window_size") {
		cm.viper.Set("security.dos_window_size", 60)
	}
	if !cm.viper.IsSet("security.event_buffer_size") {
		cm.viper.Set("security.event_buffer_size", 1000)
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

	// 验证处理器配置
	if !cm.viper.IsSet("processor.max_concurrency") {
		cm.viper.Set("processor.max_concurrency", 1000)
	}
	if !cm.viper.IsSet("processor.queue_size") {
		cm.viper.Set("processor.queue_size", 10000)
	}

	// 验证消息总线配置
	if !cm.viper.IsSet("message_bus.buffer_size") {
		cm.viper.Set("message_bus.buffer_size", 1000)
	}
	if !cm.viper.IsSet("message_bus.max_subscribers") {
		cm.viper.Set("message_bus.max_subscribers", 100)
	}

	return nil
}

// GetServerConfig 获取服务器配置
func (cm *ConfigManager) GetServerConfig() ServerConfig {
	cm.mutex.RLock()
	defer cm.mutex.RUnlock()

	return ServerConfig{
		Address:      cm.viper.GetString("server.address"),
		ReadTimeout:  cm.viper.GetInt("server.read_timeout"),
		WriteTimeout: cm.viper.GetInt("server.write_timeout"),
		IdleTimeout:  cm.viper.GetInt("server.idle_timeout"),
	}
}

// GetMonitorConfig 获取监控配置
func (cm *ConfigManager) GetMonitorConfig() MonitorConfig {
	cm.mutex.RLock()
	defer cm.mutex.RUnlock()

	return MonitorConfig{
		Interval:       cm.viper.GetInt("monitor.interval"),
		MetricsEnabled: cm.viper.GetBool("monitor.metrics.enabled"),
		MetricsAddress: cm.viper.GetString("monitor.metrics.address"),
	}
}

// GetLoggerConfig 获取日志配置
func (cm *ConfigManager) GetLoggerConfig() LoggerConfig {
	cm.mutex.RLock()
	defer cm.mutex.RUnlock()

	return LoggerConfig{
		Level:           cm.viper.GetString("logger.level"),
		Format:          cm.viper.GetString("logger.format"),
		Output:          cm.viper.GetString("logger.output"),
		LogDir:          cm.viper.GetString("logger.log_dir"),
		BaseName:        cm.viper.GetString("logger.base_name"),
		MaxSizeMB:       cm.viper.GetInt("logger.max_size_mb"),
		MaxAgeDays:      cm.viper.GetInt("logger.max_age_days"),
		MaxBackups:      cm.viper.GetInt("logger.max_backups"),
		EnableWarnFile:  cm.viper.GetBool("logger.enable_warn_file"),
		EnableErrorFile: cm.viper.GetBool("logger.enable_error_file"),
	}
}

// GetProtocolConfig 获取协议配置
func (cm *ConfigManager) GetProtocolConfig() ProtocolConfig {
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

// Reload 重新加载配置
func (cm *ConfigManager) Reload() error {
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

// ServerConfig 服务器配置
type ServerConfig struct {
	Address      string
	ReadTimeout  int
	WriteTimeout int
	IdleTimeout  int
}

// MonitorConfig 监控配置
type MonitorConfig struct {
	Interval       int
	MetricsEnabled bool
	MetricsAddress string
}

// LoggerConfig 日志配置
type LoggerConfig struct {
	Level           string
	Format          string
	Output          string
	LogDir          string
	BaseName        string
	MaxSizeMB       int
	MaxAgeDays      int
	MaxBackups      int
	EnableWarnFile  bool
	EnableErrorFile bool
}

// ProtocolConfig 协议配置
type ProtocolConfig struct {
	HTTP      HTTPConfig
	WebSocket WebSocketConfig
	GRPC      GRPCConfig
	Custom    CustomConfig
}

// HTTPConfig HTTP协议配置
type HTTPConfig struct {
	Enabled        bool
	MaxConnections int
}

// WebSocketConfig WebSocket协议配置
type WebSocketConfig struct {
	Enabled        bool
	MaxConnections int
}

// GRPCConfig GRPC协议配置
type GRPCConfig struct {
	Enabled        bool
	MaxConnections int
}

// CustomConfig 自定义协议配置
type CustomConfig struct {
	Enabled bool
}

// SecurityConfig 安全配置
type SecurityConfig struct {
	Enabled                  bool       `json:"enabled"`
	SecurityLevel            int        `json:"security_level"`
	DOSThreshold             int        `json:"dos_threshold"`
	DOSWindowSize            int        `json:"dos_window_size"`
	DOSCleanupInterval       int        `json:"dos_cleanup_interval"`
	DOSBlockDuration         int        `json:"dos_block_duration"`
	SYNCookieCleanupInterval int        `json:"syn_cookie_cleanup_interval"`
	DPIMaxPacketSize         int        `json:"dpi_max_packet_size"`
	DefaultAuthPolicy        string     `json:"default_auth_policy"`
	EventBufferSize          int        `json:"event_buffer_size"`
	MetricsInterval          int        `json:"metrics_interval"`
	TLS                      TLSConfig  `json:"tls"`
	Auth                     AuthConfig `json:"auth"`
}

// TLSConfig TLS配置
type TLSConfig struct {
	Enabled  bool   `json:"enabled"`
	CertFile string `json:"cert_file"`
	KeyFile  string `json:"key_file"`
}

// AuthConfig 认证配置
type AuthConfig struct {
	Enabled   bool   `json:"enabled"`
	JWTSecret string `json:"jwt_secret"`
}

// DetectorConfig 协议检测器配置
type DetectorConfig struct {
	BufferSize    int `json:"buffer_size"`
	ReadTimeout   int `json:"read_timeout"`
	MaxDetectSize int `json:"max_detect_size"`
}

// ObservabilityConfig 可观测性配置
type ObservabilityConfig struct {
	Enabled      bool    `json:"enabled"`
	SamplingRate float64 `json:"sampling_rate"`
	ExporterType string  `json:"exporter_type"`
	ExporterAddr string  `json:"exporter_addr"`
}

// ProcessorConfig 处理器配置
type ProcessorConfig struct {
	MaxConcurrency int `json:"max_concurrency"`
	QueueSize      int `json:"queue_size"`
	Timeout        int `json:"timeout"`
}

// MessageBusConfig 消息总线配置
type MessageBusConfig struct {
	BufferSize      int    `json:"buffer_size"`
	PublishStrategy string `json:"publish_strategy"`
	MaxSubscribers  int    `json:"max_subscribers"`
}

// GetPoolConfig 获取池配置
func (cm *ConfigManager) GetPoolConfig() PoolConfig {
	cm.mutex.RLock()
	defer cm.mutex.RUnlock()

	return PoolConfig{
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
func (cm *ConfigManager) GetReliabilityConfig() ReliabilityConfig {
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
func (cm *ConfigManager) GetRouterConfig() RouterConfig {
	cm.mutex.RLock()
	defer cm.mutex.RUnlock()

	return RouterConfig{
		MaxRoutes:          cm.viper.GetInt("router.max_routes"),
		CompressionEnabled: cm.viper.GetBool("router.compression_enabled"),
	}
}

// GetSecurityConfig 获取安全配置
func (cm *ConfigManager) GetSecurityConfig() SecurityConfig {
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
func (cm *ConfigManager) GetDetectorConfig() DetectorConfig {
	cm.mutex.RLock()
	defer cm.mutex.RUnlock()

	return DetectorConfig{
		BufferSize:    cm.viper.GetInt("detector.buffer_size"),
		ReadTimeout:   cm.viper.GetInt("detector.read_timeout"),
		MaxDetectSize: cm.viper.GetInt("detector.max_detect_size"),
	}
}

// GetObservabilityConfig 获取可观测性配置
func (cm *ConfigManager) GetObservabilityConfig() ObservabilityConfig {
	cm.mutex.RLock()
	defer cm.mutex.RUnlock()

	return ObservabilityConfig{
		Enabled:      cm.viper.GetBool("observability.enabled"),
		SamplingRate: cm.viper.GetFloat64("observability.sampling_rate"),
		ExporterType: cm.viper.GetString("observability.exporter_type"),
		ExporterAddr: cm.viper.GetString("observability.exporter_addr"),
	}
}

// GetProcessorConfig 获取处理器配置
func (cm *ConfigManager) GetProcessorConfig() ProcessorConfig {
	cm.mutex.RLock()
	defer cm.mutex.RUnlock()

	return ProcessorConfig{
		MaxConcurrency: cm.viper.GetInt("processor.max_concurrency"),
		QueueSize:      cm.viper.GetInt("processor.queue_size"),
		Timeout:        cm.viper.GetInt("processor.timeout"),
	}
}

// GetMessageBusConfig 获取消息总线配置
func (cm *ConfigManager) GetMessageBusConfig() MessageBusConfig {
	cm.mutex.RLock()
	defer cm.mutex.RUnlock()

	return MessageBusConfig{
		BufferSize:      cm.viper.GetInt("message_bus.buffer_size"),
		PublishStrategy: cm.viper.GetString("message_bus.publish_strategy"),
		MaxSubscribers:  cm.viper.GetInt("message_bus.max_subscribers"),
	}
}

// PoolConfig 池配置
type PoolConfig struct {
	Connection ConnectionPoolConfig
	Goroutine  GoroutinePoolConfig
	Buffer     BufferPoolConfig
}

// ConnectionPoolConfig 连接池配置
type ConnectionPoolConfig struct {
	MaxSize     int
	MaxIdleTime int
}

// GoroutinePoolConfig Goroutine池配置
type GoroutinePoolConfig struct {
	Workers   int
	QueueSize int
}

// BufferPoolConfig 缓冲池配置
type BufferPoolConfig struct {
	DefaultSize int
	MaxPoolSize int
}

// ReliabilityConfig 可靠性配置
type ReliabilityConfig struct {
	CircuitBreaker CircuitBreakerConfig
	Degradation    DegradationConfig
}

// CircuitBreakerConfig 熔断器配置
type CircuitBreakerConfig struct {
	Enabled          bool
	FailureThreshold float64
	ResetTimeout     int
	Protocols        ProtocolThresholds
}

// ProtocolThresholds 不同协议的失败阈值配置
type ProtocolThresholds struct {
	HTTP      ProtocolConfigThreshold
	GRPC      ProtocolConfigThreshold
	WebSocket ProtocolConfigThreshold
}

// ProtocolConfigThreshold 单个协议的阈值
type ProtocolConfigThreshold struct {
	ProtocolFailureThreshold float64
	ServiceFailureThreshold  float64
}

// DegradationConfig 降级配置
type DegradationConfig struct {
	Enabled            bool
	MonitoringInterval int
	Thresholds         DegradationThresholds
}

// DegradationThresholds 降级阈值
type DegradationThresholds struct {
	Simplified   float64
	CriticalOnly float64
	MaintainOnly float64
}

// RouterConfig 路由器配置
type RouterConfig struct {
	MaxRoutes          int
	CompressionEnabled bool
}
