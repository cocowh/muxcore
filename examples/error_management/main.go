// Copyright (c) 2025 cocowh. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package main

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/cocowh/muxcore/pkg/errors"
	"github.com/cocowh/muxcore/pkg/logger"
)

func main() {
	fmt.Println("=== MuxCore 统一错误管理系统示例 ===")

	// 初始化日志系统
	logger.InitDefaultLogger(&logger.Config{
		Level:  logger.DebugLevel,
		Format: "console",
	})

	// 1. 基本错误创建和处理
	fmt.Println("\n1. 基本错误创建和处理:")
	demoBasicErrorHandling()

	// 2. 错误转换
	fmt.Println("\n2. 错误转换:")
	demoErrorConversion()

	// 3. 错误管理器
	fmt.Println("\n3. 错误管理器:")
	demoErrorManager()

	// 4. 错误中间件
	fmt.Println("\n4. 错误中间件:")
	demoErrorMiddleware()

	// 5. 错误恢复
	fmt.Println("\n5. 错误恢复:")
	demoErrorRecovery()

	// 6. 错误指标
	fmt.Println("\n6. 错误指标:")
	demoErrorMetrics()

	fmt.Println("\n=== 示例完成 ===")
}

// 基本错误创建和处理
func demoBasicErrorHandling() {
	// 创建不同类型的错误
	sysErr := errors.SystemError(errors.ErrCodeSystemOutOfMemory, "系统内存不足")
	netErr := errors.NetworkError(errors.ErrCodeNetworkTimeout, "网络连接超时")
	configErr := errors.ConfigError(errors.ErrCodeConfigNotFound, "配置文件未找到")

	fmt.Printf("系统错误: %s\n", sysErr)
	fmt.Printf("网络错误: %s\n", netErr)
	fmt.Printf("配置错误: %s\n", configErr)

	// 添加上下文信息
	sysErrWithContext := sysErr.WithContext("memory_usage", "95%").WithContext("process_id", 12345)
	fmt.Printf("带上下文的系统错误: %s\n", sysErrWithContext)

	// 错误检查
	fmt.Printf("是否为超时错误: %v\n", errors.IsTimeout(netErr))
	fmt.Printf("是否为系统错误: %v\n", errors.IsSystemError(sysErr))
	fmt.Printf("是否可重试: %v\n", errors.IsRetryable(netErr))
}

// 错误转换
func demoErrorConversion() {
	// 标准错误转换
	stdErr := fmt.Errorf("connection refused")
	muxErr := errors.Convert(stdErr)
	fmt.Printf("标准错误转换: %s -> %s\n", stdErr, muxErr)

	// 网络错误转换
	// 模拟网络错误
	networkErr := &mockNetError{msg: "network timeout", timeout: true}
	convertedErr := errors.Convert(networkErr)
	fmt.Printf("网络错误转换: %s -> %s\n", networkErr, convertedErr)

	// 便捷错误创建
	timeoutErr := errors.TimeoutError("请求处理超时")
	notFoundErr := errors.NotFoundError("用户")
	unauthorizedErr := errors.UnauthorizedError("无效的访问令牌")

	fmt.Printf("超时错误: %s\n", timeoutErr)
	fmt.Printf("未找到错误: %s\n", notFoundErr)
	fmt.Printf("未授权错误: %s\n", unauthorizedErr)
}

// 错误管理器
func demoErrorManager() {
	// 创建错误管理器
	manager := errors.NewErrorManager()

	// 初始化内置处理器
	shutdownCh := make(chan struct{})
	errors.InitBuiltinHandlers(shutdownCh)

	// 注册自定义错误处理器
	manager.RegisterHandler(&customErrorHandler{})
	manager.RegisterReporter(&customErrorReporter{})
	manager.RegisterRecovery(&customErrorRecovery{})

	// 处理不同类型的错误
	ctx := context.Background()

	err1 := errors.SystemError(errors.ErrCodeSystemOutOfMemory, "内存不足")
	manager.Handle(ctx, err1)

	err2 := errors.NetworkError(errors.ErrCodeNetworkTimeout, "连接超时")
	manager.Handle(ctx, err2)

	err3 := errors.ValidationError(errors.ErrCodeValidationRequired, "必填字段缺失")
	manager.Handle(ctx, err3)

	// 获取错误指标
	metrics := manager.GetMetrics()
	fmt.Printf("总错误数: %d\n", metrics.TotalErrors)
	fmt.Printf("按级别统计: %+v\n", metrics.ErrorsByLevel)
	fmt.Printf("按分类统计: %+v\n", metrics.ErrorsByCategory)
}

// 错误中间件
func demoErrorMiddleware() {
	// 创建错误管理器
	manager := errors.NewErrorManager()
	shutdownCh := make(chan struct{})
	errors.InitBuiltinHandlers(shutdownCh)

	// 创建HTTP错误中间件
	httpMiddleware := errors.NewHTTPErrorMiddleware(
		manager,
		errors.WithStackTrace(true),
		errors.WithDebugMode(true),
	)

	// 创建简单的HTTP处理器
	mux := http.NewServeMux()
	mux.HandleFunc("/success", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "Success")
	})

	mux.HandleFunc("/error", func(w http.ResponseWriter, r *http.Request) {
		// 模拟错误
		err := errors.ValidationError(errors.ErrCodeValidationRequired, "参数验证失败")
		errors.HandleError(err)
		w.WriteHeader(http.StatusBadRequest)
	})

	mux.HandleFunc("/panic", func(w http.ResponseWriter, r *http.Request) {
		// 模拟panic
		panic("模拟panic错误")
	})

	// 应用中间件
	handler := httpMiddleware.Middleware()(mux)

	fmt.Println("HTTP错误中间件已配置")
	fmt.Println("可以访问以下端点测试:")
	fmt.Println("  GET /success - 正常响应")
	fmt.Println("  GET /error - 错误响应")
	fmt.Println("  GET /panic - panic响应")

	// 这里只是演示，实际使用时需要启动HTTP服务器
	_ = handler
}

// 错误恢复
func demoErrorRecovery() {
	// 演示panic恢复
	func() {
		defer func() {
			if err := errors.RecoverAndHandle(); err != nil {
				fmt.Printf("捕获到panic错误: %s\n", err)
			}
		}()

		// 模拟panic
		panic("这是一个测试panic")
	}()

	// 演示错误处理
	err := fmt.Errorf("模拟错误")
	handledErr := errors.HandleError(err)
	fmt.Printf("处理后的错误: %s\n", handledErr)

	// 演示必须处理错误（非致命错误）
	nonFatalErr := errors.ValidationError(errors.ErrCodeValidationFormat, "格式错误")
	errors.MustHandleError(nonFatalErr)
	fmt.Println("非致命错误处理完成")
}

// 错误指标
func demoErrorMetrics() {
	// 获取全局错误指标
	metrics := errors.GetMetrics()
	fmt.Printf("全局错误指标:\n")
	fmt.Printf("  总错误数: %d\n", metrics.TotalErrors)
	fmt.Printf("  最后错误时间: %s\n", metrics.LastErrorTime.Format(time.RFC3339))

	if metrics.LastError != nil {
		fmt.Printf("  最后错误: %s\n", metrics.LastError)
	}

	fmt.Println("\n按错误码统计:")
	for code, count := range metrics.ErrorsByCode {
		if count > 0 {
			fmt.Printf("  %d: %d次\n", code, count)
		}
	}

	fmt.Println("\n按错误级别统计:")
	for level, count := range metrics.ErrorsByLevel {
		if count > 0 {
			fmt.Printf("  %d: %d次\n", level, count)
		}
	}

	fmt.Println("\n按错误分类统计:")
	for category, count := range metrics.ErrorsByCategory {
		if count > 0 {
			fmt.Printf("  %s: %d次\n", category, count)
		}
	}
}

// 自定义错误处理器
type customErrorHandler struct{}

func (h *customErrorHandler) Handle(ctx context.Context, err *errors.MuxError) error {
	fmt.Printf("[自定义处理器] 处理错误: %s\n", err.Message)
	return nil
}

func (h *customErrorHandler) CanHandle(err *errors.MuxError) bool {
	return err.Category == errors.CategoryBusiness
}

func (h *customErrorHandler) Priority() int {
	return 100
}

// 自定义错误报告器
type customErrorReporter struct{}

func (r *customErrorReporter) Report(ctx context.Context, err *errors.MuxError) error {
	fmt.Printf("[自定义报告器] 报告错误: %s\n", err.Message)
	return nil
}

// 自定义错误恢复器
type customErrorRecovery struct{}

func (r *customErrorRecovery) Recover(ctx context.Context, err *errors.MuxError) error {
	if err.Code == errors.ErrCodeNetworkTimeout {
		fmt.Printf("[自定义恢复器] 尝试重连...\n")
		// 模拟重连逻辑
		time.Sleep(100 * time.Millisecond)
		fmt.Printf("[自定义恢复器] 重连成功\n")
		return nil
	}
	return fmt.Errorf("无法恢复错误: %s", err.Message)
}

func (r *customErrorRecovery) CanRecover(err *errors.MuxError) bool {
	return err.Code == errors.ErrCodeNetworkTimeout
}

// 模拟网络错误
type mockNetError struct {
	msg     string
	timeout bool
	temp    bool
}

func (e *mockNetError) Error() string   { return e.msg }
func (e *mockNetError) Timeout() bool   { return e.timeout }
func (e *mockNetError) Temporary() bool { return e.temp }
