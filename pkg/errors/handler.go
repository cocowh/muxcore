// Copyright (c) 2025 cocowh. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package errors

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/cocowh/muxcore/pkg/logger"
)

// ErrorHandler 错误处理器接口
type ErrorHandler interface {
	Handle(ctx context.Context, err *MuxError) error
	CanHandle(err *MuxError) bool
	Priority() int
}

// ErrorReporter 错误报告器接口
type ErrorReporter interface {
	Report(ctx context.Context, err *MuxError) error
}

// ErrorRecovery 错误恢复器接口
type ErrorRecovery interface {
	Recover(ctx context.Context, err *MuxError) error
	CanRecover(err *MuxError) bool
}

// ErrorManager 错误管理器
type ErrorManager struct {
	handlers   []ErrorHandler
	reporters  []ErrorReporter
	recoveries []ErrorRecovery
	metrics    *ErrorMetrics
	mutex      sync.RWMutex
}

// ErrorMetrics 错误指标
type ErrorMetrics struct {
	TotalErrors      int64                   `json:"total_errors"`
	ErrorsByCode     map[ErrorCode]int64     `json:"errors_by_code"`
	ErrorsByLevel    map[ErrorLevel]int64    `json:"errors_by_level"`
	ErrorsByCategory map[ErrorCategory]int64 `json:"errors_by_category"`
	LastError        *MuxError               `json:"last_error"`
	LastErrorTime    time.Time               `json:"last_error_time"`
	mutex            sync.RWMutex
}

// NewErrorManager 创建错误管理器
func NewErrorManager() *ErrorManager {
	return &ErrorManager{
		handlers:   make([]ErrorHandler, 0),
		reporters:  make([]ErrorReporter, 0),
		recoveries: make([]ErrorRecovery, 0),
		metrics: &ErrorMetrics{
			ErrorsByCode:     make(map[ErrorCode]int64),
			ErrorsByLevel:    make(map[ErrorLevel]int64),
			ErrorsByCategory: make(map[ErrorCategory]int64),
		},
	}
}

// RegisterHandler 注册错误处理器
func (em *ErrorManager) RegisterHandler(handler ErrorHandler) {
	em.mutex.Lock()
	defer em.mutex.Unlock()

	// 按优先级插入
	inserted := false
	for i, h := range em.handlers {
		if handler.Priority() > h.Priority() {
			em.handlers = append(em.handlers[:i], append([]ErrorHandler{handler}, em.handlers[i:]...)...)
			inserted = true
			break
		}
	}
	if !inserted {
		em.handlers = append(em.handlers, handler)
	}
}

// RegisterReporter 注册错误报告器
func (em *ErrorManager) RegisterReporter(reporter ErrorReporter) {
	em.mutex.Lock()
	defer em.mutex.Unlock()
	em.reporters = append(em.reporters, reporter)
}

// RegisterRecovery 注册错误恢复器
func (em *ErrorManager) RegisterRecovery(recovery ErrorRecovery) {
	em.mutex.Lock()
	defer em.mutex.Unlock()
	em.recoveries = append(em.recoveries, recovery)
}

// Handle 处理错误
func (em *ErrorManager) Handle(ctx context.Context, err error) error {
	var muxErr *MuxError

	// 转换为MuxError
	if me, ok := err.(*MuxError); ok {
		muxErr = me
	} else {
		muxErr = Wrap(err, ErrCodeSystemUnknown, CategorySystem, LevelError, "Unknown error")
	}

	// 更新指标
	em.updateMetrics(muxErr)

	// 记录错误日志
	em.logError(muxErr)

	// 尝试恢复
	if recoveryErr := em.tryRecover(ctx, muxErr); recoveryErr != nil {
		logger.Errorf("Failed to recover from error: %v", recoveryErr)
	}

	// 处理错误
	if handleErr := em.handleError(ctx, muxErr); handleErr != nil {
		logger.Errorf("Failed to handle error: %v", handleErr)
	}

	// 报告错误
	if reportErr := em.reportError(ctx, muxErr); reportErr != nil {
		logger.Errorf("Failed to report error: %v", reportErr)
	}

	return muxErr
}

// updateMetrics 更新错误指标
func (em *ErrorManager) updateMetrics(err *MuxError) {
	em.metrics.mutex.Lock()
	defer em.metrics.mutex.Unlock()

	em.metrics.TotalErrors++
	em.metrics.ErrorsByCode[err.Code]++
	em.metrics.ErrorsByLevel[err.Level]++
	em.metrics.ErrorsByCategory[err.Category]++
	em.metrics.LastError = err
	em.metrics.LastErrorTime = time.Now()
}

// logError 记录错误日志
func (em *ErrorManager) logError(err *MuxError) {
	switch err.Level {
	case LevelTrace:
		logger.Debugf("[TRACE][%s:%d] %s", err.Category, err.Code, err.Message)
	case LevelDebug:
		logger.Debugf("[%s:%d] %s", err.Category, err.Code, err.Message)
	case LevelInfo:
		logger.Infof("[%s:%d] %s", err.Category, err.Code, err.Message)
	case LevelWarn:
		logger.Warnf("[%s:%d] %s", err.Category, err.Code, err.Message)
	case LevelError:
		logger.Errorf("[%s:%d] %s", err.Category, err.Code, err.Message)
		if err.Stack != "" {
			logger.Errorf("Stack trace: %s", err.Stack)
		}
	case LevelFatal:
		logger.Fatalf("[%s:%d] %s", err.Category, err.Code, err.Message)
		if err.Stack != "" {
			logger.Fatalf("Stack trace: %s", err.Stack)
		}
	}
}

// tryRecover 尝试错误恢复
func (em *ErrorManager) tryRecover(ctx context.Context, err *MuxError) error {
	em.mutex.RLock()
	recoveries := make([]ErrorRecovery, len(em.recoveries))
	copy(recoveries, em.recoveries)
	em.mutex.RUnlock()

	for _, recovery := range recoveries {
		if recovery.CanRecover(err) {
			if recoverErr := recovery.Recover(ctx, err); recoverErr != nil {
				return fmt.Errorf("recovery failed: %w", recoverErr)
			}
			logger.Infof("Successfully recovered from error: %s", err.Error())
			return nil
		}
	}

	return nil
}

// handleError 处理错误
func (em *ErrorManager) handleError(ctx context.Context, err *MuxError) error {
	em.mutex.RLock()
	handlers := make([]ErrorHandler, len(em.handlers))
	copy(handlers, em.handlers)
	em.mutex.RUnlock()

	for _, handler := range handlers {
		if handler.CanHandle(err) {
			if handleErr := handler.Handle(ctx, err); handleErr != nil {
				return fmt.Errorf("handler failed: %w", handleErr)
			}
			return nil
		}
	}

	return nil
}

// reportError 报告错误
func (em *ErrorManager) reportError(ctx context.Context, err *MuxError) error {
	em.mutex.RLock()
	reporters := make([]ErrorReporter, len(em.reporters))
	copy(reporters, em.reporters)
	em.mutex.RUnlock()

	for _, reporter := range reporters {
		if reportErr := reporter.Report(ctx, err); reportErr != nil {
			logger.Errorf("Reporter failed: %v", reportErr)
			// 继续尝试其他报告器
		}
	}

	return nil
}

// GetMetrics 获取错误指标
func (em *ErrorManager) GetMetrics() *ErrorMetrics {
	em.metrics.mutex.RLock()
	defer em.metrics.mutex.RUnlock()

	// 深拷贝指标
	metrics := &ErrorMetrics{
		TotalErrors:      em.metrics.TotalErrors,
		ErrorsByCode:     make(map[ErrorCode]int64),
		ErrorsByLevel:    make(map[ErrorLevel]int64),
		ErrorsByCategory: make(map[ErrorCategory]int64),
		LastError:        em.metrics.LastError,
		LastErrorTime:    em.metrics.LastErrorTime,
	}

	for k, v := range em.metrics.ErrorsByCode {
		metrics.ErrorsByCode[k] = v
	}
	for k, v := range em.metrics.ErrorsByLevel {
		metrics.ErrorsByLevel[k] = v
	}
	for k, v := range em.metrics.ErrorsByCategory {
		metrics.ErrorsByCategory[k] = v
	}

	return metrics
}

// ResetMetrics 重置错误指标
func (em *ErrorManager) ResetMetrics() {
	em.metrics.mutex.Lock()
	defer em.metrics.mutex.Unlock()

	em.metrics.TotalErrors = 0
	em.metrics.ErrorsByCode = make(map[ErrorCode]int64)
	em.metrics.ErrorsByLevel = make(map[ErrorLevel]int64)
	em.metrics.ErrorsByCategory = make(map[ErrorCategory]int64)
	em.metrics.LastError = nil
	em.metrics.LastErrorTime = time.Time{}
}

// 全局错误管理器实例
var globalErrorManager = NewErrorManager()

// Handle 全局错误处理函数
func Handle(ctx context.Context, err error) error {
	return globalErrorManager.Handle(ctx, err)
}

// RegisterHandler 注册全局错误处理器
func RegisterHandler(handler ErrorHandler) {
	globalErrorManager.RegisterHandler(handler)
}

// RegisterReporter 注册全局错误报告器
func RegisterReporter(reporter ErrorReporter) {
	globalErrorManager.RegisterReporter(reporter)
}

// RegisterRecovery 注册全局错误恢复器
func RegisterRecovery(recovery ErrorRecovery) {
	globalErrorManager.RegisterRecovery(recovery)
}

// GetMetrics 获取全局错误指标
func GetMetrics() *ErrorMetrics {
	return globalErrorManager.GetMetrics()
}

// ResetMetrics 重置全局错误指标
func ResetMetrics() {
	globalErrorManager.ResetMetrics()
}
