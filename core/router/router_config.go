package router

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/cocowh/muxcore/pkg/logger"
)

// RouterType 路由器类型
type RouterType string

const (
	HTTPRouterType            RouterType = "http"
	MultidimensionalRouterType RouterType = "multidimensional"
)

// UnifiedRouterConfig 统一路由配置
type UnifiedRouterConfig struct {
	// 基础配置
	Type                RouterType        `json:"type"`
	Name                string            `json:"name"`
	Enabled             bool              `json:"enabled"`
	Description         string            `json:"description"`
	
	// HTTP路由配置
	HTTPConfig          *HTTPRouterConfig `json:"http_config,omitempty"`
	
	// 多维度路由配置
	MultiDimConfig      *MultidimensionalRouterConfig `json:"multi_dim_config,omitempty"`
	
	// 监控配置
	MonitoringConfig    *MonitoringConfig `json:"monitoring_config"`
	
	// 缓存配置
	CacheConfig         *CacheConfig      `json:"cache_config"`
	
	// 性能配置
	PerformanceConfig   *PerformanceConfig `json:"performance_config"`
}

// HTTPRouterConfig HTTP路由器配置
type HTTPRouterConfig struct {
	CaseSensitive       bool              `json:"case_sensitive"`
	StrictSlash         bool              `json:"strict_slash"`
	UseEncodedPath      bool              `json:"use_encoded_path"`
	SkipClean           bool              `json:"skip_clean"`
	MaxRoutes           int               `json:"max_routes"`
	TimeoutDuration     time.Duration     `json:"timeout_duration"`
	EnableCompression   bool              `json:"enable_compression"`
	EnableCORS          bool              `json:"enable_cors"`
	CORSOrigins         []string          `json:"cors_origins"`
}

// MonitoringConfig 监控配置
type MonitoringConfig struct {
	Enabled             bool              `json:"enabled"`
	MetricsInterval     time.Duration     `json:"metrics_interval"`
	HealthCheckInterval time.Duration     `json:"health_check_interval"`
	EnableTracing       bool              `json:"enable_tracing"`
	EnableProfiling     bool              `json:"enable_profiling"`
	MaxMetricsHistory   int               `json:"max_metrics_history"`
	AlertThresholds     *AlertThresholds  `json:"alert_thresholds"`
}

// AlertThresholds 告警阈值
type AlertThresholds struct {
	MaxResponseTime     time.Duration     `json:"max_response_time"`
	MaxErrorRate        float64           `json:"max_error_rate"`
	MaxCPUUsage         float64           `json:"max_cpu_usage"`
	MaxMemoryUsage      float64           `json:"max_memory_usage"`
	MinSuccessRate      float64           `json:"min_success_rate"`
}

// CacheConfig 缓存配置
type CacheConfig struct {
	Enabled             bool              `json:"enabled"`
	Type                string            `json:"type"` // memory, redis, memcached
	TTL                 time.Duration     `json:"ttl"`
	MaxSize             int               `json:"max_size"`
	EvictionPolicy      string            `json:"eviction_policy"` // lru, lfu, fifo
	CompressionEnabled  bool              `json:"compression_enabled"`
	RedisConfig         *RedisConfig      `json:"redis_config,omitempty"`
	MemcachedConfig     *MemcachedConfig  `json:"memcached_config,omitempty"`
}

// RedisConfig Redis配置
type RedisConfig struct {
	Address             string            `json:"address"`
	Password            string            `json:"password"`
	DB                  int               `json:"db"`
	PoolSize            int               `json:"pool_size"`
	MinIdleConns        int               `json:"min_idle_conns"`
	MaxRetries          int               `json:"max_retries"`
	DialTimeout         time.Duration     `json:"dial_timeout"`
	ReadTimeout         time.Duration     `json:"read_timeout"`
	WriteTimeout        time.Duration     `json:"write_timeout"`
}

// MemcachedConfig Memcached配置
type MemcachedConfig struct {
	Servers             []string          `json:"servers"`
	MaxIdleConns        int               `json:"max_idle_conns"`
	Timeout             time.Duration     `json:"timeout"`
}

// PerformanceConfig 性能配置
type PerformanceConfig struct {
	MaxConcurrentRequests int             `json:"max_concurrent_requests"`
	RequestQueueSize      int             `json:"request_queue_size"`
	WorkerPoolSize        int             `json:"worker_pool_size"`
	EnableRateLimiting    bool            `json:"enable_rate_limiting"`
	RateLimit             int             `json:"rate_limit"` // requests per second
	BurstLimit            int             `json:"burst_limit"`
	EnableCircuitBreaker  bool            `json:"enable_circuit_breaker"`
	CircuitBreakerConfig  *CircuitBreakerConfig `json:"circuit_breaker_config"`
}

// CircuitBreakerConfig 熔断器配置
type CircuitBreakerConfig struct {
	FailureThreshold      int             `json:"failure_threshold"`
	RecoveryTimeout       time.Duration   `json:"recovery_timeout"`
	MaxRequests           int             `json:"max_requests"`
	Interval              time.Duration   `json:"interval"`
}

// ConfigManager 配置管理器
type ConfigManager struct {
	configs             map[string]*UnifiedRouterConfig
	mutex               sync.RWMutex
	configFile          string
	hotReloadEnabled    bool
	configValidators    []ConfigValidator
	configChangeHandlers []ConfigChangeHandler
}

// ConfigValidator 配置验证器
type ConfigValidator func(*UnifiedRouterConfig) error

// ConfigChangeHandler 配置变更处理器
type ConfigChangeHandler func(oldConfig, newConfig *UnifiedRouterConfig) error

// NewConfigManager 创建配置管理器
func NewConfigManager(configFile string) *ConfigManager {
	return &ConfigManager{
		configs:             make(map[string]*UnifiedRouterConfig),
		configFile:          configFile,
		hotReloadEnabled:    true,
		configValidators:    make([]ConfigValidator, 0),
		configChangeHandlers: make([]ConfigChangeHandler, 0),
	}
}

// LoadConfig 加载配置
func (cm *ConfigManager) LoadConfig(name string, config *UnifiedRouterConfig) error {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()
	
	// 验证配置
	if err := cm.validateConfig(config); err != nil {
		return fmt.Errorf("config validation failed: %w", err)
	}
	
	// 设置默认值
	cm.setDefaultValues(config)
	
	oldConfig := cm.configs[name]
	cm.configs[name] = config
	
	// 触发配置变更处理器
	for _, handler := range cm.configChangeHandlers {
		if err := handler(oldConfig, config); err != nil {
			logger.Errorf("Config change handler error: %v", err)
		}
	}
	
	logger.Infof("Loaded router config: %s", name)
	return nil
}

// GetConfig 获取配置
func (cm *ConfigManager) GetConfig(name string) (*UnifiedRouterConfig, bool) {
	cm.mutex.RLock()
	defer cm.mutex.RUnlock()
	
	config, exists := cm.configs[name]
	return config, exists
}

// UpdateConfig 更新配置
func (cm *ConfigManager) UpdateConfig(name string, config *UnifiedRouterConfig) error {
	return cm.LoadConfig(name, config)
}

// DeleteConfig 删除配置
func (cm *ConfigManager) DeleteConfig(name string) {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()
	
	delete(cm.configs, name)
	logger.Infof("Deleted router config: %s", name)
}

// ListConfigs 列出所有配置
func (cm *ConfigManager) ListConfigs() []string {
	cm.mutex.RLock()
	defer cm.mutex.RUnlock()
	
	names := make([]string, 0, len(cm.configs))
	for name := range cm.configs {
		names = append(names, name)
	}
	return names
}

// AddValidator 添加配置验证器
func (cm *ConfigManager) AddValidator(validator ConfigValidator) {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()
	
	cm.configValidators = append(cm.configValidators, validator)
}

// AddChangeHandler 添加配置变更处理器
func (cm *ConfigManager) AddChangeHandler(handler ConfigChangeHandler) {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()
	
	cm.configChangeHandlers = append(cm.configChangeHandlers, handler)
}

// validateConfig 验证配置
func (cm *ConfigManager) validateConfig(config *UnifiedRouterConfig) error {
	if config == nil {
		return fmt.Errorf("config cannot be nil")
	}
	
	if config.Name == "" {
		return fmt.Errorf("config name cannot be empty")
	}
	
	if config.Type == "" {
		return fmt.Errorf("router type cannot be empty")
	}
	
	// 验证路由器类型特定配置
	switch config.Type {
	case HTTPRouterType:
		if config.HTTPConfig == nil {
			return fmt.Errorf("HTTP router config is required for HTTP router type")
		}
	case MultidimensionalRouterType:
		if config.MultiDimConfig == nil {
			return fmt.Errorf("multidimensional router config is required for multidimensional router type")
		}
	default:
		return fmt.Errorf("unsupported router type: %s", config.Type)
	}
	
	// 运行自定义验证器
	for _, validator := range cm.configValidators {
		if err := validator(config); err != nil {
			return err
		}
	}
	
	return nil
}

// setDefaultValues 设置默认值
func (cm *ConfigManager) setDefaultValues(config *UnifiedRouterConfig) {
	if config.MonitoringConfig == nil {
		config.MonitoringConfig = &MonitoringConfig{
			Enabled:             true,
			MetricsInterval:     time.Second * 30,
			HealthCheckInterval: time.Second * 60,
			EnableTracing:       false,
			EnableProfiling:     false,
			MaxMetricsHistory:   1000,
			AlertThresholds: &AlertThresholds{
				MaxResponseTime:  time.Second * 5,
				MaxErrorRate:     0.05, // 5%
				MaxCPUUsage:      0.8,  // 80%
				MaxMemoryUsage:   0.8,  // 80%
				MinSuccessRate:   0.95, // 95%
			},
		}
	}
	
	if config.CacheConfig == nil {
		config.CacheConfig = &CacheConfig{
			Enabled:            false,
			Type:               "memory",
			TTL:                time.Minute * 10,
			MaxSize:            1000,
			EvictionPolicy:     "lru",
			CompressionEnabled: false,
		}
	}
	
	if config.PerformanceConfig == nil {
		config.PerformanceConfig = &PerformanceConfig{
			MaxConcurrentRequests: 1000,
			RequestQueueSize:      10000,
			WorkerPoolSize:        100,
			EnableRateLimiting:    false,
			RateLimit:             1000,
			BurstLimit:            100,
			EnableCircuitBreaker:  false,
			CircuitBreakerConfig: &CircuitBreakerConfig{
				FailureThreshold: 5,
				RecoveryTimeout:  time.Second * 30,
				MaxRequests:      10,
				Interval:         time.Second * 60,
			},
		}
	}
	
	// HTTP路由器默认配置
	if config.Type == HTTPRouterType && config.HTTPConfig != nil {
		if config.HTTPConfig.MaxRoutes == 0 {
			config.HTTPConfig.MaxRoutes = 10000
		}
		if config.HTTPConfig.TimeoutDuration == 0 {
			config.HTTPConfig.TimeoutDuration = time.Second * 30
		}
	}
	
	// 多维度路由器默认配置
	if config.Type == MultidimensionalRouterType && config.MultiDimConfig != nil {
		if config.MultiDimConfig.HealthCheckInterval == 0 {
			config.MultiDimConfig.HealthCheckInterval = time.Second * 30
		}
		if config.MultiDimConfig.MaxFailures == 0 {
			config.MultiDimConfig.MaxFailures = 3
		}
		if config.MultiDimConfig.RecoveryThreshold == 0 {
			config.MultiDimConfig.RecoveryThreshold = 5
		}
	}
}

// ExportConfig 导出配置为JSON
func (cm *ConfigManager) ExportConfig(name string) ([]byte, error) {
	cm.mutex.RLock()
	defer cm.mutex.RUnlock()
	
	config, exists := cm.configs[name]
	if !exists {
		return nil, fmt.Errorf("config not found: %s", name)
	}
	
	return json.MarshalIndent(config, "", "  ")
}

// ImportConfig 从JSON导入配置
func (cm *ConfigManager) ImportConfig(name string, data []byte) error {
	var config UnifiedRouterConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return fmt.Errorf("failed to unmarshal config: %w", err)
	}
	
	config.Name = name
	return cm.LoadConfig(name, &config)
}

// GetDefaultHTTPConfig 获取默认HTTP路由配置
func GetDefaultHTTPConfig() *UnifiedRouterConfig {
	return &UnifiedRouterConfig{
		Type:        HTTPRouterType,
		Name:        "default-http",
		Enabled:     true,
		Description: "Default HTTP router configuration",
		HTTPConfig: &HTTPRouterConfig{
			CaseSensitive:     false,
			StrictSlash:       false,
			UseEncodedPath:    false,
			SkipClean:         false,
			MaxRoutes:         10000,
			TimeoutDuration:   time.Second * 30,
			EnableCompression: true,
			EnableCORS:        false,
			CORSOrigins:       []string{"*"},
		},
	}
}

// GetDefaultMultiDimConfig 获取默认多维度路由配置
func GetDefaultMultiDimConfig() *UnifiedRouterConfig {
	return &UnifiedRouterConfig{
		Type:        MultidimensionalRouterType,
		Name:        "default-multidim",
		Enabled:     true,
		Description: "Default multidimensional router configuration",
		MultiDimConfig: &MultidimensionalRouterConfig{
			Strategy:            WeightedRoundRobin,
			HealthCheckInterval: time.Second * 30,
			MaxFailures:         3,
			RecoveryThreshold:   5,
		},
	}
}