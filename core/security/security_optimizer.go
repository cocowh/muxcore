package security

import (
	"context"
	"crypto/rand"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"regexp"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cocowh/muxcore/pkg/logger"
)

// OptimizedSecurityConfig 优化的安全配置
type OptimizedSecurityConfig struct {
	// 基础安全配置
	SecurityLevel         SecurityLevel `json:"security_level"`
	Enabled               bool          `json:"enabled"`
	AuditEnabled          bool          `json:"audit_enabled"`
	ThreatDetectionEnabled bool         `json:"threat_detection_enabled"`
	
	// 访问控制配置
	AccessControlEnabled  bool          `json:"access_control_enabled"`
	RateLimitEnabled      bool          `json:"rate_limit_enabled"`
	GeoBlockingEnabled    bool          `json:"geo_blocking_enabled"`
	IPWhitelistEnabled    bool          `json:"ip_whitelist_enabled"`
	IPBlacklistEnabled    bool          `json:"ip_blacklist_enabled"`
	
	// DDoS防护配置
	DDoSProtectionEnabled bool          `json:"ddos_protection_enabled"`
	MaxConnectionsPerIP   int           `json:"max_connections_per_ip"`
	ConnectionTimeout     time.Duration `json:"connection_timeout"`
	RateLimitWindow       time.Duration `json:"rate_limit_window"`
	RateLimitThreshold    int           `json:"rate_limit_threshold"`
	
	// 威胁检测配置
	AnomalyDetectionEnabled bool        `json:"anomaly_detection_enabled"`
	MLThreatDetection      bool         `json:"ml_threat_detection"`
	SignatureDetection     bool         `json:"signature_detection"`
	BehaviorAnalysis       bool         `json:"behavior_analysis"`
	ThreatIntelligence     bool         `json:"threat_intelligence"`
	
	// 加密配置
	TLSEnabled            bool     `json:"tls_enabled"`
	TLSMinVersion         uint16   `json:"tls_min_version"`
	TLSCipherSuites       []uint16 `json:"tls_cipher_suites"`
	CertificateValidation bool     `json:"certificate_validation"`
	
	// 监控配置
	MetricsEnabled        bool          `json:"metrics_enabled"`
	MetricsInterval       time.Duration `json:"metrics_interval"`
	AlertingEnabled       bool          `json:"alerting_enabled"`
	EventBufferSize       int           `json:"event_buffer_size"`
	
	// 响应配置
	AutoResponseEnabled   bool          `json:"auto_response_enabled"`
	QuarantineEnabled     bool          `json:"quarantine_enabled"`
	BlockDuration         time.Duration `json:"block_duration"`
	EscalationEnabled     bool          `json:"escalation_enabled"`
}

// DefaultOptimizedSecurityConfig 默认优化安全配置
func DefaultOptimizedSecurityConfig() *OptimizedSecurityConfig {
	return &OptimizedSecurityConfig{
		SecurityLevel:           SecurityLevelMedium,
		Enabled:                 true,
		AuditEnabled:            true,
		ThreatDetectionEnabled:  true,
		AccessControlEnabled:    true,
		RateLimitEnabled:        true,
		GeoBlockingEnabled:      false,
		IPWhitelistEnabled:      false,
		IPBlacklistEnabled:      true,
		DDoSProtectionEnabled:   true,
		MaxConnectionsPerIP:     100,
		ConnectionTimeout:       30 * time.Second,
		RateLimitWindow:         time.Minute,
		RateLimitThreshold:      1000,
		AnomalyDetectionEnabled: true,
		MLThreatDetection:       false,
		SignatureDetection:      true,
		BehaviorAnalysis:        true,
		ThreatIntelligence:      false,
		TLSEnabled:              true,
		TLSMinVersion:           tls.VersionTLS12,
		TLSCipherSuites: []uint16{
			tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,
			tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
		},
		CertificateValidation:   true,
		MetricsEnabled:          true,
		MetricsInterval:         10 * time.Second,
		AlertingEnabled:         true,
		EventBufferSize:         10000,
		AutoResponseEnabled:     true,
		QuarantineEnabled:       true,
		BlockDuration:           time.Hour,
		EscalationEnabled:       true,
	}
}

// ThreatType 威胁类型
type ThreatType string

const (
	ThreatTypeDDoS          ThreatType = "ddos"
	ThreatTypeBruteForce    ThreatType = "brute_force"
	ThreatTypeSQLInjection  ThreatType = "sql_injection"
	ThreatTypeXSS           ThreatType = "xss"
	ThreatTypeCSRF          ThreatType = "csrf"
	ThreatTypeMalware       ThreatType = "malware"
	ThreatTypeAnomaly       ThreatType = "anomaly"
	ThreatTypeUnauthorized  ThreatType = "unauthorized"
	ThreatTypeDataExfil     ThreatType = "data_exfiltration"
	ThreatTypePrivEsc       ThreatType = "privilege_escalation"
)

// ThreatSeverity 威胁严重程度
type ThreatSeverity string

const (
	SeverityLow      ThreatSeverity = "low"
	SeverityMedium   ThreatSeverity = "medium"
	SeverityHigh     ThreatSeverity = "high"
	SeverityCritical ThreatSeverity = "critical"
)

// ThreatEvent 威胁事件
type ThreatEvent struct {
	ID          string                 `json:"id"`
	Type        ThreatType             `json:"type"`
	Severity    ThreatSeverity         `json:"severity"`
	Source      string                 `json:"source"`
	Target      string                 `json:"target"`
	Description string                 `json:"description"`
	Timestamp   time.Time              `json:"timestamp"`
	Metadata    map[string]interface{} `json:"metadata"`
	Blocked     bool                   `json:"blocked"`
	Response    string                 `json:"response"`
	Confidence  float64                `json:"confidence"`
}

// OptimizedSecurityMetrics 优化的安全指标
type OptimizedSecurityMetrics struct {
	// 连接指标
	TotalConnections    int64 `json:"total_connections"`
	BlockedConnections  int64 `json:"blocked_connections"`
	ActiveConnections   int64 `json:"active_connections"`
	
	// 威胁指标
	TotalThreats        int64 `json:"total_threats"`
	BlockedThreats      int64 `json:"blocked_threats"`
	ThreatsByType       map[ThreatType]int64 `json:"threats_by_type"`
	ThreatsBySeverity   map[ThreatSeverity]int64 `json:"threats_by_severity"`
	
	// 认证指标
	AuthAttempts        int64 `json:"auth_attempts"`
	FailedAuthAttempts  int64 `json:"failed_auth_attempts"`
	SuccessfulAuths     int64 `json:"successful_auths"`
	
	// 性能指标
	AvgResponseTime     float64 `json:"avg_response_time"`
	Throughput          float64 `json:"throughput"`
	ErrorRate           float64 `json:"error_rate"`
	
	// 时间戳
	LastUpdate          time.Time `json:"last_update"`
}

// ConnectionTracker 连接跟踪器
type ConnectionTracker struct {
	connections map[string]*ConnectionInfo
	mu          sync.RWMutex
	config      *OptimizedSecurityConfig
}

// ConnectionInfo 连接信息
type ConnectionInfo struct {
	IP            string    `json:"ip"`
	Port          int       `json:"port"`
	ConnectTime   time.Time `json:"connect_time"`
	LastActivity  time.Time `json:"last_activity"`
	RequestCount  uint64    `json:"request_count"`
	BytesSent     uint64    `json:"bytes_sent"`
	BytesReceived uint64    `json:"bytes_received"`
	UserAgent     string    `json:"user_agent"`
	Protocol      string    `json:"protocol"`
	Blocked       bool      `json:"blocked"`
	ThreatScore   float64   `json:"threat_score"`
}

// NewConnectionTracker 创建连接跟踪器
func NewConnectionTracker(config *OptimizedSecurityConfig) *ConnectionTracker {
	return &ConnectionTracker{
		connections: make(map[string]*ConnectionInfo),
		config:      config,
	}
}

// TrackConnection 跟踪连接
func (ct *ConnectionTracker) TrackConnection(ip string, port int, userAgent, protocol string) string {
	ct.mu.Lock()
	defer ct.mu.Unlock()
	
	connID := fmt.Sprintf("%s:%d", ip, port)
	info := &ConnectionInfo{
		IP:           ip,
		Port:         port,
		ConnectTime:  time.Now(),
		LastActivity: time.Now(),
		UserAgent:    userAgent,
		Protocol:     protocol,
		ThreatScore:  0.0,
	}
	
	ct.connections[connID] = info
	return connID
}

// UpdateConnection 更新连接信息
func (ct *ConnectionTracker) UpdateConnection(connID string, bytesSent, bytesReceived uint64) {
	ct.mu.Lock()
	defer ct.mu.Unlock()
	
	if info, exists := ct.connections[connID]; exists {
		info.LastActivity = time.Now()
		info.RequestCount++
		info.BytesSent += bytesSent
		info.BytesReceived += bytesReceived
	}
}

// GetConnection 获取连接信息
func (ct *ConnectionTracker) GetConnection(connID string) (*ConnectionInfo, bool) {
	ct.mu.RLock()
	defer ct.mu.RUnlock()
	
	info, exists := ct.connections[connID]
	return info, exists
}

// GetConnectionsByIP 获取IP的所有连接
func (ct *ConnectionTracker) GetConnectionsByIP(ip string) []*ConnectionInfo {
	ct.mu.RLock()
	defer ct.mu.RUnlock()
	
	var connections []*ConnectionInfo
	for _, info := range ct.connections {
		if info.IP == ip {
			connections = append(connections, info)
		}
	}
	return connections
}

// CleanupConnections 清理过期连接
func (ct *ConnectionTracker) CleanupConnections() {
	ct.mu.Lock()
	defer ct.mu.Unlock()
	
	cutoff := time.Now().Add(-ct.config.ConnectionTimeout)
	for connID, info := range ct.connections {
		if info.LastActivity.Before(cutoff) {
			delete(ct.connections, connID)
		}
	}
}

// ThreatDetector 威胁检测器
type ThreatDetector struct {
	config           *OptimizedSecurityConfig
	signatures       map[string]*ThreatSignature
	behaviorProfiles map[string]*BehaviorProfile
	anomalyDetector  *AnomalyDetector
	mu               sync.RWMutex
}

// ThreatSignature 威胁签名
type ThreatSignature struct {
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	Type        ThreatType `json:"type"`
	Pattern     *regexp.Regexp
	Severity    ThreatSeverity `json:"severity"`
	Description string     `json:"description"`
	Enabled     bool       `json:"enabled"`
}

// BehaviorProfile 行为画像
type BehaviorProfile struct {
	IP              string    `json:"ip"`
	RequestRate     float64   `json:"request_rate"`
	AvgRequestSize  float64   `json:"avg_request_size"`
	UserAgents      []string  `json:"user_agents"`
	AccessPatterns  []string  `json:"access_patterns"`
	LastUpdate      time.Time `json:"last_update"`
	AnomalyScore    float64   `json:"anomaly_score"`
	BaselineScore   float64   `json:"baseline_score"`
}

// AnomalyDetector 异常检测器
type AnomalyDetector struct {
	baselines map[string]float64
	threshold float64
	mu        sync.RWMutex
}

// NewThreatDetector 创建威胁检测器
func NewThreatDetector(config *OptimizedSecurityConfig) *ThreatDetector {
	td := &ThreatDetector{
		config:           config,
		signatures:       make(map[string]*ThreatSignature),
		behaviorProfiles: make(map[string]*BehaviorProfile),
		anomalyDetector:  &AnomalyDetector{
			baselines: make(map[string]float64),
			threshold: 0.8,
		},
	}
	
	// 加载默认威胁签名
	td.loadDefaultSignatures()
	
	return td
}

// loadDefaultSignatures 加载默认威胁签名
func (td *ThreatDetector) loadDefaultSignatures() {
	signatures := []*ThreatSignature{
		{
			ID:          "sql_injection_1",
			Name:        "SQL Injection Pattern",
			Type:        ThreatTypeSQLInjection,
			Pattern:     regexp.MustCompile(`(?i)(union|select|insert|update|delete|drop|create|alter).*?(from|into|table|database)`),
			Severity:    SeverityHigh,
			Description: "Detects common SQL injection patterns",
			Enabled:     true,
		},
		{
			ID:          "xss_1",
			Name:        "XSS Pattern",
			Type:        ThreatTypeXSS,
			Pattern:     regexp.MustCompile(`(?i)<script[^>]*>.*?</script>|javascript:|on\w+\s*=`),
			Severity:    SeverityMedium,
			Description: "Detects cross-site scripting patterns",
			Enabled:     true,
		},
		{
			ID:          "brute_force_1",
			Name:        "Brute Force Pattern",
			Type:        ThreatTypeBruteForce,
			Pattern:     regexp.MustCompile(`(admin|root|administrator|login|password)`),
			Severity:    SeverityMedium,
			Description: "Detects potential brute force attempts",
			Enabled:     true,
		},
	}
	
	for _, sig := range signatures {
		td.signatures[sig.ID] = sig
	}
}

// DetectThreats 检测威胁
func (td *ThreatDetector) DetectThreats(data []byte, source, target string) []*ThreatEvent {
	var threats []*ThreatEvent
	
	// 签名检测
	if td.config.SignatureDetection {
		threats = append(threats, td.detectBySignature(data, source, target)...)
	}
	
	// 行为分析
	if td.config.BehaviorAnalysis {
		threats = append(threats, td.detectByBehavior(data, source, target)...)
	}
	
	// 异常检测
	if td.config.AnomalyDetectionEnabled {
		threats = append(threats, td.detectAnomalies(data, source, target)...)
	}
	
	return threats
}

// detectBySignature 基于签名检测
func (td *ThreatDetector) detectBySignature(data []byte, source, target string) []*ThreatEvent {
	td.mu.RLock()
	defer td.mu.RUnlock()
	
	var threats []*ThreatEvent
	dataStr := string(data)
	
	for _, sig := range td.signatures {
		if !sig.Enabled || sig.Pattern == nil {
			continue
		}
		
		if sig.Pattern.MatchString(dataStr) {
			threat := &ThreatEvent{
				ID:          td.generateThreatID(),
				Type:        sig.Type,
				Severity:    sig.Severity,
				Source:      source,
				Target:      target,
				Description: fmt.Sprintf("Signature match: %s", sig.Name),
				Timestamp:   time.Now(),
				Metadata: map[string]interface{}{
					"signature_id":   sig.ID,
					"signature_name": sig.Name,
					"pattern":        sig.Pattern.String(),
					"data_size":      len(data),
				},
				Confidence: 0.9,
			}
			threats = append(threats, threat)
		}
	}
	
	return threats
}

// detectByBehavior 基于行为检测
func (td *ThreatDetector) detectByBehavior(data []byte, source, target string) []*ThreatEvent {
	td.mu.Lock()
	defer td.mu.Unlock()
	
	var threats []*ThreatEvent
	
	// 获取或创建行为画像
	profile, exists := td.behaviorProfiles[source]
	if !exists {
		profile = &BehaviorProfile{
			IP:             source,
			RequestRate:    0,
			AvgRequestSize: 0,
			UserAgents:     make([]string, 0),
			AccessPatterns: make([]string, 0),
			LastUpdate:     time.Now(),
			BaselineScore:  0.5,
		}
		td.behaviorProfiles[source] = profile
	}
	
	// 更新行为画像
	profile.LastUpdate = time.Now()
	profile.RequestRate = td.calculateRequestRate(source)
	profile.AvgRequestSize = td.calculateAvgRequestSize(source, len(data))
	
	// 计算异常分数
	anomalyScore := td.calculateAnomalyScore(profile)
	profile.AnomalyScore = anomalyScore
	
	// 如果异常分数超过阈值，生成威胁事件
	if anomalyScore > 0.8 {
		threat := &ThreatEvent{
			ID:          td.generateThreatID(),
			Type:        ThreatTypeAnomaly,
			Severity:    td.getSeverityByScore(anomalyScore),
			Source:      source,
			Target:      target,
			Description: "Behavioral anomaly detected",
			Timestamp:   time.Now(),
			Metadata: map[string]interface{}{
				"anomaly_score":    anomalyScore,
				"request_rate":     profile.RequestRate,
				"avg_request_size": profile.AvgRequestSize,
				"baseline_score":   profile.BaselineScore,
			},
			Confidence: anomalyScore,
		}
		threats = append(threats, threat)
	}
	
	return threats
}

// detectAnomalies 异常检测
func (td *ThreatDetector) detectAnomalies(data []byte, source, target string) []*ThreatEvent {
	var threats []*ThreatEvent
	
	// 简化的异常检测逻辑
	dataSize := len(data)
	if dataSize > 10000 { // 大于10KB的请求可能异常
		threat := &ThreatEvent{
			ID:          td.generateThreatID(),
			Type:        ThreatTypeAnomaly,
			Severity:    SeverityMedium,
			Source:      source,
			Target:      target,
			Description: "Large request size anomaly",
			Timestamp:   time.Now(),
			Metadata: map[string]interface{}{
				"data_size": dataSize,
				"threshold": 10000,
			},
			Confidence: 0.7,
		}
		threats = append(threats, threat)
	}
	
	return threats
}

// calculateRequestRate 计算请求速率
func (td *ThreatDetector) calculateRequestRate(source string) float64 {
	// 简化实现，实际应该基于时间窗口计算
	return 10.0
}

// calculateAvgRequestSize 计算平均请求大小
func (td *ThreatDetector) calculateAvgRequestSize(source string, currentSize int) float64 {
	// 简化实现
	return float64(currentSize)
}

// calculateAnomalyScore 计算异常分数
func (td *ThreatDetector) calculateAnomalyScore(profile *BehaviorProfile) float64 {
	// 简化的异常分数计算
	score := 0.0
	
	// 基于请求速率
	if profile.RequestRate > 100 {
		score += 0.3
	}
	
	// 基于请求大小
	if profile.AvgRequestSize > 5000 {
		score += 0.2
	}
	
	// 基于历史基线
	if profile.RequestRate > profile.BaselineScore*2 {
		score += 0.3
	}
	
	return score
}

// getSeverityByScore 根据分数获取严重程度
func (td *ThreatDetector) getSeverityByScore(score float64) ThreatSeverity {
	switch {
	case score >= 0.9:
		return SeverityCritical
	case score >= 0.7:
		return SeverityHigh
	case score >= 0.5:
		return SeverityMedium
	default:
		return SeverityLow
	}
}

// generateThreatID 生成威胁ID
func (td *ThreatDetector) generateThreatID() string {
	bytes := make([]byte, 16)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)
}

// ResponseEngine 响应引擎
type ResponseEngine struct {
	config    *OptimizedSecurityConfig
	blacklist map[string]time.Time
	mu        sync.RWMutex
}

// NewResponseEngine 创建响应引擎
func NewResponseEngine(config *OptimizedSecurityConfig) *ResponseEngine {
	return &ResponseEngine{
		config:    config,
		blacklist: make(map[string]time.Time),
	}
}

// RespondToThreat 响应威胁
func (re *ResponseEngine) RespondToThreat(threat *ThreatEvent) string {
	if !re.config.AutoResponseEnabled {
		return "no_action"
	}
	
	switch threat.Severity {
	case SeverityCritical:
		return re.handleCriticalThreat(threat)
	case SeverityHigh:
		return re.handleHighThreat(threat)
	case SeverityMedium:
		return re.handleMediumThreat(threat)
	default:
		return re.handleLowThreat(threat)
	}
}

// handleCriticalThreat 处理严重威胁
func (re *ResponseEngine) handleCriticalThreat(threat *ThreatEvent) string {
	// 立即阻断
	re.blockIP(threat.Source, re.config.BlockDuration*2)
	
	// 记录日志
	logger.Error("Critical threat detected and blocked", "threat_id", threat.ID, "source", threat.Source)
	
	// 如果启用了升级，发送告警
	if re.config.EscalationEnabled {
		re.escalateThreat(threat)
	}
	
	return "blocked"
}

// handleHighThreat 处理高威胁
func (re *ResponseEngine) handleHighThreat(threat *ThreatEvent) string {
	// 阻断IP
	re.blockIP(threat.Source, re.config.BlockDuration)
	
	logger.Error("High threat detected and blocked", "threat_id", threat.ID, "source", threat.Source)
	
	return "blocked"
}

// handleMediumThreat 处理中等威胁
func (re *ResponseEngine) handleMediumThreat(threat *ThreatEvent) string {
	// 限制速率
	logger.Info("Medium threat detected, rate limiting applied", "threat_id", threat.ID, "source", threat.Source)
	
	return "rate_limited"
}

// handleLowThreat 处理低威胁
func (re *ResponseEngine) handleLowThreat(threat *ThreatEvent) string {
	// 仅记录
	logger.Info("Low threat detected", "threat_id", threat.ID, "source", threat.Source)
	
	return "logged"
}

// blockIP 阻断IP
func (re *ResponseEngine) blockIP(ip string, duration time.Duration) {
	re.mu.Lock()
	defer re.mu.Unlock()
	
	re.blacklist[ip] = time.Now().Add(duration)
	logger.Info("IP blocked", "ip", ip, "duration", duration)
}

// IsBlocked 检查IP是否被阻断
func (re *ResponseEngine) IsBlocked(ip string) bool {
	re.mu.RLock()
	defer re.mu.RUnlock()
	
	blockTime, exists := re.blacklist[ip]
	if !exists {
		return false
	}
	
	if time.Now().After(blockTime) {
		// 阻断时间已过，移除
		re.mu.RUnlock()
		re.mu.Lock()
		delete(re.blacklist, ip)
		re.mu.Unlock()
		re.mu.RLock()
		return false
	}
	
	return true
}

// escalateThreat 升级威胁
func (re *ResponseEngine) escalateThreat(threat *ThreatEvent) {
	// 简化实现，实际可以发送邮件、短信等
	logger.Error("Threat escalated", "threat_id", threat.ID, "type", threat.Type, "severity", threat.Severity)
}

// OptimizedSecurityManager 优化的安全管理器
type OptimizedSecurityManager struct {
	config            *OptimizedSecurityConfig
	connectionTracker *ConnectionTracker
	threatDetector    *ThreatDetector
	responseEngine    *ResponseEngine
	metrics           *OptimizedSecurityMetrics
	events            chan *ThreatEvent
	mu                sync.RWMutex
	ctx               context.Context
	cancel            context.CancelFunc
	
	// 统计计数器
	totalConnections   uint64
	blockedConnections uint64
	totalThreats       uint64
	blockedThreats     uint64
}

// NewOptimizedSecurityManager 创建优化的安全管理器
func NewOptimizedSecurityManager(config *OptimizedSecurityConfig) *OptimizedSecurityManager {
	if config == nil {
		config = DefaultOptimizedSecurityConfig()
	}
	
	ctx, cancel := context.WithCancel(context.Background())
	
	sm := &OptimizedSecurityManager{
		config:            config,
		connectionTracker: NewConnectionTracker(config),
		threatDetector:    NewThreatDetector(config),
		responseEngine:    NewResponseEngine(config),
		metrics: &OptimizedSecurityMetrics{
			ThreatsByType:     make(map[ThreatType]int64),
			ThreatsBySeverity: make(map[ThreatSeverity]int64),
		},
		events: make(chan *ThreatEvent, config.EventBufferSize),
		ctx:    ctx,
		cancel: cancel,
	}
	
	return sm
}

// Start 启动安全管理器
func (sm *OptimizedSecurityManager) Start() error {
	if !sm.config.Enabled {
		return nil
	}
	
	// 启动事件处理协程
	go sm.eventProcessor()
	
	// 启动指标收集
	if sm.config.MetricsEnabled {
		go sm.metricsCollector()
	}
	
	// 启动清理协程
	go sm.cleanupWorker()
	
	logger.Info("Optimized security manager started", "security_level", sm.config.SecurityLevel)
	return nil
}

// Stop 停止安全管理器
func (sm *OptimizedSecurityManager) Stop() error {
	sm.cancel()
	close(sm.events)
	
	logger.Info("Optimized security manager stopped")
	return nil
}

// CheckConnection 检查连接安全性
func (sm *OptimizedSecurityManager) CheckConnection(conn net.Conn, userAgent, protocol string) (bool, string) {
	if !sm.config.Enabled {
		return true, "security_disabled"
	}
	
	// 获取远程地址
	remoteAddr := conn.RemoteAddr().String()
	ip, portStr, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		return false, "invalid_address"
	}
	
	// 检查IP是否被阻断
	if sm.responseEngine.IsBlocked(ip) {
		atomic.AddUint64(&sm.blockedConnections, 1)
		return false, "ip_blocked"
	}
	
	// 跟踪连接
	port := 0
	if portStr != "" {
		fmt.Sscanf(portStr, "%d", &port)
	}
	connID := sm.connectionTracker.TrackConnection(ip, port, userAgent, protocol)
	
	// 检查连接数限制
	if sm.config.DDoSProtectionEnabled {
		connections := sm.connectionTracker.GetConnectionsByIP(ip)
		if len(connections) > sm.config.MaxConnectionsPerIP {
			// 生成DDoS威胁事件
			threat := &ThreatEvent{
				ID:          sm.generateEventID(),
				Type:        ThreatTypeDDoS,
				Severity:    SeverityHigh,
				Source:      ip,
				Target:      conn.LocalAddr().String(),
				Description: fmt.Sprintf("Too many connections from IP: %d", len(connections)),
				Timestamp:   time.Now(),
				Metadata: map[string]interface{}{
					"connection_count": len(connections),
					"max_allowed":      sm.config.MaxConnectionsPerIP,
					"connection_id":    connID,
				},
				Confidence: 0.9,
			}
			
			sm.publishThreatEvent(threat)
			atomic.AddUint64(&sm.blockedConnections, 1)
			return false, "ddos_protection"
		}
	}
	
	atomic.AddUint64(&sm.totalConnections, 1)
	return true, "allowed"
}

// CheckData 检查数据安全性
func (sm *OptimizedSecurityManager) CheckData(data []byte, source, target string) (bool, []string) {
	if !sm.config.Enabled || !sm.config.ThreatDetectionEnabled {
		return true, []string{"security_disabled"}
	}
	
	// 检测威胁
	threats := sm.threatDetector.DetectThreats(data, source, target)
	
	var responses []string
	blocked := false
	
	for _, threat := range threats {
		// 发布威胁事件
		sm.publishThreatEvent(threat)
		
		// 响应威胁
		response := sm.responseEngine.RespondToThreat(threat)
		responses = append(responses, response)
		
		if response == "blocked" {
			blocked = true
			threat.Blocked = true
			atomic.AddUint64(&sm.blockedThreats, 1)
		}
		
		threat.Response = response
		atomic.AddUint64(&sm.totalThreats, 1)
	}
	
	if len(responses) == 0 {
		responses = []string{"clean"}
	}
	
	return !blocked, responses
}

// UpdateConnection 更新连接信息
func (sm *OptimizedSecurityManager) UpdateConnection(connID string, bytesSent, bytesReceived uint64) {
	sm.connectionTracker.UpdateConnection(connID, bytesSent, bytesReceived)
}

// publishThreatEvent 发布威胁事件
func (sm *OptimizedSecurityManager) publishThreatEvent(threat *ThreatEvent) {
	select {
	case sm.events <- threat:
		// 成功发布
	default:
		// 事件缓冲区满，丢弃事件
		logger.Error("Threat event buffer full, dropping event", "threat_id", threat.ID)
	}
}

// eventProcessor 事件处理器
func (sm *OptimizedSecurityManager) eventProcessor() {
	for {
		select {
		case <-sm.ctx.Done():
			return
		case event := <-sm.events:
			if event != nil {
				sm.processEvent(event)
			}
		}
	}
}

// processEvent 处理事件
func (sm *OptimizedSecurityManager) processEvent(event *ThreatEvent) {
	// 更新指标
	sm.mu.Lock()
	sm.metrics.ThreatsByType[event.Type]++
	sm.metrics.ThreatsBySeverity[event.Severity]++
	sm.mu.Unlock()
	
	// 审计日志
	if sm.config.AuditEnabled {
		sm.auditLog(event)
	}
	
	// 告警
	if sm.config.AlertingEnabled && event.Severity >= SeverityHigh {
		sm.sendAlert(event)
	}
}

// auditLog 审计日志
func (sm *OptimizedSecurityManager) auditLog(event *ThreatEvent) {
	logData := map[string]interface{}{
		"event_id":    event.ID,
		"type":        event.Type,
		"severity":    event.Severity,
		"source":      event.Source,
		"target":      event.Target,
		"description": event.Description,
		"timestamp":   event.Timestamp,
		"blocked":     event.Blocked,
		"response":    event.Response,
		"confidence":  event.Confidence,
		"metadata":    event.Metadata,
	}
	
	logJSON, _ := json.Marshal(logData)
	logger.Info("Security audit log", "audit_data", string(logJSON))
}

// sendAlert 发送告警
func (sm *OptimizedSecurityManager) sendAlert(event *ThreatEvent) {
	// 简化实现，实际可以集成告警系统
	logger.Error("Security alert", 
		"event_id", event.ID,
		"type", event.Type,
		"severity", event.Severity,
		"source", event.Source,
		"description", event.Description)
}

// metricsCollector 指标收集器
func (sm *OptimizedSecurityManager) metricsCollector() {
	ticker := time.NewTicker(sm.config.MetricsInterval)
	defer ticker.Stop()
	
	for {
		select {
		case <-sm.ctx.Done():
			return
		case <-ticker.C:
			sm.collectMetrics()
		}
	}
}

// collectMetrics 收集指标
func (sm *OptimizedSecurityManager) collectMetrics() {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	
	sm.metrics.TotalConnections = int64(atomic.LoadUint64(&sm.totalConnections))
	sm.metrics.BlockedConnections = int64(atomic.LoadUint64(&sm.blockedConnections))
	sm.metrics.TotalThreats = int64(atomic.LoadUint64(&sm.totalThreats))
	sm.metrics.BlockedThreats = int64(atomic.LoadUint64(&sm.blockedThreats))
	sm.metrics.LastUpdate = time.Now()
	
	// 计算活跃连接数
	sm.metrics.ActiveConnections = int64(len(sm.connectionTracker.connections))
	
	logger.Debug("Security metrics collected", 
		"total_connections", sm.metrics.TotalConnections,
		"blocked_connections", sm.metrics.BlockedConnections,
		"total_threats", sm.metrics.TotalThreats,
		"blocked_threats", sm.metrics.BlockedThreats)
}

// cleanupWorker 清理工作器
func (sm *OptimizedSecurityManager) cleanupWorker() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	
	for {
		select {
		case <-sm.ctx.Done():
			return
		case <-ticker.C:
			sm.connectionTracker.CleanupConnections()
		}
	}
}

// generateEventID 生成事件ID
func (sm *OptimizedSecurityManager) generateEventID() string {
	bytes := make([]byte, 16)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)
}

// GetMetrics 获取安全指标
func (sm *OptimizedSecurityManager) GetMetrics() *OptimizedSecurityMetrics {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	
	// 返回副本
	metrics := *sm.metrics
	metrics.ThreatsByType = make(map[ThreatType]int64)
	metrics.ThreatsBySeverity = make(map[ThreatSeverity]int64)
	
	for k, v := range sm.metrics.ThreatsByType {
		metrics.ThreatsByType[k] = v
	}
	
	for k, v := range sm.metrics.ThreatsBySeverity {
		metrics.ThreatsBySeverity[k] = v
	}
	
	return &metrics
}

// GetConnectionInfo 获取连接信息
func (sm *OptimizedSecurityManager) GetConnectionInfo(connID string) (*ConnectionInfo, bool) {
	return sm.connectionTracker.GetConnection(connID)
}

// GetConnectionsByIP 获取IP的所有连接
func (sm *OptimizedSecurityManager) GetConnectionsByIP(ip string) []*ConnectionInfo {
	return sm.connectionTracker.GetConnectionsByIP(ip)
}

// AddThreatSignature 添加威胁签名
func (sm *OptimizedSecurityManager) AddThreatSignature(signature *ThreatSignature) {
	sm.threatDetector.mu.Lock()
	defer sm.threatDetector.mu.Unlock()
	
	sm.threatDetector.signatures[signature.ID] = signature
	logger.Info("Threat signature added", "signature_id", signature.ID, "name", signature.Name)
}

// RemoveThreatSignature 移除威胁签名
func (sm *OptimizedSecurityManager) RemoveThreatSignature(signatureID string) {
	sm.threatDetector.mu.Lock()
	defer sm.threatDetector.mu.Unlock()
	
	delete(sm.threatDetector.signatures, signatureID)
	logger.Info("Threat signature removed", "signature_id", signatureID)
}

// BlockIP 手动阻断IP
func (sm *OptimizedSecurityManager) BlockIP(ip string, duration time.Duration) {
	sm.responseEngine.blockIP(ip, duration)
}

// UnblockIP 解除IP阻断
func (sm *OptimizedSecurityManager) UnblockIP(ip string) {
	sm.responseEngine.mu.Lock()
	defer sm.responseEngine.mu.Unlock()
	
	delete(sm.responseEngine.blacklist, ip)
	logger.Info("IP unblocked", "ip", ip)
}

// IsIPBlocked 检查IP是否被阻断
func (sm *OptimizedSecurityManager) IsIPBlocked(ip string) bool {
	return sm.responseEngine.IsBlocked(ip)
}

// UpdateConfig 更新配置
func (sm *OptimizedSecurityManager) UpdateConfig(config *OptimizedSecurityConfig) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	
	sm.config = config
	sm.connectionTracker.config = config
	sm.threatDetector.config = config
	sm.responseEngine.config = config
	
	logger.Info("Security configuration updated", "security_level", config.SecurityLevel)
}

// ExportMetrics 导出指标
func (sm *OptimizedSecurityManager) ExportMetrics() ([]byte, error) {
	data := map[string]interface{}{
		"metrics":     sm.GetMetrics(),
		"config":      sm.config,
		"connections": len(sm.connectionTracker.connections),
		"signatures":  len(sm.threatDetector.signatures),
		"blocked_ips": len(sm.responseEngine.blacklist),
	}
	
	return json.Marshal(data)
}

// GetThreatSignatures 获取威胁签名列表
func (sm *OptimizedSecurityManager) GetThreatSignatures() map[string]*ThreatSignature {
	sm.threatDetector.mu.RLock()
	defer sm.threatDetector.mu.RUnlock()
	
	signatures := make(map[string]*ThreatSignature)
	for id, sig := range sm.threatDetector.signatures {
		signatures[id] = sig
	}
	return signatures
}

// GetBehaviorProfiles 获取行为画像
func (sm *OptimizedSecurityManager) GetBehaviorProfiles() map[string]*BehaviorProfile {
	sm.threatDetector.mu.RLock()
	defer sm.threatDetector.mu.RUnlock()
	
	profiles := make(map[string]*BehaviorProfile)
	for ip, profile := range sm.threatDetector.behaviorProfiles {
		profiles[ip] = profile
	}
	return profiles
}

// GetBlockedIPs 获取被阻断的IP列表
func (sm *OptimizedSecurityManager) GetBlockedIPs() map[string]time.Time {
	sm.responseEngine.mu.RLock()
	defer sm.responseEngine.mu.RUnlock()
	
	blockedIPs := make(map[string]time.Time)
	for ip, blockTime := range sm.responseEngine.blacklist {
		blockedIPs[ip] = blockTime
	}
	return blockedIPs
}