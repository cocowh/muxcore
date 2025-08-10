package router

import (
	"sync"

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

// RouteNode 路由节点
type RouteNode struct {
	ID       string
	Address  string
	Capacity int
	Load     int
}

// MultidimensionalRouter 多维度路由矩阵
type MultidimensionalRouter struct {
	nodes            map[string]*RouteNode
	rules            map[RouteDimension]map[string][]string
	weights          map[RouteDimension]float64
	mutex            sync.RWMutex
}

// NewMultidimensionalRouter 创建多维度路由矩阵
func NewMultidimensionalRouter() *MultidimensionalRouter {
	router := &MultidimensionalRouter{
		nodes:   make(map[string]*RouteNode),
		rules:   make(map[RouteDimension]map[string][]string),
		weights: make(map[RouteDimension]float64),
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

	logger.Info("Initialized multidimensional router with default weights")
	return router
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

	// 计算节点得分
	scores := make(map[string]float64)
	for nodeID := range r.nodes {
		scores[nodeID] = 0.0
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

	// 找到得分最高的节点
	var bestNode *RouteNode
	maxScore := -1.0

	for nodeID, score := range scores {
		node := r.nodes[nodeID]
		// 考虑节点负载
		adjustedScore := score * float64(node.Capacity-node.Load) / float64(node.Capacity)

		if adjustedScore > maxScore {
			maxScore = adjustedScore
			bestNode = node
		}
	}

	if bestNode != nil {
		logger.Debug("Selected route node: ", bestNode.ID, " with score: ", maxScore)
	}

	return bestNode
}

// GetNode 获取节点信息
func (r *MultidimensionalRouter) GetNode(nodeID string) (*RouteNode, bool) {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	node, exists := r.nodes[nodeID]
	return node, exists
}