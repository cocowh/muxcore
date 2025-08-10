package main

import (
	"fmt"
	"math/rand"
	"net"
	"runtime"
	"sync"
	"time"

	"github.com/cocowh/muxcore/core/handlers"
	"github.com/cocowh/muxcore/core/performance"
	"github.com/cocowh/muxcore/pkg/logger"
)

// MockProtocolHandler 模拟协议处理器
type MockProtocolHandler struct {
	protocol string
	latency  time.Duration
}

// Handle 处理连接
func (mph *MockProtocolHandler) Handle(connID string, conn net.Conn, initialData []byte) {
	// 模拟协议处理延迟
	time.Sleep(mph.latency)
	logger.Debugf("[%s] Processed connection %s with %d bytes", mph.protocol, connID, len(initialData))
}

// MockConnection 模拟连接
type MockConnection struct {
	data []byte
}

func (mc *MockConnection) Read(b []byte) (n int, err error) {
	n = copy(b, mc.data)
	return n, nil
}

func (mc *MockConnection) Write(b []byte) (n int, err error) {
	return len(b), nil
}

func (mc *MockConnection) Close() error {
	return nil
}

func (mc *MockConnection) LocalAddr() net.Addr {
	return &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 8080}
}

func (mc *MockConnection) RemoteAddr() net.Addr {
	return &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: rand.Intn(65535)}
}

func (mc *MockConnection) SetDeadline(t time.Time) error {
	return nil
}

func (mc *MockConnection) SetReadDeadline(t time.Time) error {
	return nil
}

func (mc *MockConnection) SetWriteDeadline(t time.Time) error {
	return nil
}

func main() {
	logger.Info("=== Protocol Processor Optimization Demo ===")

	// 1. 初始化基础组件
	bufferPool := performance.NewBufferPool()
	memoryManager := performance.NewMemoryManager(50, true)
	cpuManager := performance.NewCPUAffinityManager()
	resourceManager := performance.NewResourceManager(runtime.NumCPU(), 4096, 500)
	performanceMonitor := performance.NewPerformanceMonitor(memoryManager, cpuManager, resourceManager, bufferPool)

	// 2. 创建处理器管理器
	processorManager := handlers.NewProcessorManager()

	// 3. 注册模拟协议处理器
	registerMockHandlers(processorManager)

	// 4. 创建优化的协议处理器配置
	config := &handlers.ProcessorConfig{
		MaxConcurrency:    runtime.NumCPU() * 4,
		QueueSize:         5000,
		Timeout:           10 * time.Second,
		EnablePipelining:  true,
		EnableBatching:    true,
		BatchSize:         50,
		BatchTimeout:      5 * time.Millisecond,
		EnableCompression: false,
		EnableCaching:     true,
		CacheSize:         1000,
		CacheTTL:          2 * time.Minute,
	}

	// 5. 创建优化的协议处理器
	optimizedProcessor := handlers.NewOptimizedProtocolProcessor(config, processorManager, performanceMonitor, bufferPool)

	// 6. 启动组件
	performanceMonitor.Start()
	optimizedProcessor.Start()
	defer func() {
		optimizedProcessor.Stop()
		performanceMonitor.Stop()
	}()

	// 7. 显示初始配置
	displayConfiguration(config)

	// 8. 模拟协议处理负载
	logger.Info("\n=== Starting Protocol Processing Simulation ===")
	simulateProtocolProcessing(optimizedProcessor)

	// 9. 显示处理器指标
	displayProcessorMetrics(optimizedProcessor)

	// 10. 测试批处理功能
	testBatchProcessing(optimizedProcessor)

	// 11. 测试缓存功能
	testCaching(optimizedProcessor)

	// 12. 性能压力测试
	logger.Info("\n=== Performance Stress Test ===")
	stressTestProcessor(optimizedProcessor)

	// 13. 显示缓存统计
	displayCacheStats(optimizedProcessor)

	// 14. 配置动态调整测试
	testConfigurationUpdate(optimizedProcessor)

	// 等待一段时间收集更多指标
	time.Sleep(5 * time.Second)

	// 15. 最终性能报告
	finalPerformanceReport(optimizedProcessor)

	logger.Info("\n=== Protocol Processor Optimization Demo Completed ===")
}

// registerMockHandlers 注册模拟处理器
func registerMockHandlers(pm *handlers.ProcessorManager) {
	logger.Info("\n=== Registering Mock Protocol Handlers ===")

	// HTTP处理器
	httpHandler := &MockProtocolHandler{
		protocol: "http",
		latency:  1 * time.Millisecond,
	}
	pm.AddProcessor("http", httpHandler)
	logger.Info("Registered HTTP handler")

	// WebSocket处理器
	websocketHandler := &MockProtocolHandler{
		protocol: "websocket",
		latency:  2 * time.Millisecond,
	}
	pm.AddProcessor("websocket", websocketHandler)
	logger.Info("Registered WebSocket handler")

	// gRPC处理器
	grpcHandler := &MockProtocolHandler{
		protocol: "grpc",
		latency:  3 * time.Millisecond,
	}
	pm.AddProcessor("grpc", grpcHandler)
	logger.Info("Registered gRPC handler")

	// 二进制协议处理器
	binaryHandler := &MockProtocolHandler{
		protocol: "binary",
		latency:  500 * time.Microsecond,
	}
	pm.AddProcessor("binary", binaryHandler)
	logger.Info("Registered Binary handler")

	// 流式协议处理器
	streamingHandler := &MockProtocolHandler{
		protocol: "streaming",
		latency:  4 * time.Millisecond,
	}
	pm.AddProcessor("streaming", streamingHandler)
	logger.Info("Registered Streaming handler")
}

// displayConfiguration 显示配置
func displayConfiguration(config *handlers.ProcessorConfig) {
	logger.Info("\n=== Processor Configuration ===")
	logger.Infof("Max Concurrency: %d", config.MaxConcurrency)
	logger.Infof("Queue Size: %d", config.QueueSize)
	logger.Infof("Timeout: %v", config.Timeout)
	logger.Infof("Enable Pipelining: %t", config.EnablePipelining)
	logger.Infof("Enable Batching: %t", config.EnableBatching)
	logger.Infof("Batch Size: %d", config.BatchSize)
	logger.Infof("Batch Timeout: %v", config.BatchTimeout)
	logger.Infof("Enable Caching: %t", config.EnableCaching)
	logger.Infof("Cache Size: %d", config.CacheSize)
	logger.Infof("Cache TTL: %v", config.CacheTTL)
}

// simulateProtocolProcessing 模拟协议处理
func simulateProtocolProcessing(processor *handlers.OptimizedProtocolProcessor) {
	protocols := []string{"http", "websocket", "grpc", "binary", "streaming"}
	var wg sync.WaitGroup

	// 模拟不同协议的请求
	for i := 0; i < 1000; i++ {
		wg.Add(1)
		go func(requestID int) {
			defer wg.Done()

			// 随机选择协议
			protocol := protocols[rand.Intn(len(protocols))]

			// 生成模拟数据
			data := generateMockData(protocol)

			// 创建模拟连接
			conn := &MockConnection{data: data}

			// 处理连接
			connID := fmt.Sprintf("conn-%d", requestID)
			processor.ProcessConnection(connID, conn, protocol, data)
		}(i)

		// 控制请求速率
		if i%100 == 0 {
			time.Sleep(10 * time.Millisecond)
		}
	}

	wg.Wait()
	logger.Info("Protocol processing simulation completed")
}

// generateMockData 生成模拟数据
func generateMockData(protocol string) []byte {
	switch protocol {
	case "http":
		return []byte("GET /api/test HTTP/1.1\r\nHost: example.com\r\n\r\n")
	case "websocket":
		return []byte("GET /ws HTTP/1.1\r\nUpgrade: websocket\r\nConnection: Upgrade\r\n\r\n")
	case "grpc":
		return []byte{0x00, 0x00, 0x00, 0x05, 0x00, 0x48, 0x65, 0x6c, 0x6c, 0x6f} // gRPC frame
	case "binary":
		return []byte{0xFF, 0xFE, 0xFD, 0xFC, 0x01, 0x02, 0x03, 0x04}
	case "streaming":
		return []byte("STREAM_DATA_CHUNK_001")
	default:
		return []byte("UNKNOWN_PROTOCOL_DATA")
	}
}

// displayProcessorMetrics 显示处理器指标
func displayProcessorMetrics(processor *handlers.OptimizedProtocolProcessor) {
	logger.Info("\n=== Processor Metrics ===")
	metrics := processor.GetMetrics()
	if metrics != nil {
		logger.Infof("Processed Requests: %d", metrics.ProcessedRequests)
		logger.Infof("Average Latency: %.2f ms", float64(metrics.AverageLatency)/1000000)
		logger.Infof("Throughput: %.2f RPS", metrics.Throughput)
		logger.Infof("Error Rate: %.2f%%", metrics.ErrorRate*100)
		logger.Infof("CPU Usage: %.2f%%", metrics.CPUUsage)
		logger.Infof("Memory Usage: %.2f MB", float64(metrics.MemoryUsage)/1024/1024)
		logger.Infof("Active Connections: %d", metrics.ActiveConnections)
		logger.Infof("Queue Depth: %d", metrics.QueueDepth)
		logger.Infof("Cache Hit Rate: %.2f%%", metrics.CacheHitRate*100)
		logger.Infof("Protocol Parse Time: %.2f μs", float64(metrics.ProtocolParseTime)/1000)
	}
}

// testBatchProcessing 测试批处理
func testBatchProcessing(processor *handlers.OptimizedProtocolProcessor) {
	logger.Info("\n=== Testing Batch Processing ===")

	var wg sync.WaitGroup
	batchSize := 200

	// 快速提交大量任务以触发批处理
	for i := 0; i < batchSize; i++ {
		wg.Add(1)
		go func(taskID int) {
			defer wg.Done()

			data := []byte(fmt.Sprintf("batch_task_%d", taskID))
			conn := &MockConnection{data: data}
			connID := fmt.Sprintf("batch-conn-%d", taskID)

			processor.ProcessConnection(connID, conn, "http", data)
		}(i)
	}

	wg.Wait()
	logger.Infof("Batch processing test completed with %d tasks", batchSize)
}

// testCaching 测试缓存
func testCaching(processor *handlers.OptimizedProtocolProcessor) {
	logger.Info("\n=== Testing Caching Functionality ===")

	// 重复发送相同的数据以测试缓存
	sameData := []byte("GET /cached/endpoint HTTP/1.1\r\nHost: cache-test.com\r\n\r\n")

	for i := 0; i < 50; i++ {
		conn := &MockConnection{data: sameData}
		connID := fmt.Sprintf("cache-test-%d", i)
		processor.ProcessConnection(connID, conn, "http", sameData)
	}

	logger.Info("Cache testing completed")
}

// stressTestProcessor 压力测试处理器
func stressTestProcessor(processor *handlers.OptimizedProtocolProcessor) {
	var wg sync.WaitGroup
	stressLoad := 2000
	protocols := []string{"http", "websocket", "grpc", "binary", "streaming"}

	start := time.Now()

	for i := 0; i < stressLoad; i++ {
		wg.Add(1)
		go func(requestID int) {
			defer wg.Done()

			protocol := protocols[rand.Intn(len(protocols))]
			data := generateMockData(protocol)
			conn := &MockConnection{data: data}
			connID := fmt.Sprintf("stress-%d", requestID)

			processor.ProcessConnection(connID, conn, protocol, data)
		}(i)
	}

	wg.Wait()
	duration := time.Since(start)

	logger.Infof("Stress test completed: %d requests in %v", stressLoad, duration)
	logger.Infof("Average RPS: %.2f", float64(stressLoad)/duration.Seconds())
}

// displayCacheStats 显示缓存统计
func displayCacheStats(processor *handlers.OptimizedProtocolProcessor) {
	logger.Info("\n=== Cache Statistics ===")
	stats := processor.GetCacheStats()
	for key, value := range stats {
		logger.Infof("%s: %v", key, value)
	}
}

// testConfigurationUpdate 测试配置更新
func testConfigurationUpdate(processor *handlers.OptimizedProtocolProcessor) {
	logger.Info("\n=== Testing Configuration Update ===")

	// 创建新配置
	newConfig := &handlers.ProcessorConfig{
		MaxConcurrency:    runtime.NumCPU() * 6,
		QueueSize:         8000,
		Timeout:           15 * time.Second,
		EnablePipelining:  true,
		EnableBatching:    true,
		BatchSize:         75,
		BatchTimeout:      3 * time.Millisecond,
		EnableCompression: true,
		EnableCaching:     true,
		CacheSize:         2000,
		CacheTTL:          5 * time.Minute,
	}

	// 更新配置
	processor.UpdateConfig(newConfig)
	logger.Info("Configuration updated successfully")

	// 测试新配置下的处理
	for i := 0; i < 100; i++ {
		data := generateMockData("http")
		conn := &MockConnection{data: data}
		connID := fmt.Sprintf("config-test-%d", i)
		processor.ProcessConnection(connID, conn, "http", data)
	}

	logger.Info("Configuration update test completed")
}

// finalPerformanceReport 最终性能报告
func finalPerformanceReport(processor *handlers.OptimizedProtocolProcessor) {
	logger.Info("\n=== Final Performance Report ===")

	// 显示最终指标
	displayProcessorMetrics(processor)

	// 显示缓存效果
	cacheStats := processor.GetCacheStats()
	cacheHitRate := cacheStats["cache_hit_rate"].(float64)
	logger.Infof("Final Cache Hit Rate: %.2f%%", cacheHitRate*100)

	// 系统资源使用情况
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)
	logger.Infof("Final System Memory: %.2f MB", float64(memStats.Alloc)/1024/1024)
	logger.Infof("Final Goroutine Count: %d", runtime.NumGoroutine())
	logger.Infof("GC Cycles: %d", memStats.NumGC)

	// 性能优化效果总结
	metrics := processor.GetMetrics()
	if metrics != nil {
		logger.Info("\n=== Optimization Summary ===")
		logger.Infof("Total Requests Processed: %d", metrics.ProcessedRequests)
		logger.Infof("Average Processing Latency: %.2f ms", float64(metrics.AverageLatency)/1000000)
		logger.Infof("Peak Throughput: %.2f RPS", metrics.Throughput)
		logger.Infof("Error Rate: %.4f%%", metrics.ErrorRate*100)
		logger.Infof("Cache Efficiency: %.2f%%", cacheHitRate*100)
		logger.Infof("Protocol Parse Efficiency: %.2f μs/request", float64(metrics.ProtocolParseTime)/1000)
	}
}