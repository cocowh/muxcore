// Copyright (c) 2025 cocowh. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package config

import (
	"time"
)

type ServerConfig struct {
	Address      string
	ReadTimeout  int
	WriteTimeout int
	IdleTimeout  int
}

type MonitorConfig struct {
	Interval       int
	MetricsEnabled bool
	MetricsAddress string
}

type LoggerConfig struct {
	Level           string
	Format          string
	LogDir          string
	BaseName        string
	TimeFormat      string
	MaxSizeMB       int
	MaxAgeDays      int
	MaxBackups      int
	Compress        bool
	EnableWarnFile  bool
	EnableErrorFile bool
}

type ProtocolConfig struct {
	HTTP      HTTPConfig
	WebSocket WebSocketConfig
	GRPC      GRPCConfig
	Custom    CustomConfig
}

type HTTPConfig struct {
	Enabled        bool
	MaxConnections int
}

type WebSocketConfig struct {
	Enabled        bool
	MaxConnections int
}

type GRPCConfig struct {
	Enabled        bool
	MaxConnections int
}

type CustomConfig struct {
	Enabled bool
}

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

type TLSConfig struct {
	Enabled  bool   `json:"enabled"`
	CertFile string `json:"cert_file"`
	KeyFile  string `json:"key_file"`
}

type AuthConfig struct {
	Enabled   bool   `json:"enabled"`
	JWTSecret string `json:"jwt_secret"`
}

type DetectorConfig struct {
	BufferSize    int `json:"buffer_size"`
	ReadTimeout   int `json:"read_timeout"`
	MaxDetectSize int `json:"max_detect_size"`
}

type ObservabilityConfig struct {
	Enabled      bool    `json:"enabled"`
	SamplingRate float64 `json:"sampling_rate"`
}

type ProcessorConfig struct {
	MaxConcurrency int `json:"max_concurrency"`
	QueueSize      int `json:"queue_size"`
	Timeout        int `json:"timeout"`
}

type MessageBusConfig struct {
	BufferSize      int    `json:"buffer_size"`
	PublishStrategy string `json:"publish_strategy"`
	MaxSubscribers  int    `json:"max_subscribers"`
}

type PoolConfig struct {
	Connection ConnectionPoolConfig
	Goroutine  GoroutinePoolConfig
	Buffer     BufferPoolConfig
}

type ConnectionPoolConfig struct {
	MaxSize     int
	MaxIdleTime int
}

type GoroutinePoolConfig struct {
	Workers   int
	QueueSize int
}

type BufferPoolConfig struct {
	DefaultSize int
	MaxPoolSize int
}

type ReliabilityConfig struct {
	CircuitBreaker CircuitBreakerConfig
	Degradation    DegradationConfig
}

type CircuitBreakerConfig struct {
	Enabled          bool
	FailureThreshold float64
	ResetTimeout     int
	Protocols        ProtocolThresholds
}

type ProtocolThresholds struct {
	HTTP      ProtocolConfigThreshold
	GRPC      ProtocolConfigThreshold
	WebSocket ProtocolConfigThreshold
}

type ProtocolConfigThreshold struct {
	ProtocolFailureThreshold float64
	ServiceFailureThreshold  float64
}

type DegradationConfig struct {
	Enabled            bool
	MonitoringInterval int
	Thresholds         DegradationThresholds
}

type DegradationThresholds struct {
	Simplified   float64
	CriticalOnly float64
	MaintainOnly float64
}

type RouterConfig struct {
	MaxRoutes          int
	CompressionEnabled bool
}

// 优化模块配置代理结构体 - 这些用于与其他模块通讯

// OptimizedConnectionPoolConfig 优化连接池配置代理
type OptimizedConnectionPoolConfig struct {
	MaxConnections      int           `json:"max_connections"`
	MinConnections      int           `json:"min_connections"`
	MaxIdleTime         time.Duration `json:"max_idle_time"`
	HealthCheckInterval time.Duration `json:"health_check_interval"`
	ConnectionTimeout   time.Duration `json:"connection_timeout"`
	EnableLoadBalancing bool          `json:"enable_load_balancing"`
	EnableMetrics       bool          `json:"enable_metrics"`
	BufferSize          int           `json:"buffer_size"`
}

// OptimizedGoroutinePoolConfig 优化协程池配置代理
type OptimizedGoroutinePoolConfig struct {
	MinWorkers         int           `json:"min_workers"`
	MaxWorkers         int           `json:"max_workers"`
	QueueSize          int           `json:"queue_size"`
	ScaleUpThreshold   float64       `json:"scale_up_threshold"`
	ScaleDownThreshold float64       `json:"scale_down_threshold"`
	ScaleInterval      time.Duration `json:"scale_interval"`
	WorkerTimeout      time.Duration `json:"worker_timeout"`
	EnableMetrics      bool          `json:"enable_metrics"`
}

// LoadBalanceStrategy 负载均衡策略枚举
type LoadBalanceStrategy int

const (
	LoadBalanceStrategyRoundRobin LoadBalanceStrategy = iota
	LoadBalanceStrategyWeightedRoundRobin
	LoadBalanceStrategyLeastConnections
	LoadBalanceStrategyLeastResponseTime
	LoadBalanceStrategyConsistentHash
)

// OptimizedRouterConfig 优化路由器配置代理
type OptimizedRouterConfig struct {
	MaxConcurrency      int                 `json:"max_concurrency"`
	RequestTimeout      time.Duration       `json:"request_timeout"`
	EnableLoadBalancing bool                `json:"enable_load_balancing"`
	EnableCaching       bool                `json:"enable_caching"`
	EnableFailover      bool                `json:"enable_failover"`
	EnableMetrics       bool                `json:"enable_metrics"`
	HealthCheckInterval time.Duration       `json:"health_check_interval"`
	CacheSize           int                 `json:"cache_size"`
	CacheTTL            time.Duration       `json:"cache_ttl"`
	LoadBalanceStrategy LoadBalanceStrategy `json:"load_balance_strategy"`
	FailoverThreshold   int                 `json:"failover_threshold"`
	MetricsInterval     time.Duration       `json:"metrics_interval"`
}

// HTTPProtocolConfig 供HTTP协议使用的独立配置（代理类型）
type HTTPProtocolConfig struct {
	Enabled        bool
	EnableHTTP2    bool
	MaxHeaderSize  int
	ReadTimeout    time.Duration
	WriteTimeout   time.Duration
	IdleTimeout    time.Duration
	EnableGzip     bool
	MaxRequestSize int64
}

// GRPCProtocolConfig 供gRPC协议使用的独立配置（代理类型）
type GRPCProtocolConfig struct {
	Enabled              bool
	MaxRecvMsgSize       int
	MaxSendMsgSize       int
	ConnectionTimeout    time.Duration
	KeepaliveTime        time.Duration
	KeepaliveTimeout     time.Duration
	EnableReflection     bool
	EnableHealthCheck    bool
	MaxConcurrentStreams uint32
}

// WebSocketProtocolConfig 供WebSocket协议使用的独立配置（代理类型）
type WebSocketProtocolConfig struct {
	Enabled           bool
	MaxConnections    int
	PingInterval      time.Duration
	PongTimeout       time.Duration
	MaxMessageSize    int64
	EnableCompression bool
	ReadBufferSize    int
	WriteBufferSize   int
	HandshakeTimeout  time.Duration
}
