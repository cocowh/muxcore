package interceptor

import (
	"context"

	"github.com/cocowh/muxcore/pkg/logger"
	"google.golang.org/grpc"
)

// Interceptor gRPC拦截器接口
type Interceptor interface {
	// UnaryInterceptor 返回一元拦截器
	UnaryInterceptor() grpc.UnaryServerInterceptor
}

// LoggingInterceptor 日志拦截器
type LoggingInterceptor struct {
}

// NewLoggingInterceptor 创建日志拦截器
func NewLoggingInterceptor() *LoggingInterceptor {
	return &LoggingInterceptor{}
}

// UnaryInterceptor 实现一元拦截器
func (i *LoggingInterceptor) UnaryInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		logger.Info("gRPC request: ", info.FullMethod)

		// 调用处理程序
		resp, err := handler(ctx, req)

		if err != nil {
			logger.Error("gRPC error: ", err)
		} else {
			logger.Info("gRPC response: ", info.FullMethod)
		}

		return resp, err
	}
}

// AuthInterceptor 身份验证拦截器
type AuthInterceptor struct {
}

// NewAuthInterceptor 创建身份验证拦截器
func NewAuthInterceptor() *AuthInterceptor {
	return &AuthInterceptor{}
}

// UnaryInterceptor 实现一元拦截器
func (i *AuthInterceptor) UnaryInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		// 这里实现身份验证逻辑
		logger.Info("Authenticating gRPC request: ", info.FullMethod)

		// 调用处理程序
		resp, err := handler(ctx, req)

		return resp, err
	}
}