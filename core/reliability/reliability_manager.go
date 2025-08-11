// Copyright (c) 2025 cocowh. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package reliability

import (
	"sync"
	"time"

	"github.com/cocowh/muxcore/core/pool"
	"github.com/cocowh/muxcore/pkg/logger"
)

// CircuitBreakerState 熔断状态
type CircuitBreakerState int

// 定义熔断状态
const (
	StateClosed CircuitBreakerState = iota
	StateOpen
	StateHalfOpen
)

// CircuitBreaker 熔断三维模型
type CircuitBreaker struct {
	name                       string
	state                      CircuitBreakerState
	connectionFailureThreshold float64
	protocolFailureThreshold   float64
	serviceFailureThreshold    float64
	resetTimeout               time.Duration
	lastFailureTime            time.Time
	connectionFailures         int
	protocolFailures           int
	serviceFailures            int
	totalConnections           int
	mutex                      sync.RWMutex
}

// DegradationLevel 降级级别
type DegradationLevel int

// 定义降级级别
const (
	DegradationLevelNormal DegradationLevel = iota
	DegradationLevelSimplified
	DegradationLevelCriticalOnly
	DegradationLevelMaintainOnly
)

// ReliabilityManager 可靠性管理器
type ReliabilityManager struct {
	circuitBreakers  map[string]*CircuitBreaker
	degradationLevel DegradationLevel
	mutex            sync.RWMutex
	pool             *pool.ConnectionPool
	// 配置化阈值和周期
	monitorInterval       time.Duration
	thresholdSimplified   float64
	thresholdCriticalOnly float64
	thresholdMaintainOnly float64
}

// NewReliabilityManager 创建可靠性管理器
func NewReliabilityManager(pool *pool.ConnectionPool) *ReliabilityManager {
	manager := &ReliabilityManager{
		circuitBreakers:       make(map[string]*CircuitBreaker),
		degradationLevel:      DegradationLevelNormal,
		pool:                  pool,
		monitorInterval:       10 * time.Second,
		thresholdSimplified:   0.05,
		thresholdCriticalOnly: 0.15,
		thresholdMaintainOnly: 0.30,
	}

	// 启动降级监控
	go manager.degradationMonitorLoop()

	logger.Info("Initialized reliability manager")
	return manager
}

// NewReliabilityManagerWithConfig 创建带配置的可靠性管理器
func NewReliabilityManagerWithConfig(pool *pool.ConnectionPool, enabled bool, intervalSec int, simplified, criticalOnly, maintainOnly float64) *ReliabilityManager {
	if intervalSec <= 0 {
		intervalSec = 10
	}
	manager := &ReliabilityManager{
		circuitBreakers:       make(map[string]*CircuitBreaker),
		degradationLevel:      DegradationLevelNormal,
		pool:                  pool,
		monitorInterval:       time.Duration(intervalSec) * time.Second,
		thresholdSimplified:   simplified,
		thresholdCriticalOnly: criticalOnly,
		thresholdMaintainOnly: maintainOnly,
	}

	if enabled {
		go manager.degradationMonitorLoop()
		logger.Info("Initialized reliability manager with config and degradation monitor enabled")
	} else {
		logger.Info("Initialized reliability manager with config, degradation monitor disabled")
	}
	return manager
}

// NewCircuitBreaker 创建熔断三维模型
func NewCircuitBreaker(name string, connectionThreshold, protocolThreshold, serviceThreshold float64, resetTimeout time.Duration) *CircuitBreaker {
	return &CircuitBreaker{
		name:                       name,
		state:                      StateClosed,
		connectionFailureThreshold: connectionThreshold,
		protocolFailureThreshold:   protocolThreshold,
		serviceFailureThreshold:    serviceThreshold,
		resetTimeout:               resetTimeout,
	}
}

// AddCircuitBreaker 添加熔断三维模型
func (m *ReliabilityManager) AddCircuitBreaker(circuitBreaker *CircuitBreaker) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	m.circuitBreakers[circuitBreaker.name] = circuitBreaker
	logger.Info("Added circuit breaker: ", circuitBreaker.name)
}

// RecordConnectionFailure 记录连接失败
func (c *CircuitBreaker) RecordConnectionFailure() {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	c.connectionFailures++
	c.totalConnections++
	c.lastFailureTime = time.Now()

	c.checkThresholds()
}

// RecordProtocolFailure 记录协议失败
func (c *CircuitBreaker) RecordProtocolFailure() {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	c.protocolFailures++
	c.lastFailureTime = time.Now()

	c.checkThresholds()
}

// RecordServiceFailure 记录服务失败
func (c *CircuitBreaker) RecordServiceFailure() {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	c.serviceFailures++
	c.lastFailureTime = time.Now()

	c.checkThresholds()
}

// checkThresholds 检查阈值
func (c *CircuitBreaker) checkThresholds() {
	// 如果已经打开，不需要检查
	if c.state == StateOpen {
		return
	}

	// 计算失败率
	connectionFailureRate := float64(c.connectionFailures) / float64(c.totalConnections)
	// 这里简化实现，实际应该考虑时间窗口

	// 检查是否超过阈值
	if connectionFailureRate > c.connectionFailureThreshold ||
		float64(c.protocolFailures) > c.protocolFailureThreshold ||
		float64(c.serviceFailures) > c.serviceFailureThreshold {
		c.state = StateOpen
		logger.Warn("Circuit breaker opened: ", c.name)
	}
}

// AllowRequest 检查是否允许请求
func (c *CircuitBreaker) AllowRequest() bool {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	switch c.state {
	case StateClosed:
		return true
	case StateOpen:
		// 检查是否可以尝试半开
		if time.Since(c.lastFailureTime) > c.resetTimeout {
			// 需要写锁来修改状态
			c.mutex.RUnlock()
			c.mutex.Lock()
			// 双重检查
			if c.state == StateOpen && time.Since(c.lastFailureTime) > c.resetTimeout {
				c.state = StateHalfOpen
				logger.Info("Circuit breaker half-opened: ", c.name)
			}
			c.mutex.Unlock()
			c.mutex.RLock()
			return c.state == StateHalfOpen
		}
		return false
	case StateHalfOpen:
		return true
	default:
		return false
	}
}

// RecordSuccess 记录成功请求
func (c *CircuitBreaker) RecordSuccess() {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if c.state == StateHalfOpen {
		c.state = StateClosed
		c.connectionFailures = 0
		c.protocolFailures = 0
		c.serviceFailures = 0
		c.totalConnections = 0
		logger.Info("Circuit breaker closed: ", c.name)
	}
}

// GetState 获取熔断状态
func (c *CircuitBreaker) GetState() CircuitBreakerState {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	return c.state
}

// SetDegradationLevel 设置降级级别
func (m *ReliabilityManager) SetDegradationLevel(level DegradationLevel) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	m.degradationLevel = level
	logger.Info("Set degradation level to: ", level)

	// 根据降级级别执行相应操作
	switch level {
	case DegradationLevelSimplified:
		logger.Info("Entering simplified protocol parsing mode")
	case DegradationLevelCriticalOnly:
		logger.Info("Entering critical protocols only mode")
	case DegradationLevelMaintainOnly:
		logger.Info("Entering maintain existing connections only mode")
	}
}

// GetDegradationLevel 获取降级级别
func (m *ReliabilityManager) GetDegradationLevel() DegradationLevel {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	return m.degradationLevel
}

// degradationMonitorLoop 降级监控循环
func (m *ReliabilityManager) degradationMonitorLoop() {
	ticker := time.NewTicker(m.monitorInterval)
	defer ticker.Stop()
	for range ticker.C {
		m.evaluateDegradationLevel()
	}
}

// evaluateDegradationLevel 评估降级级别
func (m *ReliabilityManager) evaluateDegradationLevel() {
	// 获取连接数和错误率
	conns := m.pool.GetActiveConnections()
	// 简化实现：实际应用中需要收集真实错误率

	// 模拟错误率计算
	errorRate := float64(len(conns)%100) / 100.0

	// 根据错误率调整降级级别
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if errorRate > m.thresholdMaintainOnly {
		m.degradationLevel = DegradationLevelMaintainOnly
	} else if errorRate > m.thresholdCriticalOnly {
		m.degradationLevel = DegradationLevelCriticalOnly
	} else if errorRate > m.thresholdSimplified {
		m.degradationLevel = DegradationLevelSimplified
	} else {
		m.degradationLevel = DegradationLevelNormal
	}

	logger.Debug("Current degradation level: ", m.degradationLevel, " error rate: ", errorRate)
}
