package security

import (
	"context"
	"crypto/rand"
	"crypto/tls"
	"encoding/hex"
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cocowh/muxcore/pkg/logger"
)

// SecurityLevel 安全级别
type SecurityLevel int

const (
	SecurityLevelLow SecurityLevel = iota
	SecurityLevelMedium
	SecurityLevelHigh
	SecurityLevelCritical
)

// SecurityEvent 安全事件
type SecurityEvent struct {
	ID          string
	Type        string
	Level       SecurityLevel
	Source      string
	Target      string
	Description string
	Timestamp   time.Time
	Metadata    map[string]interface{}
}

// SecurityMetrics 安全指标
type SecurityMetrics struct {
	TotalConnections    int64
	BlockedConnections  int64
	TotalPackets        int64
	BlockedPackets      int64
	AuthAttempts        int64
	FailedAuthAttempts  int64
	SecurityEvents      int64
	LastEventTime       time.Time
}

// SecurityManager 安全管理器
// 整合纵深防御体系的各个组件
type SecurityManager struct {
	mutex          sync.RWMutex
	dosProtector   *DOSProtector
	synCookie      *SYNCookieProtector
	dpiEngine      *DPIEngine
	authManager    *AuthManager
	config         *SecurityConfig
	enabled        bool
	securityLevel  SecurityLevel
	metrics        *SecurityMetrics
	events         chan *SecurityEvent
	eventHandlers  []func(*SecurityEvent)
	tlsConfig      *tls.Config
	whitelist      map[string]bool
	blacklist      map[string]bool
	listMutex      sync.RWMutex
}

// SecurityConfig 安全配置
type SecurityConfig struct {
	// DoS防护配置
	DOSThreshold        int
	DOSWindowSize       time.Duration
	DOSCleanupInterval  time.Duration
	DOSBlockDuration    time.Duration

	// SYN Cookie配置
	SYNCookieCleanupInterval time.Duration

	// DPI配置
	DPIMaxPacketSize int

	// 权限管理配置
	DefaultAuthPolicy string

	// 新增配置
	SecurityLevel       SecurityLevel
	EventBufferSize     int
	MetricsInterval     time.Duration
	TLSMinVersion       uint16
	TLSCipherSuites     []uint16
	EnableWhitelist     bool
	EnableBlacklist     bool
	MaxEventHandlers    int
	AuditLogEnabled     bool
	RateLimitEnabled    bool
	GeoBlockingEnabled  bool
}

// NewSecurityManager 创建安全管理器实例
func NewSecurityManager(config SecurityConfig) *SecurityManager {
	// 设置默认值
	if config.EventBufferSize == 0 {
		config.EventBufferSize = 1000
	}
	if config.MetricsInterval == 0 {
		config.MetricsInterval = 60 * time.Second
	}
	if config.MaxEventHandlers == 0 {
		config.MaxEventHandlers = 10
	}

	// 初始化各安全组件
	dosProtector := NewDOSProtector(
		config.DOSThreshold,
		config.DOSWindowSize,
		config.DOSCleanupInterval,
		config.DOSBlockDuration,
	)

	synCookie := NewSYNCookieProtector(config.SYNCookieCleanupInterval)
	dpiEngine := NewDPIEngine(config.DPIMaxPacketSize)
	authManager := NewAuthManager(config.DefaultAuthPolicy)

	// 配置TLS
	tlsConfig := &tls.Config{
		MinVersion: config.TLSMinVersion,
		CipherSuites: config.TLSCipherSuites,
	}
	if tlsConfig.MinVersion == 0 {
		tlsConfig.MinVersion = tls.VersionTLS12
	}

	// 创建安全管理器
	manager := &SecurityManager{
		dosProtector:   dosProtector,
		synCookie:      synCookie,
		dpiEngine:      dpiEngine,
		authManager:    authManager,
		config:         &config,
		enabled:        true,
		securityLevel:  config.SecurityLevel,
		metrics:        &SecurityMetrics{},
		events:         make(chan *SecurityEvent, config.EventBufferSize),
		eventHandlers:  make([]func(*SecurityEvent), 0, config.MaxEventHandlers),
		tlsConfig:      tlsConfig,
		whitelist:      make(map[string]bool),
		blacklist:      make(map[string]bool),
	}

	// 启动事件处理器
	go manager.eventProcessor()

	// 启动指标收集器
	go manager.metricsCollector()

	logger.Info("Security manager initialized with security level: ", config.SecurityLevel)
	return manager
}

// CheckConnectionSecurity 检查连接安全
func (sm *SecurityManager) CheckConnectionSecurity(conn *net.TCPConn, ip, userAgent, protocol string) bool {
	if !sm.enabled {
		return true
	}

	// 1. 检查DoS防护
	if !sm.dosProtector.CheckFingerprint(ip, userAgent, protocol) {
		logger.Warnf("Connection rejected by DoS protection: %s", conn.RemoteAddr().String())
		return false
	}

	// 2. 检查SYN Cookie (如果是TCP连接)
	// 这里简化处理，实际应在TCP握手阶段进行

	return true
}

// CheckPacketSecurity 检查数据包安全
func (sm *SecurityManager) CheckPacketSecurity(data []byte, protocol string) (bool, string) {
	if !sm.enabled {
		return true, ""
	}

	// 3. 深度包检测
	blocked, action, ruleID := sm.dpiEngine.Detect(data, protocol)
	if blocked {
		logger.Warnf("Packet blocked by DPI: rule=%s, action=%s", ruleID, action)
		return false, action
	}

	return true, ""
}

// CheckPermission 检查权限
func (sm *SecurityManager) CheckPermission(ctx context.Context, userID string, permission Permission) bool {
	if !sm.enabled {
		return true
	}

	// 4. 应用级权限校验
	return sm.authManager.CheckPermission(ctx, userID, permission)
}

// Enable 启用安全管理器
func (sm *SecurityManager) Enable() {
	sm.mutex.Lock()
	sm.enabled = true
	sm.dosProtector.Enable()
	sm.synCookie.Enable()
	sm.dpiEngine.Enable()
	sm.authManager.Enable()
	sm.mutex.Unlock()

	logger.Infof("Security manager enabled")
}

// Disable 禁用安全管理器
func (sm *SecurityManager) Disable() {
	sm.mutex.Lock()
	sm.enabled = false
	sm.dosProtector.Disable()
	sm.synCookie.Disable()
	sm.dpiEngine.Disable()
	sm.authManager.Disable()
	sm.mutex.Unlock()

	logger.Infof("Security manager disabled")
}

// GetAuthManager 获取权限管理器
func (sm *SecurityManager) GetAuthManager() *AuthManager {
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()

	return sm.authManager
}

// GetDPIEngine 获取DPI引擎
func (sm *SecurityManager) GetDPIEngine() *DPIEngine {
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()

	return sm.dpiEngine
}

// GetDOSProtector 获取DoS防护器
func (sm *SecurityManager) GetDOSProtector() *DOSProtector {
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()

	return sm.dosProtector
}

// GetSYNCookieProtector 获取SYN Cookie防护器
func (sm *SecurityManager) GetSYNCookieProtector() *SYNCookieProtector {
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()

	return sm.synCookie
}

// eventProcessor 事件处理器
func (sm *SecurityManager) eventProcessor() {
	for event := range sm.events {
		// 更新指标
		atomic.AddInt64(&sm.metrics.SecurityEvents, 1)
		sm.metrics.LastEventTime = event.Timestamp
		
		// 调用事件处理器
		sm.mutex.RLock()
		handlers := make([]func(*SecurityEvent), len(sm.eventHandlers))
		copy(handlers, sm.eventHandlers)
		sm.mutex.RUnlock()
		
		for _, handler := range handlers {
			go func(h func(*SecurityEvent)) {
				defer func() {
					if r := recover(); r != nil {
						logger.Errorf("Security event handler panic: %v", r)
					}
				}()
				h(event)
			}(handler)
		}
		
		// 记录审计日志
		if sm.config.AuditLogEnabled {
			logger.Infof("Security Event [%s]: %s from %s to %s - %s", 
				event.Type, event.Description, event.Source, event.Target, event.ID)
		}
	}
}

// metricsCollector 指标收集器
func (sm *SecurityManager) metricsCollector() {
	ticker := time.NewTicker(sm.config.MetricsInterval)
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			sm.collectMetrics()
		}
	}
}

// collectMetrics 收集指标
func (sm *SecurityManager) collectMetrics() {
	// 这里可以添加更多指标收集逻辑
	logger.Debugf("Security metrics - Events: %d, Connections: %d, Blocked: %d",
		atomic.LoadInt64(&sm.metrics.SecurityEvents),
		atomic.LoadInt64(&sm.metrics.TotalConnections),
		atomic.LoadInt64(&sm.metrics.BlockedConnections))
}

// AddEventHandler 添加事件处理器
func (sm *SecurityManager) AddEventHandler(handler func(*SecurityEvent)) error {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()
	
	if len(sm.eventHandlers) >= sm.config.MaxEventHandlers {
		return fmt.Errorf("maximum event handlers reached: %d", sm.config.MaxEventHandlers)
	}
	
	sm.eventHandlers = append(sm.eventHandlers, handler)
	return nil
}

// RemoveEventHandler 移除事件处理器
func (sm *SecurityManager) RemoveEventHandler(handler func(*SecurityEvent)) {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()
	
	for i, h := range sm.eventHandlers {
		if fmt.Sprintf("%p", h) == fmt.Sprintf("%p", handler) {
			sm.eventHandlers = append(sm.eventHandlers[:i], sm.eventHandlers[i+1:]...)
			break
		}
	}
}

// PublishEvent 发布安全事件
func (sm *SecurityManager) PublishEvent(eventType, source, target, description string, level SecurityLevel, metadata map[string]interface{}) {
	event := &SecurityEvent{
		ID:          sm.generateEventID(),
		Type:        eventType,
		Level:       level,
		Source:      source,
		Target:      target,
		Description: description,
		Timestamp:   time.Now(),
		Metadata:    metadata,
	}
	
	select {
	case sm.events <- event:
	default:
		logger.Warnf("Security event buffer full, dropping event: %s", event.ID)
	}
}

// generateEventID 生成事件ID
func (sm *SecurityManager) generateEventID() string {
	bytes := make([]byte, 16)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)
}

// GetMetrics 获取安全指标
func (sm *SecurityManager) GetMetrics() *SecurityMetrics {
	return &SecurityMetrics{
		TotalConnections:   atomic.LoadInt64(&sm.metrics.TotalConnections),
		BlockedConnections: atomic.LoadInt64(&sm.metrics.BlockedConnections),
		TotalPackets:       atomic.LoadInt64(&sm.metrics.TotalPackets),
		BlockedPackets:     atomic.LoadInt64(&sm.metrics.BlockedPackets),
		AuthAttempts:       atomic.LoadInt64(&sm.metrics.AuthAttempts),
		FailedAuthAttempts: atomic.LoadInt64(&sm.metrics.FailedAuthAttempts),
		SecurityEvents:     atomic.LoadInt64(&sm.metrics.SecurityEvents),
		LastEventTime:      sm.metrics.LastEventTime,
	}
}

// AddToWhitelist 添加到白名单
func (sm *SecurityManager) AddToWhitelist(ip string) {
	sm.listMutex.Lock()
	defer sm.listMutex.Unlock()
	
	sm.whitelist[ip] = true
	logger.Infof("Added %s to whitelist", ip)
}

// RemoveFromWhitelist 从白名单移除
func (sm *SecurityManager) RemoveFromWhitelist(ip string) {
	sm.listMutex.Lock()
	defer sm.listMutex.Unlock()
	
	delete(sm.whitelist, ip)
	logger.Infof("Removed %s from whitelist", ip)
}

// AddToBlacklist 添加到黑名单
func (sm *SecurityManager) AddToBlacklist(ip string) {
	sm.listMutex.Lock()
	defer sm.listMutex.Unlock()
	
	sm.blacklist[ip] = true
	logger.Infof("Added %s to blacklist", ip)
}

// RemoveFromBlacklist 从黑名单移除
func (sm *SecurityManager) RemoveFromBlacklist(ip string) {
	sm.listMutex.Lock()
	defer sm.listMutex.Unlock()
	
	delete(sm.blacklist, ip)
	logger.Infof("Removed %s from blacklist", ip)
}

// IsWhitelisted 检查是否在白名单
func (sm *SecurityManager) IsWhitelisted(ip string) bool {
	sm.listMutex.RLock()
	defer sm.listMutex.RUnlock()
	
	return sm.whitelist[ip]
}

// IsBlacklisted 检查是否在黑名单
func (sm *SecurityManager) IsBlacklisted(ip string) bool {
	sm.listMutex.RLock()
	defer sm.listMutex.RUnlock()
	
	return sm.blacklist[ip]
}

// UpdateSecurityLevel 更新安全级别
func (sm *SecurityManager) UpdateSecurityLevel(level SecurityLevel) {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()
	
	sm.securityLevel = level
	sm.config.SecurityLevel = level
	
	// 根据安全级别调整各组件配置
	switch level {
	case SecurityLevelLow:
		sm.config.DOSThreshold = sm.config.DOSThreshold * 2
	case SecurityLevelMedium:
		// 保持默认阈值
	case SecurityLevelHigh:
		sm.config.DOSThreshold = sm.config.DOSThreshold / 2
	case SecurityLevelCritical:
		sm.config.DOSThreshold = sm.config.DOSThreshold / 4
	}
	
	logger.Infof("Security level updated to: %d", level)
	sm.PublishEvent("security.level.changed", "system", "security_manager", 
		fmt.Sprintf("Security level changed to %d", level), SecurityLevelMedium, nil)
}

// GetSecurityLevel 获取当前安全级别
func (sm *SecurityManager) GetSecurityLevel() SecurityLevel {
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()
	return sm.securityLevel
}

// ValidateSecurityPolicy 验证安全策略
func (sm *SecurityManager) ValidateSecurityPolicy(policy map[string]interface{}) error {
	// 验证策略配置的有效性
	if policy == nil {
		return fmt.Errorf("security policy cannot be nil")
	}
	
	// 验证必需字段
	requiredFields := []string{"dos_protection", "dpi_rules", "auth_policy"}
	for _, field := range requiredFields {
		if _, exists := policy[field]; !exists {
			return fmt.Errorf("missing required field: %s", field)
		}
	}
	
	return nil
}

// ApplySecurityPolicy 应用安全策略
func (sm *SecurityManager) ApplySecurityPolicy(policy map[string]interface{}) error {
	if err := sm.ValidateSecurityPolicy(policy); err != nil {
		return fmt.Errorf("invalid security policy: %w", err)
	}
	
	sm.mutex.Lock()
	defer sm.mutex.Unlock()
	
	// 应用DoS防护策略
	if dosConfig, ok := policy["dos_protection"].(map[string]interface{}); ok {
		if threshold, exists := dosConfig["threshold"]; exists {
			if t, ok := threshold.(int); ok {
				sm.config.DOSThreshold = t
			}
		}
	}
	
	// 应用DPI规则
	if dpiRules, ok := policy["dpi_rules"].([]interface{}); ok {
		for _, rule := range dpiRules {
			if ruleMap, ok := rule.(map[string]interface{}); ok {
				// 这里可以添加DPI规则应用逻辑
				logger.Debugf("Applying DPI rule: %v", ruleMap)
			}
		}
	}
	
	// 应用认证策略
	if authPolicy, ok := policy["auth_policy"].(string); ok {
		sm.config.DefaultAuthPolicy = authPolicy
		// 注意：AuthManager当前没有UpdatePolicy方法，这里只更新配置
	}
	
	logger.Infof("Security policy applied successfully")
	sm.PublishEvent("security.policy.applied", "system", "security_manager", 
		"Security policy updated", SecurityLevelMedium, policy)
	
	return nil
}

// GetTLSConfig 获取TLS配置
func (sm *SecurityManager) GetTLSConfig() *tls.Config {
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()
	return sm.tlsConfig.Clone()
}

// UpdateTLSConfig 更新TLS配置
func (sm *SecurityManager) UpdateTLSConfig(config *tls.Config) {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()
	sm.tlsConfig = config.Clone()
	logger.Infof("TLS configuration updated")
}

// Close 关闭安全管理器
func (sm *SecurityManager) Close() error {
	sm.Disable()
	close(sm.events)
	return nil
}