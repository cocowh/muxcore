package handlers

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	common "github.com/cocowh/muxcore/core/common"
	"github.com/cocowh/muxcore/core/observability"
	"github.com/cocowh/muxcore/core/pool"
	"github.com/cocowh/muxcore/core/router"
	"github.com/cocowh/muxcore/pkg/errors"
)

// HTTPHandler HTTP处理器

type HTTPHandler struct {
	*common.BaseHandler
	router *router.RadixTree
}

// NewHTTPHandler 创建HTTP处理器
func NewHTTPHandler(pool *pool.ConnectionPool, router *router.RadixTree) *HTTPHandler {
	handler := &HTTPHandler{
		BaseHandler: common.NewBaseHandler(pool),
		router:      router,
	}
	return handler
}

// Handle 处理HTTP连接
func (h *HTTPHandler) Handle(connID string, conn net.Conn, initialData []byte) {
	// 记录连接建立
	observability.RecordConnection("http", "established")

	// 更新连接协议
	h.Pool.UpdateConnectionProtocol(connID, "http")

	// 更新活跃连接数
	activeConnCount := h.Pool.GetActiveConnectionCount("http")
	observability.SetActiveConnections("http", activeConnCount)

	// 记录请求开始时间
	startTime := time.Now()

	// 创建一个HTTP请求解析器
	reader := strings.NewReader(string(initialData))
	req, err := http.ReadRequest(bufio.NewReader(reader))
	if err != nil {
		muxErr := errors.ProtocolError(errors.ErrCodeProtocolParseError, "failed to parse HTTP request").WithCause(err).WithContext("connectionID", connID)
		errors.Handle(context.Background(), muxErr)

		// 记录失败请求
		duration := time.Since(startTime).Seconds()
		observability.RecordRequest("http", "", "400", duration)

		h.Pool.RemoveConnection(connID)

		// 更新活跃连接数
		activeConnCount = h.Pool.GetActiveConnectionCount("http")
		observability.SetActiveConnections("http", activeConnCount)

		return
	}

	// 设置远程地址
	req.RemoteAddr = conn.RemoteAddr().String()

	// 创建响应写入器
	w := NewResponseWriter(conn)

	// 使用radix树路由请求
	path := req.URL.Path
	nodeIDs := h.router.Search(path)

	statusCode := http.StatusOK
	if len(nodeIDs) > 0 {
		// 这里简化处理，实际应用中需要根据nodeIDs找到对应的处理函数
		// 为了演示，我们简单地返回一个成功响应
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Hello from Radix Tree Router!"))
	} else {
		// 未找到匹配的路由
		statusCode = http.StatusNotFound
		w.WriteHeader(statusCode)
		w.Write([]byte("404 Not Found"))
	}

	// 计算请求持续时间
	duration := time.Since(startTime).Seconds()

	// 记录请求
	observability.RecordRequest("http", path, fmt.Sprintf("%d", statusCode), duration)

	// 检查连接是否需要保持
	if !w.keepAlive {
		h.Pool.RemoveConnection(connID)

		// 更新活跃连接数
		activeConnCount = h.Pool.GetActiveConnectionCount("http")
		observability.SetActiveConnections("http", activeConnCount)
	}
}

// ResponseWriter 实现http.ResponseWriter接口
type ResponseWriter struct {
	conn       net.Conn
	statusCode int
	headers    http.Header
	keepAlive  bool
}

// NewResponseWriter 创建响应写入器
func NewResponseWriter(conn net.Conn) *ResponseWriter {
	return &ResponseWriter{
		conn:      conn,
		headers:   make(http.Header),
		keepAlive: true,
	}
}

// Write 写入响应体
func (w *ResponseWriter) Write(b []byte) (int, error) {
	if w.statusCode == 0 {
		w.WriteHeader(http.StatusOK)
	}

	// 写入响应体
	return w.conn.Write(b)
}

// WriteHeader 写入状态码
func (w *ResponseWriter) WriteHeader(statusCode int) {
	if w.statusCode != 0 {
		return
	}

	w.statusCode = statusCode

	// 构建响应头
	var buf strings.Builder
	buf.WriteString(fmt.Sprintf("HTTP/1.1 %d %s\r\n", statusCode, http.StatusText(statusCode)))

	// 写入响应头
	for k, v := range w.headers {
		for _, val := range v {
			buf.WriteString(fmt.Sprintf("%s: %s\r\n", k, val))
		}
	}

	// 检查连接是否需要保持
	connection := w.headers.Get("Connection")
	if connection == "close" {
		w.keepAlive = false
	}

	// 写入空行分隔响应头和响应体
	buf.WriteString("\r\n")

	// 写入响应头到连接
	w.conn.Write([]byte(buf.String()))
}

// Header 获取响应头
func (w *ResponseWriter) Header() http.Header {
	return w.headers
}
