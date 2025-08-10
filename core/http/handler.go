package http

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	common "github.com/cocowh/muxcore/core/common"
	"github.com/cocowh/muxcore/core/observability"
	"github.com/cocowh/muxcore/core/pool"
	"github.com/cocowh/muxcore/core/router"
	"github.com/cocowh/muxcore/pkg/errors"
	"github.com/cocowh/muxcore/pkg/logger"
)

// Middleware 中间件函数类型
type Middleware func(http.Handler) http.Handler

// HTTPConfig HTTP配置
type HTTPConfig struct {
	EnableHTTP2    bool          `yaml:"enable_http2"`
	MaxHeaderSize  int           `yaml:"max_header_size"`
	ReadTimeout    time.Duration `yaml:"read_timeout"`
	WriteTimeout   time.Duration `yaml:"write_timeout"`
	IdleTimeout    time.Duration `yaml:"idle_timeout"`
	EnableGzip     bool          `yaml:"enable_gzip"`
	MaxRequestSize int64         `yaml:"max_request_size"`
}

// DefaultHTTPConfig 默认HTTP配置
func DefaultHTTPConfig() *HTTPConfig {
	return &HTTPConfig{
		EnableHTTP2:    true,
		MaxHeaderSize:  1 << 20, // 1MB
		ReadTimeout:    30 * time.Second,
		WriteTimeout:   30 * time.Second,
		IdleTimeout:    120 * time.Second,
		EnableGzip:     true,
		MaxRequestSize: 32 << 20, // 32MB
	}
}

// HTTPHandler HTTP处理器
type HTTPHandler struct {
	*common.BaseHandler
	router      *router.RadixTree
	middlewares []Middleware
	config      *HTTPConfig
	mu          sync.RWMutex
}

// NewHTTPHandler 创建HTTP处理器
func NewHTTPHandler(pool *pool.ConnectionPool, router *router.RadixTree) *HTTPHandler {
	return NewHTTPHandlerWithConfig(pool, router, DefaultHTTPConfig())
}

// NewHTTPHandlerWithConfig 使用配置创建HTTP处理器
func NewHTTPHandlerWithConfig(pool *pool.ConnectionPool, router *router.RadixTree, config *HTTPConfig) *HTTPHandler {
	handler := &HTTPHandler{
		BaseHandler: common.NewBaseHandler(pool),
		router:      router,
		middlewares: make([]Middleware, 0),
		config:      config,
	}
	return handler
}

// AddMiddleware 添加中间件
func (h *HTTPHandler) AddMiddleware(middleware Middleware) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.middlewares = append(h.middlewares, middleware)
	logger.Info("Added HTTP middleware")
}

// buildMiddlewareChain 构建中间件链
func (h *HTTPHandler) buildMiddlewareChain(handler http.Handler) http.Handler {
	h.mu.RLock()
	defer h.mu.RUnlock()

	// 从后往前应用中间件
	for i := len(h.middlewares) - 1; i >= 0; i-- {
		handler = h.middlewares[i](handler)
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

	// 创建基础处理器
	baseHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 使用radix树路由请求
		path := r.URL.Path
		nodeIDs := h.router.Search(path)

		if len(nodeIDs) > 0 {
			// 这里简化处理，实际应用中需要根据nodeIDs找到对应的处理函数
			// 为了演示，我们简单地返回一个成功响应
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"message": "Hello from Enhanced HTTP Handler!", "path": "` + path + `"}`))
		} else {
			// 未找到匹配的路由
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte(`{"error": "404 Not Found", "path": "` + path + `"}`))
		}
	})

	// 应用中间件链
	handler := h.buildMiddlewareChain(baseHandler)

	// 处理请求
	handler.ServeHTTP(w, req)

	// 计算请求持续时间
	duration := time.Since(startTime).Seconds()

	// 记录请求
	statusCode := w.Status()
	if statusCode == 0 {
		statusCode = http.StatusOK
	}
	observability.RecordRequest("http", req.URL.Path, fmt.Sprintf("%d", statusCode), duration)

	// 检查连接是否需要保持
	if !w.keepAlive {
		h.Pool.RemoveConnection(connID)

		// 更新活跃连接数
		activeConnCount = h.Pool.GetActiveConnectionCount("http")
		observability.SetActiveConnections("http", activeConnCount)
	}
}

// LoggingMiddleware 日志中间件
func LoggingMiddleware() Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			next.ServeHTTP(w, r)
			duration := time.Since(start)

			logger.Info(fmt.Sprintf("HTTP %s %s - %v", r.Method, r.URL.Path, duration))
		})
	}
}

// RecoveryMiddleware 恢复中间件
func RecoveryMiddleware() Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if err := recover(); err != nil {
					logger.Error(fmt.Sprintf("HTTP panic recovered: %v", err))
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusInternalServerError)
					w.Write([]byte(`{"error": "Internal Server Error"}`))
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}

// CORSMiddleware CORS中间件
func CORSMiddleware() Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

			if r.Method == "OPTIONS" {
				w.WriteHeader(http.StatusOK)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// EnhancedResponseWriter 增强的响应写入器
type EnhancedResponseWriter struct {
	conn          net.Conn
	statusCode    int
	headers       http.Header
	keepAlive     bool
	headerSent    bool
	contentLength int64
	written       int64
	mu            sync.Mutex
}

// NewResponseWriter 创建响应写入器
func NewResponseWriter(conn net.Conn) *EnhancedResponseWriter {
	return &EnhancedResponseWriter{
		conn:      conn,
		headers:   make(http.Header),
		keepAlive: true,
	}
}

// Write 写入响应体
func (w *EnhancedResponseWriter) Write(b []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if !w.headerSent {
		w.WriteHeader(http.StatusOK)
	}

	n, err := w.conn.Write(b)
	w.written += int64(n)
	return n, err
}

// WriteHeader 写入状态码
func (w *EnhancedResponseWriter) WriteHeader(statusCode int) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.headerSent {
		return
	}

	w.statusCode = statusCode
	w.headerSent = true

	// 构建响应头
	var buf strings.Builder
	buf.WriteString(fmt.Sprintf("HTTP/1.1 %d %s\r\n", statusCode, http.StatusText(statusCode)))

	// 设置默认头部
	if w.headers.Get("Content-Type") == "" {
		w.headers.Set("Content-Type", "text/plain; charset=utf-8")
	}

	if w.headers.Get("Date") == "" {
		w.headers.Set("Date", time.Now().UTC().Format(http.TimeFormat))
	}

	if w.headers.Get("Server") == "" {
		w.headers.Set("Server", "MuxCore/1.0")
	}

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
func (w *EnhancedResponseWriter) Header() http.Header {
	return w.headers
}

// Flush 刷新缓冲区（实现http.Flusher接口）
func (w *EnhancedResponseWriter) Flush() {
	// TCP连接自动刷新，这里可以添加额外的刷新逻辑
}

// Hijack 劫持连接（实现http.Hijacker接口）
func (w *EnhancedResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	return w.conn, bufio.NewReadWriter(bufio.NewReader(w.conn), bufio.NewWriter(w.conn)), nil
}

// CloseNotify 连接关闭通知（实现http.CloseNotifier接口）
func (w *EnhancedResponseWriter) CloseNotify() <-chan bool {
	ch := make(chan bool, 1)
	// 简化实现，实际应该监听连接状态
	go func() {
		// 这里应该实现真正的连接关闭检测
		ch <- true
	}()
	return ch
}

// Size 返回已写入的字节数
func (w *EnhancedResponseWriter) Size() int64 {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.written
}

// Status 返回状态码
func (w *EnhancedResponseWriter) Status() int {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.statusCode
}
