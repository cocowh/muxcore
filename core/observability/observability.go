package observability

import (
	"sync"

	"github.com/cocowh/muxcore/pkg/logger"
)

// Metrics 指标收集器
type Metrics struct {
	metrics map[string]float64
	mutex   sync.RWMutex
}

// NewMetrics 创建指标收集器
func NewMetrics() *Metrics {
	return &Metrics{
		metrics: make(map[string]float64),
	}
}

// Record 记录指标
func (m *Metrics) Record(name string, value float64) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.metrics[name] = value
}

// Get 获取指标值
func (m *Metrics) Get(name string) (float64, bool) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	value, exists := m.metrics[name]
	return value, exists
}

// GetAll 获取所有指标
func (m *Metrics) GetAll() map[string]float64 {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	result := make(map[string]float64)
	for k, v := range m.metrics {
		result[k] = v
	}
	return result
}

// Tracing 追踪系统
type Tracing struct {
	// 这里可以集成OpenTelemetry等追踪系统
}

// NewTracing 创建追踪系统
func NewTracing() *Tracing {
	return &Tracing{}
}

// StartSpan 开始一个追踪 span
func (t *Tracing) StartSpan(name string) {
	// 简化实现
	logger.Info("Started span: ", name)
}

// EndSpan 结束一个追踪 span
func (t *Tracing) EndSpan(name string) {
	// 简化实现
	logger.Info("Ended span: ", name)
}

// Observability 可观测性组件
type Observability struct {
	metrics *Metrics
	tracing *Tracing
}

// New 创建可观测性组件
func New() *Observability {
	return &Observability{
		metrics: NewMetrics(),
		tracing: NewTracing(),
	}
}

// RecordMetric 记录指标
func (o *Observability) RecordMetric(name string, value float64) {
	o.metrics.Record(name, value)
	logger.Debug("Recorded metric: ", name, " = ", value)
}

// GetMetric 获取指标
func (o *Observability) GetMetric(name string) (float64, bool) {
	return o.metrics.Get(name)
}

// StartTracingSpan 开始追踪 span
func (o *Observability) StartTracingSpan(name string) {
	o.tracing.StartSpan(name)
}

// EndTracingSpan 结束追踪 span
func (o *Observability) EndTracingSpan(name string) {
	o.tracing.EndSpan(name)
}