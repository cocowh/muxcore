package control

import (
	"context"
	"time"

	"github.com/cocowh/muxcore/core/bus"
	"github.com/cocowh/muxcore/core/config"
	"github.com/cocowh/muxcore/core/detector"
	handlerpkg "github.com/cocowh/muxcore/core/handler"
	grpcpkg "github.com/cocowh/muxcore/core/grpc"
	httppkg "github.com/cocowh/muxcore/core/http"
	"github.com/cocowh/muxcore/core/listener"
	"github.com/cocowh/muxcore/core/observability"
	pkgPool "github.com/cocowh/muxcore/core/pool"
	"github.com/cocowh/muxcore/core/performance"
	"github.com/cocowh/muxcore/core/reliability"
	"github.com/cocowh/muxcore/core/router"
	ws "github.com/cocowh/muxcore/core/websocket"
)

// ControlPlaneBuilder 控制平面构建器
type ControlPlaneBuilder struct {
	configManager  *config.ConfigManager
	ctx            context.Context
	cancel         context.CancelFunc
	pool           *pkgPool.ConnectionPool
	goroutinePool  *pkgPool.GoroutinePool
	bufferPool     *performance.BufferPool
	detector       *detector.ProtocolDetector
	observability  *observability.Observability
	processorManager *handlerpkg.ProcessorManager
	messageBus     *bus.MessageBus
	reliabilityManager *reliability.ReliabilityManager
	listener       *listener.Listener
}

// NewControlPlaneBuilder 创建控制平面构建器
func NewControlPlaneBuilder(configPath string) (*ControlPlaneBuilder, error) {
	// 创建配置管理器
	configManager, err := config.NewConfigManager(configPath)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &ControlPlaneBuilder{
		configManager: configManager,
		ctx:           ctx,
		cancel:        cancel,
	},
	nil
}

// WithPool 设置连接池
func (b *ControlPlaneBuilder) WithPool(pool *pkgPool.ConnectionPool) *ControlPlaneBuilder {
	b.pool = pool
	return b
}

// WithGoroutinePool 设置goroutine池
func (b *ControlPlaneBuilder) WithGoroutinePool(goroutinePool *pkgPool.GoroutinePool) *ControlPlaneBuilder {
	b.goroutinePool = goroutinePool
	return b
}

// WithBufferPool 设置缓冲区池
func (b *ControlPlaneBuilder) WithBufferPool(bufferPool *performance.BufferPool) *ControlPlaneBuilder {
	b.bufferPool = bufferPool
	return b
}

// WithDetector 设置协议检测器
func (b *ControlPlaneBuilder) WithDetector(detector *detector.ProtocolDetector) *ControlPlaneBuilder {
	b.detector = detector
	return b
}

// WithObservability 设置可观测性组件
func (b *ControlPlaneBuilder) WithObservability(observability *observability.Observability) *ControlPlaneBuilder {
	b.observability = observability
	return b
}

// WithProcessorManager 设置处理器管理器
func (b *ControlPlaneBuilder) WithProcessorManager(processorManager *handlerpkg.ProcessorManager) *ControlPlaneBuilder {
	b.processorManager = processorManager
	return b
}

// WithMessageBus 设置消息总线
func (b *ControlPlaneBuilder) WithMessageBus(messageBus *bus.MessageBus) *ControlPlaneBuilder {
	b.messageBus = messageBus
	return b
}

// WithReliabilityManager 设置可靠性管理器
func (b *ControlPlaneBuilder) WithReliabilityManager(reliabilityManager *reliability.ReliabilityManager) *ControlPlaneBuilder {
	b.reliabilityManager = reliabilityManager
	return b
}

// WithListener 设置监听器
func (b *ControlPlaneBuilder) WithListener(listener *listener.Listener) *ControlPlaneBuilder {
	b.listener = listener
	return b
}

// Build 构建控制平面
func (b *ControlPlaneBuilder) Build() (*ControlPlane, error) {
	// 如果没有设置组件，则创建默认组件
	if b.pool == nil {
		b.pool = pkgPool.New()
	}

	if b.goroutinePool == nil {
		// 初始化goroutine池，设置工作协程数为CPU核心数的2倍
		b.goroutinePool = pkgPool.NewGoroutinePool(0, 100) // 0表示使用默认配置，100是队列大小
	}

	if b.bufferPool == nil {
		b.bufferPool = performance.NewBufferPool()
	}

	if b.detector == nil {
		b.detector = detector.New(b.pool, b.bufferPool)
	}

	if b.observability == nil {
		b.observability = observability.New()
	}

	if b.processorManager == nil {
		b.processorManager = handlerpkg.NewProcessorManager()
	}

	if b.messageBus == nil {
		b.messageBus = bus.NewMessageBus()
	}

	if b.reliabilityManager == nil {
		b.reliabilityManager = reliability.NewReliabilityManager(b.pool)
	}

	// 配置熔断三维模型
	httpCircuitBreaker := reliability.NewCircuitBreaker(
		"http",
		0.1,  // 连接失败率阈值
		100,  // 协议失败阈值
		50,   // 服务失败阈值
		30*time.Second,
	)
	b.reliabilityManager.AddCircuitBreaker(httpCircuitBreaker)

	grpcCircuitBreaker := reliability.NewCircuitBreaker(
		"grpc",
		0.15, // 连接失败率阈值
		80,   // 协议失败阈值
		40,   // 服务失败阈值
		30*time.Second,
	)
	b.reliabilityManager.AddCircuitBreaker(grpcCircuitBreaker)

	wsCircuitBreaker := reliability.NewCircuitBreaker(
		"websocket",
		0.2,  // 连接失败率阈值
		50,   // 协议失败阈值
		30,   // 服务失败阈值
		30*time.Second,
	)
	b.reliabilityManager.AddCircuitBreaker(wsCircuitBreaker)

	// 从配置中获取地址
	serverConfig := b.configManager.GetServerConfig()
	addr := serverConfig.Address
	if addr == "" {
		addr = ":8080"
	}

	if b.listener == nil {
		b.listener = listener.New(addr, b.detector, b.pool, b.goroutinePool, b.bufferPool)
	}

	// 创建并注册处理器
	// 创建radix树路由
	radixRouter := router.NewRadixTree()
	// 添加一些示例路由
	radixRouter.Insert("/", []string{"root"})
	radixRouter.Insert("/health", []string{"health"})

	// HTTP处理器
	httpHandler := httppkg.NewHTTPHandler(b.pool, radixRouter)
	b.processorManager.AddProcessor(handlerpkg.ProcessorTypeHTTP, httpHandler)
	b.detector.RegisterHandler("http", httpHandler)

	// WebSocket处理器
	wsHandler := ws.NewWebSocketHandler(b.pool, b.messageBus)
	b.processorManager.AddProcessor(handlerpkg.ProcessorTypeWebSocket, wsHandler)
	b.detector.RegisterHandler("websocket", wsHandler)

	// gRPC处理器
	grpcHandler := grpcpkg.NewGRPCHandler(b.pool)
	b.processorManager.AddProcessor(handlerpkg.ProcessorTypeGRPC, grpcHandler)
	b.detector.RegisterHandler("grpc", grpcHandler)

	return &ControlPlane{
		configManager:      b.configManager,
		listener:           b.listener,
		detector:           b.detector,
		pool:               b.pool,
		goroutinePool:      b.goroutinePool,
		bufferPool:         b.bufferPool,
		observability:      b.observability,
		processorManager:   b.processorManager,
		messageBus:         b.messageBus,
		reliabilityManager: b.reliabilityManager,
		ctx:                b.ctx,
		cancel:             b.cancel,
	},
	nil
}