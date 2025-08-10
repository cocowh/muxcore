package main

import (
	"context"
	"fmt"
	"time"

	"github.com/cocowh/muxcore/core/observability"
	"github.com/cocowh/muxcore/pkg/logger"
)

func main() {
	logger.Info("=== 分布式追踪系统演示 ===")

	// 1. 创建分布式追踪配置
	config := &observability.TracingConfig{
		ServiceName:     "muxcore-demo",
		ServiceVersion:  "1.0.0",
		Environment:     "development",
		TracingEnabled:  true,
		ExportEndpoint:  "http://localhost:14268/api/traces",
		SamplingRatio:   1.0,
		MetricsEnabled:  true,
		MetricsInterval: time.Second * 10,
		MetricsEndpoint: ":9090",
		LoggingEnabled:  true,
		LogLevel:        "info",
		ResourceAttributes: map[string]string{
			"component": "muxcore",
			"version":   "1.0.0",
			"env":       "demo",
		},
	}

	// 2. 创建分布式追踪组件
	dt, err := observability.NewDistributedTracing(config)
	if err != nil {
		logger.Errorf("Failed to create distributed tracing: %v", err)
		return
	}

	// 3. 添加控制台导出器
	consoleExporter := observability.NewConsoleExporter("console", time.Second*5)
	dt.AddExporter(consoleExporter)

	// 4. 启动分布式追踪
	if err := dt.Start(); err != nil {
		logger.Errorf("Failed to start distributed tracing: %v", err)
		return
	}
	defer dt.Stop()

	logger.Info("分布式追踪系统已启动")

	// 5. 演示分布式追踪功能
	demonstrateDistributedTracing(dt)

	// 6. 演示指标收集
	demonstrateMetrics(dt)

	// 7. 显示追踪和指标统计
	displayTracingStats(dt)

	// 8. 导出追踪数据
	exportTraces(dt, consoleExporter)

	logger.Info("=== 分布式追踪系统演示完成 ===")
}

// demonstrateDistributedTracing 演示分布式追踪功能
func demonstrateDistributedTracing(dt *observability.DistributedTracing) {
	logger.Info("\n--- 演示分布式追踪功能 ---")

	ctx := context.Background()

	// 创建根span
	rootSpan, ctx := dt.StartSpan(ctx, "http_request")
	dt.SetSpanAttributes(rootSpan, map[string]string{
		"http.method": "GET",
		"http.url":    "/api/users",
		"user.id":     "12345",
	})

	// 模拟处理时间
	time.Sleep(50 * time.Millisecond)

	// 创建子span - 数据库查询
	dbSpan, dbCtx := dt.StartSpan(ctx, "database_query", rootSpan.SpanID)
	dt.SetSpanAttributes(dbSpan, map[string]string{
		"db.system":    "postgresql",
		"db.operation": "SELECT",
		"db.table":     "users",
	})

	// 添加span事件
	dt.AddSpanEvent(dbSpan, "query_start", map[string]string{
		"query": "SELECT * FROM users WHERE id = $1",
	})

	time.Sleep(30 * time.Millisecond)

	dt.AddSpanEvent(dbSpan, "query_complete", map[string]string{
		"rows_affected": "1",
	})

	// 结束数据库span
	dt.EndSpan(dbSpan)

	// 创建另一个子span - 缓存操作
	cacheSpan, _ := dt.StartSpan(dbCtx, "cache_operation", rootSpan.SpanID)
	dt.SetSpanAttributes(cacheSpan, map[string]string{
		"cache.system": "redis",
		"cache.key":    "user:12345",
		"cache.hit":    "false",
	})

	time.Sleep(10 * time.Millisecond)

	// 模拟缓存错误
	if time.Now().UnixNano()%2 == 0 {
		dt.RecordError(cacheSpan, fmt.Errorf("cache connection timeout"))
	}

	dt.EndSpan(cacheSpan)

	// 创建外部API调用span
	apiSpan, _ := dt.StartSpan(ctx, "external_api_call", rootSpan.SpanID)
	dt.SetSpanAttributes(apiSpan, map[string]string{
		"http.method":      "POST",
		"http.url":         "https://api.external.com/notify",
		"http.status_code": "200",
	})

	time.Sleep(100 * time.Millisecond)
	dt.EndSpan(apiSpan)

	// 结束根span
	dt.EndSpan(rootSpan)

	logger.Info("分布式追踪演示完成")
}

// demonstrateMetrics 演示指标收集
func demonstrateMetrics(dt *observability.DistributedTracing) {
	logger.Info("\n--- 演示指标收集功能 ---")

	// 创建和使用计数器
	dt.IncrementCounter("http_requests_total", 1, map[string]string{
		"method": "GET",
		"status": "200",
	})

	dt.IncrementCounter("http_requests_total", 1, map[string]string{
		"method": "POST",
		"status": "201",
	})

	dt.IncrementCounter("database_queries_total", 5, map[string]string{
		"operation": "SELECT",
		"table":     "users",
	})

	// 记录直方图数据
	dt.RecordHistogram("http_request_duration_seconds", 0.125, map[string]string{
		"method":   "GET",
		"endpoint": "/api/users",
	})

	dt.RecordHistogram("http_request_duration_seconds", 0.089, map[string]string{
		"method":   "POST",
		"endpoint": "/api/users",
	})

	dt.RecordHistogram("database_query_duration_seconds", 0.045, map[string]string{
		"operation": "SELECT",
		"table":     "users",
	})

	// 设置仪表值
	dt.SetGauge("active_connections", 25, map[string]string{
		"pool": "database",
	})

	dt.SetGauge("memory_usage_bytes", 1024*1024*128, map[string]string{
		"component": "cache",
	})

	dt.SetGauge("goroutines_count", 42, map[string]string{
		"service": "muxcore",
	})

	logger.Info("指标收集演示完成")
}

// displayTracingStats 显示追踪统计信息
func displayTracingStats(dt *observability.DistributedTracing) {
	logger.Info("\n--- 追踪和指标统计 ---")

	// 显示活跃span
	activeSpans := dt.GetActiveSpans()
	logger.Info(fmt.Sprintf("活跃Span数量: %d", len(activeSpans)))

	// 显示所有追踪
	traces := dt.GetTraces()
	logger.Info(fmt.Sprintf("总追踪数量: %d", len(traces)))

	for i, trace := range traces {
		if i >= 3 { // 只显示前3个追踪
			break
		}
		logger.Info(fmt.Sprintf("  追踪 %d: ID=%s, Spans=%d, Duration=%v",
			i+1, trace.TraceID[:8], len(trace.Spans), trace.Duration))
	}

	// 显示指标
	metrics := dt.GetMetrics()
	logger.Info(fmt.Sprintf("总指标数量: %d", len(metrics)))

	for name, metric := range metrics {
		if metricMap, ok := metric.(map[string]interface{}); ok {
			logger.Info(fmt.Sprintf("  指标: %s, 类型: %s", name, metricMap["type"]))
		}
	}
}

// exportTraces 导出追踪数据
func exportTraces(dt *observability.DistributedTracing, exporter *observability.ConsoleExporter) {
	logger.Info("\n--- 导出追踪数据 ---")

	// 启动导出器
	exporter.Start()
	defer exporter.Shutdown()

	// 获取所有追踪并导出
	traces := dt.GetTraces()
	if len(traces) > 0 {
		if err := exporter.Export(traces); err != nil {
			logger.Errorf("Failed to export traces: %v", err)
		} else {
			logger.Info("追踪数据导出完成")
		}
	} else {
		logger.Info("没有追踪数据需要导出")
	}
}
