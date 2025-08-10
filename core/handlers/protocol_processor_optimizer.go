package handlers

import (
	"context"
	"fmt"
	"net"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	common "github.com/cocowh/muxcore/core/common"
	"github.com/cocowh/muxcore/core/performance"
	"github.com/cocowh/muxcore/pkg/logger"
)

// OptimizedProcessorMetrics 优化处理器性能指标
type OptimizedProcessorMetrics struct {
	ProcessedRequests   uint64  `json:"processed_requests"`
	AverageLatency      int64   `json:"average_latency_ns"`
	Throughput          float64 `json:"throughput_rps"`
	ErrorRate           float64 `json:"error_rate"`
	CPUUsage            float64 `json:"cpu_usage"`
	MemoryUsage         uint64  `json:"memory_usage"`
	ActiveConnections   int     `json:"active_connections"`
	QueueDepth          int     `json:"queue_depth"`
	CacheHitRate        float64 `json:"cache_hit_rate"`
	ProtocolParseTime   int64   `json:"protocol_parse_time_ns"`
	Timestamp           int64   `json:"timestamp"`
}

// ProcessorConfig 处理器配置
type ProcessorConfig struct {
	MaxConcurrency      int           `json:"max_concurrency"`
	QueueSize           int           `json:"queue_size"`
	Timeout             time.Duration `json:"timeout"`
	EnablePipelining    bool          `json:"enable_pipelining"`
	EnableBatching      bool          `json:"enable_batching"`
	BatchSize           int           `json:"batch_size"`
	BatchTimeout        time.Duration `json:"batch_timeout"`
	EnableCompression   bool          `json:"enable_compression"`
	EnableCaching       bool          `json:"enable_caching"`
	CacheSize           int           `json:"cache_size"`
	CacheTTL            time.Duration `json:"cache_ttl"`
}

// ProcessingTask 处理任务
type ProcessingTask struct {
	ID           string
	ConnID       string
	Conn         net.Conn
	Protocol     string
	Data         []byte
	Handler      common.ProtocolHandler
	StartTime    time.Time
	ResultChan   chan ProcessingResult
	Context      context.Context
}

// ProcessingResult 处理结果
type ProcessingResult struct {
	TaskID    string
	Success   bool
	Error     error
	Latency   time.Duration
	BytesRead int
}

// ProtocolCache 协议缓存项
type ProtocolCache struct {
	Protocol  string
	Data      []byte
	Timestamp time.Time
	HitCount  uint64
}

// OptimizedProtocolProcessor 优化的协议处理器
type OptimizedProtocolProcessor struct {
	mutex               sync.RWMutex
	config              *ProcessorConfig
	metrics             *OptimizedProcessorMetrics
	processorManager    *ProcessorManager
	performanceMonitor  *performance.PerformanceMonitor
	bufferPool          *performance.BufferPool

	// 处理队列和工作池
	taskQueue           chan *ProcessingTask
	workerPool          []*ProcessorWorker
	running             bool

	// 批处理
	batchQueue          []*ProcessingTask
	batchMutex          sync.Mutex
	batchTimer          *time.Timer

	// 缓存
	protocolCache       map[string]*ProtocolCache
	cacheMutex          sync.RWMutex

	// 统计计数器
	totalRequests       uint64
	totalLatency        uint64
	totalErrors         uint64
	cacheHits           uint64
	cacheMisses         uint64
	parseTimeTotal      uint64
	activeConnections   int32

	// 控制通道
	stopChan            chan struct{}
	ctx                 context.Context
	cancel              context.CancelFunc
}

// ProcessorWorker 处理器工作线程
type ProcessorWorker struct {
	id                  int
	processor           *OptimizedProtocolProcessor
	running             bool
	stopChan            chan struct{}
}

// NewOptimizedProtocolProcessor 创建优化的协议处理器
func NewOptimizedProtocolProcessor(config *ProcessorConfig, processorMgr *ProcessorManager, perfMonitor *performance.PerformanceMonitor, bufPool *performance.BufferPool) *OptimizedProtocolProcessor {
	ctx, cancel := context.WithCancel(context.Background())

	defaultConfig := &ProcessorConfig{
		MaxConcurrency:    runtime.NumCPU() * 2,
		QueueSize:         10000,
		Timeout:           30 * time.Second,
		EnablePipelining:  true,
		EnableBatching:    true,
		BatchSize:         100,
		BatchTimeout:      10 * time.Millisecond,
		EnableCompression: false,
		EnableCaching:     true,
		CacheSize:         10000,
		CacheTTL:          5 * time.Minute,
	}

	if config == nil {
		config = defaultConfig
	}

	opp := &OptimizedProtocolProcessor{
		config:             config,
		metrics:            &OptimizedProcessorMetrics{},
		processorManager:   processorMgr,
		performanceMonitor: perfMonitor,
		bufferPool:         bufPool,
		taskQueue:          make(chan *ProcessingTask, config.QueueSize),
		batchQueue:         make([]*ProcessingTask, 0, config.BatchSize),
		protocolCache:      make(map[string]*ProtocolCache),
		stopChan:           make(chan struct{}),
		ctx:                ctx,
		cancel:             cancel,
	}

	// 创建工作线程池
	opp.workerPool = make([]*ProcessorWorker, config.MaxConcurrency)
	for i := 0; i < config.MaxConcurrency; i++ {
		opp.workerPool[i] = &ProcessorWorker{
			id:        i,
			processor: opp,
			stopChan:  make(chan struct{}),
		}
	}

	return opp
}

// Start 启动优化处理器
func (opp *OptimizedProtocolProcessor) Start() {
	opp.mutex.Lock()
	defer opp.mutex.Unlock()

	if opp.running {
		return
	}

	opp.running = true

	// 启动工作线程
	for _, worker := range opp.workerPool {
		go worker.start()
	}

	// 启动批处理定时器
	if opp.config.EnableBatching {
		go opp.batchProcessor()
	}

	// 启动指标收集
	go opp.metricsCollector()

	// 启动缓存清理
	go opp.cacheCleanup()

	logger.Info("Optimized protocol processor started")
}

// Stop 停止优化处理器
func (opp *OptimizedProtocolProcessor) Stop() {
	opp.mutex.Lock()
	defer opp.mutex.Unlock()

	if !opp.running {
		return
	}

	opp.running = false

	// 停止工作线程
	for _, worker := range opp.workerPool {
		worker.stop()
	}

	// 停止所有goroutine
	close(opp.stopChan)
	opp.cancel()

	logger.Info("Optimized protocol processor stopped")
}

// ProcessConnection 处理连接
func (opp *OptimizedProtocolProcessor) ProcessConnection(connID string, conn net.Conn, protocol string, initialData []byte) {
	if !opp.running {
		logger.Warn("Protocol processor is not running")
		return
	}

	// 增加活跃连接数
	atomic.AddInt32(&opp.activeConnections, 1)
	defer atomic.AddInt32(&opp.activeConnections, -1)

	// 检查缓存
	if opp.config.EnableCaching {
		if cached := opp.getCachedProtocol(string(initialData[:min(len(initialData), 64)])); cached != nil {
			protocol = cached.Protocol
			atomic.AddUint64(&opp.cacheHits, 1)
			atomic.AddUint64(&cached.HitCount, 1)
		} else {
			atomic.AddUint64(&opp.cacheMisses, 1)
		}
	}

	// 获取处理器
	handler := opp.processorManager.SelectProcessor(protocol)
	if handler == nil {
		logger.Errorf("No handler found for protocol: %s", protocol)
		atomic.AddUint64(&opp.totalErrors, 1)
		return
	}

	// 创建处理任务
	task := &ProcessingTask{
		ID:         generateTaskID(),
		ConnID:     connID,
		Conn:       conn,
		Protocol:   protocol,
		Data:       initialData,
		Handler:    handler,
		StartTime:  time.Now(),
		ResultChan: make(chan ProcessingResult, 1),
		Context:    opp.ctx,
	}

	// 提交任务
	if opp.config.EnableBatching {
		opp.submitBatchTask(task)
	} else {
		opp.submitTask(task)
	}

	// 等待结果（可选）
	select {
	case result := <-task.ResultChan:
		if !result.Success {
			logger.Errorf("Task %s failed: %v", result.TaskID, result.Error)
		}
	case <-time.After(opp.config.Timeout):
		logger.Warnf("Task %s timed out", task.ID)
		atomic.AddUint64(&opp.totalErrors, 1)
	}
}

// submitTask 提交单个任务
func (opp *OptimizedProtocolProcessor) submitTask(task *ProcessingTask) {
	select {
	case opp.taskQueue <- task:
		// 任务已提交
	default:
		// 队列已满
		logger.Warn("Task queue is full, dropping task")
		atomic.AddUint64(&opp.totalErrors, 1)
		task.ResultChan <- ProcessingResult{
			TaskID:  task.ID,
			Success: false,
			Error:   fmt.Errorf("task queue full"),
		}
	}
}

// submitBatchTask 提交批处理任务
func (opp *OptimizedProtocolProcessor) submitBatchTask(task *ProcessingTask) {
	opp.batchMutex.Lock()
	defer opp.batchMutex.Unlock()

	opp.batchQueue = append(opp.batchQueue, task)

	// 如果批次已满，立即处理
	if len(opp.batchQueue) >= opp.config.BatchSize {
		opp.processBatch()
	}
}

// batchProcessor 批处理器
func (opp *OptimizedProtocolProcessor) batchProcessor() {
	ticker := time.NewTicker(opp.config.BatchTimeout)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			opp.batchMutex.Lock()
			if len(opp.batchQueue) > 0 {
				opp.processBatch()
			}
			opp.batchMutex.Unlock()
		case <-opp.stopChan:
			return
		}
	}
}

// processBatch 处理批次
func (opp *OptimizedProtocolProcessor) processBatch() {
	if len(opp.batchQueue) == 0 {
		return
	}

	batch := make([]*ProcessingTask, len(opp.batchQueue))
	copy(batch, opp.batchQueue)
	opp.batchQueue = opp.batchQueue[:0]

	// 并行处理批次中的任务
	for _, task := range batch {
		select {
		case opp.taskQueue <- task:
			// 任务已提交
		default:
			// 队列已满，直接处理
			go opp.processTask(task)
		}
	}
}

// processTask 处理单个任务
func (opp *OptimizedProtocolProcessor) processTask(task *ProcessingTask) {
	start := time.Now()
	defer func() {
		latency := time.Since(start)
		atomic.AddUint64(&opp.totalRequests, 1)
		atomic.AddUint64(&opp.totalLatency, uint64(latency.Nanoseconds()))

		// 记录性能指标
		if opp.performanceMonitor != nil {
			opp.performanceMonitor.RecordRequest(latency)
		}
	}()

	// 协议解析时间测量
	parseStart := time.Now()

	// 处理连接
	task.Handler.Handle(task.ConnID, task.Conn, task.Data)

	parseTime := time.Since(parseStart)
	atomic.AddUint64(&opp.parseTimeTotal, uint64(parseTime.Nanoseconds()))

	// 更新缓存
	if opp.config.EnableCaching {
		opp.updateCache(string(task.Data[:min(len(task.Data), 64)]), task.Protocol)
	}

	// 发送结果
	select {
	case task.ResultChan <- ProcessingResult{
		TaskID:    task.ID,
		Success:   true,
		Latency:   time.Since(task.StartTime),
		BytesRead: len(task.Data),
	}:
	default:
		// 结果通道已关闭或满
	}
}

// getCachedProtocol 获取缓存的协议
func (opp *OptimizedProtocolProcessor) getCachedProtocol(key string) *ProtocolCache {
	opp.cacheMutex.RLock()
	defer opp.cacheMutex.RUnlock()

	cached, exists := opp.protocolCache[key]
	if !exists {
		return nil
	}

	// 检查TTL
	if time.Since(cached.Timestamp) > opp.config.CacheTTL {
		return nil
	}

	return cached
}

// updateCache 更新缓存
func (opp *OptimizedProtocolProcessor) updateCache(key, protocol string) {
	opp.cacheMutex.Lock()
	defer opp.cacheMutex.Unlock()

	// 检查缓存大小限制
	if len(opp.protocolCache) >= opp.config.CacheSize {
		// 简单的LRU清理：删除最旧的条目
		oldestKey := ""
		oldestTime := time.Now()
		for k, v := range opp.protocolCache {
			if v.Timestamp.Before(oldestTime) {
				oldestTime = v.Timestamp
				oldestKey = k
			}
		}
		if oldestKey != "" {
			delete(opp.protocolCache, oldestKey)
		}
	}

	opp.protocolCache[key] = &ProtocolCache{
		Protocol:  protocol,
		Timestamp: time.Now(),
		HitCount:  0,
	}
}

// cacheCleanup 缓存清理
func (opp *OptimizedProtocolProcessor) cacheCleanup() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			opp.cleanExpiredCache()
		case <-opp.stopChan:
			return
		}
	}
}

// cleanExpiredCache 清理过期缓存
func (opp *OptimizedProtocolProcessor) cleanExpiredCache() {
	opp.cacheMutex.Lock()
	defer opp.cacheMutex.Unlock()

	now := time.Now()
	for key, cached := range opp.protocolCache {
		if now.Sub(cached.Timestamp) > opp.config.CacheTTL {
			delete(opp.protocolCache, key)
		}
	}
}

// metricsCollector 指标收集器
func (opp *OptimizedProtocolProcessor) metricsCollector() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
	case <-ticker.C:
			opp.updateMetrics()
		case <-opp.stopChan:
			return
		}
	}
}

// updateMetrics 更新指标
func (opp *OptimizedProtocolProcessor) updateMetrics() {
	opp.mutex.Lock()
	defer opp.mutex.Unlock()

	totalReqs := atomic.LoadUint64(&opp.totalRequests)
	totalLatency := atomic.LoadUint64(&opp.totalLatency)
	totalErrors := atomic.LoadUint64(&opp.totalErrors)
	cacheHits := atomic.LoadUint64(&opp.cacheHits)
	cacheMisses := atomic.LoadUint64(&opp.cacheMisses)
	parseTimeTotal := atomic.LoadUint64(&opp.parseTimeTotal)
	activeConns := atomic.LoadInt32(&opp.activeConnections)

	// 计算平均延迟
	avgLatency := int64(0)
	if totalReqs > 0 {
		avgLatency = int64(totalLatency / totalReqs)
	}

	// 计算错误率
	errorRate := float64(0)
	if totalReqs > 0 {
		errorRate = float64(totalErrors) / float64(totalReqs)
	}

	// 计算缓存命中率
	cacheHitRate := float64(0)
	totalCacheOps := cacheHits + cacheMisses
	if totalCacheOps > 0 {
		cacheHitRate = float64(cacheHits) / float64(totalCacheOps)
	}

	// 计算平均协议解析时间
	avgParseTime := int64(0)
	if totalReqs > 0 {
		avgParseTime = int64(parseTimeTotal / totalReqs)
	}

	// 计算吞吐量（简化实现）
	throughput := float64(totalReqs) / 60.0 // 每分钟请求数

	opp.metrics = &OptimizedProcessorMetrics{
		ProcessedRequests:   totalReqs,
		AverageLatency:      avgLatency,
		Throughput:          throughput,
		ErrorRate:           errorRate,
		CPUUsage:            opp.getCPUUsage(),
		MemoryUsage:         opp.getMemoryUsage(),
		ActiveConnections:   int(activeConns),
		QueueDepth:          len(opp.taskQueue),
		CacheHitRate:        cacheHitRate,
		ProtocolParseTime:   avgParseTime,
		Timestamp:           time.Now().UnixNano(),
	}
}

// getCPUUsage 获取CPU使用率（简化实现）
func (opp *OptimizedProtocolProcessor) getCPUUsage() float64 {
	return float64(runtime.NumGoroutine()) / float64(runtime.NumCPU()) * 5.0
}

// getMemoryUsage 获取内存使用量
func (opp *OptimizedProtocolProcessor) getMemoryUsage() uint64 {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)
	return memStats.Alloc
}

// GetMetrics 获取处理器指标
func (opp *OptimizedProtocolProcessor) GetMetrics() *OptimizedProcessorMetrics {
	opp.mutex.RLock()
	defer opp.mutex.RUnlock()
	return opp.metrics
}

// GetCacheStats 获取缓存统计
func (opp *OptimizedProtocolProcessor) GetCacheStats() map[string]interface{} {
	opp.cacheMutex.RLock()
	defer opp.cacheMutex.RUnlock()

	cacheHits := atomic.LoadUint64(&opp.cacheHits)
	cacheMisses := atomic.LoadUint64(&opp.cacheMisses)
	cacheHitRate := float64(0)
	totalCacheOps := cacheHits + cacheMisses
	if totalCacheOps > 0 {
		cacheHitRate = float64(cacheHits) / float64(totalCacheOps)
	}

	return map[string]interface{}{
		"cache_size":     len(opp.protocolCache),
		"cache_hits":     cacheHits,
		"cache_misses":   cacheMisses,
		"cache_hit_rate": cacheHitRate,
	}
}

// UpdateConfig 更新配置
func (opp *OptimizedProtocolProcessor) UpdateConfig(config *ProcessorConfig) {
	opp.mutex.Lock()
	defer opp.mutex.Unlock()
	opp.config = config
	logger.Info("Protocol processor configuration updated")
}

// start 启动工作线程
func (pw *ProcessorWorker) start() {
	pw.running = true
	logger.Debugf("Protocol processor worker %d started", pw.id)

	for pw.running {
		select {
		case task := <-pw.processor.taskQueue:
			pw.processor.processTask(task)
		case <-pw.stopChan:
			return
		}
	}
}

// stop 停止工作线程
func (pw *ProcessorWorker) stop() {
	pw.running = false
	close(pw.stopChan)
	logger.Debugf("Protocol processor worker %d stopped", pw.id)
}

// generateTaskID 生成任务ID
func generateTaskID() string {
	return time.Now().Format("20060102150405") + "-" + string(rune(time.Now().UnixNano()%1000))
}

// min 返回两个整数中的较小值
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}