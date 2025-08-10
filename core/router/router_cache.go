package router

import (
	"context"
	"fmt"
	"path/filepath"
	"sync"
	"time"

	"github.com/cocowh/muxcore/pkg/logger"
)

// CacheEntry 缓存条目
type CacheEntry struct {
	Key        string      `json:"key"`
	Value      interface{} `json:"value"`
	ExpireTime time.Time   `json:"expire_time"`
	AccessTime time.Time   `json:"access_time"`
	HitCount   int64       `json:"hit_count"`
	Size       int64       `json:"size"`
}

// CacheStats 缓存统计
type CacheStats struct {
	Hits        int64   `json:"hits"`
	Misses      int64   `json:"misses"`
	Evictions   int64   `json:"evictions"`
	Size        int64   `json:"size"`
	EntryCount  int     `json:"entry_count"`
	HitRate     float64 `json:"hit_rate"`
	MemoryUsage int64   `json:"memory_usage"`
}

// EvictionPolicy 淘汰策略
type EvictionPolicy int

const (
	LRU EvictionPolicy = iota // 最近最少使用
	LFU                       // 最少使用频率
	TTL                       // 基于过期时间
	FIFO                      // 先进先出
)

// RouterCache 路由缓存
type RouterCache struct {
	config          *CacheConfig
	entries         map[string]*CacheEntry
	accessOrder     []*CacheEntry // LRU链表
	mutex           sync.RWMutex
	stats           *CacheStats
	running         bool
	ctx             context.Context
	cancel          context.CancelFunc
	monitor         *RouterMonitor
	evictionPolicy  EvictionPolicy
	maxMemoryUsage  int64
	currentMemory   int64
}

// NewRouterCache 创建路由缓存
func NewRouterCache(config *CacheConfig, monitor *RouterMonitor) *RouterCache {
	if config == nil {
		config = &CacheConfig{
			Enabled:        true,
			MaxSize:        10000,
			TTL:            time.Hour,
			Type:           "memory",
			EvictionPolicy: "lru",
		}
	}
	
	ctx, cancel := context.WithCancel(context.Background())
	
	return &RouterCache{
		config:         config,
		entries:        make(map[string]*CacheEntry),
		accessOrder:    make([]*CacheEntry, 0),
		stats:          &CacheStats{},
		ctx:            ctx,
		cancel:         cancel,
		monitor:        monitor,
		evictionPolicy: LRU,
		maxMemoryUsage: 100 * 1024 * 1024, // 默认100MB
	}
}

// Start 启动缓存
func (rc *RouterCache) Start() {
	if !rc.config.Enabled {
		return
	}
	
	rc.mutex.Lock()
	if rc.running {
		rc.mutex.Unlock()
		return
	}
	rc.running = true
	rc.mutex.Unlock()
	
	// 启动清理任务
	go rc.cleanupWorker()
	
	// 启动统计任务
	go rc.statsWorker()
	
	logger.Info("Router cache started")
}

// Stop 停止缓存
func (rc *RouterCache) Stop() {
	rc.mutex.Lock()
	defer rc.mutex.Unlock()
	
	if !rc.running {
		return
	}
	
	rc.running = false
	rc.cancel()
	
	logger.Info("Router cache stopped")
}

// Get 获取缓存值
func (rc *RouterCache) Get(key string) (interface{}, bool) {
	if !rc.config.Enabled {
		return nil, false
	}
	
	rc.mutex.Lock()
	defer rc.mutex.Unlock()
	
	entry, exists := rc.entries[key]
	if !exists {
		rc.stats.Misses++
		if rc.monitor != nil {
			rc.monitor.RecordCacheMiss()
		}
		return nil, false
	}
	
	// 检查是否过期
	if time.Now().After(entry.ExpireTime) {
		rc.removeEntry(key)
		rc.stats.Misses++
		if rc.monitor != nil {
			rc.monitor.RecordCacheMiss()
		}
		return nil, false
	}
	
	// 更新访问信息
	entry.AccessTime = time.Now()
	entry.HitCount++
	rc.stats.Hits++
	
	// 更新LRU顺序
	rc.updateAccessOrder(entry)
	
	if rc.monitor != nil {
		rc.monitor.RecordCacheHit()
	}
	
	return entry.Value, true
}

// Set 设置缓存值
func (rc *RouterCache) Set(key string, value interface{}, ttl time.Duration) {
	if !rc.config.Enabled {
		return
	}
	
	rc.mutex.Lock()
	defer rc.mutex.Unlock()
	
	// 计算条目大小（简化实现）
	entrySize := rc.estimateSize(value)
	
	// 检查内存限制
	if rc.currentMemory+entrySize > rc.maxMemoryUsage {
		rc.evictToMakeSpace(entrySize)
	}
	
	// 如果key已存在，先删除旧条目
	if existingEntry, exists := rc.entries[key]; exists {
		rc.currentMemory -= existingEntry.Size
		rc.removeFromAccessOrder(existingEntry)
	}
	
	// 创建新条目
	expireTime := time.Now().Add(ttl)
	if ttl == 0 {
		expireTime = time.Now().Add(rc.config.TTL)
	}
	
	entry := &CacheEntry{
		Key:        key,
		Value:      value,
		ExpireTime: expireTime,
		AccessTime: time.Now(),
		HitCount:   0,
		Size:       entrySize,
	}
	
	rc.entries[key] = entry
	rc.accessOrder = append(rc.accessOrder, entry)
	rc.currentMemory += entrySize
	
	// 检查大小限制
	if len(rc.entries) > rc.config.MaxSize {
		rc.evictOldest()
	}
	
	logger.Debugf("Cache set: key=%s, size=%d, expire=%v", key, entrySize, expireTime)
}

// Delete 删除缓存值
func (rc *RouterCache) Delete(key string) {
	rc.mutex.Lock()
	defer rc.mutex.Unlock()
	
	rc.removeEntry(key)
}

// Clear 清空缓存
func (rc *RouterCache) Clear() {
	rc.mutex.Lock()
	defer rc.mutex.Unlock()
	
	rc.entries = make(map[string]*CacheEntry)
	rc.accessOrder = make([]*CacheEntry, 0)
	rc.currentMemory = 0
	rc.stats = &CacheStats{}
	
	logger.Info("Cache cleared")
}

// GetStats 获取缓存统计
func (rc *RouterCache) GetStats() *CacheStats {
	rc.mutex.RLock()
	defer rc.mutex.RUnlock()
	
	totalRequests := rc.stats.Hits + rc.stats.Misses
	hitRate := 0.0
	if totalRequests > 0 {
		hitRate = float64(rc.stats.Hits) / float64(totalRequests)
	}
	
	return &CacheStats{
		Hits:        rc.stats.Hits,
		Misses:      rc.stats.Misses,
		Evictions:   rc.stats.Evictions,
		Size:        rc.stats.Size,
		EntryCount:  len(rc.entries),
		HitRate:     hitRate,
		MemoryUsage: rc.currentMemory,
	}
}

// GetKeys 获取所有缓存键
func (rc *RouterCache) GetKeys() []string {
	rc.mutex.RLock()
	defer rc.mutex.RUnlock()
	
	keys := make([]string, 0, len(rc.entries))
	for key := range rc.entries {
		keys = append(keys, key)
	}
	return keys
}

// GetEntries 获取所有缓存条目（用于调试）
func (rc *RouterCache) GetEntries() map[string]*CacheEntry {
	rc.mutex.RLock()
	defer rc.mutex.RUnlock()
	
	entries := make(map[string]*CacheEntry)
	for key, entry := range rc.entries {
		entries[key] = &CacheEntry{
			Key:        entry.Key,
			Value:      entry.Value,
			ExpireTime: entry.ExpireTime,
			AccessTime: entry.AccessTime,
			HitCount:   entry.HitCount,
			Size:       entry.Size,
		}
	}
	return entries
}

// SetEvictionPolicy 设置淘汰策略
func (rc *RouterCache) SetEvictionPolicy(policy EvictionPolicy) {
	rc.mutex.Lock()
	defer rc.mutex.Unlock()
	
	rc.evictionPolicy = policy
	logger.Infof("Cache eviction policy set to: %v", policy)
}

// removeEntry 删除条目
func (rc *RouterCache) removeEntry(key string) {
	entry, exists := rc.entries[key]
	if !exists {
		return
	}
	
	delete(rc.entries, key)
	rc.removeFromAccessOrder(entry)
	rc.currentMemory -= entry.Size
	rc.stats.Evictions++
}

// updateAccessOrder 更新访问顺序（LRU）
func (rc *RouterCache) updateAccessOrder(entry *CacheEntry) {
	// 移除旧位置
	rc.removeFromAccessOrder(entry)
	// 添加到末尾
	rc.accessOrder = append(rc.accessOrder, entry)
}

// removeFromAccessOrder 从访问顺序中移除
func (rc *RouterCache) removeFromAccessOrder(entry *CacheEntry) {
	for i, e := range rc.accessOrder {
		if e == entry {
			rc.accessOrder = append(rc.accessOrder[:i], rc.accessOrder[i+1:]...)
			break
		}
	}
}

// evictOldest 淘汰最旧的条目
func (rc *RouterCache) evictOldest() {
	if len(rc.accessOrder) == 0 {
		return
	}
	
	switch rc.evictionPolicy {
	case LRU:
		rc.evictLRU()
	case LFU:
		rc.evictLFU()
	case TTL:
		rc.evictExpired()
	case FIFO:
		rc.evictFIFO()
	default:
		rc.evictLRU()
	}
}

// evictLRU 淘汰最近最少使用的条目
func (rc *RouterCache) evictLRU() {
	if len(rc.accessOrder) > 0 {
		oldest := rc.accessOrder[0]
		rc.removeEntry(oldest.Key)
		logger.Debugf("Evicted LRU entry: %s", oldest.Key)
	}
}

// evictLFU 淘汰最少使用频率的条目
func (rc *RouterCache) evictLFU() {
	if len(rc.entries) == 0 {
		return
	}
	
	var leastUsed *CacheEntry
	var leastUsedKey string
	minHitCount := int64(-1)
	
	for key, entry := range rc.entries {
		if minHitCount == -1 || entry.HitCount < minHitCount {
			minHitCount = entry.HitCount
			leastUsed = entry
			leastUsedKey = key
		}
	}
	
	if leastUsed != nil {
		rc.removeEntry(leastUsedKey)
		logger.Debugf("Evicted LFU entry: %s (hit count: %d)", leastUsedKey, minHitCount)
	}
}

// evictExpired 淘汰过期的条目
func (rc *RouterCache) evictExpired() {
	now := time.Now()
	for key, entry := range rc.entries {
		if now.After(entry.ExpireTime) {
			rc.removeEntry(key)
			logger.Debugf("Evicted expired entry: %s", key)
			return
		}
	}
	
	// 如果没有过期条目，回退到LRU
	rc.evictLRU()
}

// evictFIFO 淘汰先进先出的条目
func (rc *RouterCache) evictFIFO() {
	// FIFO与LRU在这个简化实现中相同
	rc.evictLRU()
}

// evictToMakeSpace 淘汰条目以腾出空间
func (rc *RouterCache) evictToMakeSpace(neededSpace int64) {
	for rc.currentMemory+neededSpace > rc.maxMemoryUsage && len(rc.entries) > 0 {
		rc.evictOldest()
	}
}

// estimateSize 估算值的大小
func (rc *RouterCache) estimateSize(value interface{}) int64 {
	// 简化的大小估算
	switch v := value.(type) {
	case string:
		return int64(len(v))
	case []byte:
		return int64(len(v))
	case int, int32, int64, float32, float64:
		return 8
	case bool:
		return 1
	default:
		// 对于复杂类型，使用固定大小
		return 256
	}
}

// cleanupWorker 清理工作器
func (rc *RouterCache) cleanupWorker() {
	ticker := time.NewTicker(time.Minute * 10) // 默认10分钟清理间隔
	defer ticker.Stop()
	
	for {
		select {
		case <-rc.ctx.Done():
			return
		case <-ticker.C:
			rc.cleanup()
		}
	}
}

// statsWorker 统计工作器
func (rc *RouterCache) statsWorker() {
	ticker := time.NewTicker(time.Minute) // 每分钟更新一次统计
	defer ticker.Stop()
	
	for {
		select {
		case <-rc.ctx.Done():
			return
		case <-ticker.C:
			rc.updateStats()
		}
	}
}

// cleanup 清理过期条目
func (rc *RouterCache) cleanup() {
	rc.mutex.Lock()
	defer rc.mutex.Unlock()
	
	now := time.Now()
	expiredKeys := make([]string, 0)
	
	// 收集过期的键
	for key, entry := range rc.entries {
		if now.After(entry.ExpireTime) {
			expiredKeys = append(expiredKeys, key)
		}
	}
	
	// 删除过期条目
	for _, key := range expiredKeys {
		rc.removeEntry(key)
	}
	
	if len(expiredKeys) > 0 {
		logger.Debugf("Cleaned up %d expired cache entries", len(expiredKeys))
	}
}

// updateStats 更新统计信息
func (rc *RouterCache) updateStats() {
	rc.mutex.RLock()
	defer rc.mutex.RUnlock()
	
	rc.stats.Size = int64(len(rc.entries))
	
	totalRequests := rc.stats.Hits + rc.stats.Misses
	if totalRequests > 0 {
		rc.stats.HitRate = float64(rc.stats.Hits) / float64(totalRequests)
	}
	
	logger.Debugf("Cache stats updated - Entries: %d, Hit Rate: %.2f%%, Memory: %d bytes",
		len(rc.entries), rc.stats.HitRate*100, rc.currentMemory)
}

// RouteCache 路由特定的缓存包装器
type RouteCache struct {
	cache   *RouterCache
	monitor *RouterMonitor
}

// NewRouteCache 创建路由缓存包装器
func NewRouteCache(config *CacheConfig, monitor *RouterMonitor) *RouteCache {
	return &RouteCache{
		cache:   NewRouterCache(config, monitor),
		monitor: monitor,
	}
}

// CacheRoute 缓存路由结果
func (rc *RouteCache) CacheRoute(method, path string, result interface{}) {
	key := fmt.Sprintf("%s:%s", method, path)
	rc.cache.Set(key, result, 0) // 使用默认TTL
}

// GetCachedRoute 获取缓存的路由结果
func (rc *RouteCache) GetCachedRoute(method, path string) (interface{}, bool) {
	key := fmt.Sprintf("%s:%s", method, path)
	return rc.cache.Get(key)
}

// InvalidateRoute 使路由缓存失效
func (rc *RouteCache) InvalidateRoute(method, path string) {
	key := fmt.Sprintf("%s:%s", method, path)
	rc.cache.Delete(key)
}

// InvalidatePattern 使匹配模式的路由缓存失效
func (rc *RouteCache) InvalidatePattern(pattern string) {
	keys := rc.cache.GetKeys()
	for _, key := range keys {
		if matched, _ := filepath.Match(pattern, key); matched {
			rc.cache.Delete(key)
		}
	}
}

// Start 启动路由缓存
func (rc *RouteCache) Start() {
	rc.cache.Start()
}

// Stop 停止路由缓存
func (rc *RouteCache) Stop() {
	rc.cache.Stop()
}

// GetStats 获取缓存统计
func (rc *RouteCache) GetStats() *CacheStats {
	return rc.cache.GetStats()
}