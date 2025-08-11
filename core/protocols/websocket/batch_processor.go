package websocket

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cocowh/muxcore/core/handler"
	"github.com/cocowh/muxcore/core/pool"
	"github.com/cocowh/muxcore/pkg/logger"
	"golang.org/x/net/websocket"
)

// OptimizedWebSocketConfig 优化的WebSocket配置
type OptimizedWebSocketConfig struct {
	*WebSocketConfig

	// 连接管理
	MaxConnections    int           `json:"max_connections"`
	ConnectionTimeout time.Duration `json:"connection_timeout"`
	HeartbeatInterval time.Duration `json:"heartbeat_interval"`
	HeartbeatTimeout  time.Duration `json:"heartbeat_timeout"`

	// 消息处理
	MaxMessageSize    int64 `json:"max_message_size"`
	MessageBufferSize int   `json:"message_buffer_size"`
	EnableCompression bool  `json:"enable_compression"`

	// 性能优化
	EnableBatching bool          `json:"enable_batching"`
	BatchSize      int           `json:"batch_size"`
	BatchTimeout   time.Duration `json:"batch_timeout"`
	EnableMetrics  bool          `json:"enable_metrics"`

	// 安全配置
	EnableRateLimit bool          `json:"enable_rate_limit"`
	RateLimit       int           `json:"rate_limit"`
	RateLimitWindow time.Duration `json:"rate_limit_window"`
}

// DefaultOptimizedWebSocketConfig 默认优化配置
func DefaultOptimizedWebSocketConfig() *OptimizedWebSocketConfig {
	return &OptimizedWebSocketConfig{
		WebSocketConfig:   DefaultWebSocketConfig(),
		MaxConnections:    10000,
		ConnectionTimeout: 30 * time.Second,
		HeartbeatInterval: 30 * time.Second,
		HeartbeatTimeout:  10 * time.Second,
		MaxMessageSize:    1024 * 1024, // 1MB
		MessageBufferSize: 1000,
		EnableCompression: true,
		EnableBatching:    true,
		BatchSize:         100,
		BatchTimeout:      10 * time.Millisecond,
		EnableMetrics:     true,
		EnableRateLimit:   true,
		RateLimit:         1000,
		RateLimitWindow:   time.Minute,
	}
}

// WebSocketMetrics WebSocket性能指标
type WebSocketMetrics struct {
	// 连接指标
	ActiveConnections int64 `json:"active_connections"`
	TotalConnections  int64 `json:"total_connections"`
	ConnectionErrors  int64 `json:"connection_errors"`

	// 消息指标
	MessagesSent     int64 `json:"messages_sent"`
	MessagesReceived int64 `json:"messages_received"`
	MessageErrors    int64 `json:"message_errors"`
	BytesTransferred int64 `json:"bytes_transferred"`

	// 性能指标
	AverageLatency float64 `json:"average_latency"`
	Throughput     float64 `json:"throughput"`
	ErrorRate      float64 `json:"error_rate"`

	// 批处理指标
	BatchesSent     int64   `json:"batches_sent"`
	BatchEfficiency float64 `json:"batch_efficiency"`

	// 限流指标
	RateLimitHits    int64 `json:"rate_limit_hits"`
	RateLimitRejects int64 `json:"rate_limit_rejects"`
}

// OptimizedConnection 优化的WebSocket连接
type OptimizedConnection struct {
	conn           *websocket.Conn
	id             string
	lastActivity   time.Time
	heartbeatTimer *time.Timer
	messageBuffer  chan []byte
	batchBuffer    [][]byte
	batchTimer     *time.Timer
	rateLimit      *RateLimiter
	metrics        *ConnectionMetrics
	mu             sync.RWMutex
	closed         int32
}

// ConnectionMetrics 连接级别指标
type ConnectionMetrics struct {
	MessagesSent     int64     `json:"messages_sent"`
	MessagesReceived int64     `json:"messages_received"`
	BytesTransferred int64     `json:"bytes_transferred"`
	ConnectedAt      time.Time `json:"connected_at"`
	LastActivity     time.Time `json:"last_activity"`
}

// RateLimiter 速率限制器
type RateLimiter struct {
	limit    int
	window   time.Duration
	requests []time.Time
	mu       sync.Mutex
}

// NewRateLimiter 创建速率限制器
func NewRateLimiter(limit int, window time.Duration) *RateLimiter {
	return &RateLimiter{
		limit:    limit,
		window:   window,
		requests: make([]time.Time, 0, limit),
	}
}

// Allow 检查是否允许请求
func (rl *RateLimiter) Allow() bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()

	// 清理过期请求
	cutoff := now.Add(-rl.window)
	for len(rl.requests) > 0 && rl.requests[0].Before(cutoff) {
		rl.requests = rl.requests[1:]
	}

	// 检查是否超过限制
	if len(rl.requests) >= rl.limit {
		return false
	}

	// 添加当前请求
	rl.requests = append(rl.requests, now)
	return true
}

// OptimizedWebSocketHandler 优化的WebSocket处理器
type OptimizedWebSocketHandler struct {
	*handler.BaseHandler
	config      *OptimizedWebSocketConfig
	connections map[string]*OptimizedConnection
	metrics     *WebSocketMetrics
	upgrader    websocket.Handler
	mu          sync.RWMutex
	ctx         context.Context
	cancel      context.CancelFunc
}

// NewOptimizedWebSocketHandler 创建优化的WebSocket处理器
func NewOptimizedWebSocketHandler(pool *pool.ConnectionPool, config *OptimizedWebSocketConfig) *OptimizedWebSocketHandler {
	if config == nil {
		config = DefaultOptimizedWebSocketConfig()
	}

	ctx, cancel := context.WithCancel(context.Background())

	h := &OptimizedWebSocketHandler{
		BaseHandler: handler.NewBaseHandler(),
		config:      config,
		connections: make(map[string]*OptimizedConnection),
		metrics:     &WebSocketMetrics{},
		upgrader: websocket.Handler(func(ws *websocket.Conn) {
			// WebSocket处理逻辑将在这里实现
			logger.Info("WebSocket connection established")
		}),
		ctx:    ctx,
		cancel: cancel,
	}

	// 启动指标收集
	if config.EnableMetrics {
		go h.metricsCollector()
	}

	return h
}

// Handle 处理WebSocket连接
func (h *OptimizedWebSocketHandler) Handle(conn net.Conn) error {
	// 检查连接数限制
	h.mu.RLock()
	activeConns := len(h.connections)
	h.mu.RUnlock()

	if activeConns >= h.config.MaxConnections {
		atomic.AddInt64(&h.metrics.ConnectionErrors, 1)
		return fmt.Errorf("maximum connections reached: %d", h.config.MaxConnections)
	}

	// 升级到WebSocket连接
	wsConn, err := h.upgradeConnection(conn)
	if err != nil {
		atomic.AddInt64(&h.metrics.ConnectionErrors, 1)
		return fmt.Errorf("failed to upgrade connection: %w", err)
	}

	// 创建优化连接
	optConn := h.createOptimizedConnection(wsConn)

	// 注册连接
	h.registerConnection(optConn)

	// 启动连接处理
	go h.handleConnection(optConn)

	return nil
}

// upgradeConnection 升级连接为WebSocket
func (h *OptimizedWebSocketHandler) upgradeConnection(conn net.Conn) (*websocket.Conn, error) {
	// 这里需要根据实际的HTTP请求进行升级
	// 由于我们只有net.Conn，这里简化处理
	logger.Info("Upgrading connection to WebSocket")
	return nil, fmt.Errorf("WebSocket upgrade requires HTTP request")
}

// createOptimizedConnection 创建优化连接
func (h *OptimizedWebSocketHandler) createOptimizedConnection(wsConn *websocket.Conn) *OptimizedConnection {
	connID := fmt.Sprintf("conn_%d", time.Now().UnixNano())

	optConn := &OptimizedConnection{
		conn:          wsConn,
		id:            connID,
		lastActivity:  time.Now(),
		messageBuffer: make(chan []byte, h.config.MessageBufferSize),
		batchBuffer:   make([][]byte, 0, h.config.BatchSize),
		metrics: &ConnectionMetrics{
			ConnectedAt:  time.Now(),
			LastActivity: time.Now(),
		},
	}

	// 设置速率限制
	if h.config.EnableRateLimit {
		optConn.rateLimit = NewRateLimiter(h.config.RateLimit, h.config.RateLimitWindow)
	}

	// 设置心跳定时器
	optConn.heartbeatTimer = time.AfterFunc(h.config.HeartbeatInterval, func() {
		h.sendHeartbeat(optConn)
	})

	// 设置批处理定时器
	if h.config.EnableBatching {
		optConn.batchTimer = time.AfterFunc(h.config.BatchTimeout, func() {
			h.flushBatch(optConn)
		})
	}

	return optConn
}

// registerConnection 注册连接
func (h *OptimizedWebSocketHandler) registerConnection(conn *OptimizedConnection) {
	h.mu.Lock()
	h.connections[conn.id] = conn
	h.mu.Unlock()

	atomic.AddInt64(&h.metrics.ActiveConnections, 1)
	atomic.AddInt64(&h.metrics.TotalConnections, 1)
}

// handleConnection 处理连接
func (h *OptimizedWebSocketHandler) handleConnection(conn *OptimizedConnection) {
	defer h.closeConnection(conn)

	// 启动消息发送协程
	go h.messageSender(conn)

	// 处理接收消息
	for {
		select {
		case <-h.ctx.Done():
			return
		default:
			// 设置读取超时
			conn.conn.SetReadDeadline(time.Now().Add(h.config.ConnectionTimeout))

			// 读取消息
			var data []byte
			err := websocket.Message.Receive(conn.conn, &data)
			if err != nil {
				logger.Error("WebSocket read error", "error", err)
				atomic.AddInt64(&h.metrics.MessageErrors, 1)
				return
			}

			// 处理消息
			h.processMessage(conn, data)
		}
	}
}

// processMessage 处理消息
func (h *OptimizedWebSocketHandler) processMessage(conn *OptimizedConnection, data []byte) {
	// 检查速率限制
	if h.config.EnableRateLimit && conn.rateLimit != nil {
		if !conn.rateLimit.Allow() {
			atomic.AddInt64(&h.metrics.RateLimitRejects, 1)
			return
		}
	}

	// 更新活动时间
	conn.mu.Lock()
	conn.lastActivity = time.Now()
	conn.metrics.LastActivity = time.Now()
	conn.mu.Unlock()

	// 更新指标
	atomic.AddInt64(&h.metrics.MessagesReceived, 1)
	atomic.AddInt64(&conn.metrics.MessagesReceived, 1)
	atomic.AddInt64(&h.metrics.BytesTransferred, int64(len(data)))
	atomic.AddInt64(&conn.metrics.BytesTransferred, int64(len(data)))

	// 处理消息（golang.org/x/net/websocket主要处理文本和二进制消息）
	h.handleMessage(conn, data)
}

// handleMessage 处理消息
func (h *OptimizedWebSocketHandler) handleMessage(conn *OptimizedConnection, data []byte) {
	// 解析消息
	var msg map[string]interface{}
	if err := json.Unmarshal(data, &msg); err != nil {
		logger.Error("Failed to parse text message", "error", err)
		atomic.AddInt64(&h.metrics.MessageErrors, 1)
		return
	}

	// 处理消息逻辑
	logger.Info("Received text message", "conn_id", conn.id, "message", msg)

	// 回显消息（示例）
	response := map[string]interface{}{
		"type":      "echo",
		"data":      msg,
		"timestamp": time.Now().Unix(),
	}

	responseData, _ := json.Marshal(response)
	h.sendMessage(conn, responseData)
}

// sendMessage 发送消息
func (h *OptimizedWebSocketHandler) sendMessage(conn *OptimizedConnection, data []byte) {
	if atomic.LoadInt32(&conn.closed) == 1 {
		return
	}

	select {
	case conn.messageBuffer <- data:
		// 消息已加入缓冲区
	default:
		// 缓冲区满，丢弃消息
		logger.Error("Message buffer full, dropping message", "conn_id", conn.id)
		atomic.AddInt64(&h.metrics.MessageErrors, 1)
	}
}

// messageSender 消息发送协程
func (h *OptimizedWebSocketHandler) messageSender(conn *OptimizedConnection) {
	for {
		select {
		case <-h.ctx.Done():
			return
		case data := <-conn.messageBuffer:
			if atomic.LoadInt32(&conn.closed) == 1 {
				return
			}

			if h.config.EnableBatching {
				h.addToBatch(conn, data)
			} else {
				h.sendDirectly(conn, data)
			}
		}
	}
}

// addToBatch 添加到批处理
func (h *OptimizedWebSocketHandler) addToBatch(conn *OptimizedConnection, data []byte) {
	conn.mu.Lock()
	conn.batchBuffer = append(conn.batchBuffer, data)
	batchFull := len(conn.batchBuffer) >= h.config.BatchSize
	conn.mu.Unlock()

	if batchFull {
		h.flushBatch(conn)
	}
}

// flushBatch 刷新批处理
func (h *OptimizedWebSocketHandler) flushBatch(conn *OptimizedConnection) {
	conn.mu.Lock()
	if len(conn.batchBuffer) == 0 {
		conn.mu.Unlock()
		return
	}

	batch := make([][]byte, len(conn.batchBuffer))
	copy(batch, conn.batchBuffer)
	conn.batchBuffer = conn.batchBuffer[:0]
	conn.mu.Unlock()

	// 发送批处理消息
	for _, data := range batch {
		h.sendDirectly(conn, data)
	}

	atomic.AddInt64(&h.metrics.BatchesSent, 1)

	// 重置批处理定时器
	if conn.batchTimer != nil {
		conn.batchTimer.Reset(h.config.BatchTimeout)
	}
}

// sendDirectly 直接发送消息
func (h *OptimizedWebSocketHandler) sendDirectly(conn *OptimizedConnection, data []byte) {
	if err := websocket.Message.Send(conn.conn, string(data)); err != nil {
		logger.Error("Failed to send message", "conn_id", conn.id, "error", err)
		atomic.AddInt64(&h.metrics.MessageErrors, 1)
		return
	}

	atomic.AddInt64(&h.metrics.MessagesSent, 1)
	atomic.AddInt64(&conn.metrics.MessagesSent, 1)
	atomic.AddInt64(&h.metrics.BytesTransferred, int64(len(data)))
	atomic.AddInt64(&conn.metrics.BytesTransferred, int64(len(data)))
}

// sendHeartbeat 发送心跳
func (h *OptimizedWebSocketHandler) sendHeartbeat(conn *OptimizedConnection) {
	if atomic.LoadInt32(&conn.closed) == 1 {
		return
	}

	heartbeat := map[string]interface{}{
		"type":      "heartbeat",
		"timestamp": time.Now().Unix(),
	}

	data, _ := json.Marshal(heartbeat)
	h.sendMessage(conn, data)

	// 重置心跳定时器
	if conn.heartbeatTimer != nil {
		conn.heartbeatTimer.Reset(h.config.HeartbeatInterval)
	}
}

// closeConnection 关闭连接
func (h *OptimizedWebSocketHandler) closeConnection(conn *OptimizedConnection) {
	if !atomic.CompareAndSwapInt32(&conn.closed, 0, 1) {
		return
	}

	// 停止定时器
	if conn.heartbeatTimer != nil {
		conn.heartbeatTimer.Stop()
	}
	if conn.batchTimer != nil {
		conn.batchTimer.Stop()
	}

	// 刷新剩余批处理消息
	if h.config.EnableBatching {
		h.flushBatch(conn)
	}

	// 关闭WebSocket连接
	conn.conn.Close()

	// 从连接池中移除
	h.mu.Lock()
	delete(h.connections, conn.id)
	h.mu.Unlock()

	atomic.AddInt64(&h.metrics.ActiveConnections, -1)

	logger.Info("Connection closed", "conn_id", conn.id)
}

// metricsCollector 指标收集器
func (h *OptimizedWebSocketHandler) metricsCollector() {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-h.ctx.Done():
			return
		case <-ticker.C:
			h.collectMetrics()
		}
	}
}

// collectMetrics 收集指标
func (h *OptimizedWebSocketHandler) collectMetrics() {
	// 计算错误率
	totalMessages := atomic.LoadInt64(&h.metrics.MessagesSent) + atomic.LoadInt64(&h.metrics.MessagesReceived)
	if totalMessages > 0 {
		errorRate := float64(atomic.LoadInt64(&h.metrics.MessageErrors)) / float64(totalMessages) * 100
		h.metrics.ErrorRate = errorRate
	}

	// 计算批处理效率
	if h.config.EnableBatching {
		batchesSent := atomic.LoadInt64(&h.metrics.BatchesSent)
		if batchesSent > 0 {
			efficiency := float64(atomic.LoadInt64(&h.metrics.MessagesSent)) / float64(batchesSent)
			h.metrics.BatchEfficiency = efficiency
		}
	}

	// 计算吞吐量（消息/秒）
	messagesInWindow := atomic.LoadInt64(&h.metrics.MessagesSent) + atomic.LoadInt64(&h.metrics.MessagesReceived)
	h.metrics.Throughput = float64(messagesInWindow) / 10.0

	logger.Info("WebSocket metrics collected",
		"active_connections", atomic.LoadInt64(&h.metrics.ActiveConnections),
		"total_connections", atomic.LoadInt64(&h.metrics.TotalConnections),
		"messages_sent", atomic.LoadInt64(&h.metrics.MessagesSent),
		"messages_received", atomic.LoadInt64(&h.metrics.MessagesReceived),
		"error_rate", h.metrics.ErrorRate,
		"throughput", h.metrics.Throughput,
	)
}

// GetMetrics 获取性能指标
func (h *OptimizedWebSocketHandler) GetMetrics() *WebSocketMetrics {
	return h.metrics
}

// GetConnectionMetrics 获取连接指标
func (h *OptimizedWebSocketHandler) GetConnectionMetrics(connID string) *ConnectionMetrics {
	h.mu.RLock()
	conn, exists := h.connections[connID]
	h.mu.RUnlock()

	if !exists {
		return nil
	}

	return conn.metrics
}

// BroadcastMessage 广播消息
func (h *OptimizedWebSocketHandler) BroadcastMessage(data []byte) {
	h.mu.RLock()
	connections := make([]*OptimizedConnection, 0, len(h.connections))
	for _, conn := range h.connections {
		connections = append(connections, conn)
	}
	h.mu.RUnlock()

	for _, conn := range connections {
		h.sendMessage(conn, data)
	}
}

// Stop 停止处理器
func (h *OptimizedWebSocketHandler) Stop() error {
	h.cancel()

	// 关闭所有连接
	h.mu.RLock()
	connections := make([]*OptimizedConnection, 0, len(h.connections))
	for _, conn := range h.connections {
		connections = append(connections, conn)
	}
	h.mu.RUnlock()

	for _, conn := range connections {
		h.closeConnection(conn)
	}

	return nil
}

// GetActiveConnections 获取活跃连接数
func (h *OptimizedWebSocketHandler) GetActiveConnections() int {
	h.mu.RLock()
	count := len(h.connections)
	h.mu.RUnlock()
	return count
}

// GetConnectionIDs 获取所有连接ID
func (h *OptimizedWebSocketHandler) GetConnectionIDs() []string {
	h.mu.RLock()
	ids := make([]string, 0, len(h.connections))
	for id := range h.connections {
		ids = append(ids, id)
	}
	h.mu.RUnlock()
	return ids
}
