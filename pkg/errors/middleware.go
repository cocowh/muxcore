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

// HTTPErrorResponse http error response
type HTTPErrorResponse struct {
	Error     string                 `json:"error"`
	Code      string                 `json:"code"`
	Message   string                 `json:"message"`
	Details   map[string]interface{} `json:"details,omitempty"`
	Timestamp time.Time              `json:"timestamp"`
	RequestID string                 `json:"request_id,omitempty"`
	TraceID   string                 `json:"trace_id,omitempty"`
}

// HTTPErrorMiddleware HTTP error middleware
type HTTPErrorMiddleware struct {
	manager      *ErrorManager
	includeStack bool
	debugMode    bool
}

// NewHTTPErrorMiddleware create a new HTTP error middleware
func NewHTTPErrorMiddleware(manager *ErrorManager, options ...HTTPMiddlewareOption) *HTTPErrorMiddleware {
	m := &HTTPErrorMiddleware{
		manager:      manager,
		includeStack: false,
		debugMode:    false,
	}

	for _, opt := range options {
		opt(m)
	}

	return m
}

// HTTPMiddlewareOption HTTP middleware option
type HTTPMiddlewareOption func(*HTTPErrorMiddleware)

// WithStackTrace with stack trace
func WithStackTrace(include bool) HTTPMiddlewareOption {
	return func(m *HTTPErrorMiddleware) {
		m.includeStack = include
	}
}

// WithDebugMode with debug mode
func WithDebugMode(debug bool) HTTPMiddlewareOption {
	return func(m *HTTPErrorMiddleware) {
		m.debugMode = debug
	}
}

// Middleware returns a HTTP middleware
func (m *HTTPErrorMiddleware) Middleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if rec := recover(); rec != nil {
					var err error
					if e, ok := rec.(error); ok {
						err = e
					} else {
						err = fmt.Errorf("panic: %v", rec)
					}

					// create a new error and set the stack trace
					panicErr := SystemError(ErrCodeSystemPanic, err.Error())
					if m.includeStack {
						panicErr = panicErr.WithContext("stack", string(debug.Stack()))
					}

					// handle the error
					m.manager.Handle(r.Context(), panicErr)

					// return the error response
					m.writeErrorResponse(w, r, panicErr)
				}
			}()

			// create a new response writer wrapper
			wrapper := &responseWriter{
				ResponseWriter: w,
				middleware:     m,
				request:        r,
			}

			next.ServeHTTP(wrapper, r)
		})
	}
}

// responseWriter response writer wrapper
type responseWriter struct {
	http.ResponseWriter
	middleware *HTTPErrorMiddleware
	request    *http.Request
	written    bool
}

// WriteHeader write header
func (rw *responseWriter) WriteHeader(statusCode int) {
	if rw.written {
		return
	}
	rw.written = true

	if statusCode >= 400 {
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
		default:
			muxErr = ValidationError(ErrCodeValidationFormat, http.StatusText(statusCode))
		}

		// handle error
		rw.middleware.manager.Handle(rw.request.Context(), muxErr)

		// write error response
		rw.middleware.writeErrorResponse(rw.ResponseWriter, rw.request, muxErr)
		return
	}

	rw.ResponseWriter.WriteHeader(statusCode)
}

// Write writes the data to the connection as part of an HTTP reply.
func (rw *responseWriter) Write(data []byte) (int, error) {
	if !rw.written {
		rw.WriteHeader(http.StatusOK)
	}
	return rw.ResponseWriter.Write(data)
}

// writeErrorResponse writes the error response to the response writer.
func (m *HTTPErrorMiddleware) writeErrorResponse(w http.ResponseWriter, r *http.Request, muxErr *MuxError) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Error-Code", fmt.Sprintf("%d", muxErr.Code))
	w.Header().Set("X-Error-Category", string(muxErr.Category))

	statusCode := m.getHTTPStatusCode(muxErr)
	w.WriteHeader(statusCode)

	errorResp := HTTPErrorResponse{
		Error:     muxErr.Error(),
		Code:      fmt.Sprintf("%d", muxErr.Code),
		Message:   muxErr.Message,
		Timestamp: time.Now(),
	}

	if requestID := r.Header.Get("X-Request-ID"); requestID != "" {
		errorResp.RequestID = requestID
	}
	if traceID := r.Header.Get("X-Trace-ID"); traceID != "" {
		errorResp.TraceID = traceID
	}

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

	if err := json.NewEncoder(w).Encode(errorResp); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, `{"error":"Internal server error","code":"SYSTEM_INTERNAL_ERROR"}`)
	}
}

// getHTTPStatusCode 获取HTTP状态码
func (m *HTTPErrorMiddleware) getHTTPStatusCode(muxErr *MuxError) int {
	switch muxErr.Code {
	// authorization related
	case ErrCodeAuthUnauthorized:
		return http.StatusUnauthorized
	case ErrCodeAuthForbidden:
		return http.StatusForbidden
	case ErrCodeAuthTokenExpired, ErrCodeAuthTokenInvalid:
		return http.StatusUnauthorized

	// validation related
	case ErrCodeValidationRequired, ErrCodeValidationFormat, ErrCodeValidationRange:
		return http.StatusBadRequest

	// config related
	case ErrCodeConfigNotFound:
		return http.StatusNotFound
	case ErrCodeConfigInvalid:
		return http.StatusBadRequest

	// network related
	case ErrCodeNetworkTimeout:
		return http.StatusRequestTimeout
	case ErrCodeNetworkRefused, ErrCodeNetworkUnreachable:
		return http.StatusServiceUnavailable

	// system related
	case ErrCodeSystemOutOfMemory, ErrCodeSystemResourceLimit:
		return http.StatusInsufficientStorage
	case ErrCodeSystemPanic, ErrCodeSystemInternalError:
		return http.StatusInternalServerError

	// business related
	case ErrCodeBusinessLogicError:
		return http.StatusUnprocessableEntity
	case ErrCodeBusinessRuleViolation:
		return http.StatusConflict

	default:
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

// GRPCErrorInterceptor gRPC error interceptor
type GRPCErrorInterceptor struct {
	manager      *ErrorManager
	includeStack bool
	debugMode    bool
}

// NewGRPCErrorInterceptor create a new grpc error interceptor
func NewGRPCErrorInterceptor(manager *ErrorManager, options ...GRPCInterceptorOption) *GRPCErrorInterceptor {
	i := &GRPCErrorInterceptor{
		manager:      manager,
		includeStack: false,
		debugMode:    false,
	}

	for _, opt := range options {
		opt(i)
	}

	return i
}

// GRPCInterceptorOption gRPC interceptor option
type GRPCInterceptorOption func(*GRPCErrorInterceptor)

// WithGRPCStackTrace with grpc stack trace
func WithGRPCStackTrace(include bool) GRPCInterceptorOption {
	return func(i *GRPCErrorInterceptor) {
		i.includeStack = include
	}
}

// WithGRPCDebugMode with grpc debug mode
func WithGRPCDebugMode(debug bool) GRPCInterceptorOption {
	return func(i *GRPCErrorInterceptor) {
		i.debugMode = debug
	}
}

// UnaryServerInterceptor unary server interceptor
func (i *GRPCErrorInterceptor) UnaryServerInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		defer func() {
			if rec := recover(); rec != nil {
				var err error
				if e, ok := rec.(error); ok {
					err = e
				} else {
					err = fmt.Errorf("panic: %v", rec)
				}

				panicErr := SystemError(ErrCodeSystemPanic, err.Error())
				if i.includeStack {
					panicErr = panicErr.WithContext("stack", string(debug.Stack()))
				}

				i.manager.Handle(ctx, panicErr)
			}
		}()

		resp, err := handler(ctx, req)
		if err != nil {
			muxErr := Convert(err)

			i.manager.Handle(ctx, muxErr)

			return resp, i.convertToGRPCError(muxErr)
		}

		return resp, nil
	}
}

// StreamServerInterceptor stream server interceptor
func (i *GRPCErrorInterceptor) StreamServerInterceptor() grpc.StreamServerInterceptor {
	return func(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		defer func() {
			if rec := recover(); rec != nil {
				var err error
				if e, ok := rec.(error); ok {
					err = e
				} else {
					err = fmt.Errorf("panic: %v", rec)
				}

				panicErr := SystemError(ErrCodeSystemPanic, err.Error())
				if i.includeStack {
					panicErr = panicErr.WithContext("stack", string(debug.Stack()))
				}

				i.manager.Handle(ss.Context(), panicErr)
			}
		}()

		err := handler(srv, ss)
		if err != nil {
			muxErr := Convert(err)

			i.manager.Handle(ss.Context(), muxErr)

			return i.convertToGRPCError(muxErr)
		}

		return nil
	}
}

// convertToGRPCError convert mux error to gRPC error
func (i *GRPCErrorInterceptor) convertToGRPCError(muxErr *MuxError) error {
	code := i.getGRPCCode(muxErr)

	st := status.New(code, muxErr.Message)

	return st.Err()
}

// getGRPCCode get gRPC code
func (i *GRPCErrorInterceptor) getGRPCCode(muxErr *MuxError) codes.Code {
	switch muxErr.Code {
	// authorization related
	case ErrCodeAuthUnauthorized:
		return codes.Unauthenticated
	case ErrCodeAuthForbidden:
		return codes.PermissionDenied
	case ErrCodeAuthTokenExpired, ErrCodeAuthTokenInvalid:
		return codes.Unauthenticated

	// validation related
	case ErrCodeValidationRequired, ErrCodeValidationFormat, ErrCodeValidationRange:
		return codes.InvalidArgument

	// config related
	case ErrCodeConfigNotFound:
		return codes.NotFound
	case ErrCodeConfigInvalid:
		return codes.InvalidArgument

	// network related
	case ErrCodeNetworkTimeout:
		return codes.DeadlineExceeded
	case ErrCodeNetworkRefused, ErrCodeNetworkUnreachable:
		return codes.Unavailable

	// system related
	case ErrCodeSystemOutOfMemory, ErrCodeSystemResourceLimit:
		return codes.ResourceExhausted
	case ErrCodeSystemPanic, ErrCodeSystemInternalError:
		return codes.Internal

	// business logic related
	case ErrCodeBusinessLogicError:
		return codes.FailedPrecondition
	case ErrCodeBusinessRuleViolation:
		return codes.AlreadyExists

	default:
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

// ErrorHandlerFunc error handler function
type ErrorHandlerFunc func(error) error

// HandleError handle error
func HandleError(err error) error {
	if err == nil {
		return nil
	}

	// convert error to mux error
	muxErr := Convert(err)

	// handle error
	Handle(context.Background(), muxErr)

	return muxErr
}

// MustHandleError Must handle error
func MustHandleError(err error) {
	if err == nil {
		return
	}

	// convert error to mux error
	muxErr := Convert(err)

	// handle error
	Handle(context.Background(), muxErr)

	if muxErr.Level == LevelFatal {
		panic(muxErr)
	}
}

// RecoverAndHandle recover and handle error
func RecoverAndHandle() error {
	if rec := recover(); rec != nil {
		var err error
		if e, ok := rec.(error); ok {
			err = e
		} else {
			err = fmt.Errorf("panic: %v", rec)
		}

		// create panic error
		panicErr := SystemError(ErrCodeSystemPanic, err.Error())
		panicErr = panicErr.WithContext("stack", string(debug.Stack()))

		// handle error
		Handle(context.Background(), panicErr)

		return panicErr
	}
	return nil
}
