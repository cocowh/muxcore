package websocket

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cocowh/muxcore/core/bus"
	common "github.com/cocowh/muxcore/core/common"
	"github.com/cocowh/muxcore/core/pool"
	"github.com/cocowh/muxcore/pkg/logger"
	"golang.org/x/net/websocket"
)

// WebSocketConfig WebSocket配置
type WebSocketConfig struct {
	MaxConnections    int           `yaml:"max_connections"`
	PingInterval      time.Duration `yaml:"ping_interval"`
	PongTimeout       time.Duration `yaml:"pong_timeout"`
	MaxMessageSize    int64         `yaml:"max_message_size"`
	EnableCompression bool          `yaml:"enable_compression"`
	ReadBufferSize    int           `yaml:"read_buffer_size"`
	WriteBufferSize   int           `yaml:"write_buffer_size"`
	HandshakeTimeout  time.Duration `yaml:"handshake_timeout"`
}

// DefaultWebSocketConfig 默认WebSocket配置
func DefaultWebSocketConfig() *WebSocketConfig {
	return &WebSocketConfig{
		MaxConnections:    1000,
		PingInterval:      30 * time.Second,
		PongTimeout:       10 * time.Second,
		MaxMessageSize:    1024 * 1024, // 1MB
		EnableCompression: true,
		ReadBufferSize:    4096,
		WriteBufferSize:   4096,
		HandshakeTimeout:  10 * time.Second,
	}
}

// ConnectionLimiter 连接限制器
type ConnectionLimiter struct {
	maxConnections int64
	current        int64
	mu             sync.Mutex
}

// NewConnectionLimiter 创建连接限制器
func NewConnectionLimiter(maxConnections int) *ConnectionLimiter {
	return &ConnectionLimiter{
		maxConnections: int64(maxConnections),
		current:        0,
	}
}

// TryAcquire 尝试获取连接
func (cl *ConnectionLimiter) TryAcquire() bool {
	return atomic.AddInt64(&cl.current, 1) <= cl.maxConnections
}

// Release 释放连接
func (cl *ConnectionLimiter) Release() {
	atomic.AddInt64(&cl.current, -1)
}

// Current 当前连接数
func (cl *ConnectionLimiter) Current() int64 {
	return atomic.LoadInt64(&cl.current)
}

// WebSocketConnection WebSocket连接
type WebSocketConnection struct {
	conn        *websocket.Conn
	id          string
	lastPing    time.Time
	lastPong    time.Time
	pingTicker  *time.Ticker
	isAlive     bool
	mu          sync.RWMutex
	ctx         context.Context
	cancel      context.CancelFunc
}

// NewWebSocketConnection 创建WebSocket连接
func NewWebSocketConnection(id string, conn *websocket.Conn, pingInterval time.Duration) *WebSocketConnection {
	ctx, cancel := context.WithCancel(context.Background())
	wsConn := &WebSocketConnection{
		conn:       conn,
		id:         id,
		lastPing:   time.Now(),
		lastPong:   time.Now(),
		pingTicker: time.NewTicker(pingInterval),
		isAlive:    true,
		ctx:        ctx,
		cancel:     cancel,
	}
	
	// 启动心跳检测
	go wsConn.heartbeat()
	return wsConn
}

// heartbeat 心跳检测
func (wc *WebSocketConnection) heartbeat() {
	defer wc.pingTicker.Stop()
	
	for {
		select {
		case <-wc.ctx.Done():
			return
		case <-wc.pingTicker.C:
			wc.mu.Lock()
			if !wc.isAlive {
				wc.mu.Unlock()
				return
			}
			
			// 发送ping
			err := websocket.Message.Send(wc.conn, "ping")
			if err != nil {
				logger.Error("Failed to send ping: ", err)
				wc.isAlive = false
				wc.mu.Unlock()
				return
			}
			
			wc.lastPing = time.Now()
			wc.mu.Unlock()
		}
	}
}

// UpdatePong 更新pong时间
func (wc *WebSocketConnection) UpdatePong() {
	wc.mu.Lock()
	defer wc.mu.Unlock()
	wc.lastPong = time.Now()
}

// IsAlive 检查连接是否存活
func (wc *WebSocketConnection) IsAlive() bool {
	wc.mu.RLock()
	defer wc.mu.RUnlock()
	return wc.isAlive
}

// Close 关闭连接
func (wc *WebSocketConnection) Close() error {
	wc.mu.Lock()
	defer wc.mu.Unlock()
	
	wc.isAlive = false
	wc.cancel()
	return wc.conn.Close()
}

// WebSocketHandler WebSocket处理器
type WebSocketHandler struct {
	*common.BaseHandler
	messageBus      *bus.MessageBus
	config          *WebSocketConfig
	connLimiter     *ConnectionLimiter
	connections     map[string]*WebSocketConnection
	connMu          sync.RWMutex
	upgrader        websocket.Handler
}

// responseWriter 实现http.ResponseWriter接口
 type responseWriter struct {
	conn net.Conn
}

// Write 实现http.ResponseWriter的Write方法
func (r *responseWriter) Write(b []byte) (int, error) {
	return r.conn.Write(b)
}

// WriteHeader 实现http.ResponseWriter的WriteHeader方法
func (r *responseWriter) WriteHeader(statusCode int) {
	// 简化实现，仅写入状态码
	header := fmt.Sprintf("HTTP/1.1 %d %s\r\n", statusCode, http.StatusText(statusCode))
	r.conn.Write([]byte(header))
}

// Header 实现http.ResponseWriter的Header方法
func (r *responseWriter) Header() http.Header {
	return make(http.Header)
}

// NewResponseWriter 创建一个新的ResponseWriter
func NewResponseWriter(conn net.Conn) http.ResponseWriter {
	return &responseWriter{conn: conn}
}

// NewWebSocketHandler 创建WebSocket处理器
func NewWebSocketHandler(pool *pool.ConnectionPool, messageBus *bus.MessageBus) *WebSocketHandler {
	return NewWebSocketHandlerWithConfig(pool, messageBus, DefaultWebSocketConfig())
}

// NewWebSocketHandlerWithConfig 使用配置创建WebSocket处理器
func NewWebSocketHandlerWithConfig(pool *pool.ConnectionPool, messageBus *bus.MessageBus, config *WebSocketConfig) *WebSocketHandler {
	handler := &WebSocketHandler{
		BaseHandler: common.NewBaseHandler(pool),
		messageBus:  messageBus,
		config:      config,
		connLimiter: NewConnectionLimiter(config.MaxConnections),
		connections: make(map[string]*WebSocketConnection),
	}
	
	// 配置升级器
	handler.upgrader = websocket.Handler(handler.handleWebSocket)
	
	return handler
}

// Handle 处理WebSocket连接
func (h *WebSocketHandler) Handle(connID string, conn net.Conn, initialData []byte) {
	// 检查连接限制
	if !h.connLimiter.TryAcquire() {
		logger.Warn("WebSocket connection limit exceeded")
		conn.Close()
		return
	}
	
	// 更新连接协议
	h.Pool.UpdateConnectionProtocol(connID, "websocket")

	// 创建一个虚拟的HTTP请求和响应
	rw := NewResponseWriter(conn)
	req, _ := http.NewRequest("GET", "", nil)

	// 处理WebSocket连接
	h.upgrader.ServeHTTP(rw, req)
}

// handleWebSocket 处理WebSocket连接
func (h *WebSocketHandler) handleWebSocket(ws *websocket.Conn) {
	connID := fmt.Sprintf("ws_%d", time.Now().UnixNano())
	
	// 创建增强的WebSocket连接
	wsConn := NewWebSocketConnection(connID, ws, h.config.PingInterval)
	
	// 注册连接
	h.connMu.Lock()
	h.connections[connID] = wsConn
	h.connMu.Unlock()
	
	// 将WebSocket连接添加到消息总线
	h.messageBus.RegisterConnection(connID, ws)
	
	defer func() {
		// 清理连接
		h.connMu.Lock()
		delete(h.connections, connID)
		h.connMu.Unlock()
		
		h.messageBus.UnregisterConnection(connID)
		h.Pool.RemoveConnection(connID)
		h.connLimiter.Release()
		wsConn.Close()
	}()

	// 启动读取循环
	h.readLoop(connID, wsConn)
}

// readLoop 读取WebSocket消息循环
func (h *WebSocketHandler) readLoop(connID string, wsConn *WebSocketConnection) {
	for {
		var msg string
		err := websocket.Message.Receive(wsConn.conn, &msg)
		if err != nil {
			logger.Error("Failed to receive WebSocket message: ", err)
			return
		}

		// 处理pong消息
		if msg == "pong" {
			wsConn.UpdatePong()
			continue
		}

		logger.Debug("Received WebSocket message from ", connID, ": ", msg)

		// 广播消息到所有连接
		h.messageBus.BroadcastMessage(connID, msg)
	}
}
