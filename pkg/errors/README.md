# MuxCore 统一错误管理系统

## 概述

MuxCore 统一错误管理系统提供了一套完整的错误处理、转换、报告和恢复机制，旨在提高系统的可靠性和可维护性。

## 核心特性

### 1. 统一错误类型
- **MuxError**: 统一的错误结构，包含错误码、消息、分类、级别、时间戳、堆栈跟踪等信息
- **错误分类**: 系统、网络、协议、缓冲区、配置、认证、验证、业务、WASM、治理
- **错误级别**: Trace、Debug、Info、Warn、Error、Fatal
- **错误码**: 分类化的数字错误码，便于程序化处理

### 2. 错误转换
- **自动转换**: 将标准Go错误自动转换为MuxError
- **智能识别**: 根据错误类型和消息内容智能分类
- **自定义转换器**: 支持注册自定义错误转换逻辑

### 3. 错误管理
- **ErrorManager**: 中央错误管理器，协调处理、报告和恢复
- **处理器链**: 支持多个错误处理器，按优先级执行
- **错误报告**: 支持多种报告方式（日志、指标、告警等）
- **错误恢复**: 自动尝试错误恢复（重试、资源清理等）

### 4. 中间件支持
- **HTTP中间件**: 自动捕获和处理HTTP请求中的错误和panic
- **gRPC拦截器**: 处理gRPC服务中的错误
- **统一响应格式**: 标准化的错误响应格式

### 5. 错误指标
- **实时统计**: 错误数量、分类、级别统计
- **错误趋势**: 错误发生时间和频率分析
- **性能监控**: 错误处理性能指标

## 架构设计

```
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   Application   │    │   Middleware    │    │   Error Source  │
│                 │    │                 │    │                 │
│  Business Logic │───▶│  HTTP/gRPC      │───▶│  Network/System │
│                 │    │  Interceptors   │    │  Protocol/etc   │
└─────────────────┘    └─────────────────┘    └─────────────────┘
         │                       │                       │
         ▼                       ▼                       ▼
┌─────────────────────────────────────────────────────────────────┐
│                    Error Converter                             │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────────────────┐ │
│  │   Standard  │  │   Network   │  │      Custom             │ │
│  │   Errors    │  │   Errors    │  │   Converters            │ │
│  └─────────────┘  └─────────────┘  └─────────────────────────┘ │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│                      MuxError                                  │
│  ┌─────────┐ ┌─────────┐ ┌─────────┐ ┌─────────┐ ┌─────────┐  │
│  │  Code   │ │Category │ │ Level   │ │ Context │ │ Stack   │  │
│  └─────────┘ └─────────┘ └─────────┘ └─────────┘ └─────────┘  │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│                    Error Manager                               │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────────────────┐ │
│  │   Handlers  │  │  Reporters  │  │      Recoveries         │ │
│  │             │  │             │  │                         │ │
│  │ • Logging   │  │ • Metrics   │  │ • Retry                 │ │
│  │ • Circuit   │  │ • Alerts    │  │ • Cleanup               │ │
│  │ • Shutdown  │  │ • External  │  │ • Custom                │ │
│  └─────────────┘  └─────────────┘  └─────────────────────────┘ │
└─────────────────────────────────────────────────────────────────┘
```

## 使用指南

### 基本使用

```go
package main

import (
    "context"
    "github.com/cocowh/muxcore/pkg/errors"
)

func main() {
    // 1. 创建错误
    err := errors.SystemError(errors.ErrCodeSystemOutOfMemory, "内存不足")
    
    // 2. 添加上下文
    err = err.WithContext("memory_usage", "95%")
    
    // 3. 处理错误
    ctx := context.Background()
    errors.Handle(ctx, err)
    
    // 4. 错误检查
    if errors.IsSystemError(err) {
        // 处理系统错误
    }
}
```

### 错误转换

```go
// 自动转换标准错误
stdErr := fmt.Errorf("connection timeout")
muxErr := errors.Convert(stdErr)

// 注册自定义转换器
errors.RegisterConverter("*mypackage.CustomError", func(err error) *errors.MuxError {
    if customErr, ok := err.(*mypackage.CustomError); ok {
        return errors.BusinessError(errors.ErrCodeBusinessLogicError, customErr.Message)
    }
    return nil
})
```

### HTTP中间件

```go
package main

import (
    "net/http"
    "github.com/cocowh/muxcore/pkg/errors"
)

func main() {
    // 创建错误管理器
    manager := errors.NewErrorManager()
    
    // 创建HTTP中间件
    middleware := errors.NewHTTPErrorMiddleware(
        manager,
        errors.WithStackTrace(true),
        errors.WithDebugMode(true),
    )
    
    // 应用中间件
    mux := http.NewServeMux()
    handler := middleware.Middleware()(mux)
    
    http.ListenAndServe(":8080", handler)
}
```

### gRPC拦截器

```go
package main

import (
    "google.golang.org/grpc"
    "github.com/cocowh/muxcore/pkg/errors"
)

func main() {
    // 创建错误管理器
    manager := errors.NewErrorManager()
    
    // 创建gRPC拦截器
    interceptor := errors.NewGRPCErrorInterceptor(
        manager,
        errors.WithGRPCStackTrace(true),
        errors.WithGRPCDebugMode(true),
    )
    
    // 创建gRPC服务器
    server := grpc.NewServer(
        grpc.UnaryInterceptor(interceptor.UnaryServerInterceptor()),
        grpc.StreamInterceptor(interceptor.StreamServerInterceptor()),
    )
}
```

### 自定义处理器

```go
type CustomErrorHandler struct {
    priority int
}

func (h *CustomErrorHandler) Handle(ctx context.Context, err *errors.MuxError) error {
    // 自定义错误处理逻辑
    log.Printf("处理错误: %s", err.Message)
    return nil
}

func (h *CustomErrorHandler) CanHandle(err *errors.MuxError) bool {
    return err.Category == errors.CategoryBusiness
}

func (h *CustomErrorHandler) Priority() int {
    return h.priority
}

// 注册处理器
manager := errors.NewErrorManager()
manager.RegisterHandler(&CustomErrorHandler{priority: 100})
```

## 错误码分类

### 系统错误 (1000-1999)
- `1000`: 未知系统错误
- `1001`: 内存不足
- `1002`: 资源限制
- `1003`: 内部错误
- `1004`: 系统关闭
- `1005`: 系统Panic

### 网络错误 (2000-2999)
- `2000`: 未知网络错误
- `2001`: 网络超时
- `2002`: 连接丢失
- `2003`: 连接被拒绝
- `2004`: 网络不可达
- `2005`: TLS错误

### 协议错误 (3000-3999)
- `3000`: 未知协议错误
- `3001`: 无效协议
- `3002`: 不支持的协议
- `3003`: 协议版本错误
- `3004`: 协议解析错误

### 缓冲区错误 (4000-4999)
- `4000`: 未知缓冲区错误
- `4001`: 缓冲区不足
- `4002`: 缓冲区溢出
- `4003`: 缓冲区损坏
- `4004`: 缓冲池为空

### 配置错误 (5000-5999)
- `5000`: 未知配置错误
- `5001`: 配置未找到
- `5002`: 无效配置
- `5003`: 配置解析错误
- `5004`: 配置验证错误

### 认证错误 (6000-6999)
- `6000`: 未知认证错误
- `6001`: 未授权
- `6002`: 禁止访问
- `6003`: 令牌过期
- `6004`: 无效令牌

### 验证错误 (7000-7999)
- `7000`: 未知验证错误
- `7001`: 必填字段
- `7002`: 格式错误
- `7003`: 范围错误
- `7004`: 重复错误

### 业务错误 (8000-8999)
- `8000`: 未知业务错误
- `8001`: 业务逻辑错误
- `8002`: 业务状态错误
- `8003`: 业务规则违反

### WASM错误 (9000-9999)
- `9000`: 未知WASM错误
- `9001`: 编译错误
- `9002`: 运行时错误
- `9003`: 内存错误
- `9004`: 函数未找到

### 治理错误 (10000-10999)
- `10000`: 未知治理错误
- `10001`: 策略违反
- `10002`: 熔断器
- `10003`: 限流
- `10004`: 负载均衡

## 最佳实践

### 1. 错误创建
- 使用具体的错误码而不是通用错误码
- 提供清晰、有意义的错误消息
- 添加相关的上下文信息

```go
// 好的做法
err := errors.ValidationErrorf(errors.ErrCodeValidationRequired, "用户名是必填字段")
err = err.WithContext("field", "username").WithContext("user_id", userID)

// 避免的做法
err := errors.New(errors.ErrCodeSystemUnknown, errors.CategorySystem, errors.LevelError, "错误")
```

### 2. 错误处理
- 在适当的层级处理错误
- 不要忽略错误
- 使用错误检查函数进行条件处理

```go
// 好的做法
if err := someOperation(); err != nil {
    muxErr := errors.Convert(err)
    if errors.IsRetryable(muxErr) {
        // 重试逻辑
    } else {
        return muxErr
    }
}

// 避免的做法
someOperation() // 忽略错误
```

### 3. 中间件配置
- 在生产环境中关闭调试模式
- 根据需要启用堆栈跟踪
- 配置适当的错误报告

```go
// 生产环境配置
middleware := errors.NewHTTPErrorMiddleware(
    manager,
    errors.WithStackTrace(false),
    errors.WithDebugMode(false),
)

// 开发环境配置
middleware := errors.NewHTTPErrorMiddleware(
    manager,
    errors.WithStackTrace(true),
    errors.WithDebugMode(true),
)
```

### 4. 性能考虑
- 避免在热路径中创建过多错误
- 使用错误池来减少内存分配
- 合理配置错误处理器的优先级

## 扩展性

### 自定义错误类型
可以通过实现相应接口来扩展错误管理系统：

```go
// 自定义错误处理器
type ErrorHandler interface {
    Handle(ctx context.Context, err *MuxError) error
    CanHandle(err *MuxError) bool
    Priority() int
}

// 自定义错误报告器
type ErrorReporter interface {
    Report(ctx context.Context, err *MuxError) error
}

// 自定义错误恢复器
type ErrorRecovery interface {
    Recover(ctx context.Context, err *MuxError) error
    CanRecover(err *MuxError) bool
}
```

### 集成第三方系统
错误管理系统支持与各种第三方监控和告警系统集成：

- **监控系统**: Prometheus、Grafana
- **日志系统**: ELK Stack、Fluentd
- **告警系统**: PagerDuty、Slack
- **APM系统**: Jaeger、Zipkin

## 总结

MuxCore 统一错误管理系统提供了一套完整的错误处理解决方案，通过统一的错误类型、智能的错误转换、灵活的处理机制和丰富的中间件支持，帮助开发者构建更加可靠和可维护的应用程序。

系统的设计遵循了可扩展性、性能和易用性的原则，既可以开箱即用，也可以根据具体需求进行深度定制。