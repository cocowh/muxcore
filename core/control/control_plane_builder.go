// Copyright (c) 2025 cocowh. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package control

import (
	"fmt"
	"time"

	"github.com/cocowh/muxcore/core/bus"
	"github.com/cocowh/muxcore/core/config"
	"github.com/cocowh/muxcore/core/detector"
	"github.com/cocowh/muxcore/core/handler"
	"github.com/cocowh/muxcore/core/listener"
	"github.com/cocowh/muxcore/core/observability"
	"github.com/cocowh/muxcore/core/performance"
	"github.com/cocowh/muxcore/core/pool"
	"github.com/cocowh/muxcore/core/protocols/grpc"
	"github.com/cocowh/muxcore/core/protocols/http"
	"github.com/cocowh/muxcore/core/protocols/websocket"
	"github.com/cocowh/muxcore/core/reliability"
	"github.com/cocowh/muxcore/core/router"
	"github.com/cocowh/muxcore/pkg/logger"
)

// ControlPlaneBuilder 控制平面构建器
type ControlPlaneBuilder struct {
	configManager *config.ConfigManager
}

// NewControlPlaneBuilder 创建控制平面构建器
func NewControlPlaneBuilder(configPath string) (*ControlPlaneBuilder, error) {
	configManager, err := config.NewConfigManager(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create config manager: %w", err)
	}

	return &ControlPlaneBuilder{
		configManager: configManager,
	}, nil
}

// NewControlPlaneBuilderWithLogConfig 创建控制平面构建器并初始化日志
func NewControlPlaneBuilderWithLogConfig(configPath, logLevel string, verbose bool) (*ControlPlaneBuilder, error) {
	configManager, err := config.NewConfigManager(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create config manager: %w", err)
	}

	// 初始化日志系统，支持命令行参数覆盖
	if err := initializeLoggerWithOverrides(configManager, logLevel, verbose); err != nil {
		return nil, fmt.Errorf("failed to initialize logger: %w", err)
	}

	return &ControlPlaneBuilder{
		configManager: configManager,
	}, nil
}

// initializeLogger 初始化日志系统
func initializeLogger(configManager *config.ConfigManager) error {
	loggerConfig := configManager.GetLoggerConfig()

	// 创建logger配置
	config := &logger.Config{
		LogDir:          loggerConfig.LogDir,
		BaseName:        loggerConfig.BaseName,
		Format:          loggerConfig.Format,
		Level:           logger.ParseLevel(loggerConfig.Level),
		Compress:        true,
		TimeFormat:      "2006-01-02 15:04:05",
		MaxSizeMB:       loggerConfig.MaxSizeMB,
		MaxBackups:      loggerConfig.MaxBackups,
		MaxAgeDays:      loggerConfig.MaxAgeDays,
		EnableWarnFile:  loggerConfig.EnableWarnFile,
		EnableErrorFile: loggerConfig.EnableErrorFile,
	}

	// 使用配置初始化日志系统
	return logger.InitDefaultLogger(config)
}

// initializeLoggerWithOverrides 初始化日志系统，支持命令行参数覆盖
func initializeLoggerWithOverrides(configManager *config.ConfigManager, logLevel string, verbose bool) error {
	loggerConfig := configManager.GetLoggerConfig()

	// 确定最终的日志级别
	finalLevel := loggerConfig.Level
	if logLevel != "" {
		finalLevel = logLevel
	}
	if verbose {
		finalLevel = "debug"
	}

	// 创建logger配置
	config := &logger.Config{
		LogDir:          loggerConfig.LogDir,
		BaseName:        loggerConfig.BaseName,
		Format:          loggerConfig.Format,
		Level:           logger.ParseLevel(finalLevel),
		Compress:        true,
		TimeFormat:      "2006-01-02 15:04:05",
		MaxSizeMB:       loggerConfig.MaxSizeMB,
		MaxBackups:      loggerConfig.MaxBackups,
		MaxAgeDays:      loggerConfig.MaxAgeDays,
		EnableWarnFile:  loggerConfig.EnableWarnFile,
		EnableErrorFile: loggerConfig.EnableErrorFile,
	}

	// 使用配置初始化日志系统
	return logger.InitDefaultLogger(config)
}

// Build 构建控制平面
func (b *ControlPlaneBuilder) Build() (*ControlPlane, error) {
	// 获取配置
	poolConfig := b.configManager.GetPoolConfig()
	observabilityConfig := b.configManager.GetObservabilityConfig()
	serverConfig := b.configManager.GetServerConfig()

	// 创建协程池
	goroutinePool := pool.NewGoroutinePool(poolConfig.Goroutine.Workers, poolConfig.Goroutine.QueueSize)

	// 创建缓冲池
	bufferPool := performance.NewBufferPool()

	// 创建连接池
	connectionPool := pool.New()

	// 创建协议检测器
	detector := detector.New(connectionPool, bufferPool)

	// 创建可观测性组件
	optimizedObservabilityConfig := &observability.OptimizedObservabilityConfig{
		MetricsEnabled:      true,
		MetricsInterval:     10 * time.Second,
		MetricsRetention:    24 * time.Hour,
		MetricsBufferSize:   10000,
		TracingEnabled:      observabilityConfig.Enabled,
		SamplingRate:        observabilityConfig.SamplingRate,
		MaxSpansPerTrace:    1000,
		TraceTimeout:        30 * time.Second,
		LoggingEnabled:      true,
		LogLevel:            "info",
		LogBufferSize:       5000,
		LogFlushInterval:    5 * time.Second,
		PerformanceEnabled:  true,
		ResourceMonitoring:  true,
		HealthCheckEnabled:  true,
		HealthCheckInterval: 30 * time.Second,
		AlertingEnabled:     true,
		AlertThresholds: map[string]float64{
			"cpu_usage":     80.0,
			"memory_usage":  85.0,
			"error_rate":    5.0,
			"response_time": 1000.0,
		},
		AlertCooldown: 5 * time.Minute,
	}

	// 创建可观测性组件
	observability := observability.NewOptimizedObservability(optimizedObservabilityConfig)

	// 创建处理器管理器
	processorManager := handler.NewProcessorManager()

	// 创建消息总线
	messageBus := bus.NewMessageBus()

	// 创建可靠性管理器
	reliabilityManager := reliability.NewReliabilityManager(connectionPool)

	// 创建路由器
	routerConfig := &router.OptimizedRouterConfig{
		MaxConcurrency:      1000,
		RequestTimeout:      30 * time.Second,
		EnableLoadBalancing: true,
		EnableCaching:       true,
		EnableFailover:      true,
		EnableMetrics:       true,
		HealthCheckInterval: 10 * time.Second,
		CacheSize:           10000,
		CacheTTL:            5 * time.Minute,
		LoadBalanceStrategy: router.RoundRobin,
		FailoverThreshold:   3,
		MetricsInterval:     30 * time.Second,
	}
	optimizedRouter := router.NewOptimizedRouter(routerConfig)

	// 构造控制平面实例
	controlPlane := &ControlPlane{
		configManager:      b.configManager,
		goroutinePool:      goroutinePool,
		bufferPool:         bufferPool,
		connectionPool:     connectionPool,
		detector:           detector,
		observability:      observability,
		processorManager:   processorManager,
		messageBus:         messageBus,
		reliabilityManager: reliabilityManager,
		optimizedRouter:    optimizedRouter,
	}

	// 注册协议处理器
	if err := b.registerProtocolHandlers(controlPlane); err != nil {
		return nil, fmt.Errorf("failed to register protocol handlers: %w", err)
	}

	// 创建并注入网络监听器
	controlPlane.listener = listener.New(serverConfig.Address, controlPlane.detector, controlPlane.connectionPool, controlPlane.goroutinePool, controlPlane.bufferPool)

	return controlPlane, nil
}

// registerProtocolHandlers 注册协议处理器
func (b *ControlPlaneBuilder) registerProtocolHandlers(cp *ControlPlane) error {
	// 获取协议配置
	protocolConfig := b.configManager.GetProtocolConfig()

	// 创建HTTP处理器
	if protocolConfig.HTTP.Enabled {
		httpHandler := http.NewHTTPHandler(cp.connectionPool, cp.optimizedRouter.GetRadixTree())
		cp.processorManager.RegisterHandler("http", httpHandler)
		cp.detector.RegisterHandler("http", httpHandler)
	}

	// 创建gRPC处理器
	if protocolConfig.GRPC.Enabled {
		grpcHandler := grpc.NewGRPCHandler(cp.connectionPool)
		cp.processorManager.RegisterHandler("grpc", grpcHandler)
		cp.detector.RegisterHandler("grpc", grpcHandler)
	}

	// 创建WebSocket处理器
	if protocolConfig.WebSocket.Enabled {
		websocketHandler := websocket.NewWebSocketHandler(cp.connectionPool, cp.messageBus)
		cp.processorManager.RegisterHandler("websocket", websocketHandler)
		cp.detector.RegisterHandler("websocket", websocketHandler)
	}

	return nil
}
