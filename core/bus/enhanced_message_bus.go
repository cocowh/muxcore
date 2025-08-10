package bus

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cocowh/muxcore/pkg/errors"
	"github.com/cocowh/muxcore/pkg/logger"
)

// MessageType 消息类型
type MessageType string

const (
	MessageTypeEvent        MessageType = "event"
	MessageTypeCommand      MessageType = "command"
	MessageTypeQuery        MessageType = "query"
	MessageTypeResponse     MessageType = "response"
	MessageTypeBroadcast    MessageType = "broadcast"
	MessageTypeRequest      MessageType = "request"
	MessageTypeReply        MessageType = "reply"
	MessageTypeNotification MessageType = "notification"
)

// MessagePriority 消息优先级
type MessagePriority int

const (
	PriorityLow      MessagePriority = 0
	PriorityNormal   MessagePriority = 1
	PriorityHigh     MessagePriority = 2
	PriorityCritical MessagePriority = 3
)

// DeliveryMode 投递模式
type DeliveryMode int

const (
	DeliveryModeAtMostOnce  DeliveryMode = 0 // 最多一次
	DeliveryModeAtLeastOnce DeliveryMode = 1 // 至少一次
	DeliveryModeExactlyOnce DeliveryMode = 2 // 恰好一次
)

// ProtocolType 协议类型
type ProtocolType string

const (
	ProtocolWebSocket ProtocolType = "websocket"
	ProtocolMQTT      ProtocolType = "mqtt"
	ProtocolAMQP      ProtocolType = "amqp"
	ProtocolHTTP      ProtocolType = "http"
	ProtocolGRPC      ProtocolType = "grpc"
)

// Message 统一消息结构
type Message struct {
	ID            string                 `json:"id"`
	Type          MessageType            `json:"type"`
	Topic         string                 `json:"topic"`
	Payload       interface{}            `json:"payload"`
	Headers       map[string]string      `json:"headers"`
	Metadata      map[string]interface{} `json:"metadata"`
	Timestamp     time.Time              `json:"timestamp"`
	Source        string                 `json:"source"`
	Destination   string                 `json:"destination,omitempty"`
	CorrelationID string                 `json:"correlation_id,omitempty"`
	ReplyTo       string                 `json:"reply_to,omitempty"`
	TTL           time.Duration          `json:"ttl,omitempty"`
	Priority      MessagePriority        `json:"priority"`
	DeliveryMode  DeliveryMode           `json:"delivery_mode"`
	RetryCount    int                    `json:"retry_count"`
	MaxRetries    int                    `json:"max_retries"`
	PartitionKey  string                 `json:"partition_key,omitempty"`
	Compressed    bool                   `json:"compressed"`
	Checksum      string                 `json:"checksum,omitempty"`
}

// MessageQueue 消息队列接口
type MessageQueue interface {
	Enqueue(message *Message) error
	Dequeue() (*Message, error)
	Peek() (*Message, error)
	Size() int
	Clear() error
}

// EventBus 事件总线接口
type EventBus interface {
	PublishEvent(event *Event) error
	SubscribeEvent(eventType string, handler EventHandler) (string, error)
	UnsubscribeEvent(subscriptionID string) error
}

// Event 事件结构
type Event struct {
	ID        string                 `json:"id"`
	Type      string                 `json:"type"`
	Source    string                 `json:"source"`
	Data      interface{}            `json:"data"`
	Timestamp time.Time              `json:"timestamp"`
	Metadata  map[string]interface{} `json:"metadata"`
}

// EventHandler 事件处理器
type EventHandler func(event *Event) error

// Connection 连接接口
type Connection interface {
	ID() string
	Protocol() ProtocolType
	Send(message *Message) error
	Close() error
	IsActive() bool
	LastActivity() time.Time
	GetMetrics() *ConnectionMetrics
	SetQoS(qos QoSLevel) error
}

// QoSLevel 服务质量等级
type QoSLevel int

const (
	QoSAtMostOnce  QoSLevel = 0
	QoSAtLeastOnce QoSLevel = 1
	QoSExactlyOnce QoSLevel = 2
)

// ConnectionMetrics 连接指标
type ConnectionMetrics struct {
	MessagesSent     int64         `json:"messages_sent"`
	MessagesReceived int64         `json:"messages_received"`
	BytesTransferred int64         `json:"bytes_transferred"`
	LastPingTime     time.Time     `json:"last_ping_time"`
	Latency          time.Duration `json:"latency"`
	ErrorCount       int64         `json:"error_count"`
}

// MessageHandler 消息处理器
type MessageHandler func(ctx context.Context, message *Message) error

// MessageFilter 消息过滤器
type MessageFilter func(message *Message) bool

// Subscription 订阅信息
type Subscription struct {
	ID           string
	Topic        string
	ConnectionID string
	Handler      MessageHandler
	Filter       MessageFilter
	CreatedAt    time.Time
	Active       bool
}

// MessageStore 消息存储接口
type MessageStore interface {
	Store(message *Message) error
	Retrieve(messageID string) (*Message, error)
	Delete(messageID string) error
	GetByTopic(topic string, limit int) ([]*Message, error)
	Cleanup(before time.Time) error
}

// InMemoryMessageStore 内存消息存储实现
type InMemoryMessageStore struct {
	messages map[string]*Message
	mutex    sync.RWMutex
}

func NewInMemoryMessageStore() *InMemoryMessageStore {
	return &InMemoryMessageStore{
		messages: make(map[string]*Message),
	}
}

func (s *InMemoryMessageStore) Store(message *Message) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.messages[message.ID] = message
	return nil
}

func (s *InMemoryMessageStore) Retrieve(messageID string) (*Message, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	message, exists := s.messages[messageID]
	if !exists {
		return nil, errors.New(errors.ErrCodeConfigNotFound, errors.CategorySystem, errors.LevelError, fmt.Sprintf("message not found: %s", messageID))
	}
	return message, nil
}

func (s *InMemoryMessageStore) Delete(messageID string) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	delete(s.messages, messageID)
	return nil
}

func (s *InMemoryMessageStore) GetByTopic(topic string, limit int) ([]*Message, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	var result []*Message
	count := 0
	for _, message := range s.messages {
		if message.Topic == topic {
			result = append(result, message)
			count++
			if limit > 0 && count >= limit {
				break
			}
		}
	}
	return result, nil
}

func (s *InMemoryMessageStore) Cleanup(before time.Time) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	for id, message := range s.messages {
		if message.Timestamp.Before(before) {
			delete(s.messages, id)
		}
	}
	return nil
}

// EnhancedMessageBus 增强的消息总线
type EnhancedMessageBus struct {
	connections   map[string]Connection
	subscriptions map[string]*Subscription
	topicSubs     map[string][]string // topic -> subscription IDs
	store         MessageStore
	mutex         sync.RWMutex
	ctx           context.Context
	cancel        context.CancelFunc
	wg            sync.WaitGroup

	// 新增功能组件
	router     *MessageRouter
	compressor MessageCompressor
	eventBus   *EventBusImpl
	metrics    *BusMetrics
	config     *BusConfig

	// 事件订阅
	eventSubscriptions map[string]map[string]EventHandler // eventType -> subscriptionID -> handler
	eventMutex         sync.RWMutex

	// 配置选项
	maxConnections     int
	messageTimeout     time.Duration
	cleanupInterval    time.Duration
	retryAttempts      int
	batchSize          int
	compressionEnabled bool
}

// EventBusImpl 事件总线实现
type EventBusImpl struct {
	subscriptions map[string]map[string]EventHandler
	bus           *EnhancedMessageBus
}

// BusMetrics 总线指标
type BusMetrics struct {
	MessagesPublished   int64     `json:"messages_published"`
	MessagesDelivered   int64     `json:"messages_delivered"`
	MessagesFailed      int64     `json:"messages_failed"`
	ActiveConnections   int64     `json:"active_connections"`
	ActiveSubscriptions int64     `json:"active_subscriptions"`
	EventsPublished     int64     `json:"events_published"`
	EventsDelivered     int64     `json:"events_delivered"`
	TotalLatency        int64     `json:"total_latency_ms"`
	StartTime           time.Time `json:"start_time"`
}

// PriorityMessageQueue 优先级消息队列
type PriorityMessageQueue struct {
	queues [4][]*Message // 按优先级分组的队列
	mutex  sync.RWMutex
	size   int64
}

// NewPriorityMessageQueue 创建优先级消息队列
func NewPriorityMessageQueue() *PriorityMessageQueue {
	return &PriorityMessageQueue{}
}

// Enqueue 入队
func (q *PriorityMessageQueue) Enqueue(message *Message) error {
	q.mutex.Lock()
	defer q.mutex.Unlock()

	priority := int(message.Priority)
	if priority < 0 || priority > 3 {
		priority = 1 // 默认普通优先级
	}

	q.queues[priority] = append(q.queues[priority], message)
	atomic.AddInt64(&q.size, 1)
	return nil
}

// Dequeue 出队（按优先级）
func (q *PriorityMessageQueue) Dequeue() (*Message, error) {
	q.mutex.Lock()
	defer q.mutex.Unlock()

	// 从高优先级到低优先级检查
	for priority := 3; priority >= 0; priority-- {
		if len(q.queues[priority]) > 0 {
			message := q.queues[priority][0]
			q.queues[priority] = q.queues[priority][1:]
			atomic.AddInt64(&q.size, -1)
			return message, nil
		}
	}

	return nil, errors.New(errors.ErrCodeConfigNotFound, errors.CategorySystem, errors.LevelError, "queue is empty")
}

// Peek 查看队首元素
func (q *PriorityMessageQueue) Peek() (*Message, error) {
	q.mutex.RLock()
	defer q.mutex.RUnlock()

	for priority := 3; priority >= 0; priority-- {
		if len(q.queues[priority]) > 0 {
			return q.queues[priority][0], nil
		}
	}

	return nil, errors.New(errors.ErrCodeConfigNotFound, errors.CategorySystem, errors.LevelError, "queue is empty")
}

// Size 队列大小
func (q *PriorityMessageQueue) Size() int {
	return int(atomic.LoadInt64(&q.size))
}

// Clear 清空队列
func (q *PriorityMessageQueue) Clear() error {
	q.mutex.Lock()
	defer q.mutex.Unlock()

	for i := range q.queues {
		q.queues[i] = q.queues[i][:0]
	}
	atomic.StoreInt64(&q.size, 0)
	return nil
}

// BusConfig 消息总线配置
type BusConfig struct {
	MaxConnections     int           `json:"max_connections"`
	MessageTimeout     time.Duration `json:"message_timeout"`
	CleanupInterval    time.Duration `json:"cleanup_interval"`
	RetryAttempts      int           `json:"retry_attempts"`
	BatchSize          int           `json:"batch_size"`
	CompressionEnabled bool          `json:"compression_enabled"`
	EnableMetrics      bool          `json:"enable_metrics"`
	EnableTracing      bool          `json:"enable_tracing"`
	PartitionCount     int           `json:"partition_count"`
	MaxQueueSize       int           `json:"max_queue_size"`
	EnablePersistence  bool          `json:"enable_persistence"`
	PersistencePath    string        `json:"persistence_path"`
}

// MessageCompressor 消息压缩器
type MessageCompressor interface {
	Compress(data []byte) ([]byte, error)
	Decompress(data []byte) ([]byte, error)
}

// GzipCompressor Gzip压缩器
type GzipCompressor struct{}

// Compress 压缩数据
func (c *GzipCompressor) Compress(data []byte) ([]byte, error) {
	// 简化实现，实际应使用gzip
	return data, nil
}

// Decompress 解压数据
func (c *GzipCompressor) Decompress(data []byte) ([]byte, error) {
	// 简化实现，实际应使用gzip
	return data, nil
}

// MessagePartitioner 消息分区器
type MessagePartitioner interface {
	GetPartition(message *Message, partitionCount int) int
}

// HashPartitioner 哈希分区器
type HashPartitioner struct{}

// GetPartition 获取分区
func (p *HashPartitioner) GetPartition(message *Message, partitionCount int) int {
	if partitionCount <= 0 {
		return 0
	}

	key := message.PartitionKey
	if key == "" {
		key = message.Topic
	}

	h := fnv.New32a()
	h.Write([]byte(key))
	return int(h.Sum32()) % partitionCount
}

// MessageRouter 消息路由器
type MessageRouter struct {
	partitioner MessagePartitioner
	partitions  []MessageQueue
	mutex       sync.RWMutex
}

// NewMessageRouter 创建消息路由器
func NewMessageRouter(partitionCount int) *MessageRouter {
	partitions := make([]MessageQueue, partitionCount)
	for i := range partitions {
		partitions[i] = NewPriorityMessageQueue()
	}

	return &MessageRouter{
		partitioner: &HashPartitioner{},
		partitions:  partitions,
	}
}

// Route 路由消息到分区
func (r *MessageRouter) Route(message *Message) error {
	r.mutex.RLock()
	partitionIndex := r.partitioner.GetPartition(message, len(r.partitions))
	partition := r.partitions[partitionIndex]
	r.mutex.RUnlock()

	return partition.Enqueue(message)
}

// GetPartition 获取指定分区
func (r *MessageRouter) GetPartition(index int) MessageQueue {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	if index >= 0 && index < len(r.partitions) {
		return r.partitions[index]
	}
	return nil
}

// DefaultBusConfig 默认配置
func DefaultBusConfig() *BusConfig {
	return &BusConfig{
		MaxConnections:     10000,
		MessageTimeout:     30 * time.Second,
		CleanupInterval:    5 * time.Minute,
		RetryAttempts:      3,
		BatchSize:          100,
		CompressionEnabled: true,
		EnableMetrics:      true,
		EnableTracing:      false,
		PartitionCount:     4,
		MaxQueueSize:       10000,
		EnablePersistence:  false,
		PersistencePath:    "/tmp/muxcore_bus",
	}
}

// generateID 生成唯一ID
func generateID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return fmt.Sprintf("%x", b)
}

// NewEnhancedMessageBus 创建增强消息总线
func NewEnhancedMessageBus(config *BusConfig, store MessageStore) *EnhancedMessageBus {
	if config == nil {
		config = DefaultBusConfig()
	}
	if store == nil {
		store = NewInMemoryMessageStore()
	}

	ctx, cancel := context.WithCancel(context.Background())

	// 初始化组件
	router := NewMessageRouter(config.PartitionCount)
	compressor := &GzipCompressor{}
	metrics := &BusMetrics{
		StartTime: time.Now(),
	}

	bus := &EnhancedMessageBus{
		connections:        make(map[string]Connection),
		subscriptions:      make(map[string]*Subscription),
		topicSubs:          make(map[string][]string),
		store:              store,
		ctx:                ctx,
		cancel:             cancel,
		router:             router,
		compressor:         compressor,
		metrics:            metrics,
		config:             config,
		eventSubscriptions: make(map[string]map[string]EventHandler),
		maxConnections:     config.MaxConnections,
		messageTimeout:     config.MessageTimeout,
		cleanupInterval:    config.CleanupInterval,
		retryAttempts:      config.RetryAttempts,
		batchSize:          config.BatchSize,
		compressionEnabled: config.CompressionEnabled,
	}

	// 初始化事件总线
	bus.eventBus = &EventBusImpl{
		subscriptions: make(map[string]map[string]EventHandler),
		bus:           bus,
	}

	// 启动清理协程
	bus.wg.Add(1)
	go bus.cleanupLoop()

	// 启动指标收集协程
	if config.EnableMetrics {
		bus.wg.Add(1)
		go bus.metricsLoop()
	}

	return bus
}

// RegisterConnection 注册连接
func (bus *EnhancedMessageBus) RegisterConnection(conn Connection) error {
	bus.mutex.Lock()
	defer bus.mutex.Unlock()

	if len(bus.connections) >= bus.maxConnections {
		return errors.New(errors.ErrCodeSystemResourceLimit, errors.CategorySystem, errors.LevelError, "max connections reached")
	}

	bus.connections[conn.ID()] = conn
	logger.Info("Registered connection", "id", conn.ID(), "protocol", conn.Protocol())
	return nil
}

// UnregisterConnection 注销连接
func (bus *EnhancedMessageBus) UnregisterConnection(connID string) error {
	bus.mutex.Lock()
	defer bus.mutex.Unlock()

	// 移除连接
	if conn, exists := bus.connections[connID]; exists {
		conn.Close()
		delete(bus.connections, connID)
	}

	// 移除相关订阅
	for subID, sub := range bus.subscriptions {
		if sub.ConnectionID == connID {
			bus.removeSubscriptionFromTopic(sub.Topic, subID)
			delete(bus.subscriptions, subID)
		}
	}

	logger.Info("Unregistered connection", "id", connID)
	return nil
}

// Subscribe 订阅主题
func (bus *EnhancedMessageBus) Subscribe(connID, topic string, handler MessageHandler, filter MessageFilter) (string, error) {
	bus.mutex.Lock()
	defer bus.mutex.Unlock()

	// 检查连接是否存在
	if _, exists := bus.connections[connID]; !exists {
		return "", errors.New(errors.ErrCodeConfigNotFound, errors.CategorySystem, errors.LevelError, fmt.Sprintf("connection not found: %s", connID))
	}

	subID := fmt.Sprintf("%s_%s_%d", connID, topic, time.Now().UnixNano())
	sub := &Subscription{
		ID:           subID,
		Topic:        topic,
		ConnectionID: connID,
		Handler:      handler,
		Filter:       filter,
		CreatedAt:    time.Now(),
		Active:       true,
	}

	bus.subscriptions[subID] = sub
	bus.addSubscriptionToTopic(topic, subID)

	logger.Info("Created subscription", "id", subID, "topic", topic, "connection", connID)
	return subID, nil
}

// Unsubscribe 取消订阅
func (bus *EnhancedMessageBus) Unsubscribe(subID string) error {
	bus.mutex.Lock()
	defer bus.mutex.Unlock()

	sub, exists := bus.subscriptions[subID]
	if !exists {
		return errors.New(errors.ErrCodeConfigNotFound, errors.CategorySystem, errors.LevelError, fmt.Sprintf("subscription not found: %s", subID))
	}

	bus.removeSubscriptionFromTopic(sub.Topic, subID)
	delete(bus.subscriptions, subID)

	logger.Info("Removed subscription", "id", subID)
	return nil
}

// Publish 发布消息
func (bus *EnhancedMessageBus) Publish(ctx context.Context, message *Message) error {
	start := time.Now()

	// 设置消息元数据
	if message.ID == "" {
		message.ID = generateID()
	}
	if message.Timestamp.IsZero() {
		message.Timestamp = time.Now()
	}
	if message.Priority == 0 {
		message.Priority = PriorityNormal
	}
	if message.DeliveryMode == 0 {
		message.DeliveryMode = DeliveryModeAtLeastOnce
	}

	// 消息压缩
	if bus.compressionEnabled && message.Compressed {
		if payload, err := json.Marshal(message.Payload); err == nil {
			if compressed, err := bus.compressor.Compress(payload); err == nil {
				message.Payload = compressed
				message.Compressed = true
			}
		}
	}

	// 计算校验和
	if payloadBytes, err := json.Marshal(message.Payload); err == nil {
		h := fnv.New32a()
		h.Write(payloadBytes)
		message.Checksum = fmt.Sprintf("%x", h.Sum32())
	}

	// 存储消息
	if err := bus.store.Store(message); err != nil {
		logger.Error("Failed to store message", "error", err)
	}

	// 更新指标
	atomic.AddInt64(&bus.metrics.MessagesPublished, 1)

	// 路由消息
	err := bus.routeMessage(ctx, message)
	if err != nil {
		atomic.AddInt64(&bus.metrics.MessagesFailed, 1)
	} else {
		atomic.AddInt64(&bus.metrics.MessagesDelivered, 1)
	}

	// 记录延迟
	latency := time.Since(start).Milliseconds()
	atomic.AddInt64(&bus.metrics.TotalLatency, latency)

	return err
}

// SendDirect 直接发送消息到指定连接
func (bus *EnhancedMessageBus) SendDirect(ctx context.Context, connID string, message *Message) error {
	bus.mutex.RLock()
	conn, exists := bus.connections[connID]
	bus.mutex.RUnlock()

	if !exists {
		return errors.New(errors.ErrCodeConfigNotFound, errors.CategorySystem, errors.LevelError, fmt.Sprintf("connection not found: %s", connID))
	}

	return bus.sendWithRetry(ctx, conn, message)
}

// Broadcast 广播消息
func (bus *EnhancedMessageBus) Broadcast(ctx context.Context, message *Message, excludeConnID string) error {
	bus.mutex.RLock()
	connections := make([]Connection, 0, len(bus.connections))
	for connID, conn := range bus.connections {
		if connID != excludeConnID && conn.IsActive() {
			connections = append(connections, conn)
		}
	}
	bus.mutex.RUnlock()

	// 并发发送
	errorChan := make(chan error, len(connections))
	for _, conn := range connections {
		go func(c Connection) {
			errorChan <- bus.sendWithRetry(ctx, c, message)
		}(conn)
	}

	// 收集错误
	var errors []error
	for i := 0; i < len(connections); i++ {
		if err := <-errorChan; err != nil {
			errors = append(errors, err)
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("broadcast failed for %d connections", len(errors))
	}

	return nil
}

// GetConnectionCount 获取连接数
func (bus *EnhancedMessageBus) GetConnectionCount() int {
	bus.mutex.RLock()
	defer bus.mutex.RUnlock()
	return len(bus.connections)
}

// GetSubscriptionCount 获取订阅数
func (bus *EnhancedMessageBus) GetSubscriptionCount() int {
	bus.mutex.RLock()
	defer bus.mutex.RUnlock()
	return len(bus.subscriptions)
}

// Close 关闭消息总线
func (bus *EnhancedMessageBus) Close() error {
	bus.cancel()
	bus.wg.Wait()

	bus.mutex.Lock()
	defer bus.mutex.Unlock()

	// 关闭所有连接
	for _, conn := range bus.connections {
		conn.Close()
	}

	logger.Info("Message bus closed")
	return nil
}

// 私有方法

func (bus *EnhancedMessageBus) routeMessage(ctx context.Context, message *Message) error {
	bus.mutex.RLock()
	subIDs, exists := bus.topicSubs[message.Topic]
	if !exists {
		bus.mutex.RUnlock()
		return nil // 没有订阅者
	}

	// 复制订阅ID列表
	targetSubs := make([]*Subscription, 0, len(subIDs))
	for _, subID := range subIDs {
		if sub, exists := bus.subscriptions[subID]; exists && sub.Active {
			targetSubs = append(targetSubs, sub)
		}
	}
	bus.mutex.RUnlock()

	// 并发处理订阅
	errorChan := make(chan error, len(targetSubs))
	for _, sub := range targetSubs {
		go func(s *Subscription) {
			errorChan <- bus.handleSubscription(ctx, s, message)
		}(sub)
	}

	// 收集错误
	var errors []error
	for i := 0; i < len(targetSubs); i++ {
		if err := <-errorChan; err != nil {
			errors = append(errors, err)
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("routing failed for %d subscriptions", len(errors))
	}

	return nil
}

func (bus *EnhancedMessageBus) handleSubscription(ctx context.Context, sub *Subscription, message *Message) error {
	// 应用过滤器
	if sub.Filter != nil && !sub.Filter(message) {
		return nil
	}

	// 获取连接
	bus.mutex.RLock()
	conn, exists := bus.connections[sub.ConnectionID]
	bus.mutex.RUnlock()

	if !exists || !conn.IsActive() {
		return errors.New(errors.ErrCodeConfigNotFound, errors.CategorySystem, errors.LevelError, fmt.Sprintf("connection not found: %s", sub.ConnectionID))
	}

	// 调用处理器
	if sub.Handler != nil {
		if err := sub.Handler(ctx, message); err != nil {
			return err
		}
	}

	// 发送消息到连接
	return bus.sendWithRetry(ctx, conn, message)
}

func (bus *EnhancedMessageBus) sendWithRetry(ctx context.Context, conn Connection, message *Message) error {
	var lastErr error

	for attempt := 0; attempt <= bus.retryAttempts; attempt++ {
		if attempt > 0 {
			// 指数退避
			backoff := time.Duration(attempt*attempt) * 100 * time.Millisecond
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(backoff):
			}
		}

		if err := conn.Send(message); err != nil {
			lastErr = err
			logger.Debug("Send attempt failed", "attempt", attempt+1, "error", err)
			continue
		}

		return nil
	}

	return fmt.Errorf("failed to send after %d attempts: %w", bus.retryAttempts+1, lastErr)
}

func (bus *EnhancedMessageBus) addSubscriptionToTopic(topic, subID string) {
	if subs, exists := bus.topicSubs[topic]; exists {
		bus.topicSubs[topic] = append(subs, subID)
	} else {
		bus.topicSubs[topic] = []string{subID}
	}
}

func (bus *EnhancedMessageBus) removeSubscriptionFromTopic(topic, subID string) {
	if subs, exists := bus.topicSubs[topic]; exists {
		for i, id := range subs {
			if id == subID {
				bus.topicSubs[topic] = append(subs[:i], subs[i+1:]...)
				break
			}
		}

		// 如果没有订阅者了，删除主题
		if len(bus.topicSubs[topic]) == 0 {
			delete(bus.topicSubs, topic)
		}
	}
}

func (bus *EnhancedMessageBus) cleanupLoop() {
	defer bus.wg.Done()

	ticker := time.NewTicker(bus.cleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-bus.ctx.Done():
			return
		case <-ticker.C:
			bus.cleanup()
		}
	}
}

func (bus *EnhancedMessageBus) cleanup() {
	// 清理过期消息
	cutoff := time.Now().Add(-24 * time.Hour) // 保留24小时
	if err := bus.store.Cleanup(cutoff); err != nil {
		logger.Error("Failed to cleanup messages", "error", err)
	}

	// 清理非活跃连接
	bus.mutex.Lock()
	for connID, conn := range bus.connections {
		if !conn.IsActive() {
			logger.Info("Removing inactive connection", "id", connID)
			conn.Close()
			delete(bus.connections, connID)

			// 移除相关订阅
			for subID, sub := range bus.subscriptions {
				if sub.ConnectionID == connID {
					bus.removeSubscriptionFromTopic(sub.Topic, subID)
					delete(bus.subscriptions, subID)
				}
			}
		}
	}
	bus.mutex.Unlock()

	logger.Debug("Cleanup completed", "connections", len(bus.connections), "subscriptions", len(bus.subscriptions))
}

// metricsLoop 指标收集循环
func (bus *EnhancedMessageBus) metricsLoop() {
	defer bus.wg.Done()

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-bus.ctx.Done():
			return
		case <-ticker.C:
			bus.updateMetrics()
		}
	}
}

// updateMetrics 更新指标
func (bus *EnhancedMessageBus) updateMetrics() {
	bus.mutex.RLock()
	connCount := len(bus.connections)
	subCount := len(bus.subscriptions)
	bus.mutex.RUnlock()

	atomic.StoreInt64(&bus.metrics.ActiveConnections, int64(connCount))
	atomic.StoreInt64(&bus.metrics.ActiveSubscriptions, int64(subCount))

	logger.Debug("Metrics updated", "connections", connCount, "subscriptions", subCount)
}

// GetMetrics 获取指标
func (bus *EnhancedMessageBus) GetMetrics() *BusMetrics {
	return &BusMetrics{
		MessagesPublished:   atomic.LoadInt64(&bus.metrics.MessagesPublished),
		MessagesDelivered:   atomic.LoadInt64(&bus.metrics.MessagesDelivered),
		MessagesFailed:      atomic.LoadInt64(&bus.metrics.MessagesFailed),
		ActiveConnections:   atomic.LoadInt64(&bus.metrics.ActiveConnections),
		ActiveSubscriptions: atomic.LoadInt64(&bus.metrics.ActiveSubscriptions),
		EventsPublished:     atomic.LoadInt64(&bus.metrics.EventsPublished),
		EventsDelivered:     atomic.LoadInt64(&bus.metrics.EventsDelivered),
		TotalLatency:        atomic.LoadInt64(&bus.metrics.TotalLatency),
		StartTime:           bus.metrics.StartTime,
	}
}

// PublishEvent 发布事件
func (bus *EnhancedMessageBus) PublishEvent(event *Event) error {
	if event.ID == "" {
		event.ID = generateID()
	}
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}

	atomic.AddInt64(&bus.metrics.EventsPublished, 1)

	bus.eventMutex.RLock()
	handlers, exists := bus.eventSubscriptions[event.Type]
	bus.eventMutex.RUnlock()

	if !exists {
		return nil // 没有订阅者
	}

	// 并发处理事件
	for _, handler := range handlers {
		go func(h EventHandler) {
			if err := h(event); err != nil {
				logger.Error("Event handler failed", "event", event.ID, "error", err)
			} else {
				atomic.AddInt64(&bus.metrics.EventsDelivered, 1)
			}
		}(handler)
	}

	return nil
}

// SubscribeEvent 订阅事件
func (bus *EnhancedMessageBus) SubscribeEvent(eventType string, handler EventHandler) (string, error) {
	subscriptionID := generateID()

	bus.eventMutex.Lock()
	if bus.eventSubscriptions[eventType] == nil {
		bus.eventSubscriptions[eventType] = make(map[string]EventHandler)
	}
	bus.eventSubscriptions[eventType][subscriptionID] = handler
	bus.eventMutex.Unlock()

	logger.Info("Event subscription created", "type", eventType, "id", subscriptionID)
	return subscriptionID, nil
}

// UnsubscribeEvent 取消事件订阅
func (bus *EnhancedMessageBus) UnsubscribeEvent(subscriptionID string) error {
	bus.eventMutex.Lock()
	defer bus.eventMutex.Unlock()

	for eventType, handlers := range bus.eventSubscriptions {
		if _, exists := handlers[subscriptionID]; exists {
			delete(handlers, subscriptionID)
			if len(handlers) == 0 {
				delete(bus.eventSubscriptions, eventType)
			}
			logger.Info("Event subscription removed", "id", subscriptionID)
			return nil
		}
	}

	return errors.New(errors.ErrCodeConfigNotFound, errors.CategorySystem, errors.LevelError, fmt.Sprintf("subscription not found: %s", subscriptionID))
}
