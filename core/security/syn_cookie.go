// Copyright (c) 2025 cocowh. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package security

import (
	"sync"

	"github.com/cocowh/muxcore/pkg/errors"
)

// SYNCookieProtector 用于 SYN Flood 防护
// 通过 SYN Cookie 机制验证客户端的合法性

type SYNCookieProtector struct {
	enabled bool
	secret  uint32
	mutex   sync.RWMutex
}

// NewSYNCookieProtector 创建实例
func NewSYNCookieProtector(secret uint32) *SYNCookieProtector {
	return &SYNCookieProtector{
		enabled: true,
		secret:  secret,
	}
}

// Enable 启用保护
func (scp *SYNCookieProtector) Enable() {
	scp.mutex.Lock()
	scp.enabled = true
	scp.mutex.Unlock()
}

// Disable 禁用保护
func (scp *SYNCookieProtector) Disable() {
	scp.mutex.Lock()
	scp.enabled = false
	scp.mutex.Unlock()
}

// GenerateCookie 生成 SYN Cookie
func (scp *SYNCookieProtector) GenerateCookie(clientIP string, clientPort int) uint32 {
	// 简化实现：根据 clientIP、clientPort 和 secret 生成 cookie
	// 真实实现应使用更安全的哈希算法
	var cookie uint32
	for i := 0; i < len(clientIP); i++ {
		cookie += uint32(clientIP[i])
	}
	cookie += uint32(clientPort)
	cookie ^= scp.secret
	return cookie
}

// ValidateCookie 验证 SYN Cookie（返回nil表示通过；返回错误表示拒绝）
func (scp *SYNCookieProtector) ValidateCookie(clientIP string, clientPort int, receivedCookie uint32) *errors.MuxError {
	scp.mutex.RLock()
	enabled := scp.enabled
	scp.mutex.RUnlock()

	if !enabled {
		return nil
	}

	expectedCookie := scp.GenerateCookie(clientIP, clientPort)
	if expectedCookie != receivedCookie {
		return errors.New(errors.ErrCodeAuthUnauthorized, errors.CategorySecurity, errors.LevelWarn, "invalid syn cookie")
	}

	return nil
}
