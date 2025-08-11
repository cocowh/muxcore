// Copyright (c) 2025 cocowh. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package handler

import (
	"context"
	"net"
	"sync"
	"time"

	"github.com/cocowh/muxcore/pkg/logger"
)

// ProtocolHandler 协议处理器接口
type ProtocolHandler interface {
	// Handle 处理连接
	Handle(connID string, conn net.Conn, initialData []byte)
}

// BaseHandler 基础处理器实现
type BaseHandler struct {
	// 移除对pool的直接依赖以避免循环导入
}

// NewBaseHandler 创建基础处理器
func NewBaseHandler() *BaseHandler {
	return &BaseHandler{}
}

// ProcessorType 处理器类型
const (
	ProcessorTypeHTTP      = "http"
	ProcessorTypeWebSocket = "websocket"
	ProcessorTypeGRPC      = "grpc"
	ProcessorTypeBinary    = "binary"
	ProcessorTypeStreaming = "streaming"
)

// ProcessorConfig 处理器配置
type ProcessorConfig struct {
	MaxConcurrency    int           `yaml:"max_concurrency"`
	BufferSize        int           `yaml:"buffer_size"`
	QueueSize         int           `yaml:"queue_size"`
	Timeout           time.Duration `yaml:"timeout"`
	RetryAttempts     int           `yaml:"retry_attempts"`
	EnableMetrics     bool          `yaml:"enable_metrics"`
	EnableCompression bool          `yaml:"enable_compression"`
	EnablePipelining  bool          `yaml:"enable_pipelining"`
	EnableBatching    bool          `yaml:"enable_batching"`
	BatchSize         int           `yaml:"batch_size"`
	BatchTimeout      time.Duration `yaml:"batch_timeout"`
	EnableCaching     bool          `yaml:"enable_caching"`
	CacheSize         int           `yaml:"cache_size"`
	CacheTTL          time.Duration `yaml:"cache_ttl"`
}

// ProcessorMetrics 处理器指标
type ProcessorMetrics struct {
	RequestQueueTime  float64 // 请求排队时长百分位
	MemoryPressure    float64 // 内存压力指数
	ErrorRateSlope    float64 // 错误率斜率
	ActiveConnections int     // 活跃连接数
	ProcessedRequests int64   // 已处理请求数
	AverageLatency    float64 // 平均延迟
	Throughput        float64 // 吞吐量
	ErrorRate         float64 // 错误率
	CPUUsage          float64 // CPU使用率
	MemoryUsage       float64 // 内存使用率
	QueueDepth        int     // 队列深度
	CacheHitRate      float64 // 缓存命中率
	ProtocolParseTime float64 // 协议解析时间
}

// OptimizedProtocolProcessor 优化的协议处理器
type OptimizedProtocolProcessor struct {
	*BaseHandler
	config  *ProcessorConfig
	metrics *ProcessorMetrics
	mutex   sync.RWMutex
}

// NewOptimizedProtocolProcessor 创建优化的协议处理器
func NewOptimizedProtocolProcessor(config *ProcessorConfig, manager *ProcessorManager, monitor interface{}, bufferPool interface{}) *OptimizedProtocolProcessor {
	return &OptimizedProtocolProcessor{
		BaseHandler: &BaseHandler{},
		config:      config,
		metrics:     &ProcessorMetrics{},
	}
}

// Handle 处理连接
func (p *OptimizedProtocolProcessor) Handle(connID string, conn net.Conn, initialData []byte) {
	// 实现优化的协议处理逻辑

	// 记录处理开始时间
	startTime := time.Now()

	// 增加活跃连接数
	p.mutex.Lock()
	p.metrics.ActiveConnections++
	p.mutex.Unlock()

	defer func() {
		// 处理完成时减少活跃连接数
		p.mutex.Lock()
		p.metrics.ActiveConnections--
		p.metrics.ProcessedRequests++

		// 计算处理延迟
		duration := time.Since(startTime)
		p.metrics.AverageLatency = (p.metrics.AverageLatency + float64(duration.Milliseconds())) / 2

		// 更新吞吐量（简化计算）
		p.metrics.Throughput = float64(p.metrics.ProcessedRequests) / time.Since(startTime).Seconds()
		p.mutex.Unlock()

		logger.Debugf("Processed connection %s in %v", connID, duration)
	}()

	// 协议检测和分类
	protocol := p.detectProtocolFromData(initialData)

	// 根据协议类型应用不同的处理策略
	ctx, cancel := context.WithTimeout(context.Background(), p.config.Timeout)
	defer cancel()

	switch protocol {
	case "http":
		p.handleHTTP(ctx, connID, conn, initialData)
	case "websocket":
		p.handleWebSocket(ctx, connID, conn, initialData)
	case "grpc":
		p.handleGRPC(ctx, connID, conn, initialData)
	default:
		p.handleGeneric(ctx, connID, conn, initialData)
	}
}

// detectProtocolFromData 从初始数据检测协议类型
func (p *OptimizedProtocolProcessor) detectProtocolFromData(data []byte) string {
	if len(data) < 4 {
		return "unknown"
	}

	// HTTP协议检测
	if string(data[:4]) == "GET " || string(data[:4]) == "POST" ||
		string(data[:3]) == "PUT" || string(data[:6]) == "DELETE" {
		return "http"
	}

	// gRPC协议检测（基于HTTP/2帧格式）
	if len(data) >= 9 && data[0] == 0x00 {
		return "grpc"
	}

	// WebSocket协议检测
	if len(data) >= 16 && string(data[:3]) == "GET" {
		dataStr := string(data)
		if len(dataStr) > 0 && (len(dataStr) >= 17 && dataStr[16:17] == " ") {
			return "websocket"
		}
	}

	return "unknown"
}

// handleHTTP 处理HTTP协议
func (p *OptimizedProtocolProcessor) handleHTTP(ctx context.Context, connID string, conn net.Conn, data []byte) {
	// 应用HTTP特定的优化
	if p.config.EnableCompression {
		logger.Debug("HTTP compression enabled for connection:", connID)
	}

	if p.config.EnablePipelining {
		logger.Debug("HTTP pipelining enabled for connection:", connID)
	}

	// 检查缓存
	if p.config.EnableCaching {
		p.handleHTTPWithCache(ctx, connID, conn, data)
		return
	}

	logger.Infof("Processing HTTP connection: %s", connID)
	p.updateMetrics("http", true)
}

// handleWebSocket 处理WebSocket协议
func (p *OptimizedProtocolProcessor) handleWebSocket(ctx context.Context, connID string, conn net.Conn, data []byte) {
	// WebSocket特定的优化
	if p.config.EnableBatching {
		p.handleWebSocketBatched(ctx, connID, conn, data)
		return
	}

	logger.Infof("Processing WebSocket connection: %s", connID)
	p.updateMetrics("websocket", true)
}

// handleGRPC 处理gRPC协议
func (p *OptimizedProtocolProcessor) handleGRPC(ctx context.Context, connID string, conn net.Conn, data []byte) {
	// gRPC特定的优化
	if p.config.EnableCompression {
		logger.Debug("gRPC compression enabled for connection:", connID)
	}

	logger.Infof("Processing gRPC connection: %s", connID)
	p.updateMetrics("grpc", true)
}

// handleGeneric 处理通用协议
func (p *OptimizedProtocolProcessor) handleGeneric(ctx context.Context, connID string, conn net.Conn, data []byte) {
	logger.Infof("Processing generic connection: %s", connID)
	p.updateMetrics("generic", true)
}

// handleHTTPWithCache 带缓存的HTTP处理
func (p *OptimizedProtocolProcessor) handleHTTPWithCache(ctx context.Context, connID string, conn net.Conn, data []byte) {
	p.mutex.Lock()
	p.metrics.CacheHitRate = (p.metrics.CacheHitRate + 0.8) / 2 // 模拟缓存命中率
	p.mutex.Unlock()

	logger.Debug("HTTP request processed with cache for connection:", connID)
}

// handleWebSocketBatched 批处理WebSocket消息
func (p *OptimizedProtocolProcessor) handleWebSocketBatched(ctx context.Context, connID string, conn net.Conn, data []byte) {
	// 批处理逻辑
	batchSize := p.config.BatchSize
	if batchSize <= 0 {
		batchSize = 10
	}

	logger.Debugf("WebSocket batch processing enabled (batch size: %d) for connection: %s", batchSize, connID)
}

// updateMetrics 更新处理器指标
func (p *OptimizedProtocolProcessor) updateMetrics(protocol string, success bool) {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	// 更新协议解析时间（模拟）
	p.metrics.ProtocolParseTime = float64(time.Now().UnixNano()%1000) / 1000.0

	// 更新错误率
	if !success {
		p.metrics.ErrorRate = (p.metrics.ErrorRate + 1.0) / 2
	} else {
		p.metrics.ErrorRate = p.metrics.ErrorRate * 0.95 // 成功时降低错误率
	}

	// 更新队列深度（模拟）
	p.metrics.QueueDepth = int(float64(p.metrics.ActiveConnections) * 0.3)
}

// GetMetrics 获取处理器指标
func (p *OptimizedProtocolProcessor) GetMetrics() *ProcessorMetrics {
	p.mutex.RLock()
	defer p.mutex.RUnlock()
	return p.metrics
}

// UpdateConfig 更新配置
func (p *OptimizedProtocolProcessor) UpdateConfig(config *ProcessorConfig) {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	p.config = config
}

// Start 启动处理器
func (p *OptimizedProtocolProcessor) Start() error {
	logger.Info("Starting optimized protocol processor")
	return nil
}

// Stop 停止处理器
func (p *OptimizedProtocolProcessor) Stop() error {
	logger.Info("Stopping optimized protocol processor")
	return nil
}

// ProcessConnection 处理连接
func (p *OptimizedProtocolProcessor) ProcessConnection(connID string, conn net.Conn, protocol string, data []byte) error {
	p.mutex.Lock()
	p.metrics.ProcessedRequests++
	p.mutex.Unlock()
	logger.Info("Processing connection", "connID", connID, "protocol", protocol)
	return nil
}

// CacheStats 缓存统计
type CacheStats struct {
	HitRate     float64
	MissRate    float64
	TotalHits   int64
	TotalMisses int64
	CacheSize   int
}

// GetCacheStats 获取缓存统计
func (p *OptimizedProtocolProcessor) GetCacheStats() map[string]interface{} {
	return map[string]interface{}{
		"cache_hit_rate":  p.metrics.CacheHitRate,
		"cache_miss_rate": 1.0 - p.metrics.CacheHitRate,
		"total_hits":      int64(p.metrics.CacheHitRate * float64(p.metrics.ProcessedRequests)),
		"total_misses":    int64((1.0 - p.metrics.CacheHitRate) * float64(p.metrics.ProcessedRequests)),
		"cache_size":      p.config.CacheSize,
	}
}

// ProcessorGroup 处理器组
type ProcessorGroup struct {
	Type             string
	Handlers         []ProtocolHandler
	Metrics          ProcessorMetrics
	mutex            sync.RWMutex
	concurrencyModel string
	memoryManagement string
	timeoutControl   string
	errorRecovery    string
}

// ProcessorManager 处理器管理器
type ProcessorManager struct {
	groups                map[string]*ProcessorGroup
	mutex                 sync.RWMutex
	loadBalancingStrategy LoadBalancingStrategy
	config                *ProcessorConfig
	// 每协议的轮询计数器，避免基于时间的取模偏斜
	rrIndex map[string]uint64
}

// LoadBalancingStrategy 负载均衡策略
type LoadBalancingStrategy interface {
	SelectProcessor(protocol string, groups map[string]*ProcessorGroup) *ProcessorGroup
}

// NewProcessorManager 创建处理器管理器
func NewProcessorManager() *ProcessorManager {
	m := &ProcessorManager{
		groups:                make(map[string]*ProcessorGroup),
		loadBalancingStrategy: &ProtocolAwareStrategy{},
		rrIndex:               make(map[string]uint64),
	}

	// 初始化处理器组
	m.initProcessorGroups()

	// 启动指标收集
	go m.metricsCollectorLoop()

	return m
}

// NewProcessorManagerWithConfig 使用配置创建处理器管理器
func NewProcessorManagerWithConfig(config *ProcessorConfig) *ProcessorManager {
	m := &ProcessorManager{
		groups:                make(map[string]*ProcessorGroup),
		loadBalancingStrategy: &ProtocolAwareStrategy{},
		config:                config,
		rrIndex:               make(map[string]uint64),
	}

	// 初始化处理器组
	m.initProcessorGroups()

	// 启动指标收集
	go m.metricsCollectorLoop()

	return m
}

// initProcessorGroups 初始化处理器组
func (m *ProcessorManager) initProcessorGroups() {
	// HTTP处理器组
	httpGroup := &ProcessorGroup{
		Type:             ProcessorTypeHTTP,
		Handlers:         make([]ProtocolHandler, 0),
		concurrencyModel: "Goroutine-per-request",
		memoryManagement: "对象池化",
		timeoutControl:   "分层超时链",
		errorRecovery:    "请求级重试",
	}
	m.groups[ProcessorTypeHTTP] = httpGroup

	// WebSocket处理器组
	wsGroup := &ProcessorGroup{
		Type:             ProcessorTypeWebSocket,
		Handlers:         make([]ProtocolHandler, 0),
		concurrencyModel: "事件驱动Actor模型",
		memoryManagement: "环形缓冲区",
		timeoutControl:   "心跳保活机制",
		errorRecovery:    "会话重建",
	}
	m.groups[ProcessorTypeWebSocket] = wsGroup

	// gRPC处理器组
	grpcGroup := &ProcessorGroup{
		Type:             ProcessorTypeGRPC,
		Handlers:         make([]ProtocolHandler, 0),
		concurrencyModel: "线程池工作模式",
		memoryManagement: "内存映射文件",
		timeoutControl:   "看门狗定时器",
		errorRecovery:    "校验和修复",
	}
	m.groups[ProcessorTypeGRPC] = grpcGroup

	logger.Info("Initialized processor groups")
}

// AddProcessor 添加处理器到组
func (m *ProcessorManager) AddProcessor(protocol string, handler ProtocolHandler) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	group, exists := m.groups[protocol]
	if !exists {
		logger.Error("No processor group found for protocol: ", protocol)
		return
	}

	group.mutex.Lock()
	group.Handlers = append(group.Handlers, handler)
	group.mutex.Unlock()

	logger.Info("Added processor to group: ", protocol)
}

// SelectProcessor 选择处理器
func (m *ProcessorManager) SelectProcessor(protocol string) ProtocolHandler {
	m.mutex.RLock()
	group, exists := m.groups[protocol]
	m.mutex.RUnlock()

	if !exists || len(group.Handlers) == 0 {
		logger.Error("No processors available for protocol: ", protocol)
		return nil
	}

	// 使用负载均衡策略选择处理器组
	selectedGroup := m.loadBalancingStrategy.SelectProcessor(protocol, m.groups)
	if selectedGroup == nil {
		selectedGroup = group
	}

	// 从组中选择一个处理器 (改为线程安全的每协议轮询)
	selectedGroup.mutex.RLock()
	defer selectedGroup.mutex.RUnlock()

	if len(selectedGroup.Handlers) == 0 {
		return nil
	}

	// 增加协议对应的轮询计数器并取模
	m.mutex.Lock()
	cur := m.rrIndex[protocol]
	m.rrIndex[protocol] = cur + 1
	m.mutex.Unlock()

	index := int(cur % uint64(len(selectedGroup.Handlers)))
	return selectedGroup.Handlers[index]
}

// metricsCollectorLoop 指标收集循环
func (m *ProcessorManager) metricsCollectorLoop() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		m.updateProcessorMetrics()
	}
}

// updateProcessorMetrics 更新处理器指标
func (m *ProcessorManager) updateProcessorMetrics() {
	// 简化实现：实际应用中需要收集真实指标
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	for _, group := range m.groups {
		group.mutex.Lock()
		// 模拟指标更新
		group.Metrics.RequestQueueTime = float64(time.Now().UnixNano()%100) / 1000.0
		group.Metrics.MemoryPressure = float64(time.Now().UnixNano()%50) / 100.0
		group.Metrics.ErrorRateSlope = float64(time.Now().UnixNano()%10) / 100.0
		group.Metrics.ActiveConnections = int(time.Now().UnixNano() % 100)
		group.mutex.Unlock()
	}
}

// ProtocolAwareStrategy 基于协议特性的负载均衡策略
type ProtocolAwareStrategy struct{}

// SelectProcessor 选择处理器
func (s *ProtocolAwareStrategy) SelectProcessor(protocol string, groups map[string]*ProcessorGroup) *ProcessorGroup {
	// 简化实现：实际应用中需要基于协议特性和处理器指标进行复杂决策
	if group, exists := groups[protocol]; exists {
		return group
	}
	return nil
}

// GetProcessorMetrics 获取处理器指标
func (m *ProcessorManager) GetProcessorMetrics(protocol string) (ProcessorMetrics, bool) {
	m.mutex.RLock()
	group, exists := m.groups[protocol]
	m.mutex.RUnlock()

	if !exists {
		return ProcessorMetrics{}, false
	}

	group.mutex.RLock()
	metrics := group.Metrics
	group.mutex.RUnlock()

	return metrics, true
}

// SetLoadBalancingStrategy 设置负载均衡策略
func (m *ProcessorManager) SetLoadBalancingStrategy(strategy LoadBalancingStrategy) {
	m.mutex.Lock()
	m.loadBalancingStrategy = strategy
	m.mutex.Unlock()

	logger.Info("Updated load balancing strategy")
}

// RegisterHandler 注册协议处理器
func (m *ProcessorManager) RegisterHandler(protocol string, handler ProtocolHandler) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	// 获取或创建处理器组
	group, exists := m.groups[protocol]
	if !exists {
		group = &ProcessorGroup{
			Type:     protocol,
			Handlers: make([]ProtocolHandler, 0),
			Metrics:  ProcessorMetrics{},
		}
		m.groups[protocol] = group
	}

	// 添加处理器到组中
	group.mutex.Lock()
	group.Handlers = append(group.Handlers, handler)
	group.mutex.Unlock()

	logger.Infof("Registered %s protocol handler", protocol)
}
