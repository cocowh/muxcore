package observability

import (
	"context"
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cocowh/muxcore/core/handler"
	"github.com/cocowh/muxcore/core/pool"
	"github.com/cocowh/muxcore/pkg/logger"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/status"
)

// OptimizedGRPCConfig 优化的gRPC配置
type OptimizedGRPCConfig struct {
	MaxRecvMsgSize        int           `yaml:"max_recv_msg_size"`
	MaxSendMsgSize        int           `yaml:"max_send_msg_size"`
	MaxConcurrentStreams  uint32        `yaml:"max_concurrent_streams"`
	ConnectionTimeout     time.Duration `yaml:"connection_timeout"`
	KeepAliveTime         time.Duration `yaml:"keep_alive_time"`
	KeepAliveTimeout      time.Duration `yaml:"keep_alive_timeout"`
	PermitWithoutStream   bool          `yaml:"permit_without_stream"`
	MaxConnectionIdle     time.Duration `yaml:"max_connection_idle"`
	MaxConnectionAge      time.Duration `yaml:"max_connection_age"`
	MaxConnectionAgeGrace time.Duration `yaml:"max_connection_age_grace"`
	EnableReflection      bool          `yaml:"enable_reflection"`
	EnableHealthCheck     bool          `yaml:"enable_health_check"`
	MaxRetryAttempts      int           `yaml:"max_retry_attempts"`
	RetryBackoff          time.Duration `yaml:"retry_backoff"`
}

// DefaultOptimizedGRPCConfig 默认优化gRPC配置
func DefaultOptimizedGRPCConfig() *OptimizedGRPCConfig {
	return &OptimizedGRPCConfig{
		MaxRecvMsgSize:        4 << 20, // 4MB
		MaxSendMsgSize:        4 << 20, // 4MB
		MaxConcurrentStreams:  1000,
		ConnectionTimeout:     30 * time.Second,
		KeepAliveTime:         30 * time.Second,
		KeepAliveTimeout:      5 * time.Second,
		PermitWithoutStream:   true,
		MaxConnectionIdle:     15 * time.Minute,
		MaxConnectionAge:      30 * time.Minute,
		MaxConnectionAgeGrace: 5 * time.Second,
		EnableReflection:      true,
		EnableHealthCheck:     true,
		MaxRetryAttempts:      3,
		RetryBackoff:          time.Second,
	}
}

// GRPCMetrics gRPC指标
type GRPCMetrics struct {
	TotalRequests      int64
	SuccessfulRequests int64
	FailedRequests     int64
	ActiveConnections  int64
	ActiveStreams      int64
	AverageLatency     float64
	Throughput         float64
	ErrorRate          float64
	ConnectionPoolSize int64
	mu                 sync.RWMutex
}

// OptimizedGRPCConnectionManager 优化的gRPC连接管理器
type OptimizedGRPCConnectionManager struct {
	connections  map[string]*grpc.ClientConn
	healthChecks map[string]*OptimizedHealthChecker
	mu           sync.RWMutex
	pool         *pool.ConnectionPool
	config       *OptimizedGRPCConfig
	metrics      *GRPCMetrics
}

// NewOptimizedGRPCConnectionManager 创建优化的gRPC连接管理器
func NewOptimizedGRPCConnectionManager(pool *pool.ConnectionPool, config *OptimizedGRPCConfig) *OptimizedGRPCConnectionManager {
	return &OptimizedGRPCConnectionManager{
		connections:  make(map[string]*grpc.ClientConn),
		healthChecks: make(map[string]*OptimizedHealthChecker),
		pool:         pool,
		config:       config,
		metrics:      &GRPCMetrics{},
	}
}

// GetConnection 获取或创建连接
func (cm *OptimizedGRPCConnectionManager) GetConnection(target string) (*grpc.ClientConn, error) {
	cm.mu.RLock()
	conn, exists := cm.connections[target]
	cm.mu.RUnlock()

	if exists && conn.GetState().String() != "SHUTDOWN" {
		return conn, nil
	}

	cm.mu.Lock()
	defer cm.mu.Unlock()

	// 双重检查
	if conn, exists := cm.connections[target]; exists && conn.GetState().String() != "SHUTDOWN" {
		return conn, nil
	}

	// 创建新连接
	ctx, cancel := context.WithTimeout(context.Background(), cm.config.ConnectionTimeout)
	defer cancel()

	opts := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultCallOptions(
			grpc.MaxCallRecvMsgSize(cm.config.MaxRecvMsgSize),
			grpc.MaxCallSendMsgSize(cm.config.MaxSendMsgSize),
		),
		grpc.WithKeepaliveParams(keepalive.ClientParameters{
			Time:                cm.config.KeepAliveTime,
			Timeout:             cm.config.KeepAliveTimeout,
			PermitWithoutStream: cm.config.PermitWithoutStream,
		}),
	}

	conn, err := grpc.DialContext(ctx, target, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to %s: %w", target, err)
	}

	cm.connections[target] = conn
	atomic.AddInt64(&cm.metrics.ConnectionPoolSize, 1)

	// 启动健康检查
	if cm.config.EnableHealthCheck {
		healthChecker := NewOptimizedHealthChecker(conn, target, cm.config)
		cm.healthChecks[target] = healthChecker
		go healthChecker.Start()
	}

	return conn, nil
}

// CloseConnection 关闭连接
func (cm *OptimizedGRPCConnectionManager) CloseConnection(target string) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	if conn, exists := cm.connections[target]; exists {
		err := conn.Close()
		delete(cm.connections, target)
		atomic.AddInt64(&cm.metrics.ConnectionPoolSize, -1)

		// 停止健康检查
		if healthChecker, exists := cm.healthChecks[target]; exists {
			healthChecker.Stop()
			delete(cm.healthChecks, target)
		}

		return err
	}
	return nil
}

// CloseAll 关闭所有连接
func (cm *OptimizedGRPCConnectionManager) CloseAll() error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	var lastErr error
	for target, conn := range cm.connections {
		if err := conn.Close(); err != nil {
			lastErr = err
		}

		// 停止健康检查
		if healthChecker, exists := cm.healthChecks[target]; exists {
			healthChecker.Stop()
		}
	}

	cm.connections = make(map[string]*grpc.ClientConn)
	cm.healthChecks = make(map[string]*OptimizedHealthChecker)
	atomic.StoreInt64(&cm.metrics.ConnectionPoolSize, 0)

	return lastErr
}

// OptimizedHealthChecker 优化的健康检查器
type OptimizedHealthChecker struct {
	conn     *grpc.ClientConn
	target   string
	config   *OptimizedGRPCConfig
	client   grpc_health_v1.HealthClient
	stopChan chan struct{}
	running  int32
}

// NewOptimizedHealthChecker 创建优化的健康检查器
func NewOptimizedHealthChecker(conn *grpc.ClientConn, target string, config *OptimizedGRPCConfig) *OptimizedHealthChecker {
	return &OptimizedHealthChecker{
		conn:     conn,
		target:   target,
		config:   config,
		client:   grpc_health_v1.NewHealthClient(conn),
		stopChan: make(chan struct{}),
	}
}

// Start 启动健康检查
func (hc *OptimizedHealthChecker) Start() {
	if !atomic.CompareAndSwapInt32(&hc.running, 0, 1) {
		return
	}

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			hc.checkHealth()
		case <-hc.stopChan:
			return
		}
	}
}

// Stop 停止健康检查
func (hc *OptimizedHealthChecker) Stop() {
	if atomic.CompareAndSwapInt32(&hc.running, 1, 0) {
		close(hc.stopChan)
	}
}

// checkHealth 执行健康检查
func (hc *OptimizedHealthChecker) checkHealth() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := hc.client.Check(ctx, &grpc_health_v1.HealthCheckRequest{
		Service: "",
	})

	if err != nil {
		logger.Warn("Health check failed", "target", hc.target, "error", err)
		return
	}

	if resp.Status != grpc_health_v1.HealthCheckResponse_SERVING {
		logger.Warn("Service not healthy", "target", hc.target, "status", resp.Status)
	}
}

// OptimizedGRPCHandler 优化的gRPC处理器
type OptimizedGRPCHandler struct {
	*handler.BaseHandler
	server             *grpc.Server
	connManager        *OptimizedGRPCConnectionManager
	config             *OptimizedGRPCConfig
	metrics            *GRPCMetrics
	healthServer       *health.Server
	interceptors       []grpc.UnaryServerInterceptor
	streamInterceptors []grpc.StreamServerInterceptor
	mu                 sync.RWMutex
	shutdown           chan struct{}
	running            int32
}

// NewOptimizedGRPCHandler 创建优化的gRPC处理器
func NewOptimizedGRPCHandler(pool *pool.ConnectionPool, config *OptimizedGRPCConfig) *OptimizedGRPCHandler {
	if config == nil {
		config = DefaultOptimizedGRPCConfig()
	}

	h := &OptimizedGRPCHandler{
		BaseHandler: handler.NewBaseHandler(),
		connManager: NewOptimizedGRPCConnectionManager(pool, config),
		config:      config,
		metrics:     &GRPCMetrics{},
		shutdown:    make(chan struct{}),
	}

	// 配置服务器选项
	serverOpts := []grpc.ServerOption{
		grpc.MaxRecvMsgSize(config.MaxRecvMsgSize),
		grpc.MaxSendMsgSize(config.MaxSendMsgSize),
		grpc.MaxConcurrentStreams(config.MaxConcurrentStreams),
		grpc.KeepaliveParams(keepalive.ServerParameters{
			MaxConnectionIdle:     config.MaxConnectionIdle,
			MaxConnectionAge:      config.MaxConnectionAge,
			MaxConnectionAgeGrace: config.MaxConnectionAgeGrace,
			Time:                  config.KeepAliveTime,
			Timeout:               config.KeepAliveTimeout,
		}),
		grpc.KeepaliveEnforcementPolicy(keepalive.EnforcementPolicy{
			MinTime:             5 * time.Second,
			PermitWithoutStream: config.PermitWithoutStream,
		}),
	}

	// 添加默认拦截器
	h.addDefaultInterceptors()
	serverOpts = append(serverOpts, grpc.ChainUnaryInterceptor(h.interceptors...))
	serverOpts = append(serverOpts, grpc.ChainStreamInterceptor(h.streamInterceptors...))

	h.server = grpc.NewServer(serverOpts...)

	// 配置健康检查
	if config.EnableHealthCheck {
		h.healthServer = health.NewServer()
		grpc_health_v1.RegisterHealthServer(h.server, h.healthServer)
	}

	return h
}

// addDefaultInterceptors 添加默认拦截器
func (h *OptimizedGRPCHandler) addDefaultInterceptors() {
	h.interceptors = append(h.interceptors, h.metricsInterceptor())
	h.interceptors = append(h.interceptors, h.recoveryInterceptor())
	h.interceptors = append(h.interceptors, h.loggingInterceptor())

	h.streamInterceptors = append(h.streamInterceptors, h.streamMetricsInterceptor())
	h.streamInterceptors = append(h.streamInterceptors, h.streamRecoveryInterceptor())
}

// metricsInterceptor 指标拦截器
func (h *OptimizedGRPCHandler) metricsInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		start := time.Now()
		atomic.AddInt64(&h.metrics.TotalRequests, 1)

		resp, err := handler(ctx, req)

		duration := time.Since(start)
		if err != nil {
			atomic.AddInt64(&h.metrics.FailedRequests, 1)
		} else {
			atomic.AddInt64(&h.metrics.SuccessfulRequests, 1)
		}

		// 更新平均延迟
		h.metrics.mu.Lock()
		if h.metrics.AverageLatency == 0 {
			h.metrics.AverageLatency = duration.Seconds()
		} else {
			h.metrics.AverageLatency = (h.metrics.AverageLatency + duration.Seconds()) / 2
		}
		h.metrics.mu.Unlock()

		// 记录到观测性系统
		statusCode := "OK"
		if err != nil {
			if s, ok := status.FromError(err); ok {
				statusCode = s.Code().String()
			} else {
				statusCode = "UNKNOWN"
			}
		}
		// TODO: Record request metrics
		_ = statusCode
		_ = duration

		return resp, err
	}
}

// recoveryInterceptor 恢复拦截器
func (h *OptimizedGRPCHandler) recoveryInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp interface{}, err error) {
		defer func() {
			if r := recover(); r != nil {
				logger.Error("gRPC panic recovered", "method", info.FullMethod, "panic", r)
				err = status.Errorf(codes.Internal, "internal server error")
				atomic.AddInt64(&h.metrics.FailedRequests, 1)
			}
		}()
		return handler(ctx, req)
	}
}

// loggingInterceptor 日志拦截器
func (h *OptimizedGRPCHandler) loggingInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		start := time.Now()
		resp, err := handler(ctx, req)
		duration := time.Since(start)

		if err != nil {
			logger.Error("gRPC request failed", "method", info.FullMethod, "duration", duration, "error", err)
		} else {
			logger.Info("gRPC request", "method", info.FullMethod, "duration", duration)
		}

		return resp, err
	}
}

// streamMetricsInterceptor 流指标拦截器
func (h *OptimizedGRPCHandler) streamMetricsInterceptor() grpc.StreamServerInterceptor {
	return func(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		atomic.AddInt64(&h.metrics.ActiveStreams, 1)
		defer atomic.AddInt64(&h.metrics.ActiveStreams, -1)

		err := handler(srv, ss)
		// TODO: Record stream metrics
		return err
	}
}

// streamRecoveryInterceptor 流恢复拦截器
func (h *OptimizedGRPCHandler) streamRecoveryInterceptor() grpc.StreamServerInterceptor {
	return func(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) (err error) {
		defer func() {
			if r := recover(); r != nil {
				logger.Error("gRPC stream panic recovered", "method", info.FullMethod, "panic", r)
				err = status.Errorf(codes.Internal, "internal server error")
			}
		}()
		return handler(srv, ss)
	}
}

// Handle 处理gRPC连接
func (h *OptimizedGRPCHandler) Handle(connID string, conn net.Conn, initialData []byte) {
	// 记录连接建立
	// observability.RecordConnection("grpc", "established")
	atomic.AddInt64(&h.metrics.ActiveConnections, 1)
	defer atomic.AddInt64(&h.metrics.ActiveConnections, -1)

	// 连接协议已通过协议检测确定为grpc

	// gRPC连接处理（简化实现）
	// 实际应用中需要实现完整的gRPC协议处理
	logger.Info("Handling gRPC connection", "connID", connID)

	// 连接清理由连接管理器处理
}

// Start 启动处理器
func (h *OptimizedGRPCHandler) Start() error {
	if !atomic.CompareAndSwapInt32(&h.running, 0, 1) {
		return fmt.Errorf("handler already running")
	}

	logger.Info("Starting optimized gRPC handler")
	return nil
}

// Stop 停止处理器
func (h *OptimizedGRPCHandler) Stop() error {
	if !atomic.CompareAndSwapInt32(&h.running, 1, 0) {
		return fmt.Errorf("handler not running")
	}

	// 优雅关闭服务器
	h.server.GracefulStop()

	// 关闭所有连接
	h.connManager.CloseAll()

	close(h.shutdown)
	logger.Info("Stopped optimized gRPC handler")
	return nil
}

// GetServer 获取gRPC服务器
func (h *OptimizedGRPCHandler) GetServer() *grpc.Server {
	return h.server
}

// GetConnectionManager 获取连接管理器
func (h *OptimizedGRPCHandler) GetConnectionManager() *OptimizedGRPCConnectionManager {
	return h.connManager
}

// GetMetrics 获取指标
func (h *OptimizedGRPCHandler) GetMetrics() *GRPCMetrics {
	h.metrics.mu.RLock()
	defer h.metrics.mu.RUnlock()

	total := atomic.LoadInt64(&h.metrics.TotalRequests)
	failed := atomic.LoadInt64(&h.metrics.FailedRequests)

	metrics := &GRPCMetrics{
		TotalRequests:      total,
		SuccessfulRequests: atomic.LoadInt64(&h.metrics.SuccessfulRequests),
		FailedRequests:     failed,
		ActiveConnections:  atomic.LoadInt64(&h.metrics.ActiveConnections),
		ActiveStreams:      atomic.LoadInt64(&h.metrics.ActiveStreams),
		ConnectionPoolSize: atomic.LoadInt64(&h.connManager.metrics.ConnectionPoolSize),
		AverageLatency:     h.metrics.AverageLatency,
	}

	if total > 0 {
		metrics.ErrorRate = float64(failed) / float64(total) * 100
	}

	return metrics
}

// SetHealthStatus 设置健康状态
func (h *OptimizedGRPCHandler) SetHealthStatus(service string, status grpc_health_v1.HealthCheckResponse_ServingStatus) {
	if h.healthServer != nil {
		h.healthServer.SetServingStatus(service, status)
	}
}

// AddUnaryInterceptor 添加一元拦截器
func (h *OptimizedGRPCHandler) AddUnaryInterceptor(interceptor grpc.UnaryServerInterceptor) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.interceptors = append(h.interceptors, interceptor)
}

// AddStreamInterceptor 添加流拦截器
func (h *OptimizedGRPCHandler) AddStreamInterceptor(interceptor grpc.StreamServerInterceptor) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.streamInterceptors = append(h.streamInterceptors, interceptor)
}
