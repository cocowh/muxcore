// Copyright (c) 2025 cocowh. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package main

import (
	"context"
	"fmt"

	"github.com/cocowh/muxcore/pkg/errors"
)

func simpleDemo() {
	fmt.Println("=== 简化的错误管理示例 ===")

	// 1. 创建不同类型的错误
	fmt.Println("\n1. 创建不同类型的错误:")

	// 系统错误
	sysErr := errors.SystemError(errors.ErrCodeSystemOutOfMemory, "系统内存不足")
	fmt.Printf("系统错误: %s\n", sysErr)

	// 网络错误
	netErr := errors.NetworkError(errors.ErrCodeNetworkTimeout, "网络连接超时")
	fmt.Printf("网络错误: %s\n", netErr)

	// 配置错误
	configErr := errors.ConfigError(errors.ErrCodeConfigNotFound, "配置文件未找到")
	fmt.Printf("配置错误: %s\n", configErr)

	// 2. 错误转换
	fmt.Println("\n2. 错误转换:")
	stdErr := fmt.Errorf("connection refused")
	muxErr := errors.Convert(stdErr)
	fmt.Printf("标准错误转换: %s -> %s\n", stdErr, muxErr)

	// 3. 错误检查
	fmt.Println("\n3. 错误检查:")
	fmt.Printf("是否为超时错误: %v\n", errors.IsTimeout(netErr))
	fmt.Printf("是否为系统错误: %v\n", errors.IsSystemError(sysErr))
	fmt.Printf("是否可重试: %v\n", errors.IsRetryable(netErr))

	// 4. 错误上下文
	fmt.Println("\n4. 错误上下文:")
	sysErrWithContext := sysErr.WithContext("memory_usage", "95%").WithContext("process_id", 12345)
	fmt.Printf("带上下文的系统错误: %s\n", sysErrWithContext)

	// 5. 错误处理
	fmt.Println("\n5. 错误处理:")
	ctx := context.Background()

	// 创建错误管理器
	manager := errors.NewErrorManager()

	// 注册简单的错误处理器
	manager.RegisterHandler(&simpleErrorHandler{})

	// 处理错误
	manager.Handle(ctx, sysErr)
	manager.Handle(ctx, netErr)
	manager.Handle(ctx, configErr)

	// 6. 错误指标
	fmt.Println("\n6. 错误指标:")
	metrics := manager.GetMetrics()
	fmt.Printf("总错误数: %d\n", metrics.TotalErrors)
	fmt.Printf("按级别统计: %+v\n", metrics.ErrorsByLevel)
	fmt.Printf("按分类统计: %+v\n", metrics.ErrorsByCategory)

	// 7. 便捷错误创建
	fmt.Println("\n7. 便捷错误创建:")
	timeoutErr := errors.TimeoutError("请求处理超时")
	notFoundErr := errors.NotFoundError("用户")
	unauthorizedErr := errors.UnauthorizedError("无效的访问令牌")

	fmt.Printf("超时错误: %s\n", timeoutErr)
	fmt.Printf("未找到错误: %s\n", notFoundErr)
	fmt.Printf("未授权错误: %s\n", unauthorizedErr)

	fmt.Println("\n=== 简化示例完成 ===")
}

// 简单的错误处理器
type simpleErrorHandler struct{}

func (h *simpleErrorHandler) Handle(ctx context.Context, err *errors.MuxError) error {
	fmt.Printf("[处理器] 处理错误: [%s:%d] %s\n", err.Category, err.Code, err.Message)
	return nil
}

func (h *simpleErrorHandler) CanHandle(err *errors.MuxError) bool {
	return true // 处理所有错误
}

func (h *simpleErrorHandler) Priority() int {
	return 50
}

// 注意：这个文件包含simpleDemo函数，可以在main.go中调用
// 由于main.go已经有main函数，这里不再定义main函数
