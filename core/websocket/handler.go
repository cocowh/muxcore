package websocket

import (
	"fmt"
	"net"
	"net/http"

	"github.com/cocowh/muxcore/core/bus"
	common "github.com/cocowh/muxcore/core/common"
	"github.com/cocowh/muxcore/core/pool"
	"github.com/cocowh/muxcore/pkg/logger"
	"golang.org/x/net/websocket"
)

// WebSocketHandler WebSocket处理器

type WebSocketHandler struct {
	*common.BaseHandler
	messageBus *bus.MessageBus
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
	handler := &WebSocketHandler{
		BaseHandler: common.NewBaseHandler(pool),
		messageBus:  messageBus,
	}
	return handler
}

// Handle 处理WebSocket连接
func (h *WebSocketHandler) Handle(connID string, conn net.Conn, initialData []byte) {
	// 更新连接协议
	h.Pool.UpdateConnectionProtocol(connID, "websocket")

	// 创建WebSocket处理器
	wsHandler := websocket.Handler(func(ws *websocket.Conn) {
		// 将WebSocket连接添加到消息总线
		h.messageBus.RegisterConnection(connID, ws)

		// 启动读取循环
		go h.readLoop(connID, ws)
	})

	// 创建一个虚拟的HTTP请求和响应
	rw := NewResponseWriter(conn)
	req, _ := http.NewRequest("GET", "", nil)

	// 处理WebSocket连接
	wsHandler.ServeHTTP(rw, req)
}

// readLoop 读取WebSocket消息循环
func (h *WebSocketHandler) readLoop(connID string, ws *websocket.Conn) {
	defer func() {
		// 清理连接
		h.messageBus.UnregisterConnection(connID)
		h.Pool.RemoveConnection(connID)
		ws.Close()
	}()

	for {
		var msg string
		err := websocket.Message.Receive(ws, &msg)
		if err != nil {
			logger.Error("Failed to receive WebSocket message: ", err)
			return
		}

		logger.Debug("Received WebSocket message from ", connID, ": ", msg)

		// 广播消息到所有连接
		h.messageBus.BroadcastMessage(connID, msg)
	}
}
