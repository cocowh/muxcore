package router

import (
	"context"
	"fmt"
	"hash/fnv"
	"math"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cocowh/muxcore/pkg/logger"
)

// OptimizedRouterMetrics 优化路由器指标
type OptimizedRouterMetrics struct {
	TotalRequests         int64         `json:"total_requests"`
	SuccessfulRequests    int64         `json:"successful_requests"`
	FailedRequests        int64         `json:"failed_requests"`
	AverageLatency        time.Duration `json:"average_latency"`
	Throughput            float64       `json:"throughput"`
	LoadBalanceEfficiency float64       `json:"load_balance_efficiency"`
	CacheHitRate          float64       `json:"cache_hit_rate"`
	RouteMatchTime        time.Duration `json:"route_match_time"`
	NodeUtilization       float64       `json:"node_utilization"`
	FailoverCount         int64         `json:"failover_count"`
	LastUpdated           time.Time     `json:"last_updated"`
}

// OptimizedRouterConfig 优化路由器配置
type OptimizedRouterConfig struct {
	MaxConcurrency      int                 `json:"max_concurrency"`
	RequestTimeout      time.Duration       `json:"request_timeout"`
	EnableLoadBalancing bool                `json:"enable_load_balancing"`
	EnableCaching       bool                `json:"enable_caching"`
	EnableFailover      bool                `json:"enable_failover"`
	EnableMetrics       bool                `json:"enable_metrics"`
	HealthCheckInterval time.Duration       `json:"health_check_interval"`
	CacheSize           int                 `json:"cache_size"`
	CacheTTL            time.Duration       `json:"cache_ttl"`
	LoadBalanceStrategy LoadBalanceStrategy `json:"load_balance_strategy"`
	FailoverThreshold   int                 `json:"failover_threshold"`
	MetricsInterval     time.Duration       `json:"metrics_interval"`
}

// RouteRequest 路由请求
type RouteRequest struct {
	ID        string                    `json:"id"`
	Method    string                    `json:"method"`
	Path      string                    `json:"path"`
	Headers   map[string]string         `json:"headers"`
	Params    map[string]string         `json:"params"`
	Criteria  map[RouteDimension]string `json:"criteria"`
	Timestamp time.Time                 `json:"timestamp"`
	Priority  int                       `json:"priority"`
}

// RouteResult 路由结果
type RouteResult struct {
	Node      *RouteNode    `json:"node"`
	Latency   time.Duration `json:"latency"`
	CacheHit  bool          `json:"cache_hit"`
	Error     error         `json:"error"`
	Timestamp time.Time     `json:"timestamp"`
}

// OptimizedRouteCache 优化路由缓存
type OptimizedRouteCache struct {
	entries   map[string]*RouteCacheEntry
	mutex     sync.RWMutex
	maxSize   int
	ttl       time.Duration
	hits      int64
	misses    int64
	evictions int64
}

// RouteCacheEntry 路由缓存条目
type RouteCacheEntry struct {
	Key        string     `json:"key"`
	Node       *RouteNode `json:"node"`
	ExpireTime time.Time  `json:"expire_time"`
	AccessTime time.Time  `json:"access_time"`
	HitCount   int64      `json:"hit_count"`
}

// LoadBalancer 负载均衡器
type LoadBalancer struct {
	strategy       LoadBalanceStrategy
	nodes          []*RouteNode
	currentIndex   int64
	mutex          sync.RWMutex
	consistentHash *ConsistentHashRing
}

// OptimizedRouter 优化路由器
type OptimizedRouter struct {
	config         *OptimizedRouterConfig
	multidimRouter *MultidimensionalRouter
	httpRouter     *HTTPRouter
	loadBalancer   *LoadBalancer
	routeCache     *OptimizedRouteCache
	metrics        *OptimizedRouterMetrics
	ctx            context.Context
	cancel         context.CancelFunc
	mutex          sync.RWMutex
	workers        []*RouterWorker
	requestQueue   chan *RouteRequest
	resultQueue    chan *RouteResult
}

// RouterWorker 路由工作器
type RouterWorker struct {
	id        int
	router    *OptimizedRouter
	ctx       context.Context
	cancel    context.CancelFunc
	processed int64
	errors    int64
}

// NewOptimizedRouter 创建优化路由器
func NewOptimizedRouter(config *OptimizedRouterConfig) *OptimizedRouter {
	ctx, cancel := context.WithCancel(context.Background())

	router := &OptimizedRouter{
		config:         config,
		multidimRouter: NewMultidimensionalRouter(),
		httpRouter:     NewHTTPRouter(),
		loadBalancer:   NewLoadBalancer(config.LoadBalanceStrategy),
		routeCache:     NewOptimizedRouteCache(config.CacheSize, config.CacheTTL),
		metrics:        &OptimizedRouterMetrics{LastUpdated: time.Now()},
		ctx:            ctx,
		cancel:         cancel,
		requestQueue:   make(chan *RouteRequest, config.MaxConcurrency*2),
		resultQueue:    make(chan *RouteResult, config.MaxConcurrency*2),
	}

	return router
}

// NewOptimizedRouteCache 创建优化路由缓存
func NewOptimizedRouteCache(maxSize int, ttl time.Duration) *OptimizedRouteCache {
	return &OptimizedRouteCache{
		entries: make(map[string]*RouteCacheEntry),
		maxSize: maxSize,
		ttl:     ttl,
	}
}

// NewLoadBalancer 创建负载均衡器
func NewLoadBalancer(strategy LoadBalanceStrategy) *LoadBalancer {
	return &LoadBalancer{
		strategy:       strategy,
		nodes:          make([]*RouteNode, 0),
		consistentHash: NewConsistentHashRing(100),
	}
}

// Start 启动优化路由器
func (or *OptimizedRouter) Start() error {
	logger.Info("Starting optimized router")

	// 启动工作器
	for i := 0; i < or.config.MaxConcurrency; i++ {
		worker := or.createWorker(i)
		or.workers = append(or.workers, worker)
		go worker.run()
	}

	// 启动指标收集
	if or.config.EnableMetrics {
		go or.collectMetrics()
	}

	// 启动健康检查
	go or.healthCheck()

	// 启动缓存清理
	if or.config.EnableCaching {
		go or.cacheCleanup()
	}

	logger.Info("Optimized router started successfully")
	return nil
}

// Stop 停止优化路由器
func (or *OptimizedRouter) Stop() {
	logger.Info("Stopping optimized router")

	or.cancel()

	// 停止所有工作器
	for _, worker := range or.workers {
		worker.stop()
	}

	logger.Info("Optimized router stopped")
}

// createWorker 创建工作器
func (or *OptimizedRouter) createWorker(id int) *RouterWorker {
	ctx, cancel := context.WithCancel(or.ctx)
	return &RouterWorker{
		id:     id,
		router: or,
		ctx:    ctx,
		cancel: cancel,
	}
}

// run 运行工作器
func (rw *RouterWorker) run() {
	logger.Debugf("Router worker %d started", rw.id)

	for {
		select {
		case <-rw.ctx.Done():
			logger.Debugf("Router worker %d stopped", rw.id)
			return
		case request := <-rw.router.requestQueue:
			result := rw.processRequest(request)
			select {
			case rw.router.resultQueue <- result:
			case <-rw.ctx.Done():
				return
			}
		}
	}
}

// stop 停止工作器
func (rw *RouterWorker) stop() {
	rw.cancel()
}

// processRequest 处理请求
func (rw *RouterWorker) processRequest(request *RouteRequest) *RouteResult {
	start := time.Now()
	atomic.AddInt64(&rw.processed, 1)

	result := &RouteResult{
		Timestamp: start,
	}

	// 检查缓存
	if rw.router.config.EnableCaching {
		if cachedNode := rw.router.routeCache.Get(request); cachedNode != nil {
			result.Node = cachedNode
			result.CacheHit = true
			result.Latency = time.Since(start)
			return result
		}
	}

	// 路由选择
	node, err := rw.selectNode(request)
	if err != nil {
		atomic.AddInt64(&rw.errors, 1)
		result.Error = err
		return result
	}

	result.Node = node
	result.Latency = time.Since(start)

	// 缓存结果
	if rw.router.config.EnableCaching && node != nil {
		rw.router.routeCache.Put(request, node)
	}

	return result
}

// selectNode 选择节点
func (rw *RouterWorker) selectNode(request *RouteRequest) (*RouteNode, error) {
	// 使用多维度路由器选择节点
	if len(request.Criteria) > 0 {
		node := rw.router.multidimRouter.SelectNode(request.Criteria)
		if node != nil {
			return node, nil
		}
	}

	// 使用负载均衡器选择节点
	node := rw.router.loadBalancer.SelectNode(request)
	if node == nil {
		return nil, fmt.Errorf("no available nodes")
	}

	return node, nil
}

// RouteRequest 路由请求
func (or *OptimizedRouter) RouteRequest(request *RouteRequest) (*RouteResult, error) {
	select {
	case or.requestQueue <- request:
		// 请求已提交
	case <-time.After(or.config.RequestTimeout):
		return nil, fmt.Errorf("request timeout")
	}

	// 等待结果
	select {
	case result := <-or.resultQueue:
		// 更新指标
		or.updateMetrics(result)
		return result, result.Error
	case <-time.After(or.config.RequestTimeout):
		return nil, fmt.Errorf("result timeout")
	}
}

// updateMetrics 更新指标
func (or *OptimizedRouter) updateMetrics(result *RouteResult) {
	or.mutex.Lock()
	defer or.mutex.Unlock()

	atomic.AddInt64(&or.metrics.TotalRequests, 1)

	if result.Error == nil {
		atomic.AddInt64(&or.metrics.SuccessfulRequests, 1)
	} else {
		atomic.AddInt64(&or.metrics.FailedRequests, 1)
	}

	// 更新平均延迟
	totalRequests := atomic.LoadInt64(&or.metrics.TotalRequests)
	if totalRequests > 0 {
		currentAvg := or.metrics.AverageLatency
		newAvg := time.Duration((int64(currentAvg)*(totalRequests-1) + int64(result.Latency)) / totalRequests)
		or.metrics.AverageLatency = newAvg
	}

	// 更新缓存命中率
	if result.CacheHit {
		atomic.AddInt64(&or.routeCache.hits, 1)
	} else {
		atomic.AddInt64(&or.routeCache.misses, 1)
	}

	hits := atomic.LoadInt64(&or.routeCache.hits)
	misses := atomic.LoadInt64(&or.routeCache.misses)
	if hits+misses > 0 {
		or.metrics.CacheHitRate = float64(hits) / float64(hits+misses)
	}

	or.metrics.LastUpdated = time.Now()
}

// collectMetrics 收集指标
func (or *OptimizedRouter) collectMetrics() {
	ticker := time.NewTicker(or.config.MetricsInterval)
	defer ticker.Stop()

	for {
		select {
		case <-or.ctx.Done():
			return
		case <-ticker.C:
			or.calculateMetrics()
		}
	}
}

// calculateMetrics 计算指标
func (or *OptimizedRouter) calculateMetrics() {
	or.mutex.Lock()
	defer or.mutex.Unlock()

	// 计算吞吐量
	totalRequests := atomic.LoadInt64(&or.metrics.TotalRequests)
	if or.metrics.LastUpdated.IsZero() {
		or.metrics.Throughput = 0
	} else {
		duration := time.Since(or.metrics.LastUpdated).Seconds()
		if duration > 0 {
			or.metrics.Throughput = float64(totalRequests) / duration
		}
	}

	// 计算负载均衡效率
	or.calculateLoadBalanceEfficiency()

	// 计算节点利用率
	or.calculateNodeUtilization()

	or.metrics.LastUpdated = time.Now()
}

// calculateLoadBalanceEfficiency 计算负载均衡效率
func (or *OptimizedRouter) calculateLoadBalanceEfficiency() {
	nodes := or.loadBalancer.GetNodes()
	if len(nodes) == 0 {
		or.metrics.LoadBalanceEfficiency = 0
		return
	}

	// 计算负载标准差
	var totalLoad, meanLoad float64
	for _, node := range nodes {
		totalLoad += float64(node.Load)
	}
	meanLoad = totalLoad / float64(len(nodes))

	var variance float64
	for _, node := range nodes {
		diff := float64(node.Load) - meanLoad
		variance += diff * diff
	}
	variance /= float64(len(nodes))
	stdDev := math.Sqrt(variance)

	// 效率 = 1 - (标准差 / 平均值)
	if meanLoad > 0 {
		or.metrics.LoadBalanceEfficiency = math.Max(0, 1-(stdDev/meanLoad))
	} else {
		or.metrics.LoadBalanceEfficiency = 1
	}
}

// calculateNodeUtilization 计算节点利用率
func (or *OptimizedRouter) calculateNodeUtilization() {
	nodes := or.loadBalancer.GetNodes()
	if len(nodes) == 0 {
		or.metrics.NodeUtilization = 0
		return
	}

	var totalUtilization float64
	for _, node := range nodes {
		if node.Capacity > 0 {
			utilization := float64(node.Load) / float64(node.Capacity)
			totalUtilization += utilization
		}
	}

	or.metrics.NodeUtilization = totalUtilization / float64(len(nodes))
}

// healthCheck 健康检查
func (or *OptimizedRouter) healthCheck() {
	ticker := time.NewTicker(or.config.HealthCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-or.ctx.Done():
			return
		case <-ticker.C:
			or.performHealthCheck()
		}
	}
}

// performHealthCheck 执行健康检查
func (or *OptimizedRouter) performHealthCheck() {
	nodes := or.loadBalancer.GetNodes()
	for _, node := range nodes {
		// 模拟健康检查
		if node.FailureCount >= or.config.FailoverThreshold {
			node.Status = NodeUnhealthy
			atomic.AddInt64(&or.metrics.FailoverCount, 1)
			logger.Warnf("Node %s marked as unhealthy", node.ID)
		} else if node.Status == NodeUnhealthy && node.SuccessCount > node.FailureCount {
			node.Status = NodeHealthy
			logger.Infof("Node %s recovered to healthy", node.ID)
		}

		node.LastHealthCheck = time.Now()
	}
}

// cacheCleanup 缓存清理
func (or *OptimizedRouter) cacheCleanup() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-or.ctx.Done():
			return
		case <-ticker.C:
			or.routeCache.Cleanup()
		}
	}
}

// Get 从缓存获取
func (rc *OptimizedRouteCache) Get(request *RouteRequest) *RouteNode {
	rc.mutex.RLock()
	defer rc.mutex.RUnlock()

	key := rc.generateKey(request)
	entry, exists := rc.entries[key]
	if !exists || time.Now().After(entry.ExpireTime) {
		atomic.AddInt64(&rc.misses, 1)
		return nil
	}

	entry.AccessTime = time.Now()
	atomic.AddInt64(&entry.HitCount, 1)
	atomic.AddInt64(&rc.hits, 1)
	return entry.Node
}

// Put 放入缓存
func (rc *OptimizedRouteCache) Put(request *RouteRequest, node *RouteNode) {
	rc.mutex.Lock()
	defer rc.mutex.Unlock()

	key := rc.generateKey(request)

	// 检查是否需要淘汰
	if len(rc.entries) >= rc.maxSize {
		rc.evictLRU()
	}

	rc.entries[key] = &RouteCacheEntry{
		Key:        key,
		Node:       node,
		ExpireTime: time.Now().Add(rc.ttl),
		AccessTime: time.Now(),
		HitCount:   0,
	}
}

// generateKey 生成缓存键
func (rc *OptimizedRouteCache) generateKey(request *RouteRequest) string {
	h := fnv.New64a()
	h.Write([]byte(request.Method))
	h.Write([]byte(request.Path))
	for k, v := range request.Criteria {
		h.Write([]byte(string(k)))
		h.Write([]byte(v))
	}
	return fmt.Sprintf("%x", h.Sum64())
}

// evictLRU 淘汰最近最少使用的条目
func (rc *OptimizedRouteCache) evictLRU() {
	var oldestKey string
	var oldestTime time.Time

	for key, entry := range rc.entries {
		if oldestKey == "" || entry.AccessTime.Before(oldestTime) {
			oldestKey = key
			oldestTime = entry.AccessTime
		}
	}

	if oldestKey != "" {
		delete(rc.entries, oldestKey)
		atomic.AddInt64(&rc.evictions, 1)
	}
}

// Cleanup 清理过期条目
func (rc *OptimizedRouteCache) Cleanup() {
	rc.mutex.Lock()
	defer rc.mutex.Unlock()

	now := time.Now()
	for key, entry := range rc.entries {
		if now.After(entry.ExpireTime) {
			delete(rc.entries, key)
			atomic.AddInt64(&rc.evictions, 1)
		}
	}
}

// SelectNode 选择节点
func (lb *LoadBalancer) SelectNode(request *RouteRequest) *RouteNode {
	lb.mutex.RLock()
	defer lb.mutex.RUnlock()

	healthyNodes := lb.getHealthyNodes()
	if len(healthyNodes) == 0 {
		return nil
	}

	switch lb.strategy {
	case RoundRobin:
		return lb.selectRoundRobin(healthyNodes)
	case WeightedRoundRobin:
		return lb.selectWeightedRoundRobin(healthyNodes)
	case LeastConnections:
		return lb.selectLeastConnections(healthyNodes)
	case LeastResponseTime:
		return lb.selectLeastResponseTime(healthyNodes)
	case ConsistentHashStrategy:
		return lb.selectConsistentHash(healthyNodes, request)
	default:
		return lb.selectRoundRobin(healthyNodes)
	}
}

// getHealthyNodes 获取健康节点
func (lb *LoadBalancer) getHealthyNodes() []*RouteNode {
	healthyNodes := make([]*RouteNode, 0)
	for _, node := range lb.nodes {
		if node.Status == NodeHealthy {
			healthyNodes = append(healthyNodes, node)
		}
	}
	return healthyNodes
}

// selectRoundRobin 轮询选择
func (lb *LoadBalancer) selectRoundRobin(nodes []*RouteNode) *RouteNode {
	if len(nodes) == 0 {
		return nil
	}

	index := atomic.AddInt64(&lb.currentIndex, 1) % int64(len(nodes))
	return nodes[index]
}

// selectWeightedRoundRobin 加权轮询选择
func (lb *LoadBalancer) selectWeightedRoundRobin(nodes []*RouteNode) *RouteNode {
	if len(nodes) == 0 {
		return nil
	}

	// 计算总权重
	var totalWeight float64
	for _, node := range nodes {
		totalWeight += node.Weight
	}

	if totalWeight == 0 {
		return lb.selectRoundRobin(nodes)
	}

	// 生成随机数
	random := float64(time.Now().UnixNano()%int64(totalWeight*1000)) / 1000

	// 选择节点
	var currentWeight float64
	for _, node := range nodes {
		currentWeight += node.Weight
		if random <= currentWeight {
			return node
		}
	}

	return nodes[len(nodes)-1]
}

// selectLeastConnections 最少连接选择
func (lb *LoadBalancer) selectLeastConnections(nodes []*RouteNode) *RouteNode {
	if len(nodes) == 0 {
		return nil
	}

	minConnections := nodes[0].Connections
	selectedNode := nodes[0]

	for _, node := range nodes[1:] {
		if node.Connections < minConnections {
			minConnections = node.Connections
			selectedNode = node
		}
	}

	return selectedNode
}

// selectLeastResponseTime 最短响应时间选择
func (lb *LoadBalancer) selectLeastResponseTime(nodes []*RouteNode) *RouteNode {
	if len(nodes) == 0 {
		return nil
	}

	minResponseTime := nodes[0].ResponseTime
	selectedNode := nodes[0]

	for _, node := range nodes[1:] {
		if node.ResponseTime < minResponseTime {
			minResponseTime = node.ResponseTime
			selectedNode = node
		}
	}

	return selectedNode
}

// selectConsistentHash 一致性哈希选择
func (lb *LoadBalancer) selectConsistentHash(nodes []*RouteNode, request *RouteRequest) *RouteNode {
	if len(nodes) == 0 {
		return nil
	}

	// 生成请求哈希
	h := fnv.New32a()
	h.Write([]byte(request.Path))
	hash := h.Sum32()

	// 在一致性哈希环中查找
	nodeID := lb.consistentHash.Get(hash)
	for _, node := range nodes {
		if node.ID == nodeID {
			return node
		}
	}

	// 如果没找到，返回第一个节点
	return nodes[0]
}

// AddNode 添加节点
func (lb *LoadBalancer) AddNode(node *RouteNode) {
	lb.mutex.Lock()
	defer lb.mutex.Unlock()

	lb.nodes = append(lb.nodes, node)
	lb.consistentHash.Add(node.ID)
}

// RemoveNode 移除节点
func (lb *LoadBalancer) RemoveNode(nodeID string) {
	lb.mutex.Lock()
	defer lb.mutex.Unlock()

	for i, node := range lb.nodes {
		if node.ID == nodeID {
			lb.nodes = append(lb.nodes[:i], lb.nodes[i+1:]...)
			break
		}
	}

	lb.consistentHash.Remove(nodeID)
}

// GetNodes 获取所有节点
func (lb *LoadBalancer) GetNodes() []*RouteNode {
	lb.mutex.RLock()
	defer lb.mutex.RUnlock()

	nodes := make([]*RouteNode, len(lb.nodes))
	copy(nodes, lb.nodes)
	return nodes
}

// Add 添加节点到一致性哈希环
func (chr *ConsistentHashRing) Add(nodeID string) {
	chr.mutex.Lock()
	defer chr.mutex.Unlock()

	for i := 0; i < chr.replicas; i++ {
		h := fnv.New32a()
		h.Write([]byte(fmt.Sprintf("%s:%d", nodeID, i)))
		hash := h.Sum32()
		chr.hashRing[hash] = nodeID
		chr.sortedHashes = append(chr.sortedHashes, hash)
	}

	sort.Slice(chr.sortedHashes, func(i, j int) bool {
		return chr.sortedHashes[i] < chr.sortedHashes[j]
	})
}

// Remove 从一致性哈希环移除节点
func (chr *ConsistentHashRing) Remove(nodeID string) {
	chr.mutex.Lock()
	defer chr.mutex.Unlock()

	for i := 0; i < chr.replicas; i++ {
		h := fnv.New32a()
		h.Write([]byte(fmt.Sprintf("%s:%d", nodeID, i)))
		hash := h.Sum32()
		delete(chr.hashRing, hash)

		// 从排序数组中移除
		for j, sortedHash := range chr.sortedHashes {
			if sortedHash == hash {
				chr.sortedHashes = append(chr.sortedHashes[:j], chr.sortedHashes[j+1:]...)
				break
			}
		}
	}
}

// Get 从一致性哈希环获取节点
func (chr *ConsistentHashRing) Get(hash uint32) string {
	chr.mutex.RLock()
	defer chr.mutex.RUnlock()

	if len(chr.sortedHashes) == 0 {
		return ""
	}

	// 二分查找
	idx := sort.Search(len(chr.sortedHashes), func(i int) bool {
		return chr.sortedHashes[i] >= hash
	})

	if idx == len(chr.sortedHashes) {
		idx = 0
	}

	return chr.hashRing[chr.sortedHashes[idx]]
}

// GetMetrics 获取指标
func (or *OptimizedRouter) GetMetrics() *OptimizedRouterMetrics {
	or.mutex.RLock()
	defer or.mutex.RUnlock()

	// 创建指标副本
	metrics := *or.metrics
	return &metrics
}

// GetCacheStats 获取缓存统计
func (or *OptimizedRouter) GetCacheStats() map[string]interface{} {
	or.routeCache.mutex.RLock()
	defer or.routeCache.mutex.RUnlock()

	hits := atomic.LoadInt64(&or.routeCache.hits)
	misses := atomic.LoadInt64(&or.routeCache.misses)
	evictions := atomic.LoadInt64(&or.routeCache.evictions)

	hitRate := float64(0)
	if hits+misses > 0 {
		hitRate = float64(hits) / float64(hits+misses)
	}

	return map[string]interface{}{
		"hits":        hits,
		"misses":      misses,
		"evictions":   evictions,
		"hit_rate":    hitRate,
		"entry_count": len(or.routeCache.entries),
		"max_size":    or.routeCache.maxSize,
	}
}

// AddNode 添加节点
func (or *OptimizedRouter) AddNode(node *RouteNode) {
	or.loadBalancer.AddNode(node)
	or.multidimRouter.AddNode(node)
}

// RemoveNode 移除节点
func (or *OptimizedRouter) RemoveNode(nodeID string) {
	or.loadBalancer.RemoveNode(nodeID)
	or.multidimRouter.RemoveNode(nodeID)
}

// UpdateConfig 更新配置
func (or *OptimizedRouter) UpdateConfig(config *OptimizedRouterConfig) {
	or.mutex.Lock()
	defer or.mutex.Unlock()

	or.config = config
	logger.Info("Router configuration updated")
}

// GetRadixTree 获取基数树路由器
func (or *OptimizedRouter) GetRadixTree() *RadixTree {
	return or.httpRouter.radixTree
}
