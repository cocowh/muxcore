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

	// 验证日志配置
	if !cm.viper.IsSet("logger.level") {
		cm.viper.Set("logger.level", "info")
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
		Interval: cm.viper.GetInt("monitor.interval"),
	}
}

// GetLoggerConfig 获取日志配置
func (cm *ConfigManager) GetLoggerConfig() LoggerConfig {
	cm.mutex.RLock()
	defer cm.mutex.RUnlock()

	return LoggerConfig{
		Level:    cm.viper.GetString("logger.level"),
		Output:   cm.viper.GetString("logger.output"),
		FilePath: cm.viper.GetString("logger.file_path"),
		MaxSize:  cm.viper.GetInt("logger.max_size"),
		MaxAge:   cm.viper.GetInt("logger.max_age"),
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

// GetSecurityConfig 获取安全配置
func (cm *ConfigManager) GetSecurityConfig() SecurityConfig {
	cm.mutex.RLock()
	defer cm.mutex.RUnlock()

	return SecurityConfig{
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
	Interval int
}

// LoggerConfig 日志配置
type LoggerConfig struct {
	Level    string
	Output   string
	FilePath string
	MaxSize  int
	MaxAge   int
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
	TLS  TLSConfig
	Auth AuthConfig
}

// TLSConfig TLS配置
type TLSConfig struct {
	Enabled  bool
	CertFile string
	KeyFile  string
}

// AuthConfig 认证配置
type AuthConfig struct {
	Enabled   bool
	JWTSecret string
}
