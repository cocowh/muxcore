package main

import (
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/cocowh/muxcore/core/router"
)

func main() {
	fmt.Println("=== Router Module Optimization Demo ===")

	// 1. 创建统一路由配置
	config := &router.UnifiedRouterConfig{
		Type:        router.HTTPRouterType,
		Name:        "demo-router",
		Enabled:     true,
		Description: "演示路由器优化功能",
		HTTPConfig: &router.HTTPRouterConfig{
			CaseSensitive:     false,
			StrictSlash:       true,
			UseEncodedPath:    false,
			SkipClean:         false,
			MaxRoutes:         1000,
			TimeoutDuration:   time.Second * 30,
			EnableCompression: true,
			EnableCORS:        true,
			CORSOrigins:       []string{"*"},
		},
		MonitoringConfig: &router.MonitoringConfig{
			Enabled:             true,
			MetricsInterval:     time.Second * 30,
			HealthCheckInterval: time.Minute,
			EnableTracing:       true,
			EnableProfiling:     true,
			MaxMetricsHistory:   1000,
			AlertThresholds: &router.AlertThresholds{
				MaxResponseTime: time.Millisecond * 500,
				MaxErrorRate:    0.05, // 5%
				MaxCPUUsage:     0.8,  // 80%
				MaxMemoryUsage:  0.8,  // 80%
				MinSuccessRate:  0.95, // 95%
			},
		},
		CacheConfig: &router.CacheConfig{
			Enabled:            true,
			Type:               "memory",
			TTL:                time.Hour,
			MaxSize:            10000,
			EvictionPolicy:     "lru",
			CompressionEnabled: false,
		},
		PerformanceConfig: &router.PerformanceConfig{
			MaxConcurrentRequests: 1000,
			RequestQueueSize:      5000,
			WorkerPoolSize:        50,
			EnableRateLimiting:    true,
			RateLimit:             100, // 100 requests per second
			BurstLimit:            200,
			EnableCircuitBreaker:  true,
			CircuitBreakerConfig: &router.CircuitBreakerConfig{
				FailureThreshold: 5,
				RecoveryTimeout:  time.Second * 30,
				MaxRequests:      100,
				Interval:         time.Second * 10,
			},
		},
	}

	fmt.Println("✓ 统一路由配置创建完成")

	// 2. 创建配置管理器
	configManager := router.NewConfigManager("router_config.json")
	err := configManager.LoadConfig("demo-router", config)
	if err != nil {
		log.Printf("配置加载失败: %v", err)
		return
	}
	fmt.Println("✓ 配置管理器初始化完成")

	// 3. 创建路由监控器
	monitor := router.NewRouterMonitor(config.MonitoringConfig)

	// 添加告警处理器
	monitor.AddAlertHandler(func(alert *router.AlertEvent) {
		fmt.Printf("🚨 告警触发: [%s] %s - %s\n", alert.Severity, alert.Type, alert.Message)
	})

	monitor.Start()
	fmt.Println("✓ 路由监控器启动完成")

	// 4. 创建路由缓存
	routeCache := router.NewRouteCache(config.CacheConfig, monitor)
	routeCache.Start()
	fmt.Println("✓ 路由缓存启动完成")

	// 5. 模拟路由操作
	fmt.Println("\n=== 模拟路由操作 ===")

	// 模拟路由请求
	routes := []struct {
		method string
		path   string
		result string
	}{
		{"GET", "/api/users", "用户列表"},
		{"POST", "/api/users", "创建用户"},
		{"GET", "/api/users/123", "用户详情"},
		{"PUT", "/api/users/123", "更新用户"},
		{"DELETE", "/api/users/123", "删除用户"},
		{"GET", "/api/orders", "订单列表"},
		{"POST", "/api/orders", "创建订单"},
	}

	// 缓存路由结果
	for _, route := range routes {
		routeCache.CacheRoute(route.method, route.path, route.result)
		fmt.Printf("缓存路由: %s %s -> %s\n", route.method, route.path, route.result)
	}

	// 模拟路由请求和监控记录
	for i := 0; i < 100; i++ {
		for _, route := range routes {
			// 检查缓存
			if cachedResult, found := routeCache.GetCachedRoute(route.method, route.path); found {
				// 缓存命中
				responseTime := time.Millisecond * time.Duration(50+i%100) // 模拟响应时间
				monitor.RecordRequest(route.method, route.path, responseTime, true)
				if i < 5 { // 只打印前几次
					fmt.Printf("缓存命中: %s %s -> %v (响应时间: %v)\n",
						route.method, route.path, cachedResult, responseTime)
				}
			} else {
				// 缓存未命中
				responseTime := time.Millisecond * time.Duration(200+i%300) // 模拟较长响应时间
				monitor.RecordRequest(route.method, route.path, responseTime, true)
				if i < 5 {
					fmt.Printf("缓存未命中: %s %s (响应时间: %v)\n",
						route.method, route.path, responseTime)
				}
			}
		}

		// 模拟一些错误请求
		if i%20 == 0 {
			monitor.RecordRequest("GET", "/api/error", time.Millisecond*1000, false)
		}

		// 模拟超时
		if i%30 == 0 {
			monitor.RecordTimeout()
		}
	}

	fmt.Println("\n=== 监控指标展示 ===")

	// 等待一段时间让监控收集数据
	time.Sleep(time.Second * 2)

	// 6. 显示监控指标
	metrics := monitor.GetMetrics()
	fmt.Printf("总请求数: %d\n", metrics.TotalRequests)
	fmt.Printf("成功请求数: %d\n", metrics.SuccessfulRequests)
	fmt.Printf("失败请求数: %d\n", metrics.FailedRequests)
	fmt.Printf("活跃请求数: %d\n", metrics.ActiveRequests)
	fmt.Printf("错误率: %.2f%%\n", metrics.ErrorRate*100)
	fmt.Printf("平均响应时间: %v\n", metrics.AverageResponseTime)
	fmt.Printf("最小响应时间: %v\n", metrics.MinResponseTime)
	fmt.Printf("最大响应时间: %v\n", metrics.MaxResponseTime)
	fmt.Printf("超时次数: %d\n", metrics.TimeoutCount)
	fmt.Printf("缓存命中数: %d\n", metrics.CacheHits)
	fmt.Printf("缓存未命中数: %d\n", metrics.CacheMisses)
	fmt.Printf("缓存命中率: %.2f%%\n", metrics.CacheHitRate*100)
	fmt.Printf("CPU使用率: %.2f%%\n", metrics.CPUUsage*100)
	fmt.Printf("内存使用率: %.2f%%\n", metrics.MemoryUsage*100)
	fmt.Printf("Goroutine数量: %d\n", metrics.GoroutineCount)

	// 7. 显示路由级指标
	fmt.Println("\n=== 路由级指标 ===")
	routeMetrics := monitor.GetRouteMetrics()
	for key, rm := range routeMetrics {
		fmt.Printf("%s: 请求数=%d, 成功数=%d, 错误数=%d, 平均响应时间=%v\n",
			key, rm.RequestCount, rm.SuccessCount, rm.ErrorCount, rm.AverageResponseTime)
	}

	// 8. 显示缓存统计
	fmt.Println("\n=== 缓存统计 ===")
	cacheStats := routeCache.GetStats()
	fmt.Printf("缓存命中数: %d\n", cacheStats.Hits)
	fmt.Printf("缓存未命中数: %d\n", cacheStats.Misses)
	fmt.Printf("缓存淘汰数: %d\n", cacheStats.Evictions)
	fmt.Printf("缓存条目数: %d\n", cacheStats.EntryCount)
	fmt.Printf("缓存命中率: %.2f%%\n", cacheStats.HitRate*100)
	fmt.Printf("内存使用量: %d bytes\n", cacheStats.MemoryUsage)

	// 9. 测试配置管理功能
	fmt.Println("\n=== 配置管理功能测试 ===")

	// 列出所有配置
	configNames := configManager.ListConfigs()
	fmt.Printf("当前配置列表: %v\n", configNames)

	// 获取配置
	if retrievedConfig, exists := configManager.GetConfig("demo-router"); exists {
		fmt.Printf("获取配置成功: %s (类型: %s)\n", retrievedConfig.Name, retrievedConfig.Type)
	}

	// 导出配置
	if configData, err := configManager.ExportConfig("demo-router"); err == nil {
		fmt.Printf("配置导出成功，大小: %d bytes\n", len(configData))
	}

	// 10. 测试缓存失效功能
	fmt.Println("\n=== 缓存失效测试 ===")

	// 使特定路由缓存失效
	routeCache.InvalidateRoute("GET", "/api/users")
	fmt.Println("已使 GET /api/users 缓存失效")

	// 使用模式匹配使缓存失效
	routeCache.InvalidatePattern("*users*")
	fmt.Println("已使所有包含 'users' 的路由缓存失效")

	// 11. 模拟高负载情况
	fmt.Println("\n=== 高负载模拟 ===")

	// 模拟大量并发请求
	for i := 0; i < 1000; i++ {
		go func(id int) {
			method := "GET"
			path := fmt.Sprintf("/api/load-test/%d", id%10)
			responseTime := time.Millisecond * time.Duration(100+id%500)
			success := id%50 != 0 // 2%的错误率

			monitor.RecordActiveRequest(1)
			monitor.RecordRequest(method, path, responseTime, success)
			monitor.RecordActiveRequest(-1)

			if !success {
				monitor.RecordTimeout()
			}
		}(i)
	}

	// 等待处理完成
	time.Sleep(time.Second * 3)

	// 显示最终指标
	fmt.Println("\n=== 最终监控指标 ===")
	finalMetrics := monitor.GetMetrics()
	fmt.Printf("总请求数: %d\n", finalMetrics.TotalRequests)
	fmt.Printf("成功请求数: %d\n", finalMetrics.SuccessfulRequests)
	fmt.Printf("失败请求数: %d\n", finalMetrics.FailedRequests)
	fmt.Printf("错误率: %.2f%%\n", finalMetrics.ErrorRate*100)
	fmt.Printf("平均响应时间: %v\n", finalMetrics.AverageResponseTime)
	fmt.Printf("缓存命中率: %.2f%%\n", finalMetrics.CacheHitRate*100)

	// 检查告警
	alerts := monitor.GetAlerts()
	if len(alerts) > 0 {
		fmt.Printf("\n=== 触发的告警 (%d个) ===\n", len(alerts))
		for _, alert := range alerts {
			fmt.Printf("[%s] %s: %s (时间: %v)\n",
				alert.Severity, alert.Type, alert.Message, alert.Timestamp.Format("15:04:05"))
		}
	} else {
		fmt.Println("\n✓ 没有触发告警")
	}

	// 12. 清理资源
	fmt.Println("\n=== 清理资源 ===")
	routeCache.Stop()
	monitor.Stop()
	fmt.Println("✓ 所有组件已停止")

	fmt.Println("\n=== Router模块优化演示完成 ===")
	fmt.Println("主要功能:")
	fmt.Println("- ✓ 统一配置管理")
	fmt.Println("- ✓ 实时监控和指标收集")
	fmt.Println("- ✓ 智能缓存系统")
	fmt.Println("- ✓ 告警机制")
	fmt.Println("- ✓ 性能优化")
	fmt.Println("- ✓ 配置热更新支持")
	fmt.Println("- ✓ 多种淘汰策略")
	fmt.Println("- ✓ 路由级监控")
}

// 模拟HTTP处理器
func simulateHTTPHandler(w http.ResponseWriter, r *http.Request) {
	// 模拟处理逻辑
	time.Sleep(time.Millisecond * 50)
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}
