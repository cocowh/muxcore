package control

import (
	"context"
	"sync"
	"time"

	"github.com/cocowh/muxcore/core/bus"
	"github.com/cocowh/muxcore/core/config"
	"github.com/cocowh/muxcore/core/detector"
	handlers "github.com/cocowh/muxcore/core/handler"
	"github.com/cocowh/muxcore/core/listener"
	"github.com/cocowh/muxcore/core/observability"
	"github.com/cocowh/muxcore/core/performance"
	pkgPool "github.com/cocowh/muxcore/core/pool"
	"github.com/cocowh/muxcore/core/reliability"
	"github.com/cocowh/muxcore/pkg/logger"
)

// ControlPlane 控制平面
type ControlPlane struct {
	configManager      *config.ConfigManager
	listener           *listener.Listener
	detector           *detector.ProtocolDetector
	pool               *pkgPool.ConnectionPool
	goroutinePool      *pkgPool.GoroutinePool
	bufferPool         *performance.BufferPool
	observability      *observability.Observability
	processorManager   *handlers.ProcessorManager
	messageBus         *bus.MessageBus
	reliabilityManager *reliability.ReliabilityManager
	wg                 sync.WaitGroup
	ctx                context.Context
	cancel             context.CancelFunc
}

// New 创建控制平面
func New(configPath string) (*ControlPlane, error) {
	// 使用构建器创建控制平面
	builder, err := NewControlPlaneBuilder(configPath)
	if err != nil {
		return nil, err
	}

	return builder.Build()
}

// Start 启动控制平面
func (c *ControlPlane) Start() error {
	// 启动监听器
	if err := c.listener.Start(); err != nil {
		return err
	}

	// 初始化指标服务
	// 注意：metrics配置在当前版本的配置文件中不存在，我们暂时保持硬编码
	metricsAddr := ":9090"
	observability.InitMetrics(metricsAddr)

	// 启动监控
	c.wg.Add(1)
	go c.monitorLoop()

	logger.Info("Control plane started")
	return nil
}

// Stop 停止控制平面
func (c *ControlPlane) Stop() {
	c.cancel()
	c.listener.Stop()
	c.wg.Wait()
	c.pool.Close()
	c.goroutinePool.Shutdown()
	logger.Info("Control plane stopped")
}

// monitorLoop 监控循环
func (c *ControlPlane) monitorLoop() {
	defer c.wg.Done()

	// 监控间隔
	monitorConfig := c.configManager.GetMonitorConfig()
	interval := time.Duration(monitorConfig.Interval) * time.Second
	if interval == 0 {
		interval = 10 * time.Second
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-c.ctx.Done():
			return
		case <-ticker.C:
			// 收集连接统计信息
			conns := c.pool.GetActiveConnections()
			logger.Info("Active connections: ", len(conns))

			// 收集指标
			c.observability.RecordMetric("active_connections", float64(len(conns)))
		}
	}
}

// GetDetector 获取协议检测器
func (c *ControlPlane) GetDetector() *detector.ProtocolDetector {
	return c.detector
}

// GetObservability 获取可观测性组件
func (c *ControlPlane) GetObservability() *observability.Observability {
	return c.observability
}

// GetPool 获取连接池
func (c *ControlPlane) GetPool() *pkgPool.ConnectionPool {
	return c.pool
}

// GetGoroutinePool 获取goroutine池
func (c *ControlPlane) GetGoroutinePool() *pkgPool.GoroutinePool {
	return c.goroutinePool
}

// GetBufferPool 获取缓冲区池
func (c *ControlPlane) GetBufferPool() *performance.BufferPool {
	return c.bufferPool
}
