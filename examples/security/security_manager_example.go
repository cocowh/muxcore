package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"time"

	"github.com/cocowh/muxcore/core/security"
)

func main() {
	// 创建安全配置
	config := security.SecurityConfig{
		DOSThreshold:             100,
		DOSWindowSize:            time.Minute,
		DOSCleanupInterval:       time.Minute * 5,
		DOSBlockDuration:         time.Minute * 10,
		SYNCookieCleanupInterval: time.Minute * 2,
		DPIMaxPacketSize:         1500,
		DefaultAuthPolicy:        "deny",
		SecurityLevel:            security.SecurityLevelMedium,
		EventBufferSize:          1000,
		MetricsInterval:          time.Second * 30,
		EnableWhitelist:          true,
		EnableBlacklist:          true,
		MaxEventHandlers:         10,
		AuditLogEnabled:          true,
		RateLimitEnabled:         true,
		GeoBlockingEnabled:       false,
	}

	// 创建安全管理器
	securityManager := security.NewSecurityManager(config)
	defer securityManager.Close()

	fmt.Println("=== MuxCore Security Manager Demo ===")
	fmt.Printf("Initial Security Level: %d\n", securityManager.GetSecurityLevel())

	// 添加事件处理器
	err := securityManager.AddEventHandler(func(event *security.SecurityEvent) {
		fmt.Printf("Security Event: [%s] %s - %s\n", event.Type, event.Source, event.Description)
	})
	if err != nil {
		log.Printf("Failed to add event handler: %v", err)
	}

	// 演示白名单/黑名单功能
	fmt.Println("\n=== Whitelist/Blacklist Demo ===")
	testIP := "192.168.1.100"
	securityManager.AddToWhitelist(testIP)
	fmt.Printf("IP %s is whitelisted: %v\n", testIP, securityManager.IsWhitelisted(testIP))

	maliciousIP := "10.0.0.1"
	securityManager.AddToBlacklist(maliciousIP)
	fmt.Printf("IP %s is blacklisted: %v\n", maliciousIP, securityManager.IsBlacklisted(maliciousIP))

	// 演示安全级别调整
	fmt.Println("\n=== Security Level Demo ===")
	securityManager.UpdateSecurityLevel(security.SecurityLevelHigh)
	fmt.Printf("Updated Security Level: %d\n", securityManager.GetSecurityLevel())

	// 演示连接安全检查
	fmt.Println("\n=== Connection Security Check Demo ===")
	// 模拟TCP连接（实际使用中应该是真实连接）
	tcpAddr, _ := net.ResolveTCPAddr("tcp", "127.0.0.1:8080")
	conn, err := net.DialTCP("tcp", nil, tcpAddr)
	if err != nil {
		// 如果连接失败，我们仍然可以演示安全检查逻辑
		fmt.Printf("Note: Could not establish real connection for demo: %v\n", err)
		fmt.Println("Proceeding with security check simulation...")
	}

	// 检查连接安全性
	isSecure := securityManager.CheckConnectionSecurity(conn, testIP, "Mozilla/5.0", "HTTP")
	fmt.Printf("Connection from %s is secure: %v\n", testIP, isSecure)

	// 检查恶意IP
	isSecureMalicious := securityManager.CheckConnectionSecurity(conn, maliciousIP, "BadBot/1.0", "HTTP")
	fmt.Printf("Connection from %s is secure: %v\n", maliciousIP, isSecureMalicious)

	if conn != nil {
		conn.Close()
	}

	// 演示数据包安全检查
	fmt.Println("\n=== Packet Security Check Demo ===")
	testPacket := []byte("GET / HTTP/1.1\r\nHost: example.com\r\n\r\n")
	isPacketSecure, reason := securityManager.CheckPacketSecurity(testPacket, "HTTP")
	fmt.Printf("Packet is secure: %v, Reason: %s\n", isPacketSecure, reason)

	// 演示权限检查
	fmt.Println("\n=== Permission Check Demo ===")
	ctx := context.Background()

	// 添加测试用户
	authManager := securityManager.GetAuthManager()
	testUser := &security.User{
		ID:       "user123",
		Username: "testuser",
		Roles:    []string{"user"},
	}
	authManager.AddUser(testUser)

	// 检查权限
	canRead := securityManager.CheckPermission(ctx, "user123", security.PermissionRead)
	canDelete := securityManager.CheckPermission(ctx, "user123", security.PermissionDelete)
	fmt.Printf("User can read: %v\n", canRead)
	fmt.Printf("User can delete: %v\n", canDelete)

	// 演示安全策略应用
	fmt.Println("\n=== Security Policy Demo ===")
	securityPolicy := map[string]interface{}{
		"dos_protection": map[string]interface{}{
			"threshold": 50,
		},
		"dpi_rules": []interface{}{
			map[string]interface{}{
				"pattern": "malware",
				"action":  "block",
			},
		},
		"auth_policy": "allow",
	}

	err = securityManager.ApplySecurityPolicy(securityPolicy)
	if err != nil {
		fmt.Printf("Failed to apply security policy: %v\n", err)
	} else {
		fmt.Println("Security policy applied successfully")
	}

	// 显示安全指标
	fmt.Println("\n=== Security Metrics ===")
	metrics := securityManager.GetMetrics()
	fmt.Printf("Total Connections: %d\n", metrics.TotalConnections)
	fmt.Printf("Blocked Connections: %d\n", metrics.BlockedConnections)
	fmt.Printf("Security Events: %d\n", metrics.SecurityEvents)
	fmt.Printf("Auth Attempts: %d\n", metrics.AuthAttempts)

	// 发布自定义安全事件
	fmt.Println("\n=== Custom Security Event ===")
	securityManager.PublishEvent(
		"demo.test",
		"security_example",
		"system",
		"Demo security event published",
		security.SecurityLevelMedium,
		map[string]interface{}{
			"demo":      true,
			"timestamp": time.Now().Unix(),
		},
	)

	// 等待一段时间让事件处理器处理事件
	time.Sleep(time.Second)

	fmt.Println("\n=== Demo Completed ===")
	fmt.Println("Security Manager has been successfully demonstrated with:")
	fmt.Println("- Unified security policy configuration")
	fmt.Println("- Dynamic security level adjustment")
	fmt.Println("- Whitelist/Blacklist management")
	fmt.Println("- Connection and packet security checks")
	fmt.Println("- Permission-based access control")
	fmt.Println("- Security event handling and metrics collection")
}
