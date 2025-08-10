package security

import (
	"crypto/rand"
	"encoding/binary"
	"net"
	"sync"
	"time"

	"github.com/cocowh/muxcore/pkg/logger"
)

// SYNCookieProtector 实现SYN Cookie防护机制
// 用于连接管理层的安全防护

type SYNCookieProtector struct {
	mutex           sync.RWMutex
	salt            [8]byte
	validCookies    map[uint64]time.Time
	cleanupInterval time.Duration
	enabled         bool
}

// NewSYNCookieProtector 创建SYN Cookie防护实例
func NewSYNCookieProtector(cleanupInterval time.Duration) *SYNCookieProtector {
	// 生成随机salt
	var salt [8]byte
	_, err := rand.Read(salt[:])
	if err != nil {
		logger.Errorf("Failed to generate salt for SYN cookies: %v", err)
		// 使用时间戳作为备选salt
		now := time.Now().UnixNano()
		binary.LittleEndian.PutUint64(salt[:], uint64(now))
	}

	scp := &SYNCookieProtector{
		salt:            salt,
		validCookies:    make(map[uint64]time.Time),
		cleanupInterval: cleanupInterval,
		enabled:         true,
	}

	// 启动清理过期cookie的goroutine
	go scp.cleanupExpiredCookies()

	return scp
}

// GenerateCookie 生成SYN Cookie
func (scp *SYNCookieProtector) GenerateCookie(conn *net.TCPConn) (uint64, error) {
	if !scp.enabled {
		return 0, nil
	}

	// 获取连接信息
	localAddr := conn.LocalAddr().(*net.TCPAddr)
	remoteAddr := conn.RemoteAddr().(*net.TCPAddr)

	// 生成cookie
	// 实际实现中应使用更安全的算法，这里简化处理
	now := time.Now().Unix()
	cookie := generateCookieValue(localAddr, remoteAddr, scp.salt, now)

	// 记录有效cookie
	scp.mutex.Lock()
	scp.validCookies[cookie] = time.Now().Add(2 * time.Minute) // 2分钟有效期
	scp.mutex.Unlock()

	return cookie, nil
}

// ValidateCookie 验证SYN Cookie
func (scp *SYNCookieProtector) ValidateCookie(conn *net.TCPConn, cookie uint64) bool {
	if !scp.enabled || cookie == 0 {
		return true
	}

	scp.mutex.RLock()
	expiry, valid := scp.validCookies[cookie]
	scp.mutex.RUnlock()

	if !valid {
		logger.Warnf("Invalid SYN cookie from %s", conn.RemoteAddr().String())
		return false
	}

	// 检查是否过期
	if time.Now().After(expiry) {
		logger.Warnf("Expired SYN cookie from %s", conn.RemoteAddr().String())
		scp.mutex.Lock()
		delete(scp.validCookies, cookie)
		scp.mutex.Unlock()
		return false
	}

	// 验证通过，删除cookie以防止重放攻击
	scp.mutex.Lock()
	delete(scp.validCookies, cookie)
	scp.mutex.Unlock()

	return true
}

// 生成cookie值的内部函数
func generateCookieValue(localAddr, remoteAddr *net.TCPAddr, salt [8]byte, timestamp int64) uint64 {
	// 简化实现，实际应使用更安全的算法
	// 组合地址、端口和时间戳信息
	localIP := localAddr.IP.To4()
	remoteIP := remoteAddr.IP.To4()

	if localIP == nil || remoteIP == nil {
		// 处理IPv6或其他情况
		return 0
	}

	// 构造数据
	data := make([]byte, 20)
	copy(data[0:4], localIP)
	copy(data[4:8], remoteIP)
	binary.LittleEndian.PutUint16(data[8:10], uint16(localAddr.Port))
	binary.LittleEndian.PutUint16(data[10:12], uint16(remoteAddr.Port))
	binary.LittleEndian.PutUint64(data[12:20], uint64(timestamp))

	// 简单哈希计算（实际应使用HMAC等算法）
	cookie := uint64(0)
	for i := 0; i < len(data); i += 8 {
		chunk := binary.LittleEndian.Uint64(data[i:min(i+8, len(data))])
		cookie ^= chunk
	}

	// 与salt混合
	cookie ^= binary.LittleEndian.Uint64(salt[:])

	return cookie
}

// 辅助函数，返回较小值
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// 清理过期的cookies
func (scp *SYNCookieProtector) cleanupExpiredCookies() {
	for {
		time.Sleep(scp.cleanupInterval)

		scp.mutex.Lock()
		now := time.Now()

		for cookie, expiry := range scp.validCookies {
			if now.After(expiry) {
				delete(scp.validCookies, cookie)
			}
		}

		scp.mutex.Unlock()
	}
}

// Enable 启用SYN Cookie防护
func (scp *SYNCookieProtector) Enable() {
	scp.mutex.Lock()
	scp.enabled = true
	scp.mutex.Unlock()
}

// Disable 禁用SYN Cookie防护
func (scp *SYNCookieProtector) Disable() {
	scp.mutex.Lock()
	scp.enabled = false
	scp.mutex.Unlock()
}
