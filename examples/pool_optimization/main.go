// 优化连接池和协程池演示
// 注意：此示例需要单独运行，因为包含main函数
package main

import (
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/cocowh/muxcore/core/pool"
)

func main() {
	fmt.Println("Starting pool optimization example")
	fmt.Println()

	// 演示优化连接池
	demonstrateOptimizedConnectionPool()
	fmt.Println()

	// 演示优化协程池
	demonstrateOptimizedGoroutinePool()
	fmt.Println()

	fmt.Println("=== 连接池和协程池优化演示完成 ===")
}

func demonstrateOptimizedConnectionPool() {
	fmt.Println("=== 优化连接池演示 ===")

	// 创建优化连接池配置
	config := pool.DefaultOptimizedConnectionPoolConfig()
	config.MaxConnections = 50
	config.MinConnections = 5
	config.MaxIdleTime = 30 * time.Second
	config.HealthCheckInterval = 10 * time.Second

	fmt.Printf("连接池配置: 最大连接数=%d, 最小连接数=%d, 最大空闲时间=%v\n",
		config.MaxConnections, config.MinConnections, config.MaxIdleTime)

	// 创建优化连接池
	connPool := pool.NewOptimizedConnectionPool(config)
	defer connPool.Close()

	fmt.Println("\n=== 连接管理演示 ===")

	// 模拟添加连接
	var connectionIDs []string
	for i := 0; i < 10; i++ {
		// 创建模拟连接
		conn := &mockConnection{id: fmt.Sprintf("conn-%d", i)}
		id := connPool.AddConnection(conn)
		if id != "" {
			connectionIDs = append(connectionIDs, id)
			fmt.Printf("添加连接: %s\n", id)
		}
	}

	// 更新连接协议
	for i, id := range connectionIDs {
		protocol := "http"
		if i%3 == 1 {
			protocol = "grpc"
		} else if i%3 == 2 {
			protocol = "websocket"
		}
		connPool.UpdateConnectionProtocol(id, protocol)
		fmt.Printf("连接 %s 协议设置为: %s\n", id[:8], protocol)
	}

	fmt.Println("\n=== 负载均衡演示 ===")

	// 演示按协议获取连接（负载均衡）
	protocols := []string{"http", "grpc", "websocket"}
	for _, protocol := range protocols {
		conn, found := connPool.GetConnectionByProtocol(protocol)
		if found {
			fmt.Printf("获取 %s 协议连接: %s\n", protocol, conn.ID[:8])
			// 模拟使用连接
			conn.UpdateStats(1024, 512)
		} else {
			fmt.Printf("未找到 %s 协议连接\n", protocol)
		}
	}

	fmt.Println("\n=== 连接统计演示 ===")

	// 获取连接池指标
	metrics := connPool.GetMetrics()
	fmt.Printf("连接池指标:\n")
	fmt.Printf("  总连接数: %d\n", metrics.TotalConnections)
	fmt.Printf("  活跃连接数: %d\n", metrics.ActiveConnections)
	fmt.Printf("  空闲连接数: %d\n", metrics.IdleConnections)
	fmt.Printf("  已创建连接数: %d\n", metrics.ConnectionsCreated)
	fmt.Printf("  总请求数: %d\n", metrics.TotalRequests)
	fmt.Printf("  总发送字节数: %d\n", metrics.TotalBytesSent)
	fmt.Printf("  总接收字节数: %d\n", metrics.TotalBytesReceived)

	fmt.Println("\n=== 连接健康检查演示 ===")

	// 等待健康检查
	time.Sleep(2 * time.Second)
	fmt.Println("健康检查已执行")

	// 移除部分连接
	for i := 0; i < 3; i++ {
		if i < len(connectionIDs) {
			connPool.RemoveConnection(connectionIDs[i])
			fmt.Printf("移除连接: %s\n", connectionIDs[i][:8])
		}
	}

	// 再次获取指标
	metrics = connPool.GetMetrics()
	fmt.Printf("\n移除连接后的指标:\n")
	fmt.Printf("  总连接数: %d\n", metrics.TotalConnections)
	fmt.Printf("  已关闭连接数: %d\n", metrics.ConnectionsClosed)
}

func demonstrateOptimizedGoroutinePool() {
	fmt.Println("=== 优化协程池演示 ===")

	// 创建优化协程池配置
	config := pool.DefaultOptimizedGoroutinePoolConfig()
	config.MinWorkers = 3
	config.MaxWorkers = 20
	config.QueueSize = 100
	config.ScaleUpThreshold = 0.7
	config.ScaleDownThreshold = 0.3

	fmt.Printf("协程池配置: 最小工作协程=%d, 最大工作协程=%d, 队列大小=%d\n",
		config.MinWorkers, config.MaxWorkers, config.QueueSize)
	fmt.Printf("扩容阈值=%.1f, 缩容阈值=%.1f\n", config.ScaleUpThreshold, config.ScaleDownThreshold)

	// 创建优化协程池
	goroutinePool := pool.NewOptimizedGoroutinePool(config)
	defer goroutinePool.Shutdown()

	fmt.Println("\n=== 任务提交演示 ===")

	// 提交轻量级任务
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		taskID := i
		success := goroutinePool.Submit(func() {
			defer wg.Done()
			time.Sleep(100 * time.Millisecond)
			fmt.Printf("轻量级任务 %d 完成\n", taskID)
		})
		if !success {
			fmt.Printf("任务 %d 提交失败\n", taskID)
			wg.Done()
		}
	}

	// 等待轻量级任务完成
	wg.Wait()
	fmt.Println("轻量级任务全部完成")

	fmt.Println("\n=== 高负载任务演示 ===")

	// 提交大量任务触发自动扩容
	for i := 0; i < 50; i++ {
		wg.Add(1)
		taskID := i
		success := goroutinePool.Submit(func() {
			defer wg.Done()
			// 模拟CPU密集型任务
			start := time.Now()
			for time.Since(start) < 200*time.Millisecond {
				// 忙等待
			}
			fmt.Printf("高负载任务 %d 完成\n", taskID)
		})
		if !success {
			fmt.Printf("高负载任务 %d 提交失败\n", taskID)
			wg.Done()
		}
	}

	// 检查自动扩容
	time.Sleep(1 * time.Second)
	metrics := goroutinePool.GetMetrics()
	fmt.Printf("\n高负载期间指标:\n")
	fmt.Printf("  活跃工作协程: %d\n", metrics.ActiveWorkers)
	fmt.Printf("  空闲工作协程: %d\n", metrics.IdleWorkers)
	fmt.Printf("  队列长度: %d\n", metrics.QueueLength)
	fmt.Printf("  已提交任务数: %d\n", metrics.TasksSubmitted)

	// 等待高负载任务完成
	wg.Wait()
	fmt.Println("高负载任务全部完成")

	fmt.Println("\n=== 带超时任务提交演示 ===")

	// 演示带超时的任务提交
	for i := 0; i < 5; i++ {
		taskID := i
		success := goroutinePool.SubmitWithTimeout(func() {
			time.Sleep(50 * time.Millisecond)
			fmt.Printf("超时任务 %d 完成\n", taskID)
		}, 100*time.Millisecond)
		if success {
			fmt.Printf("超时任务 %d 提交成功\n", taskID)
		} else {
			fmt.Printf("超时任务 %d 提交超时\n", taskID)
		}
	}

	fmt.Println("\n=== 协程池性能测试 ===")

	// 性能测试
	start := time.Now()
	taskCount := 1000
	for i := 0; i < taskCount; i++ {
		wg.Add(1)
		goroutinePool.Submit(func() {
			defer wg.Done()
			// 模拟快速任务
			time.Sleep(1 * time.Millisecond)
		})
	}
	wg.Wait()
	duration := time.Since(start)

	fmt.Printf("性能测试: %d个任务耗时 %v (平均 %v/任务)\n",
		taskCount, duration, duration/time.Duration(taskCount))

	fmt.Println("\n=== 最终指标统计 ===")

	// 等待自动缩容
	time.Sleep(2 * time.Second)
	metrics = goroutinePool.GetMetrics()
	fmt.Printf("最终协程池指标:\n")
	fmt.Printf("  活跃工作协程: %d\n", metrics.ActiveWorkers)
	fmt.Printf("  空闲工作协程: %d\n", metrics.IdleWorkers)
	fmt.Printf("  队列长度: %d\n", metrics.QueueLength)
	fmt.Printf("  已提交任务数: %d\n", metrics.TasksSubmitted)
	fmt.Printf("  已完成任务数: %d\n", metrics.TasksCompleted)
	fmt.Printf("  被拒绝任务数: %d\n", metrics.TasksRejected)
	fmt.Printf("  平均任务时间: %v\n", metrics.AverageTaskTime)
	fmt.Printf("  总任务时间: %v\n", metrics.TotalTaskTime)

	if !metrics.LastScaleEvent.IsZero() {
		fmt.Printf("  最后扩缩容时间: %v\n", metrics.LastScaleEvent.Format("15:04:05"))
	}
}

// mockConnection 模拟连接实现
type mockConnection struct {
	id string
}

func (m *mockConnection) Read(b []byte) (n int, err error) {
	return 0, nil
}

func (m *mockConnection) Write(b []byte) (n int, err error) {
	return len(b), nil
}

func (m *mockConnection) Close() error {
	return nil
}

func (m *mockConnection) LocalAddr() net.Addr {
	return &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 8080}
}

func (m *mockConnection) RemoteAddr() net.Addr {
	return &net.TCPAddr{IP: net.ParseIP("192.168.1.100"), Port: 12345}
}

func (m *mockConnection) SetDeadline(t time.Time) error {
	return nil
}

func (m *mockConnection) SetReadDeadline(t time.Time) error {
	return nil
}

func (m *mockConnection) SetWriteDeadline(t time.Time) error {
	return nil
}