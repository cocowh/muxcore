package pool

import (
	"context"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cocowh/muxcore/pkg/buffer"
	"github.com/cocowh/muxcore/pkg/logger"
	"github.com/google/uuid"
)

// OptimizedConnectionPoolConfig 优化连接池配置
type OptimizedConnectionPoolConfig struct {
	MaxConnections     int           `json:"max_connections"`
	MinConnections     int           `json:"min_connections"`
	MaxIdleTime        time.Duration `json:"max_idle_time"`
	HealthCheckInterval time.Duration `json:"health_check_interval"`
	ConnectionTimeout  time.Duration `json:"connection_timeout"`
	EnableLoadBalancing bool         `json:"enable_load_balancing"`
	EnableMetrics      bool         `json:"enable_metrics"`
	BufferSize         int          `json:"buffer_size"`
}

// DefaultOptimizedConnectionPoolConfig 默认配置
func DefaultOptimizedConnectionPoolConfig() *OptimizedConnectionPoolConfig {
	return &OptimizedConnectionPoolConfig{
		MaxConnections:     1000,
		MinConnections:     10,
		MaxIdleTime:        5 * time.Minute,
		HealthCheckInterval: 30 * time.Second,
		ConnectionTimeout:  10 * time.Second,
		EnableLoadBalancing: true,
		EnableMetrics:      true,
		BufferSize:         4096,
	}
}

// OptimizedConnection 优化连接结构
type OptimizedConnection struct {
	ID           string
	Conn         net.Conn
	Buffer       *buffer.BytesBuffer
	Protocol     string
	IsActive     bool
	LastUsed     time.Time
	CreatedAt    time.Time
	RequestCount int64
	BytesSent    int64
	BytesReceived int64
	Healthy      bool
	mutex        sync.RWMutex
}

// UpdateStats 更新连接统计
func (c *OptimizedConnection) UpdateStats(sent, received int64) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	atomic.AddInt64(&c.RequestCount, 1)
	atomic.AddInt64(&c.BytesSent, sent)
	atomic.AddInt64(&c.BytesReceived, received)
	c.LastUsed = time.Now()
}

// IsIdle 检查连接是否空闲
func (c *OptimizedConnection) IsIdle(maxIdleTime time.Duration) bool {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	return time.Since(c.LastUsed) > maxIdleTime
}

// GetStats 获取连接统计
func (c *OptimizedConnection) GetStats() (int64, int64, int64) {
	return atomic.LoadInt64(&c.RequestCount), atomic.LoadInt64(&c.BytesSent), atomic.LoadInt64(&c.BytesReceived)
}

// ConnectionPoolMetrics 连接池指标
type ConnectionPoolMetrics struct {
	TotalConnections   int64 `json:"total_connections"`
	ActiveConnections  int64 `json:"active_connections"`
	IdleConnections    int64 `json:"idle_connections"`
	ConnectionsCreated int64 `json:"connections_created"`
	ConnectionsClosed  int64 `json:"connections_closed"`
	ConnectionErrors   int64 `json:"connection_errors"`
	TotalRequests      int64 `json:"total_requests"`
	TotalBytesSent     int64 `json:"total_bytes_sent"`
	TotalBytesReceived int64 `json:"total_bytes_received"`
	AverageResponseTime time.Duration `json:"average_response_time"`
	LastUpdated        time.Time     `json:"last_updated"`
}

// OptimizedConnectionPool 优化连接池
type OptimizedConnectionPool struct {
	config      *OptimizedConnectionPoolConfig
	connections map[string]*OptimizedConnection
	protocolMap map[string][]*OptimizedConnection // 按协议分组
	mutex       sync.RWMutex
	metrics     *ConnectionPoolMetrics
	ctx         context.Context
	cancel      context.CancelFunc
	running     bool
}

// NewOptimizedConnectionPool 创建优化连接池
func NewOptimizedConnectionPool(config *OptimizedConnectionPoolConfig) *OptimizedConnectionPool {
	if config == nil {
		config = DefaultOptimizedConnectionPoolConfig()
	}

	ctx, cancel := context.WithCancel(context.Background())
	pool := &OptimizedConnectionPool{
		config:      config,
		connections: make(map[string]*OptimizedConnection),
		protocolMap: make(map[string][]*OptimizedConnection),
		metrics: &ConnectionPoolMetrics{
			LastUpdated: time.Now(),
		},
		ctx:     ctx,
		cancel:  cancel,
		running: true,
	}

	// 启动后台任务
	go pool.healthChecker()
	go pool.idleConnectionCleaner()
	go pool.metricsUpdater()

	logger.Info("Optimized connection pool started")
	return pool
}

// AddConnection 添加连接
func (p *OptimizedConnectionPool) AddConnection(conn net.Conn) string {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	// 检查连接数限制
	if len(p.connections) >= p.config.MaxConnections {
		logger.Warn("Connection pool is full, rejecting new connection")
		conn.Close()
		atomic.AddInt64(&p.metrics.ConnectionErrors, 1)
		return ""
	}

	id := uuid.New().String()
	optConn := &OptimizedConnection{
		ID:        id,
		Conn:      conn,
		Buffer:    buffer.NewBytesBuffer(p.config.BufferSize),
		IsActive:  true,
		LastUsed:  time.Now(),
		CreatedAt: time.Now(),
		Healthy:   true,
	}

	p.connections[id] = optConn
	atomic.AddInt64(&p.metrics.ConnectionsCreated, 1)
	atomic.AddInt64(&p.metrics.TotalConnections, 1)

	logger.Debugf("Added optimized connection to pool: %s", id)
	return id
}

// GetConnection 获取连接（支持负载均衡）
func (p *OptimizedConnectionPool) GetConnection(id string) (*OptimizedConnection, bool) {
	p.mutex.RLock()
	defer p.mutex.RUnlock()

	conn, exists := p.connections[id]
	if exists && conn.IsActive && conn.Healthy {
		conn.LastUsed = time.Now()
		return conn, true
	}
	return nil, false
}

// GetConnectionByProtocol 按协议获取最优连接
func (p *OptimizedConnectionPool) GetConnectionByProtocol(protocol string) (*OptimizedConnection, bool) {
	p.mutex.RLock()
	defer p.mutex.RUnlock()

	conns, exists := p.protocolMap[protocol]
	if !exists || len(conns) == 0 {
		return nil, false
	}

	// 负载均衡：选择请求数最少的连接
	var bestConn *OptimizedConnection
	minRequests := int64(-1)

	for _, conn := range conns {
		if conn.IsActive && conn.Healthy {
			requests := atomic.LoadInt64(&conn.RequestCount)
			if minRequests == -1 || requests < minRequests {
				minRequests = requests
				bestConn = conn
			}
		}
	}

	if bestConn != nil {
		bestConn.LastUsed = time.Now()
		return bestConn, true
	}
	return nil, false
}

// UpdateConnectionProtocol 更新连接协议
func (p *OptimizedConnectionPool) UpdateConnectionProtocol(id string, protocol string) bool {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	conn, exists := p.connections[id]
	if !exists {
		return false
	}

	// 从旧协议组中移除
	if conn.Protocol != "" {
		p.removeFromProtocolMap(conn.Protocol, id)
	}

	// 添加到新协议组
	conn.Protocol = protocol
	if _, exists := p.protocolMap[protocol]; !exists {
		p.protocolMap[protocol] = make([]*OptimizedConnection, 0)
	}
	p.protocolMap[protocol] = append(p.protocolMap[protocol], conn)

	logger.Debugf("Updated connection protocol: %s to %s", id, protocol)
	return true
}

// removeFromProtocolMap 从协议映射中移除连接
func (p *OptimizedConnectionPool) removeFromProtocolMap(protocol, id string) {
	conns, exists := p.protocolMap[protocol]
	if !exists {
		return
	}

	for i, conn := range conns {
		if conn.ID == id {
			p.protocolMap[protocol] = append(conns[:i], conns[i+1:]...)
			break
		}
	}

	// 如果协议组为空，删除映射
	if len(p.protocolMap[protocol]) == 0 {
		delete(p.protocolMap, protocol)
	}
}

// RemoveConnection 移除连接
func (p *OptimizedConnectionPool) RemoveConnection(id string) {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	conn, exists := p.connections[id]
	if !exists {
		return
	}

	// 从协议映射中移除
	if conn.Protocol != "" {
		p.removeFromProtocolMap(conn.Protocol, id)
	}

	// 关闭连接
	conn.Conn.Close()
	delete(p.connections, id)

	atomic.AddInt64(&p.metrics.ConnectionsClosed, 1)
	atomic.AddInt64(&p.metrics.TotalConnections, -1)

	logger.Debugf("Removed optimized connection from pool: %s", id)
}

// GetMetrics 获取连接池指标
func (p *OptimizedConnectionPool) GetMetrics() *ConnectionPoolMetrics {
	p.mutex.RLock()
	defer p.mutex.RUnlock()

	activeCount := int64(0)
	idleCount := int64(0)
	totalRequests := int64(0)
	totalBytesSent := int64(0)
	totalBytesReceived := int64(0)

	for _, conn := range p.connections {
		if conn.IsActive {
			activeCount++
			if conn.IsIdle(p.config.MaxIdleTime) {
				idleCount++
			}
		}
		requests, sent, received := conn.GetStats()
		totalRequests += requests
		totalBytesSent += sent
		totalBytesReceived += received
	}

	metrics := *p.metrics
	metrics.ActiveConnections = activeCount
	metrics.IdleConnections = idleCount
	metrics.TotalRequests = totalRequests
	metrics.TotalBytesSent = totalBytesSent
	metrics.TotalBytesReceived = totalBytesReceived
	metrics.LastUpdated = time.Now()

	return &metrics
}

// healthChecker 健康检查器
func (p *OptimizedConnectionPool) healthChecker() {
	ticker := time.NewTicker(p.config.HealthCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			p.performHealthCheck()
		case <-p.ctx.Done():
			return
		}
	}
}

// performHealthCheck 执行健康检查
func (p *OptimizedConnectionPool) performHealthCheck() {
	p.mutex.RLock()
	connections := make([]*OptimizedConnection, 0, len(p.connections))
	for _, conn := range p.connections {
		connections = append(connections, conn)
	}
	p.mutex.RUnlock()

	for _, conn := range connections {
		// 简单的健康检查：尝试设置读取超时
		if conn.IsActive {
			err := conn.Conn.SetReadDeadline(time.Now().Add(time.Second))
			conn.mutex.Lock()
			conn.Healthy = (err == nil)
			conn.mutex.Unlock()

			if err != nil {
				logger.Debugf("Connection %s failed health check: %v", conn.ID, err)
				p.RemoveConnection(conn.ID)
			}
		}
	}
}

// idleConnectionCleaner 空闲连接清理器
func (p *OptimizedConnectionPool) idleConnectionCleaner() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			p.cleanIdleConnections()
		case <-p.ctx.Done():
			return
		}
	}
}

// cleanIdleConnections 清理空闲连接
func (p *OptimizedConnectionPool) cleanIdleConnections() {
	p.mutex.RLock()
	idleConnections := make([]string, 0)
	activeCount := 0

	for id, conn := range p.connections {
		if conn.IsActive {
			activeCount++
			if conn.IsIdle(p.config.MaxIdleTime) {
				idleConnections = append(idleConnections, id)
			}
		}
	}
	p.mutex.RUnlock()

	// 保持最小连接数
	maxToRemove := activeCount - p.config.MinConnections
	if maxToRemove > 0 && len(idleConnections) > 0 {
		removeCount := len(idleConnections)
		if removeCount > maxToRemove {
			removeCount = maxToRemove
		}

		for i := 0; i < removeCount; i++ {
			p.RemoveConnection(idleConnections[i])
		}

		logger.Debugf("Cleaned %d idle connections", removeCount)
	}
}

// metricsUpdater 指标更新器
func (p *OptimizedConnectionPool) metricsUpdater() {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if p.config.EnableMetrics {
				p.updateMetrics()
			}
		case <-p.ctx.Done():
			return
		}
	}
}

// updateMetrics 更新指标
func (p *OptimizedConnectionPool) updateMetrics() {
	metrics := p.GetMetrics()
	logger.Debugf("Connection pool metrics - Total: %d, Active: %d, Idle: %d",
		metrics.TotalConnections, metrics.ActiveConnections, metrics.IdleConnections)
}

// Close 关闭连接池
func (p *OptimizedConnectionPool) Close() {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	if !p.running {
		return
	}

	p.running = false
	p.cancel()

	// 关闭所有连接
	for _, conn := range p.connections {
		conn.Conn.Close()
	}

	p.connections = make(map[string]*OptimizedConnection)
	p.protocolMap = make(map[string][]*OptimizedConnection)

	logger.Info("Optimized connection pool closed")
}

// OptimizedGoroutinePoolConfig 优化协程池配置
type OptimizedGoroutinePoolConfig struct {
	MinWorkers       int           `json:"min_workers"`
	MaxWorkers       int           `json:"max_workers"`
	QueueSize        int           `json:"queue_size"`
	ScaleUpThreshold float64       `json:"scale_up_threshold"`
	ScaleDownThreshold float64     `json:"scale_down_threshold"`
	ScaleInterval    time.Duration `json:"scale_interval"`
	WorkerTimeout    time.Duration `json:"worker_timeout"`
	EnableMetrics    bool          `json:"enable_metrics"`
}

// DefaultOptimizedGoroutinePoolConfig 默认配置
func DefaultOptimizedGoroutinePoolConfig() *OptimizedGoroutinePoolConfig {
	return &OptimizedGoroutinePoolConfig{
		MinWorkers:         5,
		MaxWorkers:         100,
		QueueSize:          1000,
		ScaleUpThreshold:   0.8,
		ScaleDownThreshold: 0.2,
		ScaleInterval:      30 * time.Second,
		WorkerTimeout:      5 * time.Minute,
		EnableMetrics:      true,
	}
}

// GoroutinePoolMetrics 协程池指标
type GoroutinePoolMetrics struct {
	ActiveWorkers    int64         `json:"active_workers"`
	IdleWorkers      int64         `json:"idle_workers"`
	QueueLength      int64         `json:"queue_length"`
	TasksSubmitted   int64         `json:"tasks_submitted"`
	TasksCompleted   int64         `json:"tasks_completed"`
	TasksRejected    int64         `json:"tasks_rejected"`
	AverageTaskTime  time.Duration `json:"average_task_time"`
	TotalTaskTime    time.Duration `json:"total_task_time"`
	LastScaleEvent   time.Time     `json:"last_scale_event"`
	LastUpdated      time.Time     `json:"last_updated"`
}

// OptimizedGoroutinePool 优化协程池
type OptimizedGoroutinePool struct {
	config       *OptimizedGoroutinePoolConfig
	tasks        chan func()
	workerCount  int64
	idleWorkers  int64
	metrics      *GoroutinePoolMetrics
	ctx          context.Context
	cancel       context.CancelFunc
	wg           sync.WaitGroup
	mutex        sync.RWMutex
	running      bool
}

// NewOptimizedGoroutinePool 创建优化协程池
func NewOptimizedGoroutinePool(config *OptimizedGoroutinePoolConfig) *OptimizedGoroutinePool {
	if config == nil {
		config = DefaultOptimizedGoroutinePoolConfig()
	}

	ctx, cancel := context.WithCancel(context.Background())
	pool := &OptimizedGoroutinePool{
		config: config,
		tasks:  make(chan func(), config.QueueSize),
		metrics: &GoroutinePoolMetrics{
			LastUpdated: time.Now(),
		},
		ctx:     ctx,
		cancel:  cancel,
		running: true,
	}

	// 启动最小数量的工作协程
	for i := 0; i < config.MinWorkers; i++ {
		pool.addWorker()
	}

	// 启动自动扩缩容
	go pool.autoScaler()
	go pool.metricsUpdater()

	logger.Infof("Optimized goroutine pool started with %d workers", config.MinWorkers)
	return pool
}

// addWorker 添加工作协程
func (p *OptimizedGoroutinePool) addWorker() {
	atomic.AddInt64(&p.workerCount, 1)
	atomic.AddInt64(&p.idleWorkers, 1)
	p.wg.Add(1)

	go func() {
		defer p.wg.Done()
		defer atomic.AddInt64(&p.workerCount, -1)

		for {
			select {
			case task := <-p.tasks:
				atomic.AddInt64(&p.idleWorkers, -1)
				start := time.Now()
				task()
				duration := time.Since(start)

				// 更新指标
				atomic.AddInt64(&p.metrics.TasksCompleted, 1)
				p.updateTaskTime(duration)
				atomic.AddInt64(&p.idleWorkers, 1)

			case <-time.After(p.config.WorkerTimeout):
				// 工作协程超时，检查是否可以退出
				if atomic.LoadInt64(&p.workerCount) > int64(p.config.MinWorkers) {
					atomic.AddInt64(&p.idleWorkers, -1)
					return
				}

			case <-p.ctx.Done():
				atomic.AddInt64(&p.idleWorkers, -1)
				return
			}
		}
	}()
}

// updateTaskTime 更新任务执行时间
func (p *OptimizedGoroutinePool) updateTaskTime(duration time.Duration) {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	p.metrics.TotalTaskTime += duration
	completedTasks := atomic.LoadInt64(&p.metrics.TasksCompleted)
	if completedTasks > 0 {
		p.metrics.AverageTaskTime = p.metrics.TotalTaskTime / time.Duration(completedTasks)
	}
}

// Submit 提交任务
func (p *OptimizedGoroutinePool) Submit(task func()) bool {
	if !p.running {
		atomic.AddInt64(&p.metrics.TasksRejected, 1)
		return false
	}

	select {
	case p.tasks <- task:
		atomic.AddInt64(&p.metrics.TasksSubmitted, 1)
		return true
	default:
		// 队列已满
		atomic.AddInt64(&p.metrics.TasksRejected, 1)
		logger.Warn("Goroutine pool queue is full, task rejected")
		return false
	}
}

// SubmitWithTimeout 带超时的任务提交
func (p *OptimizedGoroutinePool) SubmitWithTimeout(task func(), timeout time.Duration) bool {
	if !p.running {
		atomic.AddInt64(&p.metrics.TasksRejected, 1)
		return false
	}

	select {
	case p.tasks <- task:
		atomic.AddInt64(&p.metrics.TasksSubmitted, 1)
		return true
	case <-time.After(timeout):
		atomic.AddInt64(&p.metrics.TasksRejected, 1)
		return false
	}
}

// autoScaler 自动扩缩容
func (p *OptimizedGoroutinePool) autoScaler() {
	ticker := time.NewTicker(p.config.ScaleInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			p.performAutoScale()
		case <-p.ctx.Done():
			return
		}
	}
}

// performAutoScale 执行自动扩缩容
func (p *OptimizedGoroutinePool) performAutoScale() {
	queueLength := len(p.tasks)
	workerCount := atomic.LoadInt64(&p.workerCount)
	idleWorkers := atomic.LoadInt64(&p.idleWorkers)

	// 计算队列使用率
	queueUsage := float64(queueLength) / float64(p.config.QueueSize)
	// 计算工作协程使用率
	workerUsage := float64(workerCount-idleWorkers) / float64(workerCount)

	// 扩容条件：队列使用率或工作协程使用率超过阈值
	if (queueUsage > p.config.ScaleUpThreshold || workerUsage > p.config.ScaleUpThreshold) &&
		workerCount < int64(p.config.MaxWorkers) {
		scaleCount := int(float64(p.config.MaxWorkers-int(workerCount)) * 0.2)
		if scaleCount < 1 {
			scaleCount = 1
		}
		if scaleCount > 10 {
			scaleCount = 10
		}

		for i := 0; i < scaleCount; i++ {
			p.addWorker()
		}

		p.metrics.LastScaleEvent = time.Now()
		logger.Debugf("Scaled up goroutine pool by %d workers (total: %d)", scaleCount, atomic.LoadInt64(&p.workerCount))
	}

	// 缩容条件：队列和工作协程使用率都低于阈值
	if queueUsage < p.config.ScaleDownThreshold && workerUsage < p.config.ScaleDownThreshold &&
		workerCount > int64(p.config.MinWorkers) {
		// 缩容通过工作协程超时自然退出实现
		logger.Debug("Goroutine pool ready for scale down via worker timeout")
	}
}

// GetMetrics 获取协程池指标
func (p *OptimizedGoroutinePool) GetMetrics() *GoroutinePoolMetrics {
	p.mutex.RLock()
	defer p.mutex.RUnlock()

	metrics := *p.metrics
	metrics.ActiveWorkers = atomic.LoadInt64(&p.workerCount)
	metrics.IdleWorkers = atomic.LoadInt64(&p.idleWorkers)
	metrics.QueueLength = int64(len(p.tasks))
	metrics.LastUpdated = time.Now()

	return &metrics
}

// metricsUpdater 指标更新器
func (p *OptimizedGoroutinePool) metricsUpdater() {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if p.config.EnableMetrics {
				metrics := p.GetMetrics()
				logger.Debugf("Goroutine pool metrics - Workers: %d, Idle: %d, Queue: %d",
					metrics.ActiveWorkers, metrics.IdleWorkers, metrics.QueueLength)
			}
		case <-p.ctx.Done():
			return
		}
	}
}

// Shutdown 关闭协程池
func (p *OptimizedGoroutinePool) Shutdown() {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	if !p.running {
		return
	}

	p.running = false
	p.cancel()

	// 等待所有工作协程完成
	p.wg.Wait()

	logger.Info("Optimized goroutine pool shutdown completed")
}

// GetQueueSize 获取队列大小
func (p *OptimizedGoroutinePool) GetQueueSize() int {
	return len(p.tasks)
}

// GetWorkerCount 获取工作协程数量
func (p *OptimizedGoroutinePool) GetWorkerCount() int64 {
	return atomic.LoadInt64(&p.workerCount)
}