// Copyright (c) 2025 cocowh. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package errors

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/cocowh/muxcore/pkg/logger"
)

// ErrorHandler error handler interface
type ErrorHandler interface {
	Handle(ctx context.Context, err *MuxError) error
	CanHandle(err *MuxError) bool
	Priority() int
}

// ErrorReporter error reporter interface
type ErrorReporter interface {
	Report(ctx context.Context, err *MuxError) error
}

// ErrorRecovery error recovery interface
type ErrorRecovery interface {
	Recover(ctx context.Context, err *MuxError) error
	CanRecover(err *MuxError) bool
}

// ErrorManager error manager
type ErrorManager struct {
	handlers   []ErrorHandler
	reporters  []ErrorReporter
	recoveries []ErrorRecovery
	metrics    *ErrorMetrics
	mutex      sync.RWMutex
}

// ErrorMetrics error metrics
type ErrorMetrics struct {
	TotalErrors      int64                   `json:"total_errors"`
	ErrorsByCode     map[ErrorCode]int64     `json:"errors_by_code"`
	ErrorsByLevel    map[ErrorLevel]int64    `json:"errors_by_level"`
	ErrorsByCategory map[ErrorCategory]int64 `json:"errors_by_category"`
	LastError        *MuxError               `json:"last_error"`
	LastErrorTime    time.Time               `json:"last_error_time"`
	mutex            sync.RWMutex
}

// NewErrorManager create a new error manager
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

// RegisterHandler register a new error handler
func (em *ErrorManager) RegisterHandler(handler ErrorHandler) {
	em.mutex.Lock()
	defer em.mutex.Unlock()

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

// RegisterReporter register a new error reporter
func (em *ErrorManager) RegisterReporter(reporter ErrorReporter) {
	em.mutex.Lock()
	defer em.mutex.Unlock()
	em.reporters = append(em.reporters, reporter)
}

// RegisterRecovery register a new error recovery
func (em *ErrorManager) RegisterRecovery(recovery ErrorRecovery) {
	em.mutex.Lock()
	defer em.mutex.Unlock()
	em.recoveries = append(em.recoveries, recovery)
}

// Handle error and return a new error
func (em *ErrorManager) Handle(ctx context.Context, err error) error {
	var muxErr *MuxError
	var me *MuxError
	if errors.As(err, &me) {
		muxErr = me
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

// updateMetrics updates the error metrics.
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

// logError logs the error with the appropriate log level.
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

// tryRecover tries to recover from the error using registered recoveries.
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

// handleError handle error.
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

// reportError report error.
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

// GetMetrics get error metrics.
func (em *ErrorManager) GetMetrics() *ErrorMetrics {
	em.metrics.mutex.RLock()
	defer em.metrics.mutex.RUnlock()

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

// ResetMetrics reset the global error metrics.
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

var globalErrorManager = NewErrorManager()

// Handle error and return the error.
func Handle(ctx context.Context, err error) error {
	return globalErrorManager.Handle(ctx, err)
}

// RegisterHandler register global error handler.
func RegisterHandler(handler ErrorHandler) {
	globalErrorManager.RegisterHandler(handler)
}

// RegisterReporter register global error reporter.
func RegisterReporter(reporter ErrorReporter) {
	globalErrorManager.RegisterReporter(reporter)
}

// RegisterRecovery register global error recovery.
func RegisterRecovery(recovery ErrorRecovery) {
	globalErrorManager.RegisterRecovery(recovery)
}

// GetMetrics get the global error metrics.
func GetMetrics() *ErrorMetrics {
	return globalErrorManager.GetMetrics()
}

// ResetMetrics reset the global error metrics.
func ResetMetrics() {
	globalErrorManager.ResetMetrics()
}
