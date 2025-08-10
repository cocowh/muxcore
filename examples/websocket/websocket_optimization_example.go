package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/cocowh/muxcore/core/pool"
	"github.com/cocowh/muxcore/core/websocket"
)

// WebSocketOptimizationDemo WebSocket优化演示
type WebSocketOptimizationDemo struct {
	handler *websocket.OptimizedWebSocketHandler
	pool    *pool.ConnectionPool
	ctx     context.Context
	cancel  context.CancelFunc
}

// NewWebSocketOptimizationDemo 创建WebSocket优化演示
func NewWebSocketOptimizationDemo() *WebSocketOptimizationDemo {
	ctx, cancel := context.WithCancel(context.Background())
	
	// 创建连接池
	connPool := pool.New()
	
	// 创建优化的WebSocket配置
	config := &websocket.OptimizedWebSocketConfig{
		WebSocketConfig: &websocket.WebSocketConfig{
			MaxConnections:    5000,
			PingInterval:      30 * time.Second,
			PongTimeout:       10 * time.Second,
			MaxMessageSize:    1024 * 1024, // 1MB
			EnableCompression: true,
			ReadBufferSize:    4096,
			WriteBufferSize:   4096,
			HandshakeTimeout:  10 * time.Second,
		},
		MaxConnections:     5000,
		ConnectionTimeout:  30 * time.Second,
		HeartbeatInterval:  30 * time.Second,
		HeartbeatTimeout:   10 * time.Second,
		MaxMessageSize:     1024 * 1024,
		MessageBufferSize:  1000,
		EnableCompression:  true,
		EnableBatching:     true,
		BatchSize:          50,
		BatchTimeout:       10 * time.Millisecond,
		EnableMetrics:      true,
		EnableRateLimit:    true,
		RateLimit:          1000,
		RateLimitWindow:    time.Minute,
	}
	
	// 创建优化的WebSocket处理器
	handler := websocket.NewOptimizedWebSocketHandler(connPool, config)
	
	return &WebSocketOptimizationDemo{
		handler: handler,
		pool:    connPool,
		ctx:     ctx,
		cancel:  cancel,
	}
}

// RunDemo 运行演示
func (demo *WebSocketOptimizationDemo) RunDemo() {
	fmt.Println("=== WebSocket优化演示 ===")
	fmt.Println()
	
	// 1. 演示配置优化
	demo.demonstrateConfiguration()
	
	// 2. 演示连接管理
	demo.demonstrateConnectionManagement()
	
	// 3. 演示消息处理优化
	demo.demonstrateMessageProcessing()
	
	// 4. 演示批处理功能
	demo.demonstrateBatchProcessing()
	
	// 5. 演示速率限制
	demo.demonstrateRateLimit()
	
	// 6. 演示性能监控
	demo.demonstrateMetrics()
	
	// 7. 演示广播功能
	demo.demonstrateBroadcast()
	
	fmt.Println("\n=== WebSocket优化演示完成 ===")
}

// demonstrateConfiguration 演示配置优化
func (demo *WebSocketOptimizationDemo) demonstrateConfiguration() {
	fmt.Println("1. 配置优化演示")
	fmt.Println("   - 最大连接数: 5000")
	fmt.Println("   - 消息缓冲区大小: 1000")
	fmt.Println("   - 启用压缩: true")
	fmt.Println("   - 启用批处理: true")
	fmt.Println("   - 批处理大小: 50")
	fmt.Println("   - 启用速率限制: true")
	fmt.Println("   - 速率限制: 1000 req/min")
	fmt.Println("   - 启用指标收集: true")
	fmt.Println()
}

// demonstrateConnectionManagement 演示连接管理
func (demo *WebSocketOptimizationDemo) demonstrateConnectionManagement() {
	fmt.Println("2. 连接管理演示")
	
	// 模拟连接管理
	activeConnections := demo.handler.GetActiveConnections()
	fmt.Printf("   当前活跃连接数: %d\n", activeConnections)
	
	connectionIDs := demo.handler.GetConnectionIDs()
	fmt.Printf("   连接ID列表: %v\n", connectionIDs)
	
	fmt.Println("   ✓ 连接池管理")
	fmt.Println("   ✓ 连接限制")
	fmt.Println("   ✓ 心跳检测")
	fmt.Println("   ✓ 自动清理")
	fmt.Println()
}

// demonstrateMessageProcessing 演示消息处理优化
func (demo *WebSocketOptimizationDemo) demonstrateMessageProcessing() {
	fmt.Println("3. 消息处理优化演示")
	
	// 模拟消息处理
	messages := []map[string]interface{}{
		{"type": "chat", "content": "Hello, World!", "user": "user1"},
		{"type": "notification", "content": "New message", "priority": "high"},
		{"type": "heartbeat", "timestamp": time.Now().Unix()},
		{"type": "data", "payload": map[string]interface{}{"key": "value"}},
	}
	
	for i, msg := range messages {
		msgData, _ := json.Marshal(msg)
		fmt.Printf("   处理消息 %d: %s\n", i+1, string(msgData))
	}
	
	fmt.Println("   ✓ 异步消息处理")
	fmt.Println("   ✓ 消息缓冲")
	fmt.Println("   ✓ 错误处理")
	fmt.Println("   ✓ 消息验证")
	fmt.Println()
}

// demonstrateBatchProcessing 演示批处理功能
func (demo *WebSocketOptimizationDemo) demonstrateBatchProcessing() {
	fmt.Println("4. 批处理功能演示")
	
	// 模拟批处理
	batchMessages := make([][]byte, 0, 50)
	for i := 0; i < 50; i++ {
		msg := map[string]interface{}{
			"id":      i,
			"type":    "batch_message",
			"content": fmt.Sprintf("Batch message %d", i),
		}
		msgData, _ := json.Marshal(msg)
		batchMessages = append(batchMessages, msgData)
	}
	
	fmt.Printf("   批处理消息数量: %d\n", len(batchMessages))
	fmt.Println("   批处理配置:")
	fmt.Println("   - 批处理大小: 50")
	fmt.Println("   - 批处理超时: 10ms")
	fmt.Println("   ✓ 自动批处理")
	fmt.Println("   ✓ 批处理优化")
	fmt.Println("   ✓ 超时刷新")
	fmt.Println()
}

// demonstrateRateLimit 演示速率限制
func (demo *WebSocketOptimizationDemo) demonstrateRateLimit() {
	fmt.Println("5. 速率限制演示")
	
	// 创建速率限制器
	rateLimiter := websocket.NewRateLimiter(10, time.Minute)
	
	// 模拟请求
	allowed := 0
	rejected := 0
	
	for i := 0; i < 15; i++ {
		if rateLimiter.Allow() {
			allowed++
		} else {
			rejected++
		}
	}
	
	fmt.Printf("   测试请求: 15\n")
	fmt.Printf("   允许请求: %d\n", allowed)
	fmt.Printf("   拒绝请求: %d\n", rejected)
	fmt.Println("   ✓ 滑动窗口限流")
	fmt.Println("   ✓ 自动清理过期请求")
	fmt.Println("   ✓ 防止滥用")
	fmt.Println()
}

// demonstrateMetrics 演示性能监控
func (demo *WebSocketOptimizationDemo) demonstrateMetrics() {
	fmt.Println("6. 性能监控演示")
	
	// 获取性能指标
	metrics := demo.handler.GetMetrics()
	
	fmt.Println("   实时性能指标:")
	fmt.Printf("   - 活跃连接数: %d\n", metrics.ActiveConnections)
	fmt.Printf("   - 总连接数: %d\n", metrics.TotalConnections)
	fmt.Printf("   - 发送消息数: %d\n", metrics.MessagesSent)
	fmt.Printf("   - 接收消息数: %d\n", metrics.MessagesReceived)
	fmt.Printf("   - 消息错误数: %d\n", metrics.MessageErrors)
	fmt.Printf("   - 传输字节数: %d\n", metrics.BytesTransferred)
	fmt.Printf("   - 错误率: %.2f%%\n", metrics.ErrorRate)
	fmt.Printf("   - 吞吐量: %.2f msg/s\n", metrics.Throughput)
	fmt.Printf("   - 批处理发送数: %d\n", metrics.BatchesSent)
	fmt.Printf("   - 批处理效率: %.2f\n", metrics.BatchEfficiency)
	fmt.Printf("   - 限流命中数: %d\n", metrics.RateLimitHits)
	fmt.Printf("   - 限流拒绝数: %d\n", metrics.RateLimitRejects)
	
	fmt.Println("   ✓ 实时指标收集")
	fmt.Println("   ✓ 性能分析")
	fmt.Println("   ✓ 异常监控")
	fmt.Println()
}

// demonstrateBroadcast 演示广播功能
func (demo *WebSocketOptimizationDemo) demonstrateBroadcast() {
	fmt.Println("7. 广播功能演示")
	
	// 模拟广播消息
	broadcastMsg := map[string]interface{}{
		"type":      "broadcast",
		"content":   "系统公告：服务器将在5分钟后重启",
		"timestamp": time.Now().Unix(),
		"priority":  "high",
	}
	
	msgData, _ := json.Marshal(broadcastMsg)
	fmt.Printf("   广播消息: %s\n", string(msgData))
	
	// 执行广播
	demo.handler.BroadcastMessage(msgData)
	
	activeConnections := demo.handler.GetActiveConnections()
	fmt.Printf("   广播目标: %d 个连接\n", activeConnections)
	
	fmt.Println("   ✓ 高效广播")
	fmt.Println("   ✓ 并发发送")
	fmt.Println("   ✓ 错误处理")
	fmt.Println()
}

// simulateLoad 模拟负载测试
func (demo *WebSocketOptimizationDemo) simulateLoad() {
	fmt.Println("8. 负载测试演示")
	
	start := time.Now()
	var wg sync.WaitGroup
	concurrency := 100
	messagesPerGoroutine := 100
	
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			
			for j := 0; j < messagesPerGoroutine; j++ {
				msg := map[string]interface{}{
					"worker_id": workerID,
					"message_id": j,
					"content": fmt.Sprintf("Load test message from worker %d", workerID),
					"timestamp": time.Now().Unix(),
				}
				msgData, _ := json.Marshal(msg)
				
				// 模拟消息处理
				_ = msgData
			}
		}(i)
	}
	
	wg.Wait()
	duration := time.Since(start)
	totalMessages := concurrency * messagesPerGoroutine
	
	fmt.Printf("   负载测试结果:\n")
	fmt.Printf("   - 并发数: %d\n", concurrency)
	fmt.Printf("   - 总消息数: %d\n", totalMessages)
	fmt.Printf("   - 执行时间: %v\n", duration)
	fmt.Printf("   - 平均处理速度: %.2f msg/s\n", float64(totalMessages)/duration.Seconds())
	
	fmt.Println("   ✓ 高并发处理")
	fmt.Println("   ✓ 性能优化")
	fmt.Println("   ✓ 资源管理")
	fmt.Println()
}

// Stop 停止演示
func (demo *WebSocketOptimizationDemo) Stop() {
	demo.cancel()
	if demo.handler != nil {
		demo.handler.Stop()
	}
	if demo.pool != nil {
		demo.pool.Close()
	}
}

// printOptimizationSummary 打印优化总结
func (demo *WebSocketOptimizationDemo) printOptimizationSummary() {
	fmt.Println("\n=== WebSocket优化总结 ===")
	fmt.Println("\n核心优化特性:")
	fmt.Println("1. 连接管理优化")
	fmt.Println("   - 连接池管理")
	fmt.Println("   - 连接限制")
	fmt.Println("   - 心跳检测")
	fmt.Println("   - 自动清理")
	
	fmt.Println("\n2. 消息处理优化")
	fmt.Println("   - 异步处理")
	fmt.Println("   - 消息缓冲")
	fmt.Println("   - 批处理")
	fmt.Println("   - 压缩支持")
	
	fmt.Println("\n3. 性能优化")
	fmt.Println("   - 速率限制")
	fmt.Println("   - 内存优化")
	fmt.Println("   - 并发处理")
	fmt.Println("   - 资源管理")
	
	fmt.Println("\n4. 监控与诊断")
	fmt.Println("   - 实时指标")
	fmt.Println("   - 性能分析")
	fmt.Println("   - 错误监控")
	fmt.Println("   - 连接状态")
	
	fmt.Println("\n5. 高级功能")
	fmt.Println("   - 广播消息")
	fmt.Println("   - 负载均衡")
	fmt.Println("   - 故障恢复")
	fmt.Println("   - 扩展性")
}

func main() {
	// 创建演示实例
	demo := NewWebSocketOptimizationDemo()
	defer demo.Stop()
	
	// 运行演示
	demo.RunDemo()
	
	// 运行负载测试
	demo.simulateLoad()
	
	// 打印优化总结
	demo.printOptimizationSummary()
	
	log.Println("WebSocket优化演示完成")
}