package observability

import (
	"expvar"
	"net/http"
	"runtime"
	"time"

	"github.com/cocowh/muxcore/pkg/logger"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// 定义指标
var (
	// 连接相关指标
	connectionsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "muxcore_connections_total",
			Help: "Total number of connections",
		},
		[]string{"protocol", "status"},
	)

	activeConnections = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "muxcore_active_connections",
			Help: "Number of active connections",
		},
		[]string{"protocol"},
	)

	// 请求相关指标
	requestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "muxcore_requests_total",
			Help: "Total number of requests",
		},
		[]string{"protocol", "path", "status"},
	)

	requestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "muxcore_request_duration_seconds",
			Help:    "Request duration in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"protocol", "path", "status"},
	)

	// 内存相关指标
	memoryAllocated = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "muxcore_memory_allocated_bytes",
			Help: "Total memory allocated",
		},
	)

	// Goroutine相关指标
	goroutinesCount = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "muxcore_goroutines_count",
			Help: "Number of goroutines",
		},
	)
)

// InitMetrics 初始化指标
func InitMetrics(metricsAddr string) {
	// 注册Prometheus指标
	prometheus.MustRegister(connectionsTotal)
	prometheus.MustRegister(activeConnections)
	prometheus.MustRegister(requestsTotal)
	prometheus.MustRegister(requestDuration)
	prometheus.MustRegister(memoryAllocated)
	prometheus.MustRegister(goroutinesCount)

	// 注册expvar指标
	expvar.Publish("goroutines", expvar.Func(func() interface{} {
		return runtime.NumGoroutine()
	}))

	expvar.Publish("memory", expvar.Func(func() interface{} {
		var m runtime.MemStats
		runtime.ReadMemStats(&m)
		return m.Alloc
	}))

	// 启动指标HTTP服务
	http.Handle("/metrics", promhttp.Handler())
	go func() {
		err := http.ListenAndServe(metricsAddr, nil)
		if err != nil {
			logger.Errorf("Failed to start metrics server: %v", err)
		} else {
			logger.Infof("Metrics server started on %s", metricsAddr)
		}
	}()

	// 启动定期收集器
	go startPeriodicCollector()
}

// startPeriodicCollector 启动定期指标收集
func startPeriodicCollector() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		// 收集内存指标
		var m runtime.MemStats
		runtime.ReadMemStats(&m)
		memoryAllocated.Set(float64(m.Alloc))

		// 收集goroutine指标
		goroutinesCount.Set(float64(runtime.NumGoroutine()))
	}
}

// RecordConnection 记录连接
func RecordConnection(protocol string, status string) {
	connectionsTotal.WithLabelValues(protocol, status).Inc()
}

// SetActiveConnections 设置活跃连接数
func SetActiveConnections(protocol string, count int) {
	activeConnections.WithLabelValues(protocol).Set(float64(count))
}

// RecordRequest 记录请求
func RecordRequest(protocol string, path string, status string, duration float64) {
	requestsTotal.WithLabelValues(protocol, path, status).Inc()
	requestDuration.WithLabelValues(protocol, path, status).Observe(duration)
}
