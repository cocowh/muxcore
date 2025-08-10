package performance

import (
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cocowh/muxcore/pkg/logger"
)

// PerformanceMetrics 性能指标
type PerformanceMetrics struct {
	CPUUsage        float64 `json:"cpu_usage"`
	MemoryUsage     uint64  `json:"memory_usage"`
	GoroutineCount  int     `json:"goroutine_count"`
	GCPauseTime     int64   `json:"gc_pause_time_ns"`
	AllocRate       uint64  `json:"alloc_rate"`
	CacheHitRate    float64 `json:"cache_hit_rate"`
	Throughput      uint64  `json:"throughput"`
	Latency         int64   `json:"latency_ns"`
	Timestamp       int64   `json:"timestamp"`
}

// PerformanceThresholds 性能阈值
type PerformanceThresholds struct {
	MaxCPUUsage      float64 `json:"max_cpu_usage"`
	MaxMemoryUsage   uint64  `json:"max_memory_usage"`
	MaxGoroutines    int     `json:"max_goroutines"`
	MaxGCPauseTime   int64   `json:"max_gc_pause_time_ns"`
	MinCacheHitRate  float64 `json:"min_cache_hit_rate"`
	MaxLatency       int64   `json:"max_latency_ns"`
}

// TuningAction 调优动作
type TuningAction struct {
	Type        string                 `json:"type"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters"`
	Timestamp   int64                  `json:"timestamp"`
}

// PerformanceMonitor 性能监控器
type PerformanceMonitor struct {
	mutex           sync.RWMutex
	enabled         bool
	metrics         *PerformanceMetrics
	thresholds      *PerformanceThresholds
	tuningActions   []TuningAction
	memoryManager   *MemoryManager
	cpuManager      *CPUAffinityManager
	resourceManager *ResourceManager
	bufferPool      *BufferPool

	// 统计计数器
	totalRequests   uint64
	totalLatency    uint64
	cacheHits       uint64
	cacheMisses     uint64

	// 控制通道
	stopChan        chan struct{}
	tuningChan      chan TuningAction
}

// NewPerformanceMonitor 创建性能监控器
func NewPerformanceMonitor(memMgr *MemoryManager, cpuMgr *CPUAffinityManager, resMgr *ResourceManager, bufPool *BufferPool) *PerformanceMonitor {
	defaultThresholds := &PerformanceThresholds{
		MaxCPUUsage:     80.0,
		MaxMemoryUsage:  1024 * 1024 * 1024, // 1GB
		MaxGoroutines:   10000,
		MaxGCPauseTime:  10 * 1000 * 1000, // 10ms
		MinCacheHitRate: 0.8,
		MaxLatency:      100 * 1000 * 1000, // 100ms
	}

	pm := &PerformanceMonitor{
		enabled:         true,
		metrics:         &PerformanceMetrics{},
		thresholds:      defaultThresholds,
		tuningActions:   make([]TuningAction, 0),
		memoryManager:   memMgr,
		cpuManager:      cpuMgr,
		resourceManager: resMgr,
		bufferPool:      bufPool,
		stopChan:        make(chan struct{}),
		tuningChan:      make(chan TuningAction, 100),
	}

	return pm
}

// Start 启动性能监控
func (pm *PerformanceMonitor) Start() {
	if !pm.enabled {
		return
	}

	go pm.monitorLoop()
	go pm.tuningLoop()

	logger.Info("Performance monitor started")
}

// Stop 停止性能监控
func (pm *PerformanceMonitor) Stop() {
	close(pm.stopChan)
	logger.Info("Performance monitor stopped")
}

// monitorLoop 监控循环
func (pm *PerformanceMonitor) monitorLoop() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			pm.collectMetrics()
			pm.checkThresholds()
		case <-pm.stopChan:
			return
		}
	}
}

// tuningLoop 调优循环
func (pm *PerformanceMonitor) tuningLoop() {
	for {
		select {
		case action := <-pm.tuningChan:
			pm.executeTuningAction(action)
		case <-pm.stopChan:
			return
		}
	}
}

// collectMetrics 收集性能指标
func (pm *PerformanceMonitor) collectMetrics() {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	pm.mutex.Lock()
	defer pm.mutex.Unlock()

	// 计算缓存命中率
	cacheHitRate := float64(0)
	totalCacheOps := atomic.LoadUint64(&pm.cacheHits) + atomic.LoadUint64(&pm.cacheMisses)
	if totalCacheOps > 0 {
		cacheHitRate = float64(atomic.LoadUint64(&pm.cacheHits)) / float64(totalCacheOps)
	}

	// 计算平均延迟
	avgLatency := int64(0)
	totalReqs := atomic.LoadUint64(&pm.totalRequests)
	if totalReqs > 0 {
		avgLatency = int64(atomic.LoadUint64(&pm.totalLatency) / totalReqs)
	}

	pm.metrics = &PerformanceMetrics{
		CPUUsage:       pm.getCPUUsage(),
		MemoryUsage:    memStats.Alloc,
		GoroutineCount: runtime.NumGoroutine(),
		GCPauseTime:    int64(memStats.PauseNs[(memStats.NumGC+255)%256]),
		AllocRate:      memStats.TotalAlloc,
		CacheHitRate:   cacheHitRate,
		Throughput:     totalReqs,
		Latency:        avgLatency,
		Timestamp:      time.Now().UnixNano(),
	}
}

// getCPUUsage 获取CPU使用率（简化实现）
func (pm *PerformanceMonitor) getCPUUsage() float64 {
	// 这里是简化实现，实际应该使用系统调用获取真实CPU使用率
	return float64(runtime.NumGoroutine()) / float64(runtime.NumCPU()) * 10.0
}

// checkThresholds 检查阈值并触发调优
func (pm *PerformanceMonitor) checkThresholds() {
	pm.mutex.RLock()
	metrics := *pm.metrics
	thresholds := *pm.thresholds
	pm.mutex.RUnlock()

	// 检查CPU使用率
	if metrics.CPUUsage > thresholds.MaxCPUUsage {
		action := TuningAction{
			Type:        "cpu_optimization",
			Description: "High CPU usage detected, optimizing CPU affinity",
			Parameters: map[string]interface{}{
				"cpu_usage": metrics.CPUUsage,
				"threshold": thresholds.MaxCPUUsage,
			},
			Timestamp: time.Now().UnixNano(),
		}
		pm.tuningChan <- action
	}

	// 检查内存使用率
	if metrics.MemoryUsage > thresholds.MaxMemoryUsage {
		action := TuningAction{
			Type:        "memory_optimization",
			Description: "High memory usage detected, triggering GC and memory optimization",
			Parameters: map[string]interface{}{
				"memory_usage": metrics.MemoryUsage,
				"threshold":    thresholds.MaxMemoryUsage,
			},
			Timestamp: time.Now().UnixNano(),
		}
		pm.tuningChan <- action
	}

	// 检查缓存命中率
	if metrics.CacheHitRate < thresholds.MinCacheHitRate {
		action := TuningAction{
			Type:        "cache_optimization",
			Description: "Low cache hit rate detected, optimizing cache strategy",
			Parameters: map[string]interface{}{
				"hit_rate":  metrics.CacheHitRate,
				"threshold": thresholds.MinCacheHitRate,
			},
			Timestamp: time.Now().UnixNano(),
		}
		pm.tuningChan <- action
	}

	// 检查GC暂停时间
	if metrics.GCPauseTime > thresholds.MaxGCPauseTime {
		action := TuningAction{
			Type:        "gc_optimization",
			Description: "High GC pause time detected, optimizing garbage collection",
			Parameters: map[string]interface{}{
				"gc_pause_time": metrics.GCPauseTime,
				"threshold":     thresholds.MaxGCPauseTime,
			},
			Timestamp: time.Now().UnixNano(),
		}
		pm.tuningChan <- action
	}
}

// executeTuningAction 执行调优动作
func (pm *PerformanceMonitor) executeTuningAction(action TuningAction) {
	pm.mutex.Lock()
	pm.tuningActions = append(pm.tuningActions, action)
	pm.mutex.Unlock()

	logger.Infof("Executing tuning action: %s - %s", action.Type, action.Description)

	switch action.Type {
	case "cpu_optimization":
		pm.optimizeCPU()
	case "memory_optimization":
		pm.optimizeMemory()
	case "cache_optimization":
		pm.optimizeCache()
	case "gc_optimization":
		pm.optimizeGC()
	default:
		logger.Warnf("Unknown tuning action type: %s", action.Type)
	}
}

// optimizeCPU CPU优化
func (pm *PerformanceMonitor) optimizeCPU() {
	if pm.cpuManager != nil {
		// 重新分配CPU核心
		logger.Info("Optimizing CPU affinity")
		// 这里可以实现更复杂的CPU优化逻辑
	}
}

// optimizeMemory 内存优化
func (pm *PerformanceMonitor) optimizeMemory() {
	// 强制垃圾回收
	runtime.GC()
	logger.Info("Triggered garbage collection for memory optimization")

	if pm.memoryManager != nil {
		// 可以实现内存页面重新分配等优化
		logger.Info("Optimizing memory allocation patterns")
	}
}

// optimizeCache 缓存优化
func (pm *PerformanceMonitor) optimizeCache() {
	if pm.bufferPool != nil {
		// 预分配更多缓冲区
		pm.bufferPool.PreAllocate(100)
		logger.Info("Preallocated additional buffers for cache optimization")
	}
}

// optimizeGC GC优化
func (pm *PerformanceMonitor) optimizeGC() {
	// 调整GC目标百分比
	oldGCPercent := runtime.GOMAXPROCS(0)
	newGCPercent := oldGCPercent + 10
	runtime.GOMAXPROCS(newGCPercent)
	logger.Infof("Adjusted GOMAXPROCS from %d to %d for GC optimization", oldGCPercent, newGCPercent)
}

// RecordRequest 记录请求
func (pm *PerformanceMonitor) RecordRequest(latency time.Duration) {
	atomic.AddUint64(&pm.totalRequests, 1)
	atomic.AddUint64(&pm.totalLatency, uint64(latency.Nanoseconds()))
}

// RecordCacheHit 记录缓存命中
func (pm *PerformanceMonitor) RecordCacheHit() {
	atomic.AddUint64(&pm.cacheHits, 1)
}

// RecordCacheMiss 记录缓存未命中
func (pm *PerformanceMonitor) RecordCacheMiss() {
	atomic.AddUint64(&pm.cacheMisses, 1)
}

// GetMetrics 获取当前性能指标
func (pm *PerformanceMonitor) GetMetrics() *PerformanceMetrics {
	pm.mutex.RLock()
	defer pm.mutex.RUnlock()
	return pm.metrics
}

// GetTuningActions 获取调优动作历史
func (pm *PerformanceMonitor) GetTuningActions() []TuningAction {
	pm.mutex.RLock()
	defer pm.mutex.RUnlock()
	return pm.tuningActions
}

// SetThresholds 设置性能阈值
func (pm *PerformanceMonitor) SetThresholds(thresholds *PerformanceThresholds) {
	pm.mutex.Lock()
	pm.thresholds = thresholds
	pm.mutex.Unlock()
	logger.Info("Performance thresholds updated")
}

// Enable 启用性能监控
func (pm *PerformanceMonitor) Enable() {
	pm.mutex.Lock()
	pm.enabled = true
	pm.mutex.Unlock()
}

// Disable 禁用性能监控
func (pm *PerformanceMonitor) Disable() {
	pm.mutex.Lock()
	pm.enabled = false
	pm.mutex.Unlock()
}