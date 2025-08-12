// Copyright (c) 2025 cocowh. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package observability

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/cocowh/muxcore/pkg/errors"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cocowh/muxcore/pkg/logger"
)

// Config for observability configuration.
type Config struct {
	// metrics
	MetricsEnabled    bool          `json:"metrics_enabled"`
	MetricsInterval   time.Duration `json:"metrics_interval"`
	MetricsRetention  time.Duration `json:"metrics_retention"`
	MetricsBufferSize int           `json:"metrics_buffer_size"`

	// tracing
	TracingEnabled   bool          `json:"tracing_enabled"`
	SamplingRate     float64       `json:"sampling_rate"`
	MaxSpansPerTrace int           `json:"max_spans_per_trace"`
	TraceTimeout     time.Duration `json:"trace_timeout"`

	// performance monitoring
	PerformanceEnabled  bool          `json:"performance_enabled"`
	ResourceMonitoring  bool          `json:"resource_monitoring"`
	HealthCheckEnabled  bool          `json:"health_check_enabled"`
	HealthCheckInterval time.Duration `json:"health_check_interval"`

	// alerting
	AlertingEnabled bool               `json:"alerting_enabled"`
	AlertThresholds map[string]float64 `json:"alert_thresholds"`
	AlertCooldown   time.Duration      `json:"alert_cooldown"`
}

// MetricPoint metric point
type MetricPoint struct {
	Name      string                 `json:"name"`
	Value     float64                `json:"value"`
	Timestamp time.Time              `json:"timestamp"`
	Labels    map[string]string      `json:"labels"`
	Metadata  map[string]interface{} `json:"metadata"`
}

// Span span
type Span struct {
	TraceID   string            `json:"trace_id"`
	SpanID    string            `json:"span_id"`
	ParentID  string            `json:"parent_id"`
	Operation string            `json:"operation"`
	StartTime time.Time         `json:"start_time"`
	EndTime   time.Time         `json:"end_time"`
	Duration  time.Duration     `json:"duration"`
	Tags      map[string]string `json:"tags"`
	Logs      []LogEntry        `json:"logs"`
	Status    SpanStatus        `json:"status"`
}

// SpanStatus span status
type SpanStatus struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// LogEntry log entry
type LogEntry struct {
	Timestamp time.Time              `json:"timestamp"`
	Level     string                 `json:"level"`
	Message   string                 `json:"message"`
	Fields    map[string]interface{} `json:"fields"`
}

// Alert alert
type Alert struct {
	ID           string                 `json:"id"`
	Metric       string                 `json:"metric"`
	Threshold    float64                `json:"threshold"`
	CurrentValue float64                `json:"current_value"`
	Severity     string                 `json:"severity"`
	Message      string                 `json:"message"`
	Timestamp    time.Time              `json:"timestamp"`
	Metadata     map[string]interface{} `json:"metadata"`
}

// HealthStatus 健康状态
type HealthStatus struct {
	Status    string                 `json:"status"`
	Timestamp time.Time              `json:"timestamp"`
	Checks    map[string]CheckResult `json:"checks"`
	Uptime    time.Duration          `json:"uptime"`
	Version   string                 `json:"version"`
}

// CheckResult 检查结果
type CheckResult struct {
	Status   string                 `json:"status"`
	Message  string                 `json:"message"`
	Duration time.Duration          `json:"duration"`
	Metadata map[string]interface{} `json:"metadata"`
}

// OptimizedMetrics 优化的指标收集器
type OptimizedMetrics struct {
	metrics     map[string]*MetricSeries
	buffer      chan *MetricPoint
	mu          sync.RWMutex
	config      *Config
	ctx         context.Context
	cancel      context.CancelFunc
	lastCleanup time.Time
}

// MetricSeries 指标时间序列
type MetricSeries struct {
	Name       string         `json:"name"`
	Points     []*MetricPoint `json:"points"`
	LastUpdate time.Time      `json:"last_update"`
	mu         sync.RWMutex
}

// NewOptimizedMetrics 创建优化的指标收集器
func NewOptimizedMetrics(config *Config) *OptimizedMetrics {
	ctx, cancel := context.WithCancel(context.Background())

	m := &OptimizedMetrics{
		metrics:     make(map[string]*MetricSeries),
		buffer:      make(chan *MetricPoint, config.MetricsBufferSize),
		config:      config,
		ctx:         ctx,
		cancel:      cancel,
		lastCleanup: time.Now(),
	}

	// 启动指标处理协程
	go m.processMetrics()
	go m.cleanupMetrics()

	return m
}

// Record 记录指标
func (m *OptimizedMetrics) Record(name string, value float64, labels map[string]string) {
	point := &MetricPoint{
		Name:      name,
		Value:     value,
		Timestamp: time.Now(),
		Labels:    labels,
	}

	select {
	case m.buffer <- point:
		// 成功加入缓冲区
	default:
		// 缓冲区满，丢弃指标
		logger.Error("Metrics buffer full, dropping metric", "name", name)
	}
}

// processMetrics 处理指标
func (m *OptimizedMetrics) processMetrics() {
	for {
		select {
		case <-m.ctx.Done():
			return
		case point := <-m.buffer:
			m.storeMetric(point)
		}
	}
}

// storeMetric 存储指标
func (m *OptimizedMetrics) storeMetric(point *MetricPoint) {
	m.mu.Lock()
	defer m.mu.Unlock()

	series, exists := m.metrics[point.Name]
	if !exists {
		series = &MetricSeries{
			Name:   point.Name,
			Points: make([]*MetricPoint, 0),
		}
		m.metrics[point.Name] = series
	}

	series.mu.Lock()
	series.Points = append(series.Points, point)
	series.LastUpdate = time.Now()
	series.mu.Unlock()
}

// cleanupMetrics 清理过期指标
func (m *OptimizedMetrics) cleanupMetrics() {
	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			m.performCleanup()
		}
	}
}

// performCleanup 执行清理
func (m *OptimizedMetrics) performCleanup() {
	m.mu.Lock()
	defer m.mu.Unlock()

	cutoff := time.Now().Add(-m.config.MetricsRetention)

	for name, series := range m.metrics {
		series.mu.Lock()

		// 清理过期数据点
		validPoints := make([]*MetricPoint, 0)
		for _, point := range series.Points {
			if point.Timestamp.After(cutoff) {
				validPoints = append(validPoints, point)
			}
		}

		series.Points = validPoints
		series.mu.Unlock()

		// 如果序列为空且长时间未更新，删除它
		if len(validPoints) == 0 && series.LastUpdate.Before(cutoff) {
			delete(m.metrics, name)
		}
	}

	m.lastCleanup = time.Now()
	logger.Info("Metrics cleanup completed", "series_count", len(m.metrics))
}

// GetMetric 获取指标
func (m *OptimizedMetrics) GetMetric(name string) (*MetricSeries, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	series, exists := m.metrics[name]
	return series, exists
}

// GetAllMetrics 获取所有指标
func (m *OptimizedMetrics) GetAllMetrics() map[string]*MetricSeries {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[string]*MetricSeries)
	for name, series := range m.metrics {
		result[name] = series
	}
	return result
}

// OptimizedTracing 优化的追踪系统
type OptimizedTracing struct {
	spans       map[string]*Span
	activeSpans map[string]*Span
	buffer      chan *Span
	mu          sync.RWMutex
	config      *Config
	ctx         context.Context
	cancel      context.CancelFunc
	sampleCount int64
}

// NewOptimizedTracing 创建优化的追踪系统
func NewOptimizedTracing(config *Config) *OptimizedTracing {
	ctx, cancel := context.WithCancel(context.Background())

	t := &OptimizedTracing{
		spans:       make(map[string]*Span),
		activeSpans: make(map[string]*Span),
		buffer:      make(chan *Span, 1000),
		config:      config,
		ctx:         ctx,
		cancel:      cancel,
	}

	// 启动追踪处理协程
	go t.processSpans()
	go t.cleanupSpans()

	return t
}

// StartSpan 开始跨度
func (t *OptimizedTracing) StartSpan(traceID, spanID, parentID, operation string, tags map[string]string) *Span {
	// 采样检查
	if !t.shouldSample() {
		return nil
	}

	span := &Span{
		TraceID:   traceID,
		SpanID:    spanID,
		ParentID:  parentID,
		Operation: operation,
		StartTime: time.Now(),
		Tags:      tags,
		Logs:      make([]LogEntry, 0),
		Status:    SpanStatus{Code: 0, Message: "OK"},
	}

	t.mu.Lock()
	t.activeSpans[spanID] = span
	t.mu.Unlock()

	return span
}

// FinishSpan 结束跨度
func (t *OptimizedTracing) FinishSpan(spanID string) {
	t.mu.Lock()
	span, exists := t.activeSpans[spanID]
	if exists {
		delete(t.activeSpans, spanID)
	}
	t.mu.Unlock()

	if !exists {
		return
	}

	span.EndTime = time.Now()
	span.Duration = span.EndTime.Sub(span.StartTime)

	select {
	case t.buffer <- span:
		// 成功加入缓冲区
	default:
		// 缓冲区满，丢弃跨度
		logger.Error("Tracing buffer full, dropping span", "span_id", spanID)
	}
}

// shouldSample 检查是否应该采样
func (t *OptimizedTracing) shouldSample() bool {
	count := atomic.AddInt64(&t.sampleCount, 1)
	return float64(count%100)/100.0 < t.config.SamplingRate
}

// processSpans 处理跨度
func (t *OptimizedTracing) processSpans() {
	for {
		select {
		case <-t.ctx.Done():
			return
		case span := <-t.buffer:
			t.storeSpan(span)
		}
	}
}

// storeSpan 存储跨度
func (t *OptimizedTracing) storeSpan(span *Span) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.spans[span.SpanID] = span
}

// cleanupSpans 清理过期跨度
func (t *OptimizedTracing) cleanupSpans() {
	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-t.ctx.Done():
			return
		case <-ticker.C:
			t.performSpanCleanup()
		}
	}
}

// performSpanCleanup 执行跨度清理
func (t *OptimizedTracing) performSpanCleanup() {
	t.mu.Lock()
	defer t.mu.Unlock()

	cutoff := time.Now().Add(-t.config.TraceTimeout)

	for spanID, span := range t.spans {
		if span.EndTime.Before(cutoff) {
			delete(t.spans, spanID)
		}
	}

	// 清理超时的活跃跨度
	for spanID, span := range t.activeSpans {
		if span.StartTime.Before(cutoff) {
			delete(t.activeSpans, spanID)
		}
	}
}

// GetSpan 获取跨度
func (t *OptimizedTracing) GetSpan(spanID string) (*Span, bool) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	span, exists := t.spans[spanID]
	return span, exists
}

// AlertManager 告警管理器
type AlertManager struct {
	alerts    map[string]*Alert
	lastAlert map[string]time.Time
	mu        sync.RWMutex
	config    *Config
	ctx       context.Context
	cancel    context.CancelFunc
}

// NewAlertManager 创建告警管理器
func NewAlertManager(config *Config) *AlertManager {
	ctx, cancel := context.WithCancel(context.Background())

	return &AlertManager{
		alerts:    make(map[string]*Alert),
		lastAlert: make(map[string]time.Time),
		config:    config,
		ctx:       ctx,
		cancel:    cancel,
	}
}

// CheckThreshold 检查阈值
func (am *AlertManager) CheckThreshold(metric string, value float64) {
	if !am.config.AlertingEnabled {
		return
	}

	threshold, exists := am.config.AlertThresholds[metric]
	if !exists {
		return
	}

	if value > threshold {
		am.triggerAlert(metric, threshold, value)
	}
}

// triggerAlert 触发告警
func (am *AlertManager) triggerAlert(metric string, threshold, value float64) {
	am.mu.Lock()
	defer am.mu.Unlock()

	// 检查冷却时间
	if lastTime, exists := am.lastAlert[metric]; exists {
		if time.Since(lastTime) < am.config.AlertCooldown {
			return
		}
	}

	alert := &Alert{
		ID:           fmt.Sprintf("%s_%d", metric, time.Now().Unix()),
		Metric:       metric,
		Threshold:    threshold,
		CurrentValue: value,
		Severity:     am.getSeverity(metric, threshold, value),
		Message:      fmt.Sprintf("Metric %s exceeded threshold: %.2f > %.2f", metric, value, threshold),
		Timestamp:    time.Now(),
	}

	am.alerts[alert.ID] = alert
	am.lastAlert[metric] = time.Now()

	logger.Error("Alert triggered", "metric", metric, "value", value, "threshold", threshold)
}

// getSeverity get severity
func (am *AlertManager) getSeverity(metric string, threshold, value float64) string {
	excess := (value - threshold) / threshold
	switch {
	case excess > 0.5:
		return "critical"
	case excess > 0.2:
		return "warning"
	default:
		return "info"
	}
}

// HealthChecker health checker
type HealthChecker struct {
	checks     map[string]func() CheckResult
	lastStatus *HealthStatus
	mu         sync.RWMutex
	config     *Config
	startTime  time.Time
}

// NewHealthChecker create health checker
func NewHealthChecker(config *Config) *HealthChecker {
	hc := &HealthChecker{
		checks:    make(map[string]func() CheckResult),
		config:    config,
		startTime: time.Now(),
	}

	// register default checks
	hc.RegisterCheck("memory", hc.checkMemory)
	hc.RegisterCheck("goroutines", hc.checkGoroutines)

	return hc
}

// RegisterCheck register check
func (hc *HealthChecker) RegisterCheck(name string, check func() CheckResult) {
	hc.mu.Lock()
	defer hc.mu.Unlock()
	hc.checks[name] = check
}

// PerformHealthCheck perform health check
func (hc *HealthChecker) PerformHealthCheck() *HealthStatus {
	hc.mu.RLock()
	checks := make(map[string]func() CheckResult)
	for name, check := range hc.checks {
		checks[name] = check
	}
	hc.mu.RUnlock()

	results := make(map[string]CheckResult)
	allHealthy := true

	for name, check := range checks {
		start := time.Now()
		result := check()
		result.Duration = time.Since(start)
		results[name] = result

		if result.Status != "healthy" {
			allHealthy = false
		}
	}

	status := &HealthStatus{
		Status:    "healthy",
		Timestamp: time.Now(),
		Checks:    results,
		Uptime:    time.Since(hc.startTime),
		Version:   "1.0.0",
	}

	if !allHealthy {
		status.Status = "unhealthy"
	}

	hc.mu.Lock()
	hc.lastStatus = status
	hc.mu.Unlock()

	return status
}

// checkMemory checks memory usage and returns a CheckResult
func (hc *HealthChecker) checkMemory() CheckResult {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	memoryUsageMB := float64(m.Alloc) / 1024 / 1024
	status := "healthy"
	message := fmt.Sprintf("Memory usage: %.2f MB", memoryUsageMB)

	if memoryUsageMB > 1000 { // 1GB
		status = "warning"
		message = fmt.Sprintf("High memory usage: %.2f MB", memoryUsageMB)
	}

	if memoryUsageMB > 2000 { // 2GB
		status = "critical"
		message = fmt.Sprintf("Critical memory usage: %.2f MB", memoryUsageMB)
	}

	return CheckResult{
		Status:  status,
		Message: message,
		Metadata: map[string]interface{}{
			"alloc_mb":       memoryUsageMB,
			"total_alloc_mb": float64(m.TotalAlloc) / 1024 / 1024,
			"sys_mb":         float64(m.Sys) / 1024 / 1024,
			"gc_count":       m.NumGC,
		},
	}
}

// checkGoroutines 检查协程数量
func (hc *HealthChecker) checkGoroutines() CheckResult {
	goroutineCount := runtime.NumGoroutine()
	status := "healthy"
	message := fmt.Sprintf("Goroutines: %d", goroutineCount)

	if goroutineCount > 1000 {
		status = "warning"
		message = fmt.Sprintf("High goroutine count: %d", goroutineCount)
	}

	if goroutineCount > 5000 {
		status = "critical"
		message = fmt.Sprintf("Critical goroutine count: %d", goroutineCount)
	}

	return CheckResult{
		Status:  status,
		Message: message,
		Metadata: map[string]interface{}{
			"count": goroutineCount,
		},
	}
}

// Observability system
type Observability struct {
	config        *Config
	metrics       *OptimizedMetrics
	tracing       *OptimizedTracing
	alertManager  *AlertManager
	healthChecker *HealthChecker
	ctx           context.Context
	cancel        context.CancelFunc
}

// NewOptimizedObservability creates a new instance of the Observability system.
func NewOptimizedObservability(config *Config) (*Observability, error) {
	if config == nil {
		return nil, errors.New(errors.ErrCodeConfigNotFound, errors.CategoryConfig, errors.LevelError, "config is nil")
	}

	ctx, cancel := context.WithCancel(context.Background())

	o := &Observability{
		config:        config,
		metrics:       NewOptimizedMetrics(config),
		tracing:       NewOptimizedTracing(config),
		alertManager:  NewAlertManager(config),
		healthChecker: NewHealthChecker(config),
		ctx:           ctx,
		cancel:        cancel,
	}

	// start performance and health check monitors
	if config.PerformanceEnabled {
		go o.performanceMonitor()
	}
	if config.HealthCheckEnabled {
		go o.healthMonitor()
	}

	return o, nil
}

// RecordMetric record metric
func (o *Observability) RecordMetric(name string, value float64, labels map[string]string) {
	if !o.config.MetricsEnabled {
		return
	}

	o.metrics.Record(name, value, labels)

	// send to alert manager
	o.alertManager.CheckThreshold(name, value)
}

// StartSpan start span
func (o *Observability) StartSpan(traceID, spanID, parentID, operation string, tags map[string]string) *Span {
	if !o.config.TracingEnabled {
		return nil
	}

	return o.tracing.StartSpan(traceID, spanID, parentID, operation, tags)
}

// FinishSpan finish span
func (o *Observability) FinishSpan(spanID string) {
	if !o.config.TracingEnabled {
		return
	}
	o.tracing.FinishSpan(spanID)
}

// performanceMonitor performance monitor
func (o *Observability) performanceMonitor() {
	ticker := time.NewTicker(o.config.MetricsInterval)
	defer ticker.Stop()

	for {
		select {
		case <-o.ctx.Done():
			return
		case <-ticker.C:
			o.collectSystemMetrics()
		}
	}
}

// collectSystemMetrics 收集系统指标
func (o *Observability) collectSystemMetrics() {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	// 内存指标
	o.RecordMetric("memory.alloc", float64(m.Alloc), nil)
	o.RecordMetric("memory.total_alloc", float64(m.TotalAlloc), nil)
	o.RecordMetric("memory.sys", float64(m.Sys), nil)
	o.RecordMetric("memory.heap_alloc", float64(m.HeapAlloc), nil)
	o.RecordMetric("memory.heap_sys", float64(m.HeapSys), nil)

	// GC指标
	o.RecordMetric("gc.num_gc", float64(m.NumGC), nil)
	o.RecordMetric("gc.pause_total_ns", float64(m.PauseTotalNs), nil)

	// 协程指标
	o.RecordMetric("runtime.goroutines", float64(runtime.NumGoroutine()), nil)
	o.RecordMetric("runtime.num_cpu", float64(runtime.NumCPU()), nil)
}

// healthMonitor 健康监控
func (o *Observability) healthMonitor() {
	ticker := time.NewTicker(o.config.HealthCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-o.ctx.Done():
			return
		case <-ticker.C:
			status := o.healthChecker.PerformHealthCheck()
			if status.Status != "healthy" {
				logger.Error("Health check failed", "status", status.Status, "checks", status.Checks)
			}
		}
	}
}

// GetMetrics 获取指标
func (o *Observability) GetMetrics() map[string]*MetricSeries {
	return o.metrics.GetAllMetrics()
}

// GetHealthStatus 获取健康状态
func (o *Observability) GetHealthStatus() *HealthStatus {
	return o.healthChecker.PerformHealthCheck()
}

// GetAlerts 获取告警
func (o *Observability) GetAlerts() map[string]*Alert {
	o.alertManager.mu.RLock()
	defer o.alertManager.mu.RUnlock()

	result := make(map[string]*Alert)
	for id, alert := range o.alertManager.alerts {
		result[id] = alert
	}
	return result
}

// ExportMetrics 导出指标
func (o *Observability) ExportMetrics() ([]byte, error) {
	metrics := o.GetMetrics()
	return json.Marshal(metrics)
}

// Stop stops the observability system.
func (o *Observability) Stop() error {
	o.cancel()

	if o.metrics != nil {
		o.metrics.cancel()
	}

	if o.tracing != nil {
		o.tracing.cancel()
	}

	if o.alertManager != nil {
		o.alertManager.cancel()
	}

	return nil
}
