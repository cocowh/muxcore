// Copyright (c) 2025 cocowh. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"runtime"
	"sync"
	"time"

	"github.com/cocowh/muxcore/core/performance"
	"github.com/cocowh/muxcore/pkg/buffer"
)

func main() {
	fmt.Println("=== MuxCore Buffer Optimization Demo ===")
	fmt.Println()

	// 1. 展示统一缓冲区池的基本使用
	demoUnifiedBufferPool()
	fmt.Println()

	// 2. 展示性能缓冲区池的使用
	demoPerformanceBufferPool()
	fmt.Println()

	// 3. 展示并发性能测试
	demoPerformanceComparison()
	fmt.Println()

	// 4. 展示内存使用优化
	demoMemoryOptimization()
}

func demoUnifiedBufferPool() {
	fmt.Println("1. 统一缓冲区池演示")
	fmt.Println("-------------------")

	// 创建统一缓冲区池
	pool := buffer.NewUnifiedBufferPool(1024, 100)
	fmt.Printf("创建统一缓冲区池: 默认大小=%d, 最大池大小=%d\n", 1024, 100)

	// 预分配缓冲区
	pool.PreAllocate(10)
	fmt.Printf("预分配缓冲区数量: %d\n", pool.Count())

	// 获取和使用缓冲区
	buf := pool.Get()
	buf.WriteString("Hello, MuxCore Buffer!")
	fmt.Printf("写入数据: %s\n", string(buf.Bytes()))
	fmt.Printf("缓冲区长度: %d, 容量: %d\n", buf.Len(), buf.Cap())

	// 归还缓冲区
	pool.Put(buf)
	fmt.Printf("归还后池中缓冲区数量: %d\n", pool.Count())

	// 显示统计信息
	stats := pool.Stats()
	fmt.Printf("池统计信息: %+v\n", stats)
}

func demoPerformanceBufferPool() {
	fmt.Println("2. 性能缓冲区池演示")
	fmt.Println("-------------------")

	// 创建性能缓冲区池
	pool := performance.NewBufferPool()
	fmt.Println("创建性能缓冲区池")

	// 预分配缓冲区
	pool.PreAllocate(20)
	fmt.Printf("预分配后池中缓冲区数量: %d\n", pool.Count())

	// 获取和使用缓冲区
	buf := pool.Get()
	buf.WriteString("Performance Buffer Test")
	fmt.Printf("写入数据: %s\n", string(buf.Bytes()))

	// 归还缓冲区
	pool.Put(buf)
	fmt.Printf("归还后池中缓冲区数量: %d\n", pool.Count())

	// 显示统计信息
	stats := pool.Stats()
	fmt.Printf("池统计信息: %+v\n", stats)
}

func demoPerformanceComparison() {
	fmt.Println("3. 并发性能测试")
	fmt.Println("---------------")

	const (
		goroutines = 100
		operations = 1000
	)

	// 测试统一缓冲区池性能
	fmt.Printf("测试统一缓冲区池 (%d goroutines, %d operations each)\n", goroutines, operations)
	pool := buffer.GetGlobalUnifiedPool()
	start := time.Now()
	testBufferPool(pool, goroutines, operations)
	unifiedDuration := time.Since(start)
	fmt.Printf("统一缓冲区池耗时: %v\n", unifiedDuration)

	// 测试传统方式性能
	fmt.Printf("测试传统内存分配 (%d goroutines, %d operations each)\n", goroutines, operations)
	start = time.Now()
	testTraditionalAllocation(goroutines, operations)
	traditionalDuration := time.Since(start)
	fmt.Printf("传统分配耗时: %v\n", traditionalDuration)

	// 性能提升计算
	improvement := float64(traditionalDuration) / float64(unifiedDuration)
	fmt.Printf("性能提升: %.2fx\n", improvement)
}

func testBufferPool(pool *buffer.UnifiedBufferPool, goroutines, operations int) {
	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < operations; j++ {
				buf := pool.Get()
				buf.WriteString("test data")
				_ = buf.Bytes()
				pool.Put(buf)
			}
		}()
	}

	wg.Wait()
}

func testTraditionalAllocation(goroutines, operations int) {
	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < operations; j++ {
				buf := make([]byte, 0, 4096)
				buf = append(buf, []byte("test data")...)
				_ = buf
			}
		}()
	}

	wg.Wait()
}

func demoMemoryOptimization() {
	fmt.Println("4. 内存使用优化演示")
	fmt.Println("-------------------")

	// 获取初始内存统计
	var m1 runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&m1)
	fmt.Printf("初始内存使用: %.2f MB\n", float64(m1.Alloc)/1024/1024)

	// 使用缓冲区池进行大量操作
	pool := buffer.GetGlobalUnifiedPool()
	for i := 0; i < 10000; i++ {
		buf := pool.Get()
		buf.WriteString(fmt.Sprintf("Buffer operation %d", i))
		_ = buf.Bytes()
		pool.Put(buf)
	}

	// 获取操作后内存统计
	var m2 runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&m2)
	fmt.Printf("操作后内存使用: %.2f MB\n", float64(m2.Alloc)/1024/1024)
	fmt.Printf("内存增长: %.2f MB\n", float64(m2.Alloc-m1.Alloc)/1024/1024)

	// 显示池统计
	stats := pool.Stats()
	fmt.Printf("最终池统计: %+v\n", stats)

	fmt.Println()
	fmt.Println("=== Buffer 优化总结 ===")
	fmt.Println("1. 统一了多个 buffer 实现，减少代码重复")
	fmt.Println("2. 结合 sync.Pool 和无锁链表的优势")
	fmt.Println("3. 减少内存分配和 GC 压力")
	fmt.Println("4. 提供了更好的并发性能")
	fmt.Println("5. 支持预分配和统计监控")
}
