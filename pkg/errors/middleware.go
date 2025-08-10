// Copyright (c) 2025 cocowh. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package errors

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"runtime/debug"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// HTTPErrorResponse HTTP错误响应结构
type HTTPErrorResponse struct {
	Error     string            `json:"error"`
	Code      string            `json:"code"`
	Message   string            `json:"message"`
	Details   map[string]interface{} `json:"details,omitempty"`
	Timestamp time.Time         `json:"timestamp"`
	RequestID string            `json:"request_id,omitempty"`
	TraceID   string            `json:"trace_id,omitempty"`
}

// HTTPErrorMiddleware HTTP错误处理中间件
type HTTPErrorMiddleware struct {
	manager     *ErrorManager
	includeStack bool
	debugMode   bool
}

// NewHTTPErrorMiddleware 创建HTTP错误中间件
func NewHTTPErrorMiddleware(manager *ErrorManager, options ...HTTPMiddlewareOption) *HTTPErrorMiddleware {
	m := &HTTPErrorMiddleware{
		manager:     manager,
		includeStack: false,
		debugMode:   false,
	}
	
	for _, opt := range options {
		opt(m)
	}
	
	return m
}

// HTTPMiddlewareOption HTTP中间件选项
type HTTPMiddlewareOption func(*HTTPErrorMiddleware)

// WithStackTrace 包含堆栈跟踪
func WithStackTrace(include bool) HTTPMiddlewareOption {
	return func(m *HTTPErrorMiddleware) {
		m.includeStack = include
	}
}

// WithDebugMode 启用调试模式
func WithDebugMode(debug bool) HTTPMiddlewareOption {
	return func(m *HTTPErrorMiddleware) {
		m.debugMode = debug
	}
}

// Middleware 返回HTTP中间件函数
func (m *HTTPErrorMiddleware) Middleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// 使用defer捕获panic
			defer func() {
				if rec := recover(); rec != nil {
					var err error
					if e, ok := rec.(error); ok {
						err = e
					} else {
						err = fmt.Errorf("panic: %v", rec)
					}
					
					// 创建panic错误
					panicErr := SystemError(ErrCodeSystemPanic, err.Error())
					if m.includeStack {
						panicErr = panicErr.WithContext("stack", string(debug.Stack()))
					}
					
					// 处理panic错误
					m.manager.Handle(r.Context(), panicErr)
					
					// 返回500错误
					m.writeErrorResponse(w, r, panicErr)
				}
			}()
			
			// 创建响应写入器包装器
			wrapper := &responseWriter{
				ResponseWriter: w,
				middleware:     m,
				request:        r,
			}
			
			next.ServeHTTP(wrapper, r)
		})
	}
}

// responseWriter 响应写入器包装器
type responseWriter struct {
	http.ResponseWriter
	middleware *HTTPErrorMiddleware
	request    *http.Request
	written    bool
}

// WriteHeader 写入响应头
func (rw *responseWriter) WriteHeader(statusCode int) {
	if rw.written {
		return
	}
	rw.written = true
	
	// 如果是错误状态码，尝试处理
	if statusCode >= 400 {
		// 创建对应的错误
		var muxErr *MuxError
		switch {
		case statusCode >= 500:
			muxErr = SystemError(ErrCodeSystemInternalError, http.StatusText(statusCode))
		case statusCode == 404:
			muxErr = NotFoundError(rw.request.URL.Path)
		case statusCode == 401:
			muxErr = UnauthorizedError("Authentication required")
		case statusCode == 403:
			muxErr = ForbiddenError("Access denied")
		case statusCode >= 400:
			muxErr = ValidationError(ErrCodeValidationFormat, http.StatusText(statusCode))
		default:
			muxErr = SystemError(ErrCodeSystemUnknown, http.StatusText(statusCode))
		}
		
		// 处理错误
		rw.middleware.manager.Handle(rw.request.Context(), muxErr)
		
		// 写入错误响应
		rw.middleware.writeErrorResponse(rw.ResponseWriter, rw.request, muxErr)
		return
	}
	
	rw.ResponseWriter.WriteHeader(statusCode)
}

// Write 写入响应体
func (rw *responseWriter) Write(data []byte) (int, error) {
	if !rw.written {
		rw.WriteHeader(http.StatusOK)
	}
	return rw.ResponseWriter.Write(data)
}

// writeErrorResponse 写入错误响应
func (m *HTTPErrorMiddleware) writeErrorResponse(w http.ResponseWriter, r *http.Request, muxErr *MuxError) {
	// 设置响应头
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Error-Code", fmt.Sprintf("%d", muxErr.Code))
	w.Header().Set("X-Error-Category", string(muxErr.Category))
	
	// 确定HTTP状态码
	statusCode := m.getHTTPStatusCode(muxErr)
	w.WriteHeader(statusCode)
	
	// 构建错误响应
	errorResp := HTTPErrorResponse{
		Error:     muxErr.Error(),
		Code:      fmt.Sprintf("%d", muxErr.Code),
		Message:   muxErr.Message,
		Timestamp: time.Now(),
	}
	
	// 添加请求ID和追踪ID
	if requestID := r.Header.Get("X-Request-ID"); requestID != "" {
		errorResp.RequestID = requestID
	}
	if traceID := r.Header.Get("X-Trace-ID"); traceID != "" {
		errorResp.TraceID = traceID
	}
	
	// 在调试模式下添加详细信息
	if m.debugMode {
		errorResp.Details = make(map[string]interface{})
		errorResp.Details["level"] = fmt.Sprintf("%d", muxErr.Level)
		errorResp.Details["category"] = string(muxErr.Category)
		
		if muxErr.Context != nil {
			errorResp.Details["context"] = muxErr.Context
		}
		
		if m.includeStack && muxErr.Stack != "" {
			errorResp.Details["stack"] = muxErr.Stack
		}
		
		if muxErr.Cause != nil {
			errorResp.Details["cause"] = muxErr.Cause.Error()
		}
	}
	
	// 编码并写入响应
	if err := json.NewEncoder(w).Encode(errorResp); err != nil {
		// 如果JSON编码失败，写入简单的错误响应
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, `{"error":"Internal server error","code":"SYSTEM_INTERNAL_ERROR"}`)
	}
}

// getHTTPStatusCode 获取HTTP状态码
func (m *HTTPErrorMiddleware) getHTTPStatusCode(muxErr *MuxError) int {
	switch muxErr.Code {
	// 认证相关
	case ErrCodeAuthUnauthorized:
		return http.StatusUnauthorized
	case ErrCodeAuthForbidden:
		return http.StatusForbidden
	case ErrCodeAuthTokenExpired, ErrCodeAuthTokenInvalid:
		return http.StatusUnauthorized
	
	// 验证相关
	case ErrCodeValidationRequired, ErrCodeValidationFormat, ErrCodeValidationRange:
		return http.StatusBadRequest
	
	// 配置相关
	case ErrCodeConfigNotFound:
		return http.StatusNotFound
	case ErrCodeConfigInvalid:
		return http.StatusBadRequest
	
	// 网络相关
	case ErrCodeNetworkTimeout:
		return http.StatusRequestTimeout
	case ErrCodeNetworkRefused, ErrCodeNetworkUnreachable:
		return http.StatusServiceUnavailable
	
	// 系统相关
	case ErrCodeSystemOutOfMemory, ErrCodeSystemResourceLimit:
		return http.StatusInsufficientStorage
	case ErrCodeSystemPanic, ErrCodeSystemInternalError:
		return http.StatusInternalServerError
	
	// 业务相关
	case ErrCodeBusinessLogicError:
		return http.StatusUnprocessableEntity
	case ErrCodeBusinessRuleViolation:
		return http.StatusConflict
	
	default:
		// 根据错误级别确定状态码
		switch muxErr.Level {
		case LevelFatal:
			return http.StatusInternalServerError
		case LevelError:
			return http.StatusInternalServerError
		case LevelWarn:
			return http.StatusBadRequest
		default:
			return http.StatusInternalServerError
		}
	}
}

// GRPCErrorInterceptor gRPC错误拦截器
type GRPCErrorInterceptor struct {
	manager     *ErrorManager
	includeStack bool
	debugMode   bool
}

// NewGRPCErrorInterceptor 创建gRPC错误拦截器
func NewGRPCErrorInterceptor(manager *ErrorManager, options ...GRPCInterceptorOption) *GRPCErrorInterceptor {
	i := &GRPCErrorInterceptor{
		manager:     manager,
		includeStack: false,
		debugMode:   false,
	}
	
	for _, opt := range options {
		opt(i)
	}
	
	return i
}

// GRPCInterceptorOption gRPC拦截器选项
type GRPCInterceptorOption func(*GRPCErrorInterceptor)

// WithGRPCStackTrace 包含堆栈跟踪
func WithGRPCStackTrace(include bool) GRPCInterceptorOption {
	return func(i *GRPCErrorInterceptor) {
		i.includeStack = include
	}
}

// WithGRPCDebugMode 启用调试模式
func WithGRPCDebugMode(debug bool) GRPCInterceptorOption {
	return func(i *GRPCErrorInterceptor) {
		i.debugMode = debug
	}
}

// UnaryServerInterceptor 一元服务器拦截器
func (i *GRPCErrorInterceptor) UnaryServerInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		// 使用defer捕获panic
		defer func() {
			if rec := recover(); rec != nil {
				var err error
				if e, ok := rec.(error); ok {
					err = e
				} else {
					err = fmt.Errorf("panic: %v", rec)
				}
				
				// 创建panic错误
				panicErr := SystemError(ErrCodeSystemPanic, err.Error())
				if i.includeStack {
					panicErr = panicErr.WithContext("stack", string(debug.Stack()))
				}
				
				// 处理panic错误
				i.manager.Handle(ctx, panicErr)
			}
		}()
		
		// 调用处理器
		resp, err := handler(ctx, req)
		if err != nil {
			// 转换错误
			muxErr := Convert(err)
			
			// 处理错误
			i.manager.Handle(ctx, muxErr)
			
			// 转换为gRPC错误
			return resp, i.convertToGRPCError(muxErr)
		}
		
		return resp, nil
	}
}

// StreamServerInterceptor 流服务器拦截器
func (i *GRPCErrorInterceptor) StreamServerInterceptor() grpc.StreamServerInterceptor {
	return func(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		// 使用defer捕获panic
		defer func() {
			if rec := recover(); rec != nil {
				var err error
				if e, ok := rec.(error); ok {
					err = e
				} else {
					err = fmt.Errorf("panic: %v", rec)
				}
				
				// 创建panic错误
				panicErr := SystemError(ErrCodeSystemPanic, err.Error())
				if i.includeStack {
					panicErr = panicErr.WithContext("stack", string(debug.Stack()))
				}
				
				// 处理panic错误
				i.manager.Handle(ss.Context(), panicErr)
			}
		}()
		
		// 调用处理器
		err := handler(srv, ss)
		if err != nil {
			// 转换错误
			muxErr := Convert(err)
			
			// 处理错误
			i.manager.Handle(ss.Context(), muxErr)
			
			// 转换为gRPC错误
			return i.convertToGRPCError(muxErr)
		}
		
		return nil
	}
}

// convertToGRPCError 转换为gRPC错误
func (i *GRPCErrorInterceptor) convertToGRPCError(muxErr *MuxError) error {
	// 确定gRPC状态码
	code := i.getGRPCCode(muxErr)
	
	// 创建状态
	st := status.New(code, muxErr.Message)
	
	// 在调试模式下添加详细信息
	if i.debugMode {
		// 可以添加详细信息到status.Details
		// 这里简化处理，只返回基本错误
	}
	
	return st.Err()
}

// getGRPCCode 获取gRPC状态码
func (i *GRPCErrorInterceptor) getGRPCCode(muxErr *MuxError) codes.Code {
	switch muxErr.Code {
	// 认证相关
	case ErrCodeAuthUnauthorized:
		return codes.Unauthenticated
	case ErrCodeAuthForbidden:
		return codes.PermissionDenied
	case ErrCodeAuthTokenExpired, ErrCodeAuthTokenInvalid:
		return codes.Unauthenticated
	
	// 验证相关
	case ErrCodeValidationRequired, ErrCodeValidationFormat, ErrCodeValidationRange:
		return codes.InvalidArgument
	
	// 配置相关
	case ErrCodeConfigNotFound:
		return codes.NotFound
	case ErrCodeConfigInvalid:
		return codes.InvalidArgument
	
	// 网络相关
	case ErrCodeNetworkTimeout:
		return codes.DeadlineExceeded
	case ErrCodeNetworkRefused, ErrCodeNetworkUnreachable:
		return codes.Unavailable
	
	// 系统相关
	case ErrCodeSystemOutOfMemory, ErrCodeSystemResourceLimit:
		return codes.ResourceExhausted
	case ErrCodeSystemPanic, ErrCodeSystemInternalError:
		return codes.Internal
	
	// 业务相关
	case ErrCodeBusinessLogicError:
		return codes.FailedPrecondition
	case ErrCodeBusinessRuleViolation:
		return codes.AlreadyExists
	
	default:
		// 根据错误级别确定状态码
		switch muxErr.Level {
		case LevelFatal:
			return codes.Internal
		case LevelError:
			return codes.Internal
		case LevelWarn:
			return codes.InvalidArgument
		default:
			return codes.Unknown
		}
	}
}

// ErrorHandlerFunc 错误处理函数类型
type ErrorHandlerFunc func(error) error

// HandleError 处理错误的便捷函数
func HandleError(err error) error {
	if err == nil {
		return nil
	}
	
	// 转换错误
	muxErr := Convert(err)
	
	// 使用全局错误管理器处理
	Handle(context.Background(), muxErr)
	
	return muxErr
}

// MustHandleError 必须处理错误，如果是致命错误则panic
func MustHandleError(err error) {
	if err == nil {
		return
	}
	
	// 转换错误
	muxErr := Convert(err)
	
	// 使用全局错误管理器处理
	Handle(context.Background(), muxErr)
	
	// 如果是致命错误，panic
	if muxErr.Level == LevelFatal {
		panic(muxErr)
	}
}

// RecoverAndHandle 恢复panic并处理错误
func RecoverAndHandle() error {
	if rec := recover(); rec != nil {
		var err error
		if e, ok := rec.(error); ok {
			err = e
		} else {
			err = fmt.Errorf("panic: %v", rec)
		}
		
		// 创建panic错误
		panicErr := SystemError(ErrCodeSystemPanic, err.Error())
		panicErr = panicErr.WithContext("stack", string(debug.Stack()))
		
		// 处理错误
		Handle(context.Background(), panicErr)
		
		return panicErr
	}
	return nil
}