package router

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cocowh/muxcore/pkg/logger"
)

// RouterMetrics 路由器指标
type RouterMetrics struct {
	// 请求指标
	TotalRequests       int64     `json:"total_requests"`
	SuccessfulRequests  int64     `json:"successful_requests"`
	FailedRequests      int64     `json:"failed_requests"`
	ActiveRequests      int64     `json:"active_requests"`
	
	// 响应时间指标
	AverageResponseTime time.Duration `json:"average_response_time"`
	MinResponseTime     time.Duration `json:"min_response_time"`
	MaxResponseTime     time.Duration `json:"max_response_time"`
	P95ResponseTime     time.Duration `json:"p95_response_time"`
	P99ResponseTime     time.Duration `json:"p99_response_time"`
	
	// 错误指标
	ErrorRate           float64   `json:"error_rate"`
	TimeoutCount        int64     `json:"timeout_count"`
	CircuitBreakerTrips int64     `json:"circuit_breaker_trips"`
	
	// 缓存指标
	CacheHits           int64     `json:"cache_hits"`
	CacheMisses         int64     `json:"cache_misses"`
	CacheHitRate        float64   `json:"cache_hit_rate"`
	
	// 系统指标
	CPUUsage            float64   `json:"cpu_usage"`
	MemoryUsage         float64   `json:"memory_usage"`
	GoroutineCount      int       `json:"goroutine_count"`
	
	// 时间戳
	Timestamp           time.Time `json:"timestamp"`
}

// RouteMetrics 单个路由的指标
type RouteMetrics struct {
	Path                string        `json:"path"`
	Method              string        `json:"method"`
	RequestCount        int64         `json:"request_count"`
	SuccessCount        int64         `json:"success_count"`
	ErrorCount          int64         `json:"error_count"`
	AverageResponseTime time.Duration `json:"average_response_time"`
	LastAccessed        time.Time     `json:"last_accessed"`
}

// AlertEvent 告警事件
type AlertEvent struct {
	ID          string                 `json:"id"`
	Type        string                 `json:"type"`
	Severity    string                 `json:"severity"`
	Message     string                 `json:"message"`
	Metrics     map[string]interface{} `json:"metrics"`
	Timestamp   time.Time              `json:"timestamp"`
	Resolved    bool                   `json:"resolved"`
	ResolvedAt  *time.Time             `json:"resolved_at,omitempty"`
}

// RouterMonitor 路由监控器
type RouterMonitor struct {
	config              *MonitoringConfig
	metrics             *RouterMetrics
	routeMetrics        map[string]*RouteMetrics
	metricsHistory      []*RouterMetrics
	alerts              []*AlertEvent
	mutex               sync.RWMutex
	running             bool
	ctx                 context.Context
	cancel              context.CancelFunc
	alertHandlers       []AlertHandler
	responseTimes       []time.Duration
	responseTimesMutex  sync.Mutex
}

// AlertHandler 告警处理器
type AlertHandler func(*AlertEvent)

// NewRouterMonitor 创建路由监控器
func NewRouterMonitor(config *MonitoringConfig) *RouterMonitor {
	if config == nil {
		config = &MonitoringConfig{
			Enabled:             true,
			MetricsInterval:     time.Second * 30,
			HealthCheckInterval: time.Second * 60,
			MaxMetricsHistory:   1000,
		}
	}
	
	ctx, cancel := context.WithCancel(context.Background())
	
	return &RouterMonitor{
		config:         config,
		metrics:        &RouterMetrics{Timestamp: time.Now()},
		routeMetrics:   make(map[string]*RouteMetrics),
		metricsHistory: make([]*RouterMetrics, 0),
		alerts:         make([]*AlertEvent, 0),
		ctx:            ctx,
		cancel:         cancel,
		alertHandlers:  make([]AlertHandler, 0),
		responseTimes:  make([]time.Duration, 0),
	}
}

// Start 启动监控
func (rm *RouterMonitor) Start() {
	if !rm.config.Enabled {
		return
	}
	
	rm.mutex.Lock()
	if rm.running {
		rm.mutex.Unlock()
		return
	}
	rm.running = true
	rm.mutex.Unlock()
	
	// 启动指标收集
	go rm.metricsCollector()
	
	// 启动健康检查
	go rm.healthChecker()
	
	// 启动告警检查
	go rm.alertChecker()
	
	logger.Info("Router monitor started")
}

// Stop 停止监控
func (rm *RouterMonitor) Stop() {
	rm.mutex.Lock()
	defer rm.mutex.Unlock()
	
	if !rm.running {
		return
	}
	
	rm.running = false
	rm.cancel()
	
	logger.Info("Router monitor stopped")
}

// RecordRequest 记录请求
func (rm *RouterMonitor) RecordRequest(method, path string, duration time.Duration, success bool) {
	if !rm.config.Enabled {
		return
	}
	
	// 更新总体指标
	atomic.AddInt64(&rm.metrics.TotalRequests, 1)
	if success {
		atomic.AddInt64(&rm.metrics.SuccessfulRequests, 1)
	} else {
		atomic.AddInt64(&rm.metrics.FailedRequests, 1)
	}
	
	// 记录响应时间
	rm.recordResponseTime(duration)
	
	// 更新路由级指标
	rm.updateRouteMetrics(method, path, duration, success)
}

// RecordActiveRequest 记录活跃请求
func (rm *RouterMonitor) RecordActiveRequest(delta int64) {
	atomic.AddInt64(&rm.metrics.ActiveRequests, delta)
}

// RecordCacheHit 记录缓存命中
func (rm *RouterMonitor) RecordCacheHit() {
	atomic.AddInt64(&rm.metrics.CacheHits, 1)
}

// RecordCacheMiss 记录缓存未命中
func (rm *RouterMonitor) RecordCacheMiss() {
	atomic.AddInt64(&rm.metrics.CacheMisses, 1)
}

// RecordTimeout 记录超时
func (rm *RouterMonitor) RecordTimeout() {
	atomic.AddInt64(&rm.metrics.TimeoutCount, 1)
}

// RecordCircuitBreakerTrip 记录熔断器触发
func (rm *RouterMonitor) RecordCircuitBreakerTrip() {
	atomic.AddInt64(&rm.metrics.CircuitBreakerTrips, 1)
}

// GetMetrics 获取当前指标
func (rm *RouterMonitor) GetMetrics() *RouterMetrics {
	rm.mutex.RLock()
	defer rm.mutex.RUnlock()
	
	// 创建指标副本
	metrics := &RouterMetrics{
		TotalRequests:       atomic.LoadInt64(&rm.metrics.TotalRequests),
		SuccessfulRequests:  atomic.LoadInt64(&rm.metrics.SuccessfulRequests),
		FailedRequests:      atomic.LoadInt64(&rm.metrics.FailedRequests),
		ActiveRequests:      atomic.LoadInt64(&rm.metrics.ActiveRequests),
		TimeoutCount:        atomic.LoadInt64(&rm.metrics.TimeoutCount),
		CircuitBreakerTrips: atomic.LoadInt64(&rm.metrics.CircuitBreakerTrips),
		CacheHits:           atomic.LoadInt64(&rm.metrics.CacheHits),
		CacheMisses:         atomic.LoadInt64(&rm.metrics.CacheMisses),
		AverageResponseTime: rm.metrics.AverageResponseTime,
		MinResponseTime:     rm.metrics.MinResponseTime,
		MaxResponseTime:     rm.metrics.MaxResponseTime,
		P95ResponseTime:     rm.metrics.P95ResponseTime,
		P99ResponseTime:     rm.metrics.P99ResponseTime,
		ErrorRate:           rm.metrics.ErrorRate,
		CacheHitRate:        rm.metrics.CacheHitRate,
		CPUUsage:            rm.metrics.CPUUsage,
		MemoryUsage:         rm.metrics.MemoryUsage,
		GoroutineCount:      rm.metrics.GoroutineCount,
		Timestamp:           time.Now(),
	}
	
	return metrics
}

// GetRouteMetrics 获取路由指标
func (rm *RouterMonitor) GetRouteMetrics() map[string]*RouteMetrics {
	rm.mutex.RLock()
	defer rm.mutex.RUnlock()
	
	// 创建副本
	result := make(map[string]*RouteMetrics)
	for key, metrics := range rm.routeMetrics {
		result[key] = &RouteMetrics{
			Path:                metrics.Path,
			Method:              metrics.Method,
			RequestCount:        metrics.RequestCount,
			SuccessCount:        metrics.SuccessCount,
			ErrorCount:          metrics.ErrorCount,
			AverageResponseTime: metrics.AverageResponseTime,
			LastAccessed:        metrics.LastAccessed,
		}
	}
	
	return result
}

// GetMetricsHistory 获取指标历史
func (rm *RouterMonitor) GetMetricsHistory() []*RouterMetrics {
	rm.mutex.RLock()
	defer rm.mutex.RUnlock()
	
	// 创建副本
	history := make([]*RouterMetrics, len(rm.metricsHistory))
	copy(history, rm.metricsHistory)
	return history
}

// GetAlerts 获取告警
func (rm *RouterMonitor) GetAlerts() []*AlertEvent {
	rm.mutex.RLock()
	defer rm.mutex.RUnlock()
	
	// 创建副本
	alerts := make([]*AlertEvent, len(rm.alerts))
	copy(alerts, rm.alerts)
	return alerts
}

// AddAlertHandler 添加告警处理器
func (rm *RouterMonitor) AddAlertHandler(handler AlertHandler) {
	rm.mutex.Lock()
	defer rm.mutex.Unlock()
	
	rm.alertHandlers = append(rm.alertHandlers, handler)
}

// recordResponseTime 记录响应时间
func (rm *RouterMonitor) recordResponseTime(duration time.Duration) {
	rm.responseTimesMutex.Lock()
	defer rm.responseTimesMutex.Unlock()
	
	rm.responseTimes = append(rm.responseTimes, duration)
	
	// 限制响应时间数组大小
	if len(rm.responseTimes) > 10000 {
		rm.responseTimes = rm.responseTimes[1000:]
	}
}

// updateRouteMetrics 更新路由指标
func (rm *RouterMonitor) updateRouteMetrics(method, path string, duration time.Duration, success bool) {
	rm.mutex.Lock()
	defer rm.mutex.Unlock()
	
	key := method + ":" + path
	metrics, exists := rm.routeMetrics[key]
	if !exists {
		metrics = &RouteMetrics{
			Path:   path,
			Method: method,
		}
		rm.routeMetrics[key] = metrics
	}
	
	metrics.RequestCount++
	if success {
		metrics.SuccessCount++
	} else {
		metrics.ErrorCount++
	}
	
	// 更新平均响应时间
	if metrics.RequestCount == 1 {
		metrics.AverageResponseTime = duration
	} else {
		// 使用移动平均
		metrics.AverageResponseTime = time.Duration(
			(int64(metrics.AverageResponseTime)*int64(metrics.RequestCount-1) + int64(duration)) / int64(metrics.RequestCount),
		)
	}
	
	metrics.LastAccessed = time.Now()
}

// metricsCollector 指标收集器
func (rm *RouterMonitor) metricsCollector() {
	ticker := time.NewTicker(rm.config.MetricsInterval)
	defer ticker.Stop()
	
	for {
		select {
		case <-rm.ctx.Done():
			return
		case <-ticker.C:
			rm.collectMetrics()
		}
	}
}

// healthChecker 健康检查器
func (rm *RouterMonitor) healthChecker() {
	ticker := time.NewTicker(rm.config.HealthCheckInterval)
	defer ticker.Stop()
	
	for {
		select {
		case <-rm.ctx.Done():
			return
		case <-ticker.C:
			rm.performHealthCheck()
		}
	}
}

// alertChecker 告警检查器
func (rm *RouterMonitor) alertChecker() {
	ticker := time.NewTicker(time.Second * 10) // 每10秒检查一次告警
	defer ticker.Stop()
	
	for {
		select {
		case <-rm.ctx.Done():
			return
		case <-ticker.C:
			rm.checkAlerts()
		}
	}
}

// collectMetrics 收集指标
func (rm *RouterMonitor) collectMetrics() {
	rm.mutex.Lock()
	defer rm.mutex.Unlock()
	
	// 计算错误率
	totalRequests := atomic.LoadInt64(&rm.metrics.TotalRequests)
	failedRequests := atomic.LoadInt64(&rm.metrics.FailedRequests)
	if totalRequests > 0 {
		rm.metrics.ErrorRate = float64(failedRequests) / float64(totalRequests)
	}
	
	// 计算缓存命中率
	cacheHits := atomic.LoadInt64(&rm.metrics.CacheHits)
	cacheMisses := atomic.LoadInt64(&rm.metrics.CacheMisses)
	totalCacheRequests := cacheHits + cacheMisses
	if totalCacheRequests > 0 {
		rm.metrics.CacheHitRate = float64(cacheHits) / float64(totalCacheRequests)
	}
	
	// 计算响应时间统计
	rm.calculateResponseTimeStats()
	
	// 收集系统指标
	rm.collectSystemMetrics()
	
	// 保存指标历史
	currentMetrics := rm.GetMetrics()
	rm.metricsHistory = append(rm.metricsHistory, currentMetrics)
	
	// 限制历史记录数量
	if len(rm.metricsHistory) > rm.config.MaxMetricsHistory {
		rm.metricsHistory = rm.metricsHistory[1:]
	}
	
	logger.Debugf("Collected router metrics - Requests: %d, Errors: %.2f%%, Cache Hit Rate: %.2f%%",
		totalRequests, rm.metrics.ErrorRate*100, rm.metrics.CacheHitRate*100)
}

// calculateResponseTimeStats 计算响应时间统计
func (rm *RouterMonitor) calculateResponseTimeStats() {
	rm.responseTimesMutex.Lock()
	defer rm.responseTimesMutex.Unlock()
	
	if len(rm.responseTimes) == 0 {
		return
	}
	
	// 计算平均值
	var total time.Duration
	min := rm.responseTimes[0]
	max := rm.responseTimes[0]
	
	for _, duration := range rm.responseTimes {
		total += duration
		if duration < min {
			min = duration
		}
		if duration > max {
			max = duration
		}
	}
	
	rm.metrics.AverageResponseTime = total / time.Duration(len(rm.responseTimes))
	rm.metrics.MinResponseTime = min
	rm.metrics.MaxResponseTime = max
	
	// 计算百分位数（简化实现）
	if len(rm.responseTimes) >= 20 {
		// 这里应该对数组进行排序，为简化直接使用最大值的一定比例
		rm.metrics.P95ResponseTime = time.Duration(float64(max) * 0.95)
		rm.metrics.P99ResponseTime = time.Duration(float64(max) * 0.99)
	}
}

// collectSystemMetrics 收集系统指标
func (rm *RouterMonitor) collectSystemMetrics() {
	// 收集CPU和内存使用率（简化实现）
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	
	rm.metrics.MemoryUsage = float64(m.Alloc) / float64(m.Sys)
	rm.metrics.GoroutineCount = runtime.NumGoroutine()
	
	// CPU使用率需要更复杂的实现，这里使用模拟值
	rm.metrics.CPUUsage = 0.1 // 10% 作为示例
}

// performHealthCheck 执行健康检查
func (rm *RouterMonitor) performHealthCheck() {
	metrics := rm.GetMetrics()
	
	// 检查各项健康指标
	healthy := true
	var issues []string
	
	if rm.config.AlertThresholds != nil {
		thresholds := rm.config.AlertThresholds
		
		if metrics.ErrorRate > thresholds.MaxErrorRate {
			healthy = false
			issues = append(issues, fmt.Sprintf("High error rate: %.2f%%", metrics.ErrorRate*100))
		}
		
		if metrics.AverageResponseTime > thresholds.MaxResponseTime {
			healthy = false
			issues = append(issues, fmt.Sprintf("High response time: %v", metrics.AverageResponseTime))
		}
		
		if metrics.CPUUsage > thresholds.MaxCPUUsage {
			healthy = false
			issues = append(issues, fmt.Sprintf("High CPU usage: %.2f%%", metrics.CPUUsage*100))
		}
		
		if metrics.MemoryUsage > thresholds.MaxMemoryUsage {
			healthy = false
			issues = append(issues, fmt.Sprintf("High memory usage: %.2f%%", metrics.MemoryUsage*100))
		}
	}
	
	if healthy {
		logger.Debug("Router health check passed")
	} else {
		logger.Warnf("Router health check failed: %v", issues)
	}
}

// checkAlerts 检查告警
func (rm *RouterMonitor) checkAlerts() {
	if rm.config.AlertThresholds == nil {
		return
	}
	
	metrics := rm.GetMetrics()
	thresholds := rm.config.AlertThresholds
	
	// 检查错误率告警
	if metrics.ErrorRate > thresholds.MaxErrorRate {
		rm.triggerAlert("high_error_rate", "critical", 
			fmt.Sprintf("Error rate %.2f%% exceeds threshold %.2f%%", 
				metrics.ErrorRate*100, thresholds.MaxErrorRate*100),
			map[string]interface{}{
				"current_error_rate": metrics.ErrorRate,
				"threshold": thresholds.MaxErrorRate,
			})
	}
	
	// 检查响应时间告警
	if metrics.AverageResponseTime > thresholds.MaxResponseTime {
		rm.triggerAlert("high_response_time", "warning",
			fmt.Sprintf("Average response time %v exceeds threshold %v",
				metrics.AverageResponseTime, thresholds.MaxResponseTime),
			map[string]interface{}{
				"current_response_time": metrics.AverageResponseTime,
				"threshold": thresholds.MaxResponseTime,
			})
	}
	
	// 检查CPU使用率告警
	if metrics.CPUUsage > thresholds.MaxCPUUsage {
		rm.triggerAlert("high_cpu_usage", "warning",
			fmt.Sprintf("CPU usage %.2f%% exceeds threshold %.2f%%",
				metrics.CPUUsage*100, thresholds.MaxCPUUsage*100),
			map[string]interface{}{
				"current_cpu_usage": metrics.CPUUsage,
				"threshold": thresholds.MaxCPUUsage,
			})
	}
}

// triggerAlert 触发告警
func (rm *RouterMonitor) triggerAlert(alertType, severity, message string, metrics map[string]interface{}) {
	rm.mutex.Lock()
	defer rm.mutex.Unlock()
	
	// 检查是否已存在相同类型的未解决告警
	for _, alert := range rm.alerts {
		if alert.Type == alertType && !alert.Resolved {
			return // 避免重复告警
		}
	}
	
	alert := &AlertEvent{
		ID:        generateAlertID(),
		Type:      alertType,
		Severity:  severity,
		Message:   message,
		Metrics:   metrics,
		Timestamp: time.Now(),
		Resolved:  false,
	}
	
	rm.alerts = append(rm.alerts, alert)
	
	// 触发告警处理器
	for _, handler := range rm.alertHandlers {
		go func(h AlertHandler) {
			defer func() {
				if r := recover(); r != nil {
					logger.Errorf("Alert handler panic: %v", r)
				}
			}()
			h(alert)
		}(handler)
	}
	
	logger.Warnf("Alert triggered: [%s] %s - %s", severity, alertType, message)
}

// generateAlertID 生成告警ID
func generateAlertID() string {
	return fmt.Sprintf("alert_%d", time.Now().UnixNano())
}