package router

import (
	"sync"
	"time"

	"github.com/cocowh/muxcore/pkg/logger"
)

// RouteDimension 路由维度
type RouteDimension string

// 定义路由维度
const (
	DimensionProtocol  RouteDimension = "protocol"
	DimensionGeo       RouteDimension = "geo"
	DimensionQoS       RouteDimension = "qos"
	DimensionAPIVersion RouteDimension = "api_version"
)

// NodeStatus 节点状态
type NodeStatus int

const (
	NodeHealthy NodeStatus = iota
	NodeUnhealthy
	NodeMaintenance
)

// RouteNode 路由节点
type RouteNode struct {
	ID              string
	Address         string
	Capacity        int
	Load            int
	Status          NodeStatus
	LastHealthCheck time.Time
	ResponseTime    time.Duration
	Weight          float64
	Connections     int
	FailureCount    int
	SuccessCount    int
}

// LoadBalanceStrategy 负载均衡策略
type LoadBalanceStrategy int

const (
	RoundRobin LoadBalanceStrategy = iota
	WeightedRoundRobin
	LeastConnections
	LeastResponseTime
	ConsistentHashStrategy
)

// MultidimensionalRouterConfig 路由器配置
type MultidimensionalRouterConfig struct {
	Strategy            LoadBalanceStrategy
	HealthCheckInterval time.Duration
	MaxFailures         int
	RecoveryThreshold   int
}

// MultidimensionalRouter 多维度路由矩阵
type MultidimensionalRouter struct {
	nodes            map[string]*RouteNode
	rules            map[RouteDimension]map[string][]string
	weights          map[RouteDimension]float64
	config           *MultidimensionalRouterConfig
	mutex            sync.RWMutex
	roundRobinIndex  map[string]int
	consistentHash   *ConsistentHashRing
}

// ConsistentHashRing 一致性哈希环
type ConsistentHashRing struct {
	hashRing map[uint32]string
	sortedHashes []uint32
	replicas int
	mutex sync.RWMutex
}

// NewConsistentHashRing 创建一致性哈希环
func NewConsistentHashRing(replicas int) *ConsistentHashRing {
	return &ConsistentHashRing{
		hashRing:     make(map[uint32]string),
		sortedHashes: make([]uint32, 0),
		replicas:     replicas,
	}
}

// NewMultidimensionalRouter 创建多维度路由矩阵
func NewMultidimensionalRouter() *MultidimensionalRouter {
	return NewMultidimensionalRouterWithConfig(&MultidimensionalRouterConfig{
		Strategy:            WeightedRoundRobin,
		HealthCheckInterval: 30 * time.Second,
		MaxFailures:         3,
		RecoveryThreshold:   5,
	})
}

// NewMultidimensionalRouterWithConfig 使用配置创建多维度路由矩阵
func NewMultidimensionalRouterWithConfig(config *MultidimensionalRouterConfig) *MultidimensionalRouter {
	router := &MultidimensionalRouter{
		nodes:           make(map[string]*RouteNode),
		rules:           make(map[RouteDimension]map[string][]string),
		weights:         make(map[RouteDimension]float64),
		config:          config,
		roundRobinIndex: make(map[string]int),
		consistentHash:  NewConsistentHashRing(100),
	}

	// 设置默认权重
	router.weights[DimensionProtocol] = 0.4
	router.weights[DimensionGeo] = 0.2
	router.weights[DimensionQoS] = 0.3
	router.weights[DimensionAPIVersion] = 0.1

	// 初始化规则映射
	for _, dim := range []RouteDimension{
		DimensionProtocol,
		DimensionGeo,
		DimensionQoS,
		DimensionAPIVersion,
	} {
		router.rules[dim] = make(map[string][]string)
	}

	// 启动健康检查
	go router.startHealthCheck()

	logger.Info("Initialized multidimensional router with strategy: ", config.Strategy)
	return router
}

// startHealthCheck 启动健康检查
func (r *MultidimensionalRouter) startHealthCheck() {
	ticker := time.NewTicker(r.config.HealthCheckInterval)
	defer ticker.Stop()

	for range ticker.C {
		r.performHealthCheck()
	}
}

// performHealthCheck 执行健康检查
func (r *MultidimensionalRouter) performHealthCheck() {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	for _, node := range r.nodes {
		// 简单的健康检查逻辑
		if node.FailureCount >= r.config.MaxFailures {
			node.Status = NodeUnhealthy
			logger.Warn("Node marked as unhealthy: ", node.ID)
		} else if node.Status == NodeUnhealthy && node.SuccessCount >= r.config.RecoveryThreshold {
			node.Status = NodeHealthy
			node.FailureCount = 0
			logger.Info("Node recovered: ", node.ID)
		}
		node.LastHealthCheck = time.Now()
	}
}

// AddNode 添加路由节点
func (r *MultidimensionalRouter) AddNode(node *RouteNode) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	r.nodes[node.ID] = node
	logger.Info("Added route node: ", node.ID)
}

// RemoveNode 移除路由节点
func (r *MultidimensionalRouter) RemoveNode(nodeID string) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	if _, exists := r.nodes[nodeID]; exists {
		delete(r.nodes, nodeID)
		logger.Info("Removed route node: ", nodeID)
	}
}

// UpdateNodeLoad 更新节点负载
func (r *MultidimensionalRouter) UpdateNodeLoad(nodeID string, load int) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	if node, exists := r.nodes[nodeID]; exists {
		node.Load = load
		logger.Debug("Updated load for node ", nodeID, ": ", load)
	}
}

// AddRule 添加路由规则
func (r *MultidimensionalRouter) AddRule(dimension RouteDimension, value string, nodeIDs []string) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	if _, exists := r.rules[dimension]; !exists {
		r.rules[dimension] = make(map[string][]string)
	}

	r.rules[dimension][value] = nodeIDs
	logger.Info("Added rule for dimension ", dimension, " value ", value)
}

// SetWeight 设置维度权重
func (r *MultidimensionalRouter) SetWeight(dimension RouteDimension, weight float64) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	// 确保权重在0-1之间
	if weight < 0 {
		weight = 0
	} else if weight > 1 {
		weight = 1
	}

	r.weights[dimension] = weight
	logger.Info("Set weight for dimension ", dimension, ": ", weight)

	// 重新归一化权重
	r.normalizeWeights()
}

// normalizeWeights 归一化权重
func (r *MultidimensionalRouter) normalizeWeights() {
	total := 0.0
	for _, weight := range r.weights {
		total += weight
	}

	if total == 0 {
		return
	}

	for dim, weight := range r.weights {
		r.weights[dim] = weight / total
	}
}

// SelectNode 选择路由节点
func (r *MultidimensionalRouter) SelectNode(criteria map[RouteDimension]string) *RouteNode {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	// 如果没有节点，返回nil
	if len(r.nodes) == 0 {
		logger.Error("No route nodes available")
		return nil
	}

	// 获取候选节点
	candidates := r.getCandidateNodes(criteria)
	if len(candidates) == 0 {
		logger.Warn("No candidate nodes found for criteria")
		return nil
	}

	// 根据负载均衡策略选择节点
	var selectedNode *RouteNode
	switch r.config.Strategy {
	case RoundRobin:
		selectedNode = r.selectRoundRobin(candidates)
	case WeightedRoundRobin:
		selectedNode = r.selectWeightedRoundRobin(candidates)
	case LeastConnections:
		selectedNode = r.selectLeastConnections(candidates)
	case LeastResponseTime:
		selectedNode = r.selectLeastResponseTime(candidates)
	case ConsistentHashStrategy:
		selectedNode = r.selectConsistentHash(candidates, criteria)
	default:
		selectedNode = r.selectWeightedRoundRobin(candidates)
	}

	if selectedNode != nil {
		logger.Debug("Selected route node: ", selectedNode.ID, " using strategy: ", r.config.Strategy)
		// 更新连接数
		selectedNode.Connections++
	}

	return selectedNode
}

// getCandidateNodes 获取候选节点
func (r *MultidimensionalRouter) getCandidateNodes(criteria map[RouteDimension]string) []*RouteNode {
	// 计算节点得分
	scores := make(map[string]float64)
	for nodeID := range r.nodes {
		node := r.nodes[nodeID]
		// 只考虑健康的节点
		if node.Status == NodeHealthy {
			scores[nodeID] = 0.0
		}
	}

	// 根据每个维度的规则和权重计算得分
	for dim, value := range criteria {
		weight := r.weights[dim]
		if nodeIDs, exists := r.rules[dim][value]; exists {
			for _, nodeID := range nodeIDs {
				if _, exists := scores[nodeID]; exists {
					scores[nodeID] += weight
				}
			}
		}
	}

	// 收集候选节点
	var candidates []*RouteNode
	for nodeID, score := range scores {
		if score > 0 {
			candidates = append(candidates, r.nodes[nodeID])
		}
	}

	return candidates
}

// selectRoundRobin 轮询选择
func (r *MultidimensionalRouter) selectRoundRobin(candidates []*RouteNode) *RouteNode {
	if len(candidates) == 0 {
		return nil
	}

	key := "default"
	index := r.roundRobinIndex[key]
	selected := candidates[index%len(candidates)]
	r.roundRobinIndex[key] = (index + 1) % len(candidates)

	return selected
}

// selectWeightedRoundRobin 加权轮询选择
func (r *MultidimensionalRouter) selectWeightedRoundRobin(candidates []*RouteNode) *RouteNode {
	if len(candidates) == 0 {
		return nil
	}

	// 计算总权重
	totalWeight := 0.0
	for _, node := range candidates {
		totalWeight += node.Weight
	}

	if totalWeight == 0 {
		return r.selectRoundRobin(candidates)
	}

	// 加权选择
	var bestNode *RouteNode
	maxScore := -1.0

	for _, node := range candidates {
		// 考虑权重和负载
		score := node.Weight * float64(node.Capacity-node.Load) / float64(node.Capacity)
		if score > maxScore {
			maxScore = score
			bestNode = node
		}
	}

	return bestNode
}

// selectLeastConnections 最少连接选择
func (r *MultidimensionalRouter) selectLeastConnections(candidates []*RouteNode) *RouteNode {
	if len(candidates) == 0 {
		return nil
	}

	var bestNode *RouteNode
	minConnections := int(^uint(0) >> 1) // 最大int值

	for _, node := range candidates {
		if node.Connections < minConnections {
			minConnections = node.Connections
			bestNode = node
		}
	}

	return bestNode
}

// selectLeastResponseTime 最短响应时间选择
func (r *MultidimensionalRouter) selectLeastResponseTime(candidates []*RouteNode) *RouteNode {
	if len(candidates) == 0 {
		return nil
	}

	var bestNode *RouteNode
	minResponseTime := time.Duration(^uint64(0) >> 1) // 最大duration值

	for _, node := range candidates {
		if node.ResponseTime < minResponseTime {
			minResponseTime = node.ResponseTime
			bestNode = node
		}
	}

	return bestNode
}

// selectConsistentHash 一致性哈希选择
func (r *MultidimensionalRouter) selectConsistentHash(candidates []*RouteNode, criteria map[RouteDimension]string) *RouteNode {
	if len(candidates) == 0 {
		return nil
	}

	// 简单实现：使用第一个候选节点
	// 实际实现应该基于一致性哈希算法
	return candidates[0]
}

// GetNode 获取节点信息
func (r *MultidimensionalRouter) GetNode(nodeID string) (*RouteNode, bool) {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	node, exists := r.nodes[nodeID]
	return node, exists
}