package observability

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	"github.com/cocowh/muxcore/pkg/logger"
)

// TracingConfig 分布式追踪配置
type TracingConfig struct {
	// 服务信息
	ServiceName    string `json:"service_name"`
	ServiceVersion string `json:"service_version"`
	Environment    string `json:"environment"`
	
	// 追踪配置
	TracingEnabled bool    `json:"tracing_enabled"`
	ExportEndpoint string  `json:"export_endpoint"`
	SamplingRatio  float64 `json:"sampling_ratio"`
	
	// 指标配置
	MetricsEnabled     bool          `json:"metrics_enabled"`
	MetricsInterval    time.Duration `json:"metrics_interval"`
	MetricsEndpoint    string        `json:"metrics_endpoint"`
	
	// 日志配置
	LoggingEnabled bool   `json:"logging_enabled"`
	LogLevel       string `json:"log_level"`
	
	// 资源属性
	ResourceAttributes map[string]string `json:"resource_attributes"`
}

// DistributedTracing 分布式追踪组件
type DistributedTracing struct {
	config           *TracingConfig
	running          bool
	mutex            sync.RWMutex
	
	// 自定义指标
	counters         map[string]*Counter
	histograms       map[string]*Histogram
	gauges           map[string]*Gauge
	metricsRegistry  sync.RWMutex
	
	// 活跃span追踪
	activeSpans      map[string]*Span
	spansMutex       sync.RWMutex
	
	// 追踪数据存储
	traces           map[string]*Trace
	tracesMutex      sync.RWMutex
	
	// 导出器
	exporters        []TraceExporter
}

// Span 追踪span
type Span struct {
	TraceID      string            `json:"trace_id"`
	SpanID       string            `json:"span_id"`
	ParentSpanID string            `json:"parent_span_id,omitempty"`
	OperationName string           `json:"operation_name"`
	Attributes   map[string]string `json:"attributes"`
	StartTime    time.Time         `json:"start_time"`
	EndTime      *time.Time        `json:"end_time,omitempty"`
	Duration     time.Duration     `json:"duration"`
	Status       string            `json:"status"`
	Error        string            `json:"error,omitempty"`
	Events       []SpanEvent       `json:"events"`
	mutex        sync.RWMutex
}

// SpanEvent span事件
type SpanEvent struct {
	Name       string            `json:"name"`
	Timestamp  time.Time         `json:"timestamp"`
	Attributes map[string]string `json:"attributes"`
}

// Trace 完整的追踪链路
type Trace struct {
	TraceID   string           `json:"trace_id"`
	Spans     map[string]*Span `json:"spans"`
	StartTime time.Time        `json:"start_time"`
	EndTime   *time.Time       `json:"end_time,omitempty"`
	Duration  time.Duration    `json:"duration"`
	mutex     sync.RWMutex
}

// Counter 计数器指标
type Counter struct {
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Value       int64             `json:"value"`
	Labels      map[string]string `json:"labels"`
	mutex       sync.RWMutex
}

// Histogram 直方图指标
type Histogram struct {
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Buckets     []float64         `json:"buckets"`
	Counts      []int64           `json:"counts"`
	Sum         float64           `json:"sum"`
	Count       int64             `json:"count"`
	Labels      map[string]string `json:"labels"`
	mutex       sync.RWMutex
}

// Gauge 仪表指标
type Gauge struct {
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Value       int64             `json:"value"`
	Labels      map[string]string `json:"labels"`
	mutex       sync.RWMutex
}

// TraceExporter 追踪导出器接口
type TraceExporter interface {
	Export(traces []*Trace) error
	Shutdown() error
}

// MetricValue 指标值
type MetricValue struct {
	Name       string            `json:"name"`
	Type       string            `json:"type"`
	Value      float64           `json:"value"`
	Labels     map[string]string `json:"labels"`
	Timestamp  time.Time         `json:"timestamp"`
	Unit       string            `json:"unit"`
}

// NewDistributedTracing 创建分布式追踪组件
func NewDistributedTracing(config *TracingConfig) (*DistributedTracing, error) {
	if config == nil {
		config = getDefaultTracingConfig()
	}
	
	dt := &DistributedTracing{
		config:      config,
		counters:    make(map[string]*Counter),
		histograms:  make(map[string]*Histogram),
		gauges:      make(map[string]*Gauge),
		activeSpans: make(map[string]*Span),
		traces:      make(map[string]*Trace),
		exporters:   make([]TraceExporter, 0),
	}
	
	return dt, nil
}

// Start 启动分布式追踪组件
func (dt *DistributedTracing) Start() error {
	dt.mutex.Lock()
	defer dt.mutex.Unlock()
	
	if dt.running {
		return nil
	}
	
	dt.running = true
	logger.Info("Distributed tracing started")
	return nil
}

// Stop 停止分布式追踪组件
func (dt *DistributedTracing) Stop() error {
	dt.mutex.Lock()
	defer dt.mutex.Unlock()
	
	if !dt.running {
		return nil
	}
	
	// 结束所有活跃span
	dt.spansMutex.Lock()
	for _, span := range dt.activeSpans {
		dt.endSpan(span)
	}
	dt.spansMutex.Unlock()
	
	// 关闭导出器
	for _, exporter := range dt.exporters {
		if err := exporter.Shutdown(); err != nil {
			logger.Errorf("Failed to shutdown exporter: %v", err)
		}
	}
	
	dt.running = false
	logger.Info("Distributed tracing stopped")
	return nil
}

// StartSpan 开始一个新的span
func (dt *DistributedTracing) StartSpan(ctx context.Context, operationName string, parentSpanID ...string) (*Span, context.Context) {
	if !dt.config.TracingEnabled {
		return nil, ctx
	}
	
	traceID := dt.getTraceIDFromContext(ctx)
	if traceID == "" {
		traceID = dt.generateTraceID()
	}
	
	spanID := dt.generateSpanID()
	var parentID string
	if len(parentSpanID) > 0 {
		parentID = parentSpanID[0]
	}
	
	span := &Span{
		TraceID:       traceID,
		SpanID:        spanID,
		ParentSpanID:  parentID,
		OperationName: operationName,
		Attributes:    make(map[string]string),
		StartTime:     time.Now(),
		Status:        "active",
		Events:        make([]SpanEvent, 0),
	}
	
	// 记录活跃span
	dt.spansMutex.Lock()
	dt.activeSpans[spanID] = span
	dt.spansMutex.Unlock()
	
	// 更新或创建trace
	dt.updateTrace(span)
	
	// 将span信息添加到context
	newCtx := dt.addSpanToContext(ctx, span)
	
	return span, newCtx
}

// EndSpan 结束span
func (dt *DistributedTracing) EndSpan(span *Span) {
	if span == nil {
		return
	}
	
	dt.endSpan(span)
}

// endSpan 内部结束span方法
func (dt *DistributedTracing) endSpan(span *Span) {
	span.mutex.Lock()
	if span.EndTime == nil {
		now := time.Now()
		span.EndTime = &now
		span.Duration = now.Sub(span.StartTime)
		if span.Status == "active" {
			span.Status = "completed"
		}
	}
	span.mutex.Unlock()
	
	// 从活跃span中移除
	dt.spansMutex.Lock()
	delete(dt.activeSpans, span.SpanID)
	dt.spansMutex.Unlock()
	
	// 更新trace
	dt.updateTrace(span)
}

// AddSpanEvent 添加span事件
func (dt *DistributedTracing) AddSpanEvent(span *Span, name string, attributes map[string]string) {
	if span == nil {
		return
	}
	
	span.mutex.Lock()
	defer span.mutex.Unlock()
	
	event := SpanEvent{
		Name:       name,
		Timestamp:  time.Now(),
		Attributes: attributes,
	}
	span.Events = append(span.Events, event)
}

// SetSpanAttributes 设置span属性
func (dt *DistributedTracing) SetSpanAttributes(span *Span, attributes map[string]string) {
	if span == nil {
		return
	}
	
	span.mutex.Lock()
	defer span.mutex.Unlock()
	
	for key, value := range attributes {
		span.Attributes[key] = value
	}
}

// RecordError 记录错误到span
func (dt *DistributedTracing) RecordError(span *Span, err error) {
	if span == nil || err == nil {
		return
	}
	
	span.mutex.Lock()
	defer span.mutex.Unlock()
	
	span.Status = "error"
	span.Error = err.Error()
	span.Attributes["error"] = "true"
	span.Attributes["error.message"] = err.Error()
}

// GetOrCreateCounter 获取或创建计数器
func (dt *DistributedTracing) GetOrCreateCounter(name, description string, labels map[string]string) *Counter {
	dt.metricsRegistry.Lock()
	defer dt.metricsRegistry.Unlock()
	
	key := dt.getMetricKey(name, labels)
	if counter, exists := dt.counters[key]; exists {
		return counter
	}
	
	counter := &Counter{
		Name:        name,
		Description: description,
		Value:       0,
		Labels:      labels,
	}
	
	dt.counters[key] = counter
	return counter
}

// GetOrCreateHistogram 获取或创建直方图
func (dt *DistributedTracing) GetOrCreateHistogram(name, description string, labels map[string]string) *Histogram {
	dt.metricsRegistry.Lock()
	defer dt.metricsRegistry.Unlock()
	
	key := dt.getMetricKey(name, labels)
	if histogram, exists := dt.histograms[key]; exists {
		return histogram
	}
	
	// 默认bucket
	buckets := []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10}
	counts := make([]int64, len(buckets))
	
	histogram := &Histogram{
		Name:        name,
		Description: description,
		Buckets:     buckets,
		Counts:      counts,
		Sum:         0,
		Count:       0,
		Labels:      labels,
	}
	
	dt.histograms[key] = histogram
	return histogram
}

// GetOrCreateGauge 获取或创建仪表
func (dt *DistributedTracing) GetOrCreateGauge(name, description string, labels map[string]string) *Gauge {
	dt.metricsRegistry.Lock()
	defer dt.metricsRegistry.Unlock()
	
	key := dt.getMetricKey(name, labels)
	if gauge, exists := dt.gauges[key]; exists {
		return gauge
	}
	
	gauge := &Gauge{
		Name:        name,
		Description: description,
		Value:       0,
		Labels:      labels,
	}
	
	dt.gauges[key] = gauge
	return gauge
}

// IncrementCounter 增加计数器
func (dt *DistributedTracing) IncrementCounter(name string, value int64, labels map[string]string) {
	counter := dt.GetOrCreateCounter(name, fmt.Sprintf("Counter for %s", name), labels)
	counter.mutex.Lock()
	counter.Value += value
	counter.mutex.Unlock()
}

// RecordHistogram 记录直方图值
func (dt *DistributedTracing) RecordHistogram(name string, value float64, labels map[string]string) {
	histogram := dt.GetOrCreateHistogram(name, fmt.Sprintf("Histogram for %s", name), labels)
	histogram.mutex.Lock()
	defer histogram.mutex.Unlock()
	
	histogram.Sum += value
	histogram.Count++
	
	// 更新bucket计数
	for i, bucket := range histogram.Buckets {
		if value <= bucket {
			histogram.Counts[i]++
		}
	}
}

// SetGauge 设置仪表值
func (dt *DistributedTracing) SetGauge(name string, value int64, labels map[string]string) {
	gauge := dt.GetOrCreateGauge(name, fmt.Sprintf("Gauge for %s", name), labels)
	gauge.mutex.Lock()
	gauge.Value = value
	gauge.mutex.Unlock()
}

// GetActiveSpans 获取活跃span列表
func (dt *DistributedTracing) GetActiveSpans() []*Span {
	dt.spansMutex.RLock()
	defer dt.spansMutex.RUnlock()
	
	spans := make([]*Span, 0, len(dt.activeSpans))
	for _, span := range dt.activeSpans {
		spans = append(spans, span)
	}
	return spans
}

// GetTraces 获取所有追踪
func (dt *DistributedTracing) GetTraces() []*Trace {
	dt.tracesMutex.RLock()
	defer dt.tracesMutex.RUnlock()
	
	traces := make([]*Trace, 0, len(dt.traces))
	for _, trace := range dt.traces {
		traces = append(traces, trace)
	}
	return traces
}

// GetTrace 获取指定的追踪
func (dt *DistributedTracing) GetTrace(traceID string) *Trace {
	dt.tracesMutex.RLock()
	defer dt.tracesMutex.RUnlock()
	
	return dt.traces[traceID]
}

// GetMetrics 获取所有指标
func (dt *DistributedTracing) GetMetrics() map[string]interface{} {
	dt.metricsRegistry.RLock()
	defer dt.metricsRegistry.RUnlock()
	
	metrics := make(map[string]interface{})
	
	// 添加计数器
	for key, counter := range dt.counters {
		counter.mutex.RLock()
		metrics[key] = map[string]interface{}{
			"type":        "counter",
			"name":        counter.Name,
			"description": counter.Description,
			"value":       counter.Value,
			"labels":      counter.Labels,
		}
		counter.mutex.RUnlock()
	}
	
	// 添加直方图
	for key, histogram := range dt.histograms {
		histogram.mutex.RLock()
		metrics[key] = map[string]interface{}{
			"type":        "histogram",
			"name":        histogram.Name,
			"description": histogram.Description,
			"buckets":     histogram.Buckets,
			"counts":      histogram.Counts,
			"sum":         histogram.Sum,
			"count":       histogram.Count,
			"labels":      histogram.Labels,
		}
		histogram.mutex.RUnlock()
	}
	
	// 添加仪表
	for key, gauge := range dt.gauges {
		gauge.mutex.RLock()
		metrics[key] = map[string]interface{}{
			"type":        "gauge",
			"name":        gauge.Name,
			"description": gauge.Description,
			"value":       gauge.Value,
			"labels":      gauge.Labels,
		}
		gauge.mutex.RUnlock()
	}
	
	return metrics
}

// 辅助方法

// generateTraceID 生成追踪ID
func (dt *DistributedTracing) generateTraceID() string {
	bytes := make([]byte, 16)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)
}

// generateSpanID 生成SpanID
func (dt *DistributedTracing) generateSpanID() string {
	bytes := make([]byte, 8)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)
}

// getTraceIDFromContext 从上下文获取追踪ID
func (dt *DistributedTracing) getTraceIDFromContext(ctx context.Context) string {
	if traceID, ok := ctx.Value("trace_id").(string); ok {
		return traceID
	}
	return ""
}

// addSpanToContext 将span添加到上下文
func (dt *DistributedTracing) addSpanToContext(ctx context.Context, span *Span) context.Context {
	ctx = context.WithValue(ctx, "trace_id", span.TraceID)
	ctx = context.WithValue(ctx, "span_id", span.SpanID)
	ctx = context.WithValue(ctx, "span", span)
	return ctx
}

// updateTrace 更新追踪信息
func (dt *DistributedTracing) updateTrace(span *Span) {
	dt.tracesMutex.Lock()
	defer dt.tracesMutex.Unlock()
	
	trace, exists := dt.traces[span.TraceID]
	if !exists {
		trace = &Trace{
			TraceID:   span.TraceID,
			Spans:     make(map[string]*Span),
			StartTime: span.StartTime,
		}
		dt.traces[span.TraceID] = trace
	}
	
	trace.mutex.Lock()
	trace.Spans[span.SpanID] = span
	
	// 更新trace的开始和结束时间
	if span.StartTime.Before(trace.StartTime) {
		trace.StartTime = span.StartTime
	}
	
	if span.EndTime != nil {
		if trace.EndTime == nil || span.EndTime.After(*trace.EndTime) {
			trace.EndTime = span.EndTime
			trace.Duration = trace.EndTime.Sub(trace.StartTime)
		}
	}
	trace.mutex.Unlock()
}

// getMetricKey 生成指标键
func (dt *DistributedTracing) getMetricKey(name string, labels map[string]string) string {
	key := name
	for k, v := range labels {
		key += fmt.Sprintf(",%s=%s", k, v)
	}
	return key
}

// AddExporter 添加导出器
func (dt *DistributedTracing) AddExporter(exporter TraceExporter) {
	dt.exporters = append(dt.exporters, exporter)
}

// getDefaultTracingConfig 获取默认配置
func getDefaultTracingConfig() *TracingConfig {
	return &TracingConfig{
		ServiceName:        "muxcore",
		ServiceVersion:     "1.0.0",
		Environment:        "development",
		TracingEnabled:     true,
		ExportEndpoint:     "http://localhost:14268/api/traces",
		SamplingRatio:      1.0,
		MetricsEnabled:     true,
		MetricsInterval:    time.Second * 30,
		MetricsEndpoint:    ":9090",
		LoggingEnabled:     true,
		LogLevel:           "info",
		ResourceAttributes: map[string]string{
			"component": "muxcore",
			"version":   "1.0.0",
		},
	}
}