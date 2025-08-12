// Copyright (c) 2025 cocowh. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package control

import (
	"fmt"

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

// PlaneBuilder builds a control plane for managing network protocols and services.
// It allows for the configuration of various components such as router,
// listener, handler, pool, reliability, performance, observability, and detector.
type PlaneBuilder struct {
	configManager *config.Manager
}

// convertToRouterConfig 将config包的配置转换为router包的配置
func convertToRouterConfig(cfg *config.OptimizedRouterConfig) *router.OptimizedRouterConfig {
	return &router.OptimizedRouterConfig{
		MaxConcurrency:      cfg.MaxConcurrency,
		RequestTimeout:      cfg.RequestTimeout,
		EnableLoadBalancing: cfg.EnableLoadBalancing,
		EnableCaching:       cfg.EnableCaching,
		EnableFailover:      cfg.EnableFailover,
		EnableMetrics:       cfg.EnableMetrics,
		HealthCheckInterval: cfg.HealthCheckInterval,
		CacheSize:           cfg.CacheSize,
		CacheTTL:            cfg.CacheTTL,
		LoadBalanceStrategy: router.LoadBalanceStrategy(cfg.LoadBalanceStrategy),
		FailoverThreshold:   cfg.FailoverThreshold,
		MetricsInterval:     cfg.MetricsInterval,
	}
}

// NewControlPlaneBuilderWithLogConfig create a new control plane builder with log configuration
func NewControlPlaneBuilderWithLogConfig(configPath, logLevel string, verbose bool) (*PlaneBuilder, error) {
	configManager, err := config.NewConfigManager(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create config manager: %w", err)
	}

	if err = initializeLoggerWithOverrides(configManager, logLevel, verbose); err != nil {
		return nil, fmt.Errorf("failed to initialize logger: %w", err)
	}

	return &PlaneBuilder{
		configManager: configManager,
	}, nil
}

// initializeLoggerWithOverrides 初始化日志系统，支持命令行参数覆盖
func initializeLoggerWithOverrides(configManager *config.Manager, logLevel string, verbose bool) error {
	loggerConfig := configManager.GetLoggerConfig()

	finalLevel := loggerConfig.Level
	if logLevel != "" {
		finalLevel = logLevel
	}
	if verbose {
		finalLevel = "debug"
	}

	c := &logger.Config{
		LogDir:          loggerConfig.LogDir,
		BaseName:        loggerConfig.BaseName,
		Format:          loggerConfig.Format,
		Level:           logger.ParseLevel(finalLevel),
		Compress:        loggerConfig.Compress,
		TimeFormat:      loggerConfig.TimeFormat,
		MaxSizeMB:       loggerConfig.MaxSizeMB,
		MaxBackups:      loggerConfig.MaxBackups,
		MaxAgeDays:      loggerConfig.MaxAgeDays,
		EnableWarnFile:  loggerConfig.EnableWarnFile,
		EnableErrorFile: loggerConfig.EnableErrorFile,
	}

	return logger.InitDefaultLogger(c)
}

// Build 构建控制平面
func (b *PlaneBuilder) Build() (*Plane, error) {
	// create goroutine pool
	poolConfig := b.configManager.GetPoolConfig()
	goroutinePool := pool.NewGoroutinePool(poolConfig.Goroutine.Workers, poolConfig.Goroutine.QueueSize)

	// create buffer pool
	bufferPool := performance.NewBufferPool()

	// create connection pool
	connectionPool := pool.New()

	// create protocolDetector
	protocolDetector := detector.New(connectionPool, bufferPool)

	// create observability
	optimizedObservabilityConfig := b.configManager.GetObservabilityConfig()
	optimizedObservability, err := observability.NewOptimizedObservability(optimizedObservabilityConfig)
	if err != nil {
		return nil, err
	}

	// create processor manager
	processorManager := handler.NewProcessorManager()

	// create message bus
	messageBus := bus.NewMessageBus()

	// create reliability manager
	reliabilityManager := reliability.NewReliabilityManager(connectionPool)

	// create optimized router
	routerConfig := convertToRouterConfig(b.configManager.GetOptimizedRouterConfig())
	optimizedRouter := router.NewOptimizedRouter(routerConfig)

	// 构造控制平面实例
	controlPlane := &Plane{
		configManager:      b.configManager,
		goroutinePool:      goroutinePool,
		bufferPool:         bufferPool,
		connectionPool:     connectionPool,
		detector:           protocolDetector,
		observability:      optimizedObservability,
		processorManager:   processorManager,
		messageBus:         messageBus,
		reliabilityManager: reliabilityManager,
		optimizedRouter:    optimizedRouter,
	}

	// 注册协议处理器
	if err := b.registerProtocolHandlers(controlPlane); err != nil {
		return nil, fmt.Errorf("failed to register protocol handlers: %w", err)
	}

	// create listener
	serverConfig := b.configManager.GetServerConfig()
	controlPlane.listener = listener.New(serverConfig.Address, controlPlane.detector, controlPlane.connectionPool, controlPlane.goroutinePool, controlPlane.bufferPool)

	return controlPlane, nil
}

// registerProtocolHandlers 注册协议处理器
func (b *PlaneBuilder) registerProtocolHandlers(cp *Plane) error {
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
