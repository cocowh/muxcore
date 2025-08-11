// Copyright (c) 2025 cocowh. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package security

import (
	"sync"
	"time"

	"github.com/cocowh/muxcore/pkg/errors"
	"github.com/cocowh/muxcore/pkg/logger"
)

// DOSProtection 实现协议识别层的DoS防护
// 防DoS指纹伪造功能

type Fingerprint struct {
	IP        string
	UserAgent string
	Protocol  string
	Timestamp time.Time
	Count     int
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
func (dp *DOSProtector) CheckFingerprint(ip, userAgent, protocol string) error {
	if !dp.enabled {
		return nil
	}

	// 检查是否被阻止
	if blockedTime, blocked := dp.isBlocked(ip); blocked {
		if time.Since(blockedTime) > dp.blockDuration {
			dp.mutex.Lock()
			delete(dp.blockedIPs, ip)
			dp.mutex.Unlock()
		} else {
			logger.Warnf("Blocked IP detected: %s", ip)
			return errors.New(errors.ErrCodeGovernanceRateLimit, errors.CategorySecurity, errors.LevelWarn, "Rate limit exceeded")
		}
	}

	// 生成指纹ID
	fingerprintID := generateFingerprintID(ip, userAgent, protocol)

	dp.mutex.Lock()
	defer dp.mutex.Unlock()

	now := time.Now()
	fingerprint, exists := dp.fingerprints[fingerprintID]

	if !exists {
		// 新指纹
		dp.fingerprints[fingerprintID] = &Fingerprint{
			IP:        ip,
			UserAgent: userAgent,
			Protocol:  protocol,
			Timestamp: now,
			Count:     1,
		}
		return nil
	}

	// 检查是否在时间窗口内
	if now.Sub(fingerprint.Timestamp) < dp.windowSize {
		fingerprint.Count++
		if fingerprint.Count > dp.threshold {
			logger.Warnf("Potential DoS attack detected from %s, count: %d", ip, fingerprint.Count)
			// avoid nested locking: we already hold dp.mutex here
			dp.blockedIPs[ip] = time.Now()
			return errors.New(errors.ErrCodeGovernanceRateLimit, errors.CategorySecurity, errors.LevelWarn, "Potential DoS attack")
		}
	} else {
		// 重置计数器
		fingerprint.Timestamp = now
		fingerprint.Count = 1
	}

	return nil
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

		now := time.Now()

		dp.mutex.Lock()

		// 清理过期的指纹
		newFingerprints := make(map[string]*Fingerprint)
		for id, fingerprint := range dp.fingerprints {
			if now.Sub(fingerprint.Timestamp) <= dp.windowSize {
				newFingerprints[id] = fingerprint
			}
		}
		dp.fingerprints = newFingerprints

		// 清理过期的阻止
		newBlockedIPs := make(map[string]time.Time)
		for ip, blockedTime := range dp.blockedIPs {
			if now.Sub(blockedTime) <= dp.blockDuration {
				newBlockedIPs[ip] = blockedTime
			}
		}
		dp.blockedIPs = newBlockedIPs

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
