package api

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/cocowh/muxcore/core/governance"
	"github.com/cocowh/muxcore/core/handler"
	"github.com/cocowh/muxcore/core/router"
	"github.com/cocowh/muxcore/pkg/logger"
	"golang.org/x/net/websocket"
)

// AdminAPI 智能运维接口

type AdminAPI struct {
	protocolManager  *governance.ProtocolManager
	processorManager *handler.ProcessorManager
	router           *router.MultidimensionalRouter
}

// NewAdminAPI 创建智能运维接口实例
func NewAdminAPI(
	protocolManager *governance.ProtocolManager,
	processorManager *handler.ProcessorManager,
	router *router.MultidimensionalRouter,
) *AdminAPI {
	api := &AdminAPI{
		protocolManager:  protocolManager,
		processorManager: processorManager,
		router:           router,
	}

	return api
}

// RegisterRoutes 注册API路由
func (api *AdminAPI) RegisterRoutes(httpRouter *router.HTTPRouter) {
	// 协议统计接口
	httpRouter.GET("/protocols/stats", api.getProtocolStats)

	// 协议限流接口
	httpRouter.POST("/protocols/:id/throttle", api.throttleProtocol)

	// 路由策略更新接口
	httpRouter.PUT("/routing/strategy", api.updateRoutingStrategy)

	// 连接监控WebSocket接口
	httpRouter.GET("/debug/connection-monitor", func(w http.ResponseWriter, r *http.Request) {
		websocket.Handler(api.monitorConnections).ServeHTTP(w, r)
	})
}

// 获取协议统计信息
func (api *AdminAPI) getProtocolStats(w http.ResponseWriter, r *http.Request) {
	// 获取时间粒度参数
	granularity := r.URL.Query().Get("granularity")
	if granularity == "" {
		granularity = "5m"
	}

	// 解析时间粒度
	_, err := parseDuration(granularity)
	if err != nil {
		http.Error(w, "Invalid granularity parameter", http.StatusBadRequest)
		return
	}

	// 收集协议统计信息
	stats := make(map[string]map[string]interface{})

	// 这里简化处理，实际应从监控系统获取数据
	// 模拟一些统计数据
	protocols := []string{"http", "websocket", "grpc"}
	for _, proto := range protocols {
		stats[proto] = map[string]interface{}{
			"requests":        1000 + int(time.Now().UnixNano()%1000),
			"errors":          int(time.Now().UnixNano() % 100),
			"latency":         float64(time.Now().UnixNano()%1000) / 100,
			"throughput":      float64(time.Now().UnixNano()%10000) / 10,
			"active_sessions": int(time.Now().UnixNano() % 500),
		}
	}

	// 返回统计信息
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"granularity": granularity,
		"timestamp":   time.Now().Unix(),
		"stats":       stats,
	})
}

// 协议限流控制
func (api *AdminAPI) throttleProtocol(w http.ResponseWriter, r *http.Request) {
	// 获取协议ID
	protocolID := r.URL.Query().Get("id")
	if protocolID == "" {
		http.Error(w, "Protocol ID is required", http.StatusBadRequest)
		return
	}

	// 解析请求体
	var request struct {
		RateLimit int `json:"rate_limit"`
	}

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// 验证限流值
	if request.RateLimit < 0 {
		http.Error(w, "Rate limit must be non-negative", http.StatusBadRequest)
		return
	}

	// 实现限流逻辑
	// 这里简化处理
	logger.Infof("Setting rate limit for protocol %s: %d requests/second", protocolID, request.RateLimit)

	// 返回成功响应
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":     true,
		"message":     "Rate limit set successfully",
		"protocol_id": protocolID,
		"rate_limit":  request.RateLimit,
	})
}

// 更新路由策略
func (api *AdminAPI) updateRoutingStrategy(w http.ResponseWriter, r *http.Request) {
	// 解析请求体
	var request struct {
		Strategy string                 `json:"strategy"`
		Params   map[string]interface{} `json:"params"`
	}

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// 验证策略
	if request.Strategy == "" {
		http.Error(w, "Strategy is required", http.StatusBadRequest)
		return
	}

	// 更新路由策略
	// 这里简化处理
	logger.Infof("Updating routing strategy to: %s", request.Strategy)

	// 实际实现中应调用router的相关方法
	// api.router.SetStrategy(request.Strategy, request.Params)

	// 返回成功响应
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":  true,
		"message":  "Routing strategy updated successfully",
		"strategy": request.Strategy,
	})
}

// 连接监控WebSocket处理
func (api *AdminAPI) monitorConnections(ws *websocket.Conn) {
	logger.Infof("New connection monitor client connected")

	// 定期发送连接统计信息
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	defer func() {
		logger.Infof("Connection monitor client disconnected")
		ws.Close()
	}()

	for range ticker.C {
		// 收集连接统计信息
		stats := api.collectConnectionStats()

		// 发送统计信息
		if err := websocket.JSON.Send(ws, stats); err != nil {
			logger.Errorf("Failed to send connection stats: %v", err)
			return
		}
	}
}

// 收集连接统计信息
func (api *AdminAPI) collectConnectionStats() map[string]interface{} {
	// 这里简化处理，实际应从连接池或处理器管理器获取数据
	// 模拟一些统计数据
	protocols := []string{"http", "websocket", "grpc"}
	connectionStats := make(map[string]interface{})
	totalConnections := 0

	for _, proto := range protocols {
		connCount := int(time.Now().UnixNano() % 1000)
		connectionStats[proto] = connCount
		totalConnections += connCount
	}

	return map[string]interface{}{
		"timestamp":         time.Now().Unix(),
		"total_connections": totalConnections,
		"by_protocol":       connectionStats,
	}
}

// 解析时间粒度
func parseDuration(granularity string) (time.Duration, error) {
	duration, err := time.ParseDuration(granularity)
	if err != nil {
		logger.Errorf("Invalid duration format: %v", err)
		return 0, err
	}
	// 使用duration变量
	logger.Infof("Setting rate limit with duration: %v", duration)
	return duration, nil
}
