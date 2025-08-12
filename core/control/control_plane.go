// Copyright (c) 2025 cocowh. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package control

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/cocowh/muxcore/core/bus"
	"github.com/cocowh/muxcore/core/config"
	"github.com/cocowh/muxcore/core/detector"
	"github.com/cocowh/muxcore/core/handler"
	"github.com/cocowh/muxcore/core/listener"
	"github.com/cocowh/muxcore/core/observability"
	"github.com/cocowh/muxcore/core/performance"
	"github.com/cocowh/muxcore/core/pool"
	"github.com/cocowh/muxcore/core/reliability"
	"github.com/cocowh/muxcore/core/router"
	"github.com/cocowh/muxcore/pkg/logger"
)

// Plane 控制平面
type Plane struct {
	configManager      *config.Manager
	goroutinePool      *pool.GoroutinePool
	bufferPool         *performance.BufferPool
	connectionPool     *pool.ConnectionPool
	detector           *detector.ProtocolDetector
	observability      *observability.Observability
	processorManager   *handler.ProcessorManager
	messageBus         *bus.MessageBus
	reliabilityManager *reliability.ReliabilityManager
	optimizedRouter    *router.OptimizedRouter
	listener           *listener.Listener

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
	mu     sync.RWMutex
	state  string
}

// Start 启动控制平面
func (cp *Plane) Start() error {
	cp.mu.Lock()
	defer cp.mu.Unlock()

	if cp.state == "running" {
		return fmt.Errorf("control plane is already running")
	}

	cp.ctx, cp.cancel = context.WithCancel(context.Background())
	cp.state = "starting"

	logger.Infof("Starting control plane...")

	// 启动路由器
	if err := cp.optimizedRouter.Start(); err != nil {
		return fmt.Errorf("failed to start router: %w", err)
	}

	// 启动网络监听器
	if err := cp.listener.Start(); err != nil {
		return fmt.Errorf("failed to start listener: %w", err)
	}

	// 启动监控循环
	cp.wg.Add(1)
	go cp.monitorLoop()

	cp.state = "running"
	logger.Infof("Control plane started successfully")

	return nil
}

// Stop stops the control plane.
func (cp *Plane) Stop() error {
	cp.mu.Lock()
	defer cp.mu.Unlock()

	if cp.state != "running" {
		return fmt.Errorf("control plane is not running")
	}

	cp.state = "stopping"
	logger.Infof("Stopping control plane...")

	// 取消上下文
	if cp.cancel != nil {
		cp.cancel()
	}

	// 停止监听器
	cp.listener.Stop()

	// 停止路由器
	cp.optimizedRouter.Stop()

	// 停止可观测性组件
	cp.observability.Stop()

	// 等待所有goroutine完成
	cp.wg.Wait()

	cp.state = "stopped"
	logger.Infof("Control plane stopped successfully")

	return nil
}

// GetState gets the control plane state
func (cp *Plane) GetState() string {
	cp.mu.RLock()
	defer cp.mu.RUnlock()
	return cp.state
}

// monitorLoop monitors the control plane health
func (cp *Plane) monitorLoop() {
	defer cp.wg.Done()

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-cp.ctx.Done():
			return
		case <-ticker.C:
			cp.performHealthCheck()
		}
	}
}

// performHealthCheck performs health check
func (cp *Plane) performHealthCheck() {
	// 记录健康检查指标
	cp.observability.RecordMetric("health_check", 1, map[string]string{
		"component": "control_plane",
		"status":    "healthy",
	})

	logger.Debugf("Health check completed")
}
