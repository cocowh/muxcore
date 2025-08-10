package security

import (
	"sync"
	"time"

	"github.com/cocowh/muxcore/pkg/logger"
)

// DOSProtection 实现协议识别层的DoS防护
// 防DoS指纹伪造功能

type Fingerprint struct {
	IP        string
	UserAgent string
	Protocol  string
	Timestamp time.Time
}

// DOSProtector  DoS防护管理器

type DOSProtector struct {
	mutex           sync.RWMutex
	fingerprints    map[string]*Fingerprint
	threshold       int           // 阈值配置
	windowSize      time.Duration // 时间窗口
	cleanupInterval time.Duration // 清理间隔
	blockDuration   time.Duration // 阻止 duration
	blockedIPs      map[string]time.Time
	enabled         bool
}

// NewDOSProtector 创建DoS防护实例
func NewDOSProtector(threshold int, windowSize, cleanupInterval, blockDuration time.Duration) *DOSProtector {
	dp := &DOSProtector{
		fingerprints:    make(map[string]*Fingerprint),
		threshold:       threshold,
		windowSize:      windowSize,
		cleanupInterval: cleanupInterval,
		blockDuration:   blockDuration,
		blockedIPs:      make(map[string]time.Time),
		enabled:         true,
	}

	// 启动清理过期指纹的goroutine
	go dp.cleanupExpiredFingerprints()

	return dp
}

// CheckFingerprint 检查指纹是否可疑
func (dp *DOSProtector) CheckFingerprint(ip, userAgent, protocol string) bool {
	if !dp.enabled {
		return true
	}

	// 检查是否被阻止
	if blockedTime, blocked := dp.isBlocked(ip); blocked {
		if time.Since(blockedTime) > dp.blockDuration {
			dp.mutex.Lock()
			delete(dp.blockedIPs, ip)
			dp.mutex.Unlock()
		} else {
			logger.Warnf("Blocked IP detected: %s", ip)
			return false
		}
	}

	// 生成指纹ID
	fingerprintID := generateFingerprintID(ip, userAgent, protocol)

	dp.mutex.RLock()
	fingerprint, exists := dp.fingerprints[fingerprintID]
	dp.mutex.RUnlock()

	now := time.Now()

	if !exists {
		// 新指纹
		dp.mutex.Lock()
		dp.fingerprints[fingerprintID] = &Fingerprint{
			IP:        ip,
			UserAgent: userAgent,
			Protocol:  protocol,
			Timestamp: now,
		}
		dp.mutex.Unlock()
		return true
	}

	// 检查是否在时间窗口内超过阈值
	if now.Sub(fingerprint.Timestamp) < dp.windowSize {
		// 在时间窗口内，增加计数
		// 实际实现中这里应该有一个计数器
		// 为简化，我们假设计数器超过阈值就返回false
		// 这里应该有一个真实的计数逻辑
		logger.Warnf("Potential DoS attack detected from %s", ip)
		dp.blockIP(ip)
		return false
	}

	// 更新时间戳
	dp.mutex.Lock()
	fingerprint.Timestamp = now
	dp.mutex.Unlock()

	return true
}

// 生成指纹ID
func generateFingerprintID(ip, userAgent, protocol string) string {
	return ip + ":" + userAgent + ":" + protocol
}

// 检查IP是否被阻止
func (dp *DOSProtector) isBlocked(ip string) (time.Time, bool) {
	dp.mutex.RLock()
	blockedTime, exists := dp.blockedIPs[ip]
	dp.mutex.RUnlock()
	return blockedTime, exists
}

// 阻止IP
func (dp *DOSProtector) blockIP(ip string) {
	dp.mutex.Lock()
	dp.blockedIPs[ip] = time.Now()
	dp.mutex.Unlock()
}

// 清理过期的指纹
func (dp *DOSProtector) cleanupExpiredFingerprints() {
	for {
		time.Sleep(dp.cleanupInterval)

		dp.mutex.Lock()
		now := time.Now()

		// 清理过期指纹
		for id, fingerprint := range dp.fingerprints {
			if now.Sub(fingerprint.Timestamp) > dp.windowSize {
				delete(dp.fingerprints, id)
			}
		}

		// 清理过期阻止
		for ip, blockedTime := range dp.blockedIPs {
			if now.Sub(blockedTime) > dp.blockDuration {
				delete(dp.blockedIPs, ip)
			}
		}

		dp.mutex.Unlock()
	}
}

// Enable 启用DoS防护
func (dp *DOSProtector) Enable() {
	dp.mutex.Lock()
	dp.enabled = true
	dp.mutex.Unlock()
}

// Disable 禁用DoS防护
func (dp *DOSProtector) Disable() {
	dp.mutex.Lock()
	dp.enabled = false
	dp.mutex.Unlock()
}
