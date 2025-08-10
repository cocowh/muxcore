package http

import (
	"bufio"
	"compress/gzip"
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	common "github.com/cocowh/muxcore/core/common"
	"github.com/cocowh/muxcore/core/observability"
	"github.com/cocowh/muxcore/core/pool"
	"github.com/cocowh/muxcore/core/router"
	"github.com/cocowh/muxcore/pkg/errors"
	"github.com/cocowh/muxcore/pkg/logger"
	"golang.org/x/net/http2"
)

// OptimizedHTTPConfig 优化的HTTP配置
type OptimizedHTTPConfig struct {
	EnableHTTP2        bool          `yaml:"enable_http2"`
	EnableHTTP3        bool          `yaml:"enable_http3"`
	MaxHeaderSize      int           `yaml:"max_header_size"`
	ReadTimeout        time.Duration `yaml:"read_timeout"`
	WriteTimeout       time.Duration `yaml:"write_timeout"`
	IdleTimeout        time.Duration `yaml:"idle_timeout"`
	EnableGzip         bool          `yaml:"enable_gzip"`
	GzipLevel          int           `yaml:"gzip_level"`
	MaxRequestSize     int64         `yaml:"max_request_size"`
	MaxConcurrentConns int           `yaml:"max_concurrent_conns"`
	KeepAliveTimeout   time.Duration `yaml:"keep_alive_timeout"`
	EnableCompression  bool          `yaml:"enable_compression"`
	CompressionLevel   int           `yaml:"compression_level"`
	TLSConfig          *tls.Config   `yaml:"-"`
}

// DefaultOptimizedHTTPConfig 默认优化HTTP配置
func DefaultOptimizedHTTPConfig() *OptimizedHTTPConfig {
	return &OptimizedHTTPConfig{
		EnableHTTP2:        true,
		EnableHTTP3:        false,
		MaxHeaderSize:      1 << 20, // 1MB
		ReadTimeout:        30 * time.Second,
		WriteTimeout:       30 * time.Second,
		IdleTimeout:        120 * time.Second,
		EnableGzip:         true,
		GzipLevel:          gzip.DefaultCompression,
		MaxRequestSize:     32 << 20, // 32MB
		MaxConcurrentConns: 10000,
		KeepAliveTimeout:   60 * time.Second,
		EnableCompression:  true,
		CompressionLevel:   6,
	}
}

// HTTPMetrics HTTP指标
type HTTPMetrics struct {
	TotalRequests      int64
	SuccessfulRequests int64
	FailedRequests     int64
	ActiveConnections  int64
	AverageLatency     float64
	Throughput         float64
	ErrorRate          float64
	mu                 sync.RWMutex
}

// MiddlewareChain 中间件链
type MiddlewareChain struct {
	middlewares []Middleware
	mu          sync.RWMutex
}

// Add 添加中间件
func (mc *MiddlewareChain) Add(middleware Middleware) {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	mc.middlewares = append(mc.middlewares, middleware)
}

// Build 构建中间件链
func (mc *MiddlewareChain) Build(handler http.Handler) http.Handler {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	result := handler
	for i := len(mc.middlewares) - 1; i >= 0; i-- {
		result = mc.middlewares[i](result)
	}
	return result
}

// OptimizedHTTPHandler 优化的HTTP处理器
type OptimizedHTTPHandler struct {
	*common.BaseHandler
	router          *router.RadixTree
	middlewareChain *MiddlewareChain
	config          *OptimizedHTTPConfig
	metrics         *HTTPMetrics
	connLimiter     *ConnectionLimiter
	compressionPool *sync.Pool
	responsePool    *sync.Pool
	http2Server     *http2.Server
	shutdown        chan struct{}
	running         int32
}

// ConnectionLimiter 连接限制器
type ConnectionLimiter struct {
	maxConnections int
	current        int64
	mu             sync.Mutex
}

// NewConnectionLimiter 创建连接限制器
func NewConnectionLimiter(max int) *ConnectionLimiter {
	return &ConnectionLimiter{
		maxConnections: max,
	}
}

// Acquire 获取连接
func (cl *ConnectionLimiter) Acquire() bool {
	cl.mu.Lock()
	defer cl.mu.Unlock()

	if int(cl.current) >= cl.maxConnections {
		return false
	}
	atomic.AddInt64(&cl.current, 1)
	return true
}

// Release 释放连接
func (cl *ConnectionLimiter) Release() {
	atomic.AddInt64(&cl.current, -1)
}

// GetCurrent 获取当前连接数
func (cl *ConnectionLimiter) GetCurrent() int64 {
	return atomic.LoadInt64(&cl.current)
}

// OptimizedResponseWriter 优化的响应写入器
type OptimizedResponseWriter struct {
	conn       net.Conn
	header     http.Header
	status     int
	written    int64
	headerSent bool
	mu         sync.Mutex
}

// Reset 重置响应写入器
func (w *OptimizedResponseWriter) Reset(conn net.Conn) {
	w.conn = conn
	w.status = 0
	w.written = 0
	w.headerSent = false
	for k := range w.header {
		delete(w.header, k)
	}
}

// Header 返回响应头
func (w *OptimizedResponseWriter) Header() http.Header {
	return w.header
}

// Write 写入响应体
func (w *OptimizedResponseWriter) Write(data []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if !w.headerSent {
		w.WriteHeader(http.StatusOK)
	}

	n, err := w.conn.Write(data)
	w.written += int64(n)
	return n, err
}

// WriteHeader 写入响应头
func (w *OptimizedResponseWriter) WriteHeader(statusCode int) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.headerSent {
		return
	}

	w.status = statusCode
	w.headerSent = true

	// 构建HTTP响应头
	response := fmt.Sprintf("HTTP/1.1 %d %s\r\n", statusCode, http.StatusText(statusCode))
	for key, values := range w.header {
		for _, value := range values {
			response += fmt.Sprintf("%s: %s\r\n", key, value)
		}
	}
	response += "\r\n"

	w.conn.Write([]byte(response))
}

// Status 返回状态码
func (w *OptimizedResponseWriter) Status() int {
	return w.status
}

// Size 返回写入字节数
func (w *OptimizedResponseWriter) Size() int64 {
	return w.written
}

// NewOptimizedHTTPHandler 创建优化的HTTP处理器
func NewOptimizedHTTPHandler(pool *pool.ConnectionPool, router *router.RadixTree, config *OptimizedHTTPConfig) *OptimizedHTTPHandler {
	if config == nil {
		config = DefaultOptimizedHTTPConfig()
	}

	h := &OptimizedHTTPHandler{
		BaseHandler:     common.NewBaseHandler(pool),
		router:          router,
		middlewareChain: &MiddlewareChain{},
		config:          config,
		metrics:         &HTTPMetrics{},
		connLimiter:     NewConnectionLimiter(config.MaxConcurrentConns),
		shutdown:        make(chan struct{}),
	}

	// 初始化对象池
	h.compressionPool = &sync.Pool{
		New: func() interface{} {
			w, _ := gzip.NewWriterLevel(nil, config.GzipLevel)
			return w
		},
	}

	h.responsePool = &sync.Pool{
		New: func() interface{} {
			return &OptimizedResponseWriter{
				header: make(http.Header),
			}
		},
	}

	// 配置HTTP/2
	if config.EnableHTTP2 {
		h.http2Server = &http2.Server{
			MaxConcurrentStreams:         250,
			MaxReadFrameSize:             1 << 20,
			IdleTimeout:                  config.IdleTimeout,
			MaxUploadBufferPerConnection: 1 << 20,
			MaxUploadBufferPerStream:     1 << 20,
		}
	}

	// 添加默认中间件
	h.AddDefaultMiddlewares()

	return h
}

// AddDefaultMiddlewares 添加默认中间件
func (h *OptimizedHTTPHandler) AddDefaultMiddlewares() {
	h.middlewareChain.Add(LoggingMiddleware())
	h.middlewareChain.Add(RecoveryMiddleware())
	h.middlewareChain.Add(MetricsMiddleware(h.metrics))
	if h.config.EnableGzip {
		h.middlewareChain.Add(CompressionMiddleware(h.compressionPool))
	}
	h.middlewareChain.Add(CORSMiddleware())
	h.middlewareChain.Add(SecurityHeadersMiddleware())
}

// MetricsMiddleware 指标中间件
func MetricsMiddleware(metrics *HTTPMetrics) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			next.ServeHTTP(w, r)
			duration := time.Since(start)

			// 更新平均延迟
			metrics.mu.Lock()
			if metrics.AverageLatency == 0 {
				metrics.AverageLatency = duration.Seconds()
			} else {
				metrics.AverageLatency = (metrics.AverageLatency + duration.Seconds()) / 2
			}
			metrics.mu.Unlock()
		})
	}
}

// CompressionMiddleware 压缩中间件
func CompressionMiddleware(pool *sync.Pool) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
				w.Header().Set("Content-Encoding", "gzip")
				gzw := pool.Get().(*gzip.Writer)
				defer pool.Put(gzw)

				gzw.Reset(w)
				defer gzw.Close()

				next.ServeHTTP(&gzipResponseWriter{ResponseWriter: w, Writer: gzw}, r)
			} else {
				next.ServeHTTP(w, r)
			}
		})
	}
}

// SecurityHeadersMiddleware 安全头中间件
func SecurityHeadersMiddleware() Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-Content-Type-Options", "nosniff")
			w.Header().Set("X-Frame-Options", "DENY")
			w.Header().Set("X-XSS-Protection", "1; mode=block")
			w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
			next.ServeHTTP(w, r)
		})
	}
}

// gzipResponseWriter gzip响应写入器
type gzipResponseWriter struct {
	http.ResponseWriter
	*gzip.Writer
}

func (w *gzipResponseWriter) Write(b []byte) (int, error) {
	return w.Writer.Write(b)
}

func (w *gzipResponseWriter) Header() http.Header {
	return w.ResponseWriter.Header()
}

func (w *gzipResponseWriter) WriteHeader(statusCode int) {
	w.ResponseWriter.WriteHeader(statusCode)
}

// AddMiddleware 添加中间件
func (h *OptimizedHTTPHandler) AddMiddleware(middleware Middleware) {
	h.middlewareChain.Add(middleware)
}

// Start 启动处理器
func (h *OptimizedHTTPHandler) Start() error {
	if !atomic.CompareAndSwapInt32(&h.running, 0, 1) {
		return fmt.Errorf("handler already running")
	}

	logger.Info("Starting optimized HTTP handler")
	return nil
}

// Stop 停止处理器
func (h *OptimizedHTTPHandler) Stop() error {
	if !atomic.CompareAndSwapInt32(&h.running, 1, 0) {
		return fmt.Errorf("handler not running")
	}

	close(h.shutdown)
	logger.Info("Stopped optimized HTTP handler")
	return nil
}

// Handle 处理HTTP请求
func (h *OptimizedHTTPHandler) Handle(connID string, conn net.Conn, initialData []byte) {
	// 检查连接限制
	if !h.connLimiter.Acquire() {
		logger.Warn("Connection limit exceeded", "connID", connID)
		conn.Close()
		return
	}
	defer h.connLimiter.Release()

	// 记录连接建立
	observability.RecordConnection("http", "established")
	atomic.AddInt64(&h.metrics.ActiveConnections, 1)
	defer atomic.AddInt64(&h.metrics.ActiveConnections, -1)

	// 更新连接协议
	h.Pool.UpdateConnectionProtocol(connID, "http")

	// 记录请求开始时间
	startTime := time.Now()

	// 解析HTTP请求
	reader := strings.NewReader(string(initialData))
	req, err := http.ReadRequest(bufio.NewReader(reader))
	if err != nil {
		h.handleError(connID, conn, err, startTime)
		return
	}

	// 设置请求上下文
	ctx, cancel := context.WithTimeout(context.Background(), h.config.ReadTimeout)
	defer cancel()
	req = req.WithContext(ctx)
	req.RemoteAddr = conn.RemoteAddr().String()

	// 创建优化的响应写入器
	w := h.getResponseWriter(conn)
	defer h.putResponseWriter(w)

	// 检测HTTP版本并处理
	if h.config.EnableHTTP2 && req.ProtoMajor == 2 {
		h.handleHTTP2(w, req)
	} else {
		h.handleHTTP1(w, req)
	}

	// 记录指标
	duration := time.Since(startTime)
	h.recordMetrics(req, w, duration)

	// 检查连接保持
	if !h.shouldKeepAlive(req, w) {
		h.Pool.RemoveConnection(connID)
	}
}

// handleHTTP1 处理HTTP/1.x请求
func (h *OptimizedHTTPHandler) handleHTTP1(w *OptimizedResponseWriter, req *http.Request) {
	// 创建基础处理器
	baseHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h.routeRequest(w, r)
	})

	// 应用中间件链
	handler := h.middlewareChain.Build(baseHandler)
	handler.ServeHTTP(w, req)
}

// handleHTTP2 处理HTTP/2请求
func (h *OptimizedHTTPHandler) handleHTTP2(w *OptimizedResponseWriter, req *http.Request) {
	if h.http2Server != nil {
		// HTTP/2处理逻辑
		h.handleHTTP1(w, req) // 简化处理，实际应用中需要完整的HTTP/2实现
	}
}

// routeRequest 路由请求
func (h *OptimizedHTTPHandler) routeRequest(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	nodeIDs := h.router.Search(path)

	if len(nodeIDs) > 0 {
		// 成功路由
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Handler", "optimized-http")
		w.WriteHeader(http.StatusOK)
		response := fmt.Sprintf(`{"message": "Optimized HTTP Handler", "path": "%s", "method": "%s", "timestamp": "%s"}`,
			path, r.Method, time.Now().Format(time.RFC3339))
		w.Write([]byte(response))
	} else {
		// 未找到路由
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		response := fmt.Sprintf(`{"error": "404 Not Found", "path": "%s", "method": "%s"}`, path, r.Method)
		w.Write([]byte(response))
	}
}

// handleError 处理错误
func (h *OptimizedHTTPHandler) handleError(connID string, conn net.Conn, err error, startTime time.Time) {
	muxErr := errors.ProtocolError(errors.ErrCodeProtocolParseError, "failed to parse HTTP request").WithCause(err).WithContext("connectionID", connID)
	errors.Handle(context.Background(), muxErr)

	// 记录失败请求
	duration := time.Since(startTime).Seconds()
	observability.RecordRequest("http", "", "400", duration)
	atomic.AddInt64(&h.metrics.FailedRequests, 1)

	h.Pool.RemoveConnection(connID)
}

// recordMetrics 记录指标
func (h *OptimizedHTTPHandler) recordMetrics(req *http.Request, w *OptimizedResponseWriter, duration time.Duration) {
	atomic.AddInt64(&h.metrics.TotalRequests, 1)

	statusCode := w.Status()
	if statusCode == 0 {
		statusCode = http.StatusOK
	}

	if statusCode >= 200 && statusCode < 400 {
		atomic.AddInt64(&h.metrics.SuccessfulRequests, 1)
	} else {
		atomic.AddInt64(&h.metrics.FailedRequests, 1)
	}

	// 记录到观测性系统
	observability.RecordRequest("http", req.URL.Path, fmt.Sprintf("%d", statusCode), duration.Seconds())
}

// shouldKeepAlive 判断是否保持连接
func (h *OptimizedHTTPHandler) shouldKeepAlive(req *http.Request, w *OptimizedResponseWriter) bool {
	if req.Header.Get("Connection") == "close" {
		return false
	}
	if req.ProtoMajor == 1 && req.ProtoMinor == 0 {
		return false
	}
	return true
}

// getResponseWriter 获取响应写入器
func (h *OptimizedHTTPHandler) getResponseWriter(conn net.Conn) *OptimizedResponseWriter {
	w := h.responsePool.Get().(*OptimizedResponseWriter)
	w.Reset(conn)
	return w
}

// putResponseWriter 归还响应写入器
func (h *OptimizedHTTPHandler) putResponseWriter(w *OptimizedResponseWriter) {
	h.responsePool.Put(w)
}

// GetMetrics 获取指标
func (h *OptimizedHTTPHandler) GetMetrics() *HTTPMetrics {
	h.metrics.mu.RLock()
	defer h.metrics.mu.RUnlock()

	total := atomic.LoadInt64(&h.metrics.TotalRequests)
	failed := atomic.LoadInt64(&h.metrics.FailedRequests)

	metrics := &HTTPMetrics{
		TotalRequests:      total,
		SuccessfulRequests: atomic.LoadInt64(&h.metrics.SuccessfulRequests),
		FailedRequests:     failed,
		ActiveConnections:  atomic.LoadInt64(&h.metrics.ActiveConnections),
	}

	if total > 0 {
		metrics.ErrorRate = float64(failed) / float64(total) * 100
	}

	return metrics
}
