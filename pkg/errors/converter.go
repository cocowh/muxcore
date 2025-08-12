// Copyright (c) 2025 cocowh. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package errors

import (
	"errors"
	"fmt"
	"net"
	"syscall"
)

// ErrorConverter 错误转换器
type ErrorConverter struct {
	converters map[string]func(error) *MuxError
}

// NewErrorConverter 创建错误转换器
func NewErrorConverter() *ErrorConverter {
	return &ErrorConverter{
		converters: make(map[string]func(error) *MuxError),
	}
}

// RegisterConverter 注册错误转换器
func (ec *ErrorConverter) RegisterConverter(errorType string, converter func(error) *MuxError) {
	ec.converters[errorType] = converter
}

// Convert 转换错误
func (ec *ErrorConverter) Convert(err error) *MuxError {
	if err == nil {
		return nil
	}

	// 如果已经是MuxError，直接返回
	var muxErr *MuxError
	if errors.As(err, &muxErr) {
		return muxErr
	}

	// 尝试使用注册的转换器
	errorType := fmt.Sprintf("%T", err)
	if converter, exists := ec.converters[errorType]; exists {
		return converter(err)
	}

	// 使用内置转换逻辑
	return ec.convertBuiltin(err)
}

// convertBuiltin 内置错误转换逻辑
func (ec *ErrorConverter) convertBuiltin(err error) *MuxError {
	// 网络错误
	var netErr net.Error
	if errors.As(err, &netErr) {
		if netErr.Timeout() {
			return New(ErrCodeNetworkTimeout, CategoryNetwork, LevelError, "Network timeout")
		}
		if netErr.Temporary() {
			return New(ErrCodeNetworkConnectionLost, CategoryNetwork, LevelWarn, "Temporary network error")
		}
	}

	// 系统调用错误
	var errno syscall.Errno
	if errors.As(err, &errno) {
		switch {
		case errors.Is(errno, syscall.ECONNREFUSED):
			return New(ErrCodeNetworkRefused, CategoryNetwork, LevelError, "Connection refused")
		case errors.Is(errno, syscall.ENETUNREACH):
			return New(ErrCodeNetworkUnreachable, CategoryNetwork, LevelError, "Network unreachable")
		case errors.Is(errno, syscall.ETIMEDOUT):
			return New(ErrCodeNetworkTimeout, CategoryNetwork, LevelError, "Connection timeout")
		case errors.Is(errno, syscall.ENOMEM):
			return New(ErrCodeSystemOutOfMemory, CategorySystem, LevelFatal, "Out of memory")
		case errors.Is(errno, syscall.EMFILE), errors.Is(errno, syscall.ENFILE):
			return New(ErrCodeSystemResourceLimit, CategorySystem, LevelError, "Too many open files")
		}
	}

	// 检查错误消息中的关键词
	errorMsg := err.Error()
	switch {
	case contains(errorMsg, "timeout", "timed out"):
		return New(ErrCodeNetworkTimeout, CategoryNetwork, LevelError, errorMsg)
	case contains(errorMsg, "connection refused", "refused"):
		return New(ErrCodeNetworkRefused, CategoryNetwork, LevelError, errorMsg)
	case contains(errorMsg, "connection reset", "reset"):
		return New(ErrCodeNetworkConnectionLost, CategoryNetwork, LevelError, errorMsg)
	case contains(errorMsg, "out of memory", "memory"):
		return New(ErrCodeSystemOutOfMemory, CategorySystem, LevelFatal, errorMsg)
	case contains(errorMsg, "not found", "no such"):
		return New(ErrCodeConfigNotFound, CategoryConfig, LevelError, errorMsg)
	case contains(errorMsg, "permission denied", "forbidden"):
		return New(ErrCodeAuthForbidden, CategoryAuth, LevelError, errorMsg)
	case contains(errorMsg, "invalid", "malformed"):
		return New(ErrCodeValidationFormat, CategoryValidation, LevelError, errorMsg)
	case contains(errorMsg, "overflow", "too large"):
		return New(ErrCodeBufferOverflow, CategoryBuffer, LevelError, errorMsg)
	case contains(errorMsg, "not enough", "insufficient"):
		return New(ErrCodeBufferNotEnough, CategoryBuffer, LevelError, errorMsg)
	}

	// 默认转换为系统未知错误
	return New(ErrCodeSystemUnknown, CategorySystem, LevelError, errorMsg)
}

// contains 检查字符串是否包含任意关键词
func contains(s string, keywords ...string) bool {
	for _, keyword := range keywords {
		if len(s) >= len(keyword) {
			for i := 0; i <= len(s)-len(keyword); i++ {
				match := true
				for j := 0; j < len(keyword); j++ {
					if s[i+j] != keyword[j] && s[i+j] != keyword[j]-32 && s[i+j] != keyword[j]+32 {
						match = false
						break
					}
				}
				if match {
					return true
				}
			}
		}
	}
	return false
}

// 全局错误转换器
var globalConverter = NewErrorConverter()

// Convert 全局错误转换函数
func Convert(err error) *MuxError {
	return globalConverter.Convert(err)
}

// RegisterConverter 注册全局错误转换器
func RegisterConverter(errorType string, converter func(error) *MuxError) {
	globalConverter.RegisterConverter(errorType, converter)
}

// 便捷的错误创建函数

// SystemError 创建系统错误
func SystemError(code ErrorCode, message string) *MuxError {
	return New(code, CategorySystem, LevelError, message)
}

// SystemErrorf 创建格式化系统错误
func SystemErrorf(code ErrorCode, format string, args ...interface{}) *MuxError {
	return Newf(code, CategorySystem, LevelError, format, args...)
}

// NetworkError 创建网络错误
func NetworkError(code ErrorCode, message string) *MuxError {
	return New(code, CategoryNetwork, LevelError, message)
}

// NetworkErrorf 创建格式化网络错误
func NetworkErrorf(code ErrorCode, format string, args ...interface{}) *MuxError {
	return Newf(code, CategoryNetwork, LevelError, format, args...)
}

// ProtocolError 创建协议错误
func ProtocolError(code ErrorCode, message string) *MuxError {
	return New(code, CategoryProtocol, LevelError, message)
}

// ProtocolErrorf 创建格式化协议错误
func ProtocolErrorf(code ErrorCode, format string, args ...interface{}) *MuxError {
	return Newf(code, CategoryProtocol, LevelError, format, args...)
}

// BufferError 创建缓冲区错误
func BufferError(code ErrorCode, message string) *MuxError {
	return New(code, CategoryBuffer, LevelError, message)
}

// BufferErrorf 创建格式化缓冲区错误
func BufferErrorf(code ErrorCode, format string, args ...interface{}) *MuxError {
	return Newf(code, CategoryBuffer, LevelError, format, args...)
}

// ConfigError 创建配置错误
func ConfigError(code ErrorCode, message string) *MuxError {
	return New(code, CategoryConfig, LevelError, message)
}

// ConfigErrorf 创建格式化配置错误
func ConfigErrorf(code ErrorCode, format string, args ...interface{}) *MuxError {
	return Newf(code, CategoryConfig, LevelError, format, args...)
}

// AuthError 创建认证错误
func AuthError(code ErrorCode, message string) *MuxError {
	return New(code, CategoryAuth, LevelError, message)
}

// AuthErrorf 创建格式化认证错误
func AuthErrorf(code ErrorCode, format string, args ...interface{}) *MuxError {
	return Newf(code, CategoryAuth, LevelError, format, args...)
}

// ValidationError 创建验证错误
func ValidationError(code ErrorCode, message string) *MuxError {
	return New(code, CategoryValidation, LevelError, message)
}

// ValidationErrorf 创建格式化验证错误
func ValidationErrorf(code ErrorCode, format string, args ...interface{}) *MuxError {
	return Newf(code, CategoryValidation, LevelError, format, args...)
}

// BusinessError 创建业务错误
func BusinessError(code ErrorCode, message string) *MuxError {
	return New(code, CategoryBusiness, LevelError, message)
}

// BusinessErrorf 创建格式化业务错误
func BusinessErrorf(code ErrorCode, format string, args ...interface{}) *MuxError {
	return Newf(code, CategoryBusiness, LevelError, format, args...)
}

// WASMError 创建WASM错误
func WASMError(code ErrorCode, message string) *MuxError {
	return New(code, CategoryWASM, LevelError, message)
}

// WASMErrorf 创建格式化WASM错误
func WASMErrorf(code ErrorCode, format string, args ...interface{}) *MuxError {
	return Newf(code, CategoryWASM, LevelError, format, args...)
}

// GovernanceError 创建治理错误
func GovernanceError(code ErrorCode, message string) *MuxError {
	return New(code, CategoryGovernance, LevelError, message)
}

// GovernanceErrorf 创建格式化治理错误
func GovernanceErrorf(code ErrorCode, format string, args ...interface{}) *MuxError {
	return Newf(code, CategoryGovernance, LevelError, format, args...)
}

// 特殊错误创建函数

// TimeoutError 创建超时错误
func TimeoutError(message string) *MuxError {
	return NetworkError(ErrCodeNetworkTimeout, message)
}

// TimeoutErrorf 创建格式化超时错误
func TimeoutErrorf(format string, args ...interface{}) *MuxError {
	return NetworkErrorf(ErrCodeNetworkTimeout, format, args...)
}

// NotFoundError 创建未找到错误
func NotFoundError(resource string) *MuxError {
	return ConfigErrorf(ErrCodeConfigNotFound, "%s not found", resource)
}

// UnauthorizedError 创建未授权错误
func UnauthorizedError(message string) *MuxError {
	return AuthError(ErrCodeAuthUnauthorized, message)
}

// ForbiddenError 创建禁止访问错误
func ForbiddenError(message string) *MuxError {
	return AuthError(ErrCodeAuthForbidden, message)
}

// InvalidInputError 创建无效输入错误
func InvalidInputError(field string) *MuxError {
	return ValidationErrorf(ErrCodeValidationFormat, "Invalid input for field: %s", field)
}

// RequiredFieldError 创建必填字段错误
func RequiredFieldError(field string) *MuxError {
	return ValidationErrorf(ErrCodeValidationRequired, "Required field missing: %s", field)
}

// OutOfMemoryError 创建内存不足错误
func OutOfMemoryError() *MuxError {
	return SystemError(ErrCodeSystemOutOfMemory, "System out of memory")
}

// ResourceLimitError 创建资源限制错误
func ResourceLimitError(resource string) *MuxError {
	return SystemErrorf(ErrCodeSystemResourceLimit, "Resource limit exceeded: %s", resource)
}

// InternalError 创建内部错误
func InternalError(message string) *MuxError {
	return SystemError(ErrCodeSystemInternalError, message)
}

// InternalErrorf 创建格式化内部错误
func InternalErrorf(format string, args ...interface{}) *MuxError {
	return SystemErrorf(ErrCodeSystemInternalError, format, args...)
}

// 错误检查函数

// IsTimeout 检查是否为超时错误
func IsTimeout(err error) bool {
	if muxErr, ok := err.(*MuxError); ok {
		return muxErr.Code == ErrCodeNetworkTimeout
	}
	return false
}

// IsNotFound 检查是否为未找到错误
func IsNotFound(err error) bool {
	if muxErr, ok := err.(*MuxError); ok {
		return muxErr.Code == ErrCodeConfigNotFound
	}
	return false
}

// IsUnauthorized 检查是否为未授权错误
func IsUnauthorized(err error) bool {
	if muxErr, ok := err.(*MuxError); ok {
		return muxErr.Code == ErrCodeAuthUnauthorized
	}
	return false
}

// IsForbidden 检查是否为禁止访问错误
func IsForbidden(err error) bool {
	if muxErr, ok := err.(*MuxError); ok {
		return muxErr.Code == ErrCodeAuthForbidden
	}
	return false
}

// IsValidationError 检查是否为验证错误
func IsValidationError(err error) bool {
	if muxErr, ok := err.(*MuxError); ok {
		return muxErr.Category == CategoryValidation
	}
	return false
}

// IsNetworkError 检查是否为网络错误
func IsNetworkError(err error) bool {
	if muxErr, ok := err.(*MuxError); ok {
		return muxErr.Category == CategoryNetwork
	}
	return false
}

// IsSystemError 检查是否为系统错误
func IsSystemError(err error) bool {
	if muxErr, ok := err.(*MuxError); ok {
		return muxErr.Category == CategorySystem
	}
	return false
}

// IsFatal 检查是否为致命错误
func IsFatal(err error) bool {
	if muxErr, ok := err.(*MuxError); ok {
		return muxErr.Level == LevelFatal
	}
	return false
}

// IsRetryable 检查错误是否可重试
func IsRetryable(err error) bool {
	if muxErr, ok := err.(*MuxError); ok {
		// 网络错误和临时系统错误通常可重试
		return muxErr.Category == CategoryNetwork ||
			muxErr.Code == ErrCodeSystemResourceLimit ||
			muxErr.Code == ErrCodeBufferPoolEmpty
	}
	return false
}

// InitBuiltinConverters 初始化内置错误转换器
func InitBuiltinConverters() {
	// 注册标准库错误转换器
	RegisterConverter("*errors.errorString", func(err error) *MuxError {
		return Convert(err) // 使用内置转换逻辑
	})

	// 注册网络错误转换器
	RegisterConverter("*net.OpError", func(err error) *MuxError {
		if opErr, ok := err.(*net.OpError); ok {
			if opErr.Timeout() {
				return NetworkError(ErrCodeNetworkTimeout, opErr.Error())
			}
			if opErr.Temporary() {
				return NetworkError(ErrCodeNetworkConnectionLost, opErr.Error())
			}
			return NetworkError(ErrCodeNetworkUnknown, opErr.Error())
		}
		return nil
	})

	// 注册DNS错误转换器
	RegisterConverter("*net.DNSError", func(err error) *MuxError {
		if dnsErr, ok := err.(*net.DNSError); ok {
			if dnsErr.Timeout() {
				return NetworkError(ErrCodeNetworkTimeout, dnsErr.Error())
			}
			return NetworkError(ErrCodeNetworkUnreachable, dnsErr.Error())
		}
		return nil
	})
}
