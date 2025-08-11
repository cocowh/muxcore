package observability

import (
	"context"
	"encoding/json"
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cocowh/muxcore/pkg/logger"
)

// OptimizedObservabilityConfig 优化的可观测性配置
type OptimizedObservabilityConfig struct {
	// 指标配置
	MetricsEnabled    bool          `json:"metrics_enabled"`
	MetricsInterval   time.Duration `json:"metrics_interval"`
	MetricsRetention  time.Duration `json:"metrics_retention"`
	MetricsBufferSize int           `json:"metrics_buffer_size"`

	// 追踪配置
	TracingEnabled   bool          `json:"tracing_enabled"`
	SamplingRate     float64       `json:"sampling_rate"`
	MaxSpansPerTrace int           `json:"max_spans_per_trace"`
	TraceTimeout     time.Duration `json:"trace_timeout"`

	// 日志配置
	LoggingEnabled   bool          `json:"logging_enabled"`
	LogLevel         string        `json:"log_level"`
	LogBufferSize    int           `json:"log_buffer_size"`
	LogFlushInterval time.Duration `json:"log_flush_interval"`

	// 性能监控
	PerformanceEnabled  bool          `json:"performance_enabled"`
	ResourceMonitoring  bool          `json:"resource_monitoring"`
	HealthCheckEnabled  bool          `json:"health_check_enabled"`
	HealthCheckInterval time.Duration `json:"health_check_interval"`

	// 告警配置
	AlertingEnabled bool               `json:"alerting_enabled"`
	AlertThresholds map[string]float64 `json:"alert_thresholds"`
	AlertCooldown   time.Duration      `json:"alert_cooldown"`
}

// DefaultOptimizedObservabilityConfig 默认优化配置
func DefaultOptimizedObservabilityConfig() *OptimizedObservabilityConfig {
	return &OptimizedObservabilityConfig{
		MetricsEnabled:      true,
		MetricsInterval:     10 * time.Second,
		MetricsRetention:    24 * time.Hour,
		MetricsBufferSize:   10000,
		TracingEnabled:      true,
		SamplingRate:        0.1, // 10% 采样率
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
			"response_time": 1000.0, // ms
		},
		AlertCooldown: 5 * time.Minute,
	}
}

// MetricPoint 指标数据点
type MetricPoint struct {
	Name      string                 `json:"name"`
	Value     float64                `json:"value"`
	Timestamp time.Time              `json:"timestamp"`
	Labels    map[string]string      `json:"labels"`
	Metadata  map[string]interface{} `json:"metadata"`
}

// OptimizedSpan 追踪跨度
type OptimizedSpan struct {
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

// SpanStatus 跨度状态
type SpanStatus struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// LogEntry 日志条目
type LogEntry struct {
	Timestamp time.Time              `json:"timestamp"`
	Level     string                 `json:"level"`
	Message   string                 `json:"message"`
	Fields    map[string]interface{} `json:"fields"`
}

// Alert 告警
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
	config      *OptimizedObservabilityConfig
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
func NewOptimizedMetrics(config *OptimizedObservabilityConfig) *OptimizedMetrics {
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
	spans       map[string]*OptimizedSpan
	activeSpans map[string]*OptimizedSpan
	buffer      chan *OptimizedSpan
	mu          sync.RWMutex
	config      *OptimizedObservabilityConfig
	ctx         context.Context
	cancel      context.CancelFunc
	sampleCount int64
}

// NewOptimizedTracing 创建优化的追踪系统
func NewOptimizedTracing(config *OptimizedObservabilityConfig) *OptimizedTracing {
	ctx, cancel := context.WithCancel(context.Background())

	t := &OptimizedTracing{
		spans:       make(map[string]*OptimizedSpan),
		activeSpans: make(map[string]*OptimizedSpan),
		buffer:      make(chan *OptimizedSpan, 1000),
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
func (t *OptimizedTracing) StartSpan(traceID, spanID, parentID, operation string, tags map[string]string) *OptimizedSpan {
	// 采样检查
	if !t.shouldSample() {
		return nil
	}

	span := &OptimizedSpan{
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
func (t *OptimizedTracing) storeSpan(span *OptimizedSpan) {
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
func (t *OptimizedTracing) GetSpan(spanID string) (*OptimizedSpan, bool) {
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
	config    *OptimizedObservabilityConfig
	ctx       context.Context
	cancel    context.CancelFunc
}

// NewAlertManager 创建告警管理器
func NewAlertManager(config *OptimizedObservabilityConfig) *AlertManager {
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

// getSeverity 获取严重程度
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

// HealthChecker 健康检查器
type HealthChecker struct {
	checks     map[string]func() CheckResult
	lastStatus *HealthStatus
	mu         sync.RWMutex
	config     *OptimizedObservabilityConfig
	startTime  time.Time
}

// NewHealthChecker 创建健康检查器
func NewHealthChecker(config *OptimizedObservabilityConfig) *HealthChecker {
	hc := &HealthChecker{
		checks:    make(map[string]func() CheckResult),
		config:    config,
		startTime: time.Now(),
	}

	// 注册默认检查
	hc.RegisterCheck("memory", hc.checkMemory)
	hc.RegisterCheck("goroutines", hc.checkGoroutines)

	return hc
}

// RegisterCheck 注册健康检查
func (hc *HealthChecker) RegisterCheck(name string, check func() CheckResult) {
	hc.mu.Lock()
	defer hc.mu.Unlock()
	hc.checks[name] = check
}

// PerformHealthCheck 执行健康检查
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

// checkMemory 检查内存使用
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

// OptimizedObservability 优化的可观测性系统
type OptimizedObservability struct {
	config        *OptimizedObservabilityConfig
	metrics       *OptimizedMetrics
	tracing       *OptimizedTracing
	alertManager  *AlertManager
	healthChecker *HealthChecker
	ctx           context.Context
	cancel        context.CancelFunc
}

// NewOptimizedObservability 创建优化的可观测性系统
func NewOptimizedObservability(config *OptimizedObservabilityConfig) *OptimizedObservability {
	if config == nil {
		config = DefaultOptimizedObservabilityConfig()
	}

	ctx, cancel := context.WithCancel(context.Background())

	o := &OptimizedObservability{
		config:        config,
		metrics:       NewOptimizedMetrics(config),
		tracing:       NewOptimizedTracing(config),
		alertManager:  NewAlertManager(config),
		healthChecker: NewHealthChecker(config),
		ctx:           ctx,
		cancel:        cancel,
	}

	// 启动监控协程
	if config.PerformanceEnabled {
		go o.performanceMonitor()
	}

	if config.HealthCheckEnabled {
		go o.healthMonitor()
	}

	return o
}

// RecordMetric 记录指标
func (o *OptimizedObservability) RecordMetric(name string, value float64, labels map[string]string) {
	if !o.config.MetricsEnabled {
		return
	}

	o.metrics.Record(name, value, labels)

	// 检查告警阈值
	o.alertManager.CheckThreshold(name, value)
}

// StartSpan 开始追踪跨度
func (o *OptimizedObservability) StartSpan(traceID, spanID, parentID, operation string, tags map[string]string) *OptimizedSpan {
	if !o.config.TracingEnabled {
		return nil
	}

	return o.tracing.StartSpan(traceID, spanID, parentID, operation, tags)
}

// FinishSpan 结束追踪跨度
func (o *OptimizedObservability) FinishSpan(spanID string) {
	if !o.config.TracingEnabled {
		return
	}

	o.tracing.FinishSpan(spanID)
}

// performanceMonitor 性能监控
func (o *OptimizedObservability) performanceMonitor() {
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
func (o *OptimizedObservability) collectSystemMetrics() {
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
func (o *OptimizedObservability) healthMonitor() {
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
func (o *OptimizedObservability) GetMetrics() map[string]*MetricSeries {
	return o.metrics.GetAllMetrics()
}

// GetHealthStatus 获取健康状态
func (o *OptimizedObservability) GetHealthStatus() *HealthStatus {
	return o.healthChecker.PerformHealthCheck()
}

// GetAlerts 获取告警
func (o *OptimizedObservability) GetAlerts() map[string]*Alert {
	o.alertManager.mu.RLock()
	defer o.alertManager.mu.RUnlock()

	result := make(map[string]*Alert)
	for id, alert := range o.alertManager.alerts {
		result[id] = alert
	}
	return result
}

// ExportMetrics 导出指标
func (o *OptimizedObservability) ExportMetrics() ([]byte, error) {
	metrics := o.GetMetrics()
	return json.Marshal(metrics)
}

// Stop 停止可观测性系统
func (o *OptimizedObservability) Stop() error {
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
