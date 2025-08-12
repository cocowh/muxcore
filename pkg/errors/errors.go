// Copyright (c) 2025 cocowh. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package errors

import (
	"errors"
	"fmt"
	"runtime"
	"strings"
	"time"
)

// ErrorCode error code
type ErrorCode int

// ErrorLevel error level
type ErrorLevel int

// ErrorCategory error category
type ErrorCategory string

const (
	LevelTrace ErrorLevel = iota
	LevelDebug
	LevelInfo
	LevelWarn
	LevelError
	LevelFatal
)

const (
	CategorySystem     ErrorCategory = "system"     // 系统错误
	CategoryNetwork    ErrorCategory = "network"    // 网络错误
	CategoryProtocol   ErrorCategory = "protocol"   // 协议错误
	CategoryBuffer     ErrorCategory = "buffer"     // 缓冲区错误
	CategoryConfig     ErrorCategory = "config"     // 配置错误
	CategoryAuth       ErrorCategory = "auth"       // 认证错误
	CategoryValidation ErrorCategory = "validation" // 验证错误
	CategoryBusiness   ErrorCategory = "business"   // 业务错误
	CategoryWASM       ErrorCategory = "wasm"       // WASM错误
	CategoryGovernance ErrorCategory = "governance" // 治理错误
	CategorySecurity   ErrorCategory = "security"   // 安全错误
)

// system error code (1000-1999)
const (
	ErrCodeSystemUnknown       ErrorCode = 1000
	ErrCodeSystemOutOfMemory   ErrorCode = 1001
	ErrCodeSystemResourceLimit ErrorCode = 1002
	ErrCodeSystemInternalError ErrorCode = 1003
	ErrCodeSystemShutdown      ErrorCode = 1004
	ErrCodeSystemPanic         ErrorCode = 1005
)

// network error code (2000-2999)
const (
	ErrCodeNetworkUnknown        ErrorCode = 2000
	ErrCodeNetworkTimeout        ErrorCode = 2001
	ErrCodeNetworkConnectionLost ErrorCode = 2002
	ErrCodeNetworkRefused        ErrorCode = 2003
	ErrCodeNetworkUnreachable    ErrorCode = 2004
	ErrCodeNetworkTLSError       ErrorCode = 2005
)

// protocol error code (3000-3999)
const (
	ErrCodeProtocolUnknown      ErrorCode = 3000
	ErrCodeProtocolInvalid      ErrorCode = 3001
	ErrCodeProtocolUnsupported  ErrorCode = 3002
	ErrCodeProtocolVersionError ErrorCode = 3003
	ErrCodeProtocolParseError   ErrorCode = 3004
)

// buffer error code (4000-4999)
const (
	ErrCodeBufferUnknown   ErrorCode = 4000
	ErrCodeBufferNotEnough ErrorCode = 4001
	ErrCodeBufferOverflow  ErrorCode = 4002
	ErrCodeBufferCorrupted ErrorCode = 4003
	ErrCodeBufferPoolEmpty ErrorCode = 4004
)

// config error code (5000-5999)
const (
	ErrCodeConfigUnknown    ErrorCode = 5000
	ErrCodeConfigNotFound   ErrorCode = 5001
	ErrCodeConfigInvalid    ErrorCode = 5002
	ErrCodeConfigParseError ErrorCode = 5003
	ErrCodeConfigValidation ErrorCode = 5004
)

// auth error code (6000-6999)
const (
	ErrCodeAuthUnknown      ErrorCode = 6000
	ErrCodeAuthUnauthorized ErrorCode = 6001
	ErrCodeAuthForbidden    ErrorCode = 6002
	ErrCodeAuthTokenExpired ErrorCode = 6003
	ErrCodeAuthTokenInvalid ErrorCode = 6004
)

// validation error code (7000-7999)
const (
	ErrCodeValidationUnknown   ErrorCode = 7000
	ErrCodeValidationRequired  ErrorCode = 7001
	ErrCodeValidationFormat    ErrorCode = 7002
	ErrCodeValidationRange     ErrorCode = 7003
	ErrCodeValidationDuplicate ErrorCode = 7004
)

// business error code (8000-8999)
const (
	ErrCodeBusinessUnknown       ErrorCode = 8000
	ErrCodeBusinessLogicError    ErrorCode = 8001
	ErrCodeBusinessStateError    ErrorCode = 8002
	ErrCodeBusinessRuleViolation ErrorCode = 8003
)

// WASM error code (9000-9999)
const (
	ErrCodeWASMUnknown          ErrorCode = 9000
	ErrCodeWASMCompileError     ErrorCode = 9001
	ErrCodeWASMRuntimeError     ErrorCode = 9002
	ErrCodeWASMMemoryError      ErrorCode = 9003
	ErrCodeWASMFunctionNotFound ErrorCode = 9004
)

// Governance error code (10000-10999)
const (
	ErrCodeGovernanceUnknown         ErrorCode = 10000
	ErrCodeGovernancePolicyViolation ErrorCode = 10001
	ErrCodeGovernanceCircuitBreaker  ErrorCode = 10002
	ErrCodeGovernanceRateLimit       ErrorCode = 10003
	ErrCodeGovernanceLoadBalancer    ErrorCode = 10004
)

type MuxError struct {
	Code      ErrorCode              `json:"code"`
	Message   string                 `json:"message"`
	Category  ErrorCategory          `json:"category"`
	Level     ErrorLevel             `json:"level"`
	Timestamp time.Time              `json:"timestamp"`
	Stack     string                 `json:"stack,omitempty"`
	Cause     error                  `json:"cause,omitempty"`
	Context   map[string]interface{} `json:"context,omitempty"`
}

// Error implements error interface
func (e *MuxError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("[%s:%d] %s: %v", e.Category, e.Code, e.Message, e.Cause)
	}
	return fmt.Sprintf("[%s:%d] %s", e.Category, e.Code, e.Message)
}

// Unwrap 实现errors.Unwrap接口
func (e *MuxError) Unwrap() error {
	return e.Cause
}

// Is 实现errors.Is接口
func (e *MuxError) Is(target error) bool {
	var t *MuxError
	if errors.As(target, &t) {
		return e.Code == t.Code
	}
	return false
}

// WithContext with context
func (e *MuxError) WithContext(key string, value interface{}) *MuxError {
	if e.Context == nil {
		e.Context = make(map[string]interface{})
	}
	e.Context[key] = value
	return e
}

// WithCause with cause
func (e *MuxError) WithCause(cause error) *MuxError {
	e.Cause = cause
	return e
}

// New create error
func New(code ErrorCode, category ErrorCategory, level ErrorLevel, message string) *MuxError {
	return &MuxError{
		Code:      code,
		Message:   message,
		Category:  category,
		Level:     level,
		Timestamp: time.Now(),
		Stack:     getStack(),
	}
}

// Newf create error with format message
func Newf(code ErrorCode, category ErrorCategory, level ErrorLevel, format string, args ...interface{}) *MuxError {
	return New(code, category, level, fmt.Sprintf(format, args...))
}

// Wrap existing error with code, category, level and message
func Wrap(err error, code ErrorCode, category ErrorCategory, level ErrorLevel, message string) *MuxError {
	return &MuxError{
		Code:      code,
		Message:   message,
		Category:  category,
		Level:     level,
		Timestamp: time.Now(),
		Stack:     getStack(),
		Cause:     err,
	}
}

// Wrapf wrap existing error with code, category, level and format message
func Wrapf(err error, code ErrorCode, category ErrorCategory, level ErrorLevel, format string, args ...interface{}) *MuxError {
	return Wrap(err, code, category, level, fmt.Sprintf(format, args...))
}

// getStack get error stack
func getStack() string {
	var buf [4096]byte
	n := runtime.Stack(buf[:], false)
	stack := string(buf[:n])

	lines := strings.Split(stack, "\n")
	filtered := make([]string, 0, len(lines))

	for i := 0; i < len(lines); i++ {
		if strings.Contains(lines[i], "runtime.Stack") ||
			strings.Contains(lines[i], "muxcore/pkg/errors.getStack") ||
			(strings.Contains(lines[i], "muxcore/pkg/errors.New") && i+1 < len(lines)) ||
			(strings.Contains(lines[i], "muxcore/pkg/errors.Newf") && i+1 < len(lines)) ||
			(strings.Contains(lines[i], "muxcore/pkg/errors.Wrap") && i+1 < len(lines)) ||
			(strings.Contains(lines[i], "muxcore/pkg/errors.Wrapf") && i+1 < len(lines)) {
			i++
			continue
		}
		filtered = append(filtered, lines[i])
	}

	return strings.Join(filtered, "\n")
}

// GetErrorMessage get error message by error code
func GetErrorMessage(code ErrorCode) string {
	switch code {
	// system errors
	case ErrCodeSystemUnknown:
		return "Unknown system error"
	case ErrCodeSystemOutOfMemory:
		return "System out of memory"
	case ErrCodeSystemResourceLimit:
		return "System resource limit exceeded"
	case ErrCodeSystemInternalError:
		return "Internal system error"
	case ErrCodeSystemShutdown:
		return "System is shutting down"

	// network errors
	case ErrCodeNetworkUnknown:
		return "Unknown network error"
	case ErrCodeNetworkTimeout:
		return "Network operation timeout"
	case ErrCodeNetworkConnectionLost:
		return "Network connection lost"
	case ErrCodeNetworkRefused:
		return "Network connection refused"
	case ErrCodeNetworkUnreachable:
		return "Network unreachable"
	case ErrCodeNetworkTLSError:
		return "TLS/SSL error"

	// protocol errors
	case ErrCodeProtocolUnknown:
		return "Unknown protocol error"
	case ErrCodeProtocolInvalid:
		return "Invalid protocol"
	case ErrCodeProtocolUnsupported:
		return "Unsupported protocol"
	case ErrCodeProtocolVersionError:
		return "Protocol version error"
	case ErrCodeProtocolParseError:
		return "Protocol parse error"

	// buffer errors
	case ErrCodeBufferUnknown:
		return "Unknown buffer error"
	case ErrCodeBufferNotEnough:
		return "Buffer not enough"
	case ErrCodeBufferOverflow:
		return "Buffer overflow"
	case ErrCodeBufferCorrupted:
		return "Buffer corrupted"
	case ErrCodeBufferPoolEmpty:
		return "Buffer pool empty"

	// config errors
	case ErrCodeConfigUnknown:
		return "Unknown config error"
	case ErrCodeConfigNotFound:
		return "Config not found"
	case ErrCodeConfigInvalid:
		return "Invalid config"
	case ErrCodeConfigParseError:
		return "Config parse error"
	case ErrCodeConfigValidation:
		return "Config validation error"

	// auth errors
	case ErrCodeAuthUnknown:
		return "Unknown auth error"
	case ErrCodeAuthUnauthorized:
		return "Unauthorized"
	case ErrCodeAuthForbidden:
		return "Forbidden"
	case ErrCodeAuthTokenExpired:
		return "Token expired"
	case ErrCodeAuthTokenInvalid:
		return "Invalid token"

	// validation errors
	case ErrCodeValidationUnknown:
		return "Unknown validation error"
	case ErrCodeValidationRequired:
		return "Required field missing"
	case ErrCodeValidationFormat:
		return "Invalid format"
	case ErrCodeValidationRange:
		return "Value out of range"
	case ErrCodeValidationDuplicate:
		return "Duplicate value"

	// business errors
	case ErrCodeBusinessUnknown:
		return "Unknown business error"
	case ErrCodeBusinessLogicError:
		return "Business logic error"
	case ErrCodeBusinessStateError:
		return "Invalid business state"
	case ErrCodeBusinessRuleViolation:
		return "Business rule violation"

	// WASM errors
	case ErrCodeWASMUnknown:
		return "Unknown WASM error"
	case ErrCodeWASMCompileError:
		return "WASM compile error"
	case ErrCodeWASMRuntimeError:
		return "WASM runtime error"
	case ErrCodeWASMMemoryError:
		return "WASM memory error"
	case ErrCodeWASMFunctionNotFound:
		return "WASM function not found"

	// governance errors
	case ErrCodeGovernanceUnknown:
		return "Unknown governance error"
	case ErrCodeGovernancePolicyViolation:
		return "Policy violation"
	case ErrCodeGovernanceCircuitBreaker:
		return "Circuit breaker activated"
	case ErrCodeGovernanceRateLimit:
		return "Rate limit exceeded"
	case ErrCodeGovernanceLoadBalancer:
		return "Load balancer error"

	default:
		return "Unknown error"
	}
}

// GetErrorCategory returns the error category for the given error code.
func GetErrorCategory(code ErrorCode) ErrorCategory {
	switch {
	case code >= 1000 && code < 2000:
		return CategorySystem
	case code >= 2000 && code < 3000:
		return CategoryNetwork
	case code >= 3000 && code < 4000:
		return CategoryProtocol
	case code >= 4000 && code < 5000:
		return CategoryBuffer
	case code >= 5000 && code < 6000:
		return CategoryConfig
	case code >= 6000 && code < 7000:
		return CategoryAuth
	case code >= 7000 && code < 8000:
		return CategoryValidation
	case code >= 8000 && code < 9000:
		return CategoryBusiness
	case code >= 9000 && code < 10000:
		return CategoryWASM
	case code >= 10000 && code < 11000:
		return CategoryGovernance
	default:
		return CategorySystem
	}
}
