package security

import (
	"context"
	"net"
	"sync"
	"time"

	"github.com/cocowh/muxcore/pkg/logger"
)

// SecurityManager 安全管理器
// 整合纵深防御体系的各个组件

type SecurityManager struct {
	mutex          sync.RWMutex
	dosProtector   *DOSProtector
	synCookie      *SYNCookieProtector
	dpiEngine      *DPIEngine
	authManager    *AuthManager
	enabled        bool
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
}

// NewSecurityManager 创建安全管理器实例
func NewSecurityManager(config SecurityConfig) *SecurityManager {
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

	// 创建安全管理器
	manager := &SecurityManager{
	dosProtector:   dosProtector,
	synCookie:      synCookie,
	dpiEngine:      dpiEngine,
	authManager:    authManager,
	enabled:        true,
	}

	logger.Infof("Security manager initialized with %d DoS rules, %d DPI rules, %d roles",
		len(dosProtector.fingerprints),
		len(dpiEngine.GetRules()),
		len(authManager.roles),
	)

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