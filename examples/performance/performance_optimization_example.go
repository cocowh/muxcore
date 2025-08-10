package main

import (
	"math/rand"
	"runtime"
	"sync"
	"time"

	"github.com/cocowh/muxcore/core/performance"
	"github.com/cocowh/muxcore/pkg/logger"
)

func main() {
	logger.Info("=== Performance Optimization Demo ===")

	// 1. 初始化性能组件
	memoryManager := performance.NewMemoryManager(100, true) // 预分配100个内存页，启用NUMA感知
	cpuManager := performance.NewCPUAffinityManager()
	resourceManager := performance.NewResourceManager(runtime.NumCPU(), 8192, 1000) // CPU核心数, 8GB内存, 1GB/s IO
	bufferPool := performance.NewBufferPool()

	// 2. 创建性能监控器
	performanceMonitor := performance.NewPerformanceMonitor(memoryManager, cpuManager, resourceManager, bufferPool)

	// 3. 启动性能监控
	performanceMonitor.Start()
	defer performanceMonitor.Stop()

	// 4. 展示初始状态
	displaySystemInfo()
	displayResourceQuotas(resourceManager)

	// 5. 模拟高负载场景
	logger.Info("\n=== Starting High Load Simulation ===")
	simulateHighLoad(performanceMonitor, bufferPool, memoryManager)

	// 6. 展示性能指标
	displayPerformanceMetrics(performanceMonitor)

	// 7. 测试CPU亲和性
	testCPUAffinity(cpuManager)

	// 8. 测试内存管理
	testMemoryManagement(memoryManager)

	// 9. 测试资源配额管理
	testResourceQuotaManagement(resourceManager)

	// 10. 展示缓冲池统计
	displayBufferPoolStats(bufferPool)

	// 11. 展示调优动作历史
	displayTuningActions(performanceMonitor)

	// 12. 性能压力测试
	logger.Info("\n=== Performance Stress Test ===")
	stressTest(performanceMonitor, bufferPool)

	// 等待一段时间让监控器收集更多数据
	time.Sleep(10 * time.Second)

	// 最终性能报告
	finalPerformanceReport(performanceMonitor)

	logger.Info("\n=== Performance Optimization Demo Completed ===")
}

// displaySystemInfo 显示系统信息
func displaySystemInfo() {
	logger.Info("\n=== System Information ===")
	logger.Infof("CPU Cores: %d", runtime.NumCPU())
	logger.Infof("GOMAXPROCS: %d", runtime.GOMAXPROCS(0))
	logger.Infof("Goroutines: %d", runtime.NumGoroutine())

	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)
	logger.Infof("Memory Allocated: %.2f MB", float64(memStats.Alloc)/1024/1024)
	logger.Infof("Total Allocations: %.2f MB", float64(memStats.TotalAlloc)/1024/1024)
	logger.Infof("GC Cycles: %d", memStats.NumGC)
}

// displayResourceQuotas 显示资源配额
func displayResourceQuotas(rm *performance.ResourceManager) {
	logger.Info("\n=== Resource Quotas ===")
	resourceTypes := []performance.ResourceType{
		performance.ResourceTypeHTTP,
		performance.ResourceTypeStreaming,
		performance.ResourceTypeSystem,
		performance.ResourceTypeManagement,
	}

	for _, rt := range resourceTypes {
		quota, exists := rm.GetQuota(rt)
		if exists {
			logger.Infof("%s: CPU=%d%%, Memory=%d%%, IO=%d%%", rt, quota.CPU, quota.Memory, quota.IO)
			cpu, memory, io := rm.GetAvailableResources(rt)
			logger.Infof("  Available: CPU=%d cores, Memory=%d MB, IO=%d MB/s", cpu, memory, io)
		}
	}
}

// simulateHighLoad 模拟高负载
func simulateHighLoad(pm *performance.PerformanceMonitor, bp *performance.BufferPool, mm *performance.MemoryManager) {
	var wg sync.WaitGroup
	numWorkers := runtime.NumCPU() * 2

	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			for j := 0; j < 1000; j++ {
				start := time.Now()

				// 模拟缓冲区操作
				buf := bp.Get()
				if buf != nil {
					// 模拟缓存命中/未命中
					if rand.Float32() < 0.7 {
						pm.RecordCacheHit()
					} else {
						pm.RecordCacheMiss()
					}
					bp.Put(buf)
				}

				// 模拟内存分配
				if j%100 == 0 {
					memory, err := mm.AllocateAlignedMemory(4096, 64)
					if err == nil {
						// 使用内存
						for k := 0; k < len(memory); k += 64 {
							memory[k] = byte(k % 256)
						}
						// 释放内存
						mm.FreeMemory(memory)
					}
				}

				// 模拟CPU密集型操作
				sum := 0
				for k := 0; k < 10000; k++ {
					sum += k * k
				}

				// 记录请求延迟
				latency := time.Since(start)
				pm.RecordRequest(latency)

				// 随机休眠
				time.Sleep(time.Duration(rand.Intn(10)) * time.Millisecond)
			}
		}(i)
	}

	wg.Wait()
	logger.Info("High load simulation completed")
}

// displayPerformanceMetrics 显示性能指标
func displayPerformanceMetrics(pm *performance.PerformanceMonitor) {
	logger.Info("\n=== Performance Metrics ===")
	metrics := pm.GetMetrics()
	if metrics != nil {
		logger.Infof("CPU Usage: %.2f%%", metrics.CPUUsage)
		logger.Infof("Memory Usage: %.2f MB", float64(metrics.MemoryUsage)/1024/1024)
		logger.Infof("Goroutine Count: %d", metrics.GoroutineCount)
		logger.Infof("GC Pause Time: %.2f ms", float64(metrics.GCPauseTime)/1000000)
		logger.Infof("Cache Hit Rate: %.2f%%", metrics.CacheHitRate*100)
		logger.Infof("Throughput: %d requests", metrics.Throughput)
		logger.Infof("Average Latency: %.2f ms", float64(metrics.Latency)/1000000)
	}
}

// testCPUAffinity 测试CPU亲和性
func testCPUAffinity(cam *performance.CPUAffinityManager) {
	logger.Info("\n=== CPU Affinity Test ===")

	// 分配CPU核心
	coreID, assigned := cam.AssignCore()
	if assigned {
		logger.Infof("Assigned CPU core: %d", coreID)

		// 绑定到核心
		err := cam.BindToCore(coreID)
		if err != nil {
			logger.Errorf("Failed to bind to core %d: %v", coreID, err)
		} else {
			logger.Infof("Successfully bound to CPU core %d", coreID)
		}

		// 释放核心
		cam.ReleaseCore(coreID)
		logger.Infof("Released CPU core: %d", coreID)
	} else {
		logger.Warn("No available CPU cores")
	}
}

// testMemoryManagement 测试内存管理
func testMemoryManagement(mm *performance.MemoryManager) {
	logger.Info("\n=== Memory Management Test ===")

	// 获取共享内存页
	page, exists := mm.GetSharedPage("prealloc_0")
	if exists {
		logger.Infof("Retrieved shared memory page: %d bytes", len(page))
		// 写入数据
		for i := 0; i < min(len(page), 100); i++ {
			page[i] = byte(i % 256)
		}
		logger.Info("Written test data to shared memory page")
	} else {
		logger.Warn("Shared memory page not found")
	}

	// 分配对齐内存
	alignedMem, err := mm.AllocateAlignedMemory(8192, 64)
	if err != nil {
		logger.Errorf("Failed to allocate aligned memory: %v", err)
	} else {
		logger.Infof("Allocated aligned memory: %d bytes", len(alignedMem))
		// 释放内存
		err = mm.FreeMemory(alignedMem)
		if err != nil {
			logger.Errorf("Failed to free memory: %v", err)
		} else {
			logger.Info("Successfully freed aligned memory")
		}
	}
}

// testResourceQuotaManagement 测试资源配额管理
func testResourceQuotaManagement(rm *performance.ResourceManager) {
	logger.Info("\n=== Resource Quota Management Test ===")

	// 尝试修改HTTP资源配额
	newQuota := performance.ResourceQuota{CPU: 50, Memory: 50, IO: 50}
	success := rm.SetQuota(performance.ResourceTypeHTTP, newQuota)
	if success {
		logger.Info("Successfully updated HTTP resource quota")
	} else {
		logger.Warn("Failed to update HTTP resource quota")
	}

	// 获取更新后的配额
	quota, exists := rm.GetQuota(performance.ResourceTypeHTTP)
	if exists {
		logger.Infof("Current HTTP quota: CPU=%d%%, Memory=%d%%, IO=%d%%", quota.CPU, quota.Memory, quota.IO)
	}

	// 测试资源禁用/启用
	rm.Disable()
	logger.Info("Resource management disabled")
	cpu, memory, io := rm.GetAvailableResources(performance.ResourceTypeHTTP)
	logger.Infof("Available resources (disabled): CPU=%d, Memory=%d, IO=%d", cpu, memory, io)

	rm.Enable()
	logger.Info("Resource management enabled")
	cpu, memory, io = rm.GetAvailableResources(performance.ResourceTypeHTTP)
	logger.Infof("Available resources (enabled): CPU=%d, Memory=%d, IO=%d", cpu, memory, io)
}

// displayBufferPoolStats 显示缓冲池统计
func displayBufferPoolStats(bp *performance.BufferPool) {
	logger.Info("\n=== Buffer Pool Statistics ===")
	logger.Infof("Buffer count: %d", bp.Count())
	stats := bp.Stats()
	for key, value := range stats {
		logger.Infof("%s: %v", key, value)
	}
}

// displayTuningActions 显示调优动作
func displayTuningActions(pm *performance.PerformanceMonitor) {
	logger.Info("\n=== Tuning Actions History ===")
	actions := pm.GetTuningActions()
	if len(actions) == 0 {
		logger.Info("No tuning actions performed yet")
		return
	}

	for i, action := range actions {
		logger.Infof("Action %d: %s - %s", i+1, action.Type, action.Description)
		for key, value := range action.Parameters {
			logger.Infof("  %s: %v", key, value)
		}
	}
}

// stressTest 压力测试
func stressTest(pm *performance.PerformanceMonitor, bp *performance.BufferPool) {
	var wg sync.WaitGroup
	stressWorkers := runtime.NumCPU() * 4

	for i := 0; i < stressWorkers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			for j := 0; j < 500; j++ {
				start := time.Now()

				// 大量内存分配
				data := make([][]byte, 100)
				for k := range data {
					data[k] = make([]byte, 1024)
				}

				// 缓冲区操作
				buf := bp.Get()
				if buf != nil {
					bp.Put(buf)
					pm.RecordCacheHit()
				} else {
					pm.RecordCacheMiss()
				}

				// CPU密集型计算
				result := 0
				for k := 0; k < 50000; k++ {
					result += k * k % 1000
				}

				latency := time.Since(start)
				pm.RecordRequest(latency)
			}
		}(i)
	}

	wg.Wait()
	logger.Info("Stress test completed")
}

// finalPerformanceReport 最终性能报告
func finalPerformanceReport(pm *performance.PerformanceMonitor) {
	logger.Info("\n=== Final Performance Report ===")

	// 显示最新性能指标
	displayPerformanceMetrics(pm)

	// 显示调优动作总结
	actions := pm.GetTuningActions()
	logger.Infof("Total tuning actions performed: %d", len(actions))

	// 按类型统计调优动作
	actionCounts := make(map[string]int)
	for _, action := range actions {
		actionCounts[action.Type]++
	}

	logger.Info("Tuning actions by type:")
	for actionType, count := range actionCounts {
		logger.Infof("  %s: %d times", actionType, count)
	}

	// 系统资源使用情况
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)
	logger.Infof("Final memory usage: %.2f MB", float64(memStats.Alloc)/1024/1024)
	logger.Infof("Total GC cycles: %d", memStats.NumGC)
	logger.Infof("Final goroutine count: %d", runtime.NumGoroutine())
}

// min 返回两个整数中的较小值
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}