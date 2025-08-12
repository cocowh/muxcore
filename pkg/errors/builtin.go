// Copyright (c) 2025 cocowh. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package errors

import (
	"context"
	"fmt"
	"time"

	"github.com/cocowh/muxcore/pkg/logger"
)

// LoggingErrorHandler logging error handler
type LoggingErrorHandler struct {
	priority int
}

// NewLoggingErrorHandler create a new logging error handler
func NewLoggingErrorHandler(priority int) *LoggingErrorHandler {
	return &LoggingErrorHandler{
		priority: priority,
	}
}

func (h *LoggingErrorHandler) Handle(ctx context.Context, err *MuxError) error {
	// 日志处理器只记录日志，不做其他处理
	return nil
}

func (h *LoggingErrorHandler) CanHandle(err *MuxError) bool {
	// 可以处理所有错误
	return true
}

func (h *LoggingErrorHandler) Priority() int {
	return h.priority
}

// CircuitBreakerErrorHandler circuit breaker error handler
type CircuitBreakerErrorHandler struct {
	priority         int
	failureThreshold int
	failureCount     int
	lastFailureTime  time.Time
	resetTimeout     time.Duration
	isOpen           bool
}

// NewCircuitBreakerErrorHandler create a new circuit breaker error handler
func NewCircuitBreakerErrorHandler(priority, failureThreshold int, resetTimeout time.Duration) *CircuitBreakerErrorHandler {
	return &CircuitBreakerErrorHandler{
		priority:         priority,
		failureThreshold: failureThreshold,
		resetTimeout:     resetTimeout,
	}
}

func (h *CircuitBreakerErrorHandler) Handle(ctx context.Context, err *MuxError) error {
	// record failure count and last failure time
	h.failureCount++
	h.lastFailureTime = time.Now()

	// check if circuit breaker should be opened
	if h.failureCount >= h.failureThreshold {
		h.isOpen = true
		logger.Warnf("Circuit breaker opened due to error: %s", err.Error())
	}

	return nil
}

func (h *CircuitBreakerErrorHandler) CanHandle(err *MuxError) bool {
	return err.Category == CategoryNetwork || err.Category == CategorySystem
}

func (h *CircuitBreakerErrorHandler) Priority() int {
	return h.priority
}

// IsOpen check if circuit breaker is open
func (h *CircuitBreakerErrorHandler) IsOpen() bool {
	// check if circuit breaker should be reset
	if h.isOpen && time.Since(h.lastFailureTime) > h.resetTimeout {
		h.isOpen = false
		h.failureCount = 0
		logger.Infof("Circuit breaker reset")
	}
	return h.isOpen
}

// RetryErrorRecovery recovery from error
type RetryErrorRecovery struct {
	maxRetries int
	retryDelay time.Duration
}

// NewRetryErrorRecovery create a new retry error recovery
func NewRetryErrorRecovery(maxRetries int, retryDelay time.Duration) *RetryErrorRecovery {
	return &RetryErrorRecovery{
		maxRetries: maxRetries,
		retryDelay: retryDelay,
	}
}

func (r *RetryErrorRecovery) Recover(ctx context.Context, err *MuxError) error {
	retryCount := 0
	if count, ok := err.Context["retry_count"]; ok {
		if c, ok := count.(int); ok {
			retryCount = c
		}
	}

	if retryCount >= r.maxRetries {
		return fmt.Errorf("max retries exceeded: %d", r.maxRetries)
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(r.retryDelay):
	}

	err.WithContext("retry_count", retryCount+1)
	logger.Infof("Retrying operation, attempt %d/%d", retryCount+1, r.maxRetries)

	return err
}

func (r *RetryErrorRecovery) CanRecover(err *MuxError) bool {
	// 只恢复网络和临时错误
	return err.Category == CategoryNetwork ||
		err.Code == ErrCodeSystemResourceLimit ||
		err.Code == ErrCodeBufferPoolEmpty
}

// MetricsErrorReporter 指标错误报告器
type MetricsErrorReporter struct {
	metrics map[string]interface{}
}

// NewMetricsErrorReporter 创建指标错误报告器
func NewMetricsErrorReporter() *MetricsErrorReporter {
	return &MetricsErrorReporter{
		metrics: make(map[string]interface{}),
	}
}

func (r *MetricsErrorReporter) Report(ctx context.Context, err *MuxError) error {
	// 更新指标
	r.metrics["last_error_code"] = err.Code
	r.metrics["last_error_category"] = err.Category
	r.metrics["last_error_level"] = err.Level
	r.metrics["last_error_time"] = err.Timestamp

	// 这里可以集成到Prometheus或其他指标系统
	logger.Debugf("Error metrics updated: code=%d, category=%s, level=%d",
		err.Code, err.Category, err.Level)

	return nil
}

// AlertErrorReporter 告警错误报告器
type AlertErrorReporter struct {
	alertThreshold ErrorLevel
}

// NewAlertErrorReporter 创建告警错误报告器
func NewAlertErrorReporter(alertThreshold ErrorLevel) *AlertErrorReporter {
	return &AlertErrorReporter{
		alertThreshold: alertThreshold,
	}
}

func (r *AlertErrorReporter) Report(ctx context.Context, err *MuxError) error {
	if err.Level >= r.alertThreshold {
		// 发送告警
		logger.Errorf("ALERT: Critical error occurred - [%s:%d] %s",
			err.Category, err.Code, err.Message)

		// 这里可以集成到告警系统，如邮件、短信、Slack等
		// 例如：sendAlert(err)
	}
	return nil
}

// GracefulShutdownErrorHandler 优雅关闭错误处理器
type GracefulShutdownErrorHandler struct {
	priority        int
	shutdownChannel chan struct{}
}

// NewGracefulShutdownErrorHandler 创建优雅关闭错误处理器
func NewGracefulShutdownErrorHandler(priority int, shutdownChannel chan struct{}) *GracefulShutdownErrorHandler {
	return &GracefulShutdownErrorHandler{
		priority:        priority,
		shutdownChannel: shutdownChannel,
	}
}

func (h *GracefulShutdownErrorHandler) Handle(ctx context.Context, err *MuxError) error {
	if err.Level == LevelFatal || err.Code == ErrCodeSystemShutdown {
		logger.Errorf("Fatal error detected, initiating graceful shutdown: %s", err.Error())

		// 发送关闭信号
		select {
		case h.shutdownChannel <- struct{}{}:
			logger.Infof("Graceful shutdown signal sent")
		default:
			// 通道已满或已关闭
			logger.Warnf("Shutdown signal already sent or channel closed")
		}
	}
	return nil
}

func (h *GracefulShutdownErrorHandler) CanHandle(err *MuxError) bool {
	return err.Level == LevelFatal || err.Code == ErrCodeSystemShutdown
}

func (h *GracefulShutdownErrorHandler) Priority() int {
	return h.priority
}

// ResourceCleanupErrorRecovery 资源清理错误恢复器
type ResourceCleanupErrorRecovery struct {
	cleanupFuncs []func() error
}

// NewResourceCleanupErrorRecovery 创建资源清理错误恢复器
func NewResourceCleanupErrorRecovery() *ResourceCleanupErrorRecovery {
	return &ResourceCleanupErrorRecovery{
		cleanupFuncs: make([]func() error, 0),
	}
}

// AddCleanupFunc 添加清理函数
func (r *ResourceCleanupErrorRecovery) AddCleanupFunc(cleanup func() error) {
	r.cleanupFuncs = append(r.cleanupFuncs, cleanup)
}

func (r *ResourceCleanupErrorRecovery) Recover(ctx context.Context, err *MuxError) error {
	logger.Infof("Starting resource cleanup due to error: %s", err.Error())

	for i, cleanup := range r.cleanupFuncs {
		if cleanupErr := cleanup(); cleanupErr != nil {
			logger.Errorf("Cleanup function %d failed: %v", i, cleanupErr)
			// 继续执行其他清理函数
		} else {
			logger.Debugf("Cleanup function %d completed successfully", i)
		}
	}

	logger.Infof("Resource cleanup completed")
	return nil
}

func (r *ResourceCleanupErrorRecovery) CanRecover(err *MuxError) bool {
	// 对于系统错误和内存错误进行资源清理
	return err.Category == CategorySystem ||
		err.Code == ErrCodeSystemOutOfMemory ||
		err.Code == ErrCodeSystemResourceLimit
}

// InitBuiltinHandlers 初始化内置错误处理器
func InitBuiltinHandlers(shutdownChannel chan struct{}) {
	// 注册日志处理器（最低优先级）
	RegisterHandler(NewLoggingErrorHandler(0))

	// 注册熔断处理器
	RegisterHandler(NewCircuitBreakerErrorHandler(100, 5, 30*time.Second))

	// 注册优雅关闭处理器（最高优先级）
	RegisterHandler(NewGracefulShutdownErrorHandler(1000, shutdownChannel))

	// 注册重试恢复器
	RegisterRecovery(NewRetryErrorRecovery(3, 1*time.Second))

	// 注册资源清理恢复器
	RegisterRecovery(NewResourceCleanupErrorRecovery())

	// 注册指标报告器
	RegisterReporter(NewMetricsErrorReporter())

	// 注册告警报告器（错误级别及以上）
	RegisterReporter(NewAlertErrorReporter(LevelError))

	logger.Infof("Builtin error handlers initialized")
}
