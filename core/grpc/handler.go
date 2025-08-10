package grpc

import (
	"net"
	"sync"

	common "github.com/cocowh/muxcore/core/common"
	"github.com/cocowh/muxcore/core/pool"
	"github.com/cocowh/muxcore/pkg/logger"
	"google.golang.org/grpc"
)

// GRPCHandler gRPC处理器
type GRPCHandler struct {
	*common.BaseHandler
	grpcServer   *grpc.Server
	interceptors []grpc.UnaryServerInterceptor
	mutex        sync.Mutex
}

// NewGRPCHandler 创建gRPC处理器
func NewGRPCHandler(pool *pool.ConnectionPool) *GRPCHandler {
	handler := &GRPCHandler{
		BaseHandler:  common.NewBaseHandler(pool),
		interceptors: make([]grpc.UnaryServerInterceptor, 0),
	}
	return handler
}

// AddInterceptor 添加拦截器
func (h *GRPCHandler) AddInterceptor(interceptor grpc.UnaryServerInterceptor) {
	h.mutex.Lock()
	defer h.mutex.Unlock()
	h.interceptors = append(h.interceptors, interceptor)
	logger.Info("Added gRPC interceptor")
}

// InitializeServer 初始化gRPC服务器
func (h *GRPCHandler) InitializeServer() {
	// 创建拦截器链
	var opts []grpc.ServerOption
	if len(h.interceptors) > 0 {
		var unaryInterceptors []grpc.UnaryServerInterceptor
		for _, interceptor := range h.interceptors {
			unaryInterceptors = append(unaryInterceptors, interceptor)
		}
		opts = append(opts, grpc.ChainUnaryInterceptor(unaryInterceptors...))
	}

	// 创建gRPC服务器
	h.grpcServer = grpc.NewServer(opts...)
	logger.Info("gRPC server initialized")
}

// Handle 处理gRPC连接
func (h *GRPCHandler) Handle(connID string, conn net.Conn, initialData []byte) {
	// 更新连接协议
	h.Pool.UpdateConnectionProtocol(connID, "grpc")

	// 如果服务器尚未初始化，则初始化
	if h.grpcServer == nil {
		h.InitializeServer()
	}

	// 启动gRPC服务器处理连接
	go func() {
		defer func() {
			h.Pool.RemoveConnection(connID)
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
