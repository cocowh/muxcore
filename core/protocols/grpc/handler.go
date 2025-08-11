package grpc

import (
	"context"
	"net"
	"sync"

	"github.com/cocowh/muxcore/core/handler"
	"github.com/cocowh/muxcore/core/pool"
	"github.com/cocowh/muxcore/pkg/logger"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health/grpc_health_v1"
)

// GRPCConnectionManager 连接管理器
type GRPCConnectionManager struct {
	connections map[string]*grpc.ClientConn
	mu          sync.RWMutex
	pool        *pool.ConnectionPool
}

// NewGRPCConnectionManager 创建连接管理器
func NewGRPCConnectionManager(pool *pool.ConnectionPool) *GRPCConnectionManager {
	return &GRPCConnectionManager{
		connections: make(map[string]*grpc.ClientConn),
		pool:        pool,
	}
}

// GetConnection 获取或创建连接
func (cm *GRPCConnectionManager) GetConnection(target string) (*grpc.ClientConn, error) {
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
	newConn, err := grpc.Dial(target, grpc.WithInsecure())
	if err != nil {
		return nil, err
	}

	cm.connections[target] = newConn
	return newConn, nil
}

// CloseConnection 关闭连接
func (cm *GRPCConnectionManager) CloseConnection(target string) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	if conn, exists := cm.connections[target]; exists {
		delete(cm.connections, target)
		return conn.Close()
	}
	return nil
}

// CloseAll 关闭所有连接
func (cm *GRPCConnectionManager) CloseAll() {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	for target, conn := range cm.connections {
		conn.Close()
		delete(cm.connections, target)
	}
}

// HealthChecker 健康检查器
type HealthChecker struct {
	services map[string]grpc_health_v1.HealthServer
	mu       sync.RWMutex
}

// NewHealthChecker 创建健康检查器
func NewHealthChecker() *HealthChecker {
	return &HealthChecker{
		services: make(map[string]grpc_health_v1.HealthServer),
	}
}

// RegisterService 注册服务健康检查
func (hc *HealthChecker) RegisterService(serviceName string, server grpc_health_v1.HealthServer) {
	hc.mu.Lock()
	defer hc.mu.Unlock()
	hc.services[serviceName] = server
}

// Check 检查服务健康状态
func (hc *HealthChecker) Check(ctx context.Context, req *grpc_health_v1.HealthCheckRequest) (*grpc_health_v1.HealthCheckResponse, error) {
	hc.mu.RLock()
	defer hc.mu.RUnlock()

	if server, exists := hc.services[req.Service]; exists {
		return server.Check(ctx, req)
	}

	return &grpc_health_v1.HealthCheckResponse{
		Status: grpc_health_v1.HealthCheckResponse_SERVING,
	}, nil
}

// Watch 监控服务健康状态
func (hc *HealthChecker) Watch(req *grpc_health_v1.HealthCheckRequest, stream grpc_health_v1.Health_WatchServer) error {
	hc.mu.RLock()
	defer hc.mu.RUnlock()

	if server, exists := hc.services[req.Service]; exists {
		return server.Watch(req, stream)
	}

	// 默认返回健康状态
	return stream.Send(&grpc_health_v1.HealthCheckResponse{
		Status: grpc_health_v1.HealthCheckResponse_SERVING,
	})
}

// GRPCHandler gRPC处理器
type GRPCHandler struct {
	*handler.BaseHandler
	grpcServer         *grpc.Server
	unaryInterceptors  []grpc.UnaryServerInterceptor
	streamInterceptors []grpc.StreamServerInterceptor
	config             *GRPCConfig
	connManager        *GRPCConnectionManager
	healthChecker      *HealthChecker
	mutex              sync.Mutex
}

// NewGRPCHandler 创建gRPC处理器
func NewGRPCHandler(pool *pool.ConnectionPool) *GRPCHandler {
	return NewGRPCHandlerWithConfig(pool, DefaultGRPCConfig())
}

// NewGRPCHandlerWithConfig 使用配置创建gRPC处理器
func NewGRPCHandlerWithConfig(pool *pool.ConnectionPool, config *GRPCConfig) *GRPCHandler {
	handler := &GRPCHandler{
		BaseHandler:        handler.NewBaseHandler(),
		unaryInterceptors:  make([]grpc.UnaryServerInterceptor, 0),
		streamInterceptors: make([]grpc.StreamServerInterceptor, 0),
		config:             config,
		connManager:        NewGRPCConnectionManager(pool),
		healthChecker:      NewHealthChecker(),
	}
	return handler
}

// AddUnaryInterceptor 添加一元拦截器
func (h *GRPCHandler) AddUnaryInterceptor(interceptor grpc.UnaryServerInterceptor) {
	h.mutex.Lock()
	defer h.mutex.Unlock()
	h.unaryInterceptors = append(h.unaryInterceptors, interceptor)
	logger.Info("Added gRPC unary interceptor")
}

// AddStreamInterceptor 添加流拦截器
func (h *GRPCHandler) AddStreamInterceptor(interceptor grpc.StreamServerInterceptor) {
	h.mutex.Lock()
	defer h.mutex.Unlock()
	h.streamInterceptors = append(h.streamInterceptors, interceptor)
	logger.Info("Added gRPC stream interceptor")
}

// InitializeServer 初始化gRPC服务器
func (h *GRPCHandler) InitializeServer() {
	// 创建服务器选项
	var opts []grpc.ServerOption

	// 添加一元拦截器
	if len(h.unaryInterceptors) > 0 {
		opts = append(opts, grpc.ChainUnaryInterceptor(h.unaryInterceptors...))
	}

	// 添加流拦截器
	if len(h.streamInterceptors) > 0 {
		opts = append(opts, grpc.ChainStreamInterceptor(h.streamInterceptors...))
	}

	// 添加配置选项
	opts = append(opts,
		grpc.MaxRecvMsgSize(h.config.MaxRecvMsgSize),
		grpc.MaxSendMsgSize(h.config.MaxSendMsgSize),
		grpc.MaxConcurrentStreams(h.config.MaxConcurrentStreams),
	)

	// 创建gRPC服务器
	h.grpcServer = grpc.NewServer(opts...)

	// 注册健康检查服务
	if h.config.EnableHealthCheck {
		grpc_health_v1.RegisterHealthServer(h.grpcServer, h.healthChecker)
		logger.Info("gRPC health check service registered")
	}

	logger.Info("gRPC server initialized")
}

// Handle 处理gRPC连接
func (h *GRPCHandler) Handle(connID string, conn net.Conn, initialData []byte) {
	// 通过连接管理器处理连接协议更新
	// h.connManager 可以处理连接相关操作

	// 如果服务器尚未初始化，则初始化
	if h.grpcServer == nil {
		h.InitializeServer()
	}

	// 启动gRPC服务器处理连接
	go func() {
		defer func() {
			// 通过连接管理器清理连接
			conn.Close()
		}()

		logger.Debug("Starting to handle gRPC connection: ", connID)
		// 创建一个临时的监听器来处理单个连接
		listener := &singleListener{conn: conn}
		h.grpcServer.Serve(listener)
	}()
}

// singleListener 是一个只处理单个连接的监听器
type singleListener struct {
	conn net.Conn
}

// Accept 实现net.Listener接口
func (l *singleListener) Accept() (net.Conn, error) {
	conn := l.conn
	l.conn = nil
	return conn, nil
}

// Close 实现net.Listener接口
func (l *singleListener) Close() error {
	if l.conn != nil {
		return l.conn.Close()
	}
	return nil
}

// Addr 实现net.Listener接口
func (l *singleListener) Addr() net.Addr {
	return l.conn.LocalAddr()
}

// RegisterService 注册gRPC服务
func (h *GRPCHandler) RegisterService(sd *grpc.ServiceDesc, ss interface{}) {
	if h.grpcServer == nil {
		h.InitializeServer()
	}
	h.grpcServer.RegisterService(sd, ss)
	logger.Info("Registered gRPC service: ", sd.ServiceName)
}

// GracefulStop 优雅停止gRPC服务器
func (h *GRPCHandler) GracefulStop() {
	if h.grpcServer != nil {
		h.grpcServer.GracefulStop()
	}
}
