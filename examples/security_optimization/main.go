package main

// 注意：这个示例需要单独运行，因为与security_manager_example.go存在main函数冲突
// 运行方式：go run security_optimization_example.go

import (
	"fmt"
	"net"
	"time"

	"github.com/cocowh/muxcore/core/security"
	"github.com/cocowh/muxcore/pkg/logger"
)

func main() {
	// 初始化日志
	logger.Info("Starting security optimization example")

	// 创建优化的安全配置
	config := security.DefaultOptimizedSecurityConfig()
	config.SecurityLevel = security.SecurityLevelHigh
	config.DDoSProtectionEnabled = true
	config.MaxConnectionsPerIP = 50
	config.ThreatDetectionEnabled = true
	config.AnomalyDetectionEnabled = true
	config.SignatureDetection = true
	config.BehaviorAnalysis = true
	config.AutoResponseEnabled = true
	config.MetricsEnabled = true
	config.AlertingEnabled = true

	fmt.Println("\n=== 优化安全管理器演示 ===")
	fmt.Printf("安全级别: %v\n", config.SecurityLevel)
	fmt.Printf("DDoS防护: %v\n", config.DDoSProtectionEnabled)
	fmt.Printf("威胁检测: %v\n", config.ThreatDetectionEnabled)
	fmt.Printf("异常检测: %v\n", config.AnomalyDetectionEnabled)
	fmt.Printf("自动响应: %v\n", config.AutoResponseEnabled)

	// 创建优化的安全管理器
	securityManager := security.NewOptimizedSecurityManager(config)

	// 启动安全管理器
	err := securityManager.Start()
	if err != nil {
		fmt.Printf("启动安全管理器失败: %v\n", err)
		return
	}
	defer securityManager.Stop()

	fmt.Println("\n=== 连接安全检查演示 ===")
	// 模拟连接检查
	testConnections := []struct {
		ip        string
		port      int
		userAgent string
		protocol  string
	}{
		{"192.168.1.100", 8080, "Mozilla/5.0", "HTTP/1.1"},
		{"10.0.0.1", 443, "curl/7.68.0", "HTTPS"},
		{"172.16.0.50", 80, "BadBot/1.0", "HTTP/1.1"},
		{"203.0.113.1", 22, "ssh-client", "SSH"},
	}

	for _, conn := range testConnections {
		// 创建模拟连接
		mockConn := &MockConnection{
			remoteAddr: fmt.Sprintf("%s:%d", conn.ip, conn.port),
			localAddr:  "127.0.0.1:8080",
		}

		allowed, reason := securityManager.CheckConnection(mockConn, conn.userAgent, conn.protocol)
		fmt.Printf("连接 %s:%d - 允许: %v, 原因: %s\n", conn.ip, conn.port, allowed, reason)
	}

	fmt.Println("\n=== 威胁检测演示 ===")
	// 模拟威胁检测
	testData := []struct {
		name   string
		data   string
		source string
		target string
	}{
		{
			name:   "正常HTTP请求",
			data:   "GET /api/users HTTP/1.1\r\nHost: example.com\r\n\r\n",
			source: "192.168.1.100",
			target: "127.0.0.1:8080",
		},
		{
			name:   "SQL注入攻击",
			data:   "GET /api/users?id=1' UNION SELECT * FROM users-- HTTP/1.1\r\n",
			source: "203.0.113.1",
			target: "127.0.0.1:8080",
		},
		{
			name:   "XSS攻击",
			data:   "POST /comment HTTP/1.1\r\nContent: <script>alert('xss')</script>",
			source: "172.16.0.50",
			target: "127.0.0.1:8080",
		},
		{
			name:   "大数据包异常",
			data:   generateLargeData(15000), // 15KB数据
			source: "10.0.0.1",
			target: "127.0.0.1:8080",
		},
	}

	for _, test := range testData {
		fmt.Printf("\n检测: %s\n", test.name)
		allowed, responses := securityManager.CheckData([]byte(test.data), test.source, test.target)
		fmt.Printf("允许: %v, 响应: %v\n", allowed, responses)
	}

	fmt.Println("\n=== DDoS防护演示 ===")
	// 模拟DDoS攻击
	attackerIP := "203.0.113.100"
	for i := 0; i < 60; i++ { // 超过MaxConnectionsPerIP限制
		mockConn := &MockConnection{
			remoteAddr: fmt.Sprintf("%s:%d", attackerIP, 8000+i),
			localAddr:  "127.0.0.1:8080",
		}

		allowed, reason := securityManager.CheckConnection(mockConn, "AttackBot/1.0", "HTTP/1.1")
		if !allowed {
			fmt.Printf("DDoS防护触发 - 连接 %d 被阻断, 原因: %s\n", i+1, reason)
			break
		}
	}

	fmt.Println("\n=== 威胁签名管理演示 ===")
	// 添加自定义威胁签名
	customSignature := &security.ThreatSignature{
		ID:          "custom_malware_1",
		Name:        "Custom Malware Pattern",
		Type:        security.ThreatTypeMalware,
		Severity:    security.SeverityHigh,
		Description: "Detects custom malware patterns",
		Enabled:     true,
	}
	// 注意：在实际使用中需要设置Pattern字段，这里跳过以避免nil指针异常
	// customSignature.Pattern = regexp.MustCompile(`malware_pattern`)
	securityManager.AddThreatSignature(customSignature)
	fmt.Printf("添加自定义威胁签名: %s\n", customSignature.Name)

	// 获取所有威胁签名
	signatures := securityManager.GetThreatSignatures()
	fmt.Printf("当前威胁签名数量: %d\n", len(signatures))

	fmt.Println("\n=== IP黑白名单管理演示 ===")
	// 手动阻断IP
	maliciousIP := "203.0.113.200"
	securityManager.BlockIP(maliciousIP, 5*time.Minute)
	fmt.Printf("手动阻断IP: %s (5分钟)\n", maliciousIP)

	// 检查IP是否被阻断
	isBlocked := securityManager.IsIPBlocked(maliciousIP)
	fmt.Printf("IP %s 是否被阻断: %v\n", maliciousIP, isBlocked)

	// 获取被阻断的IP列表
	blockedIPs := securityManager.GetBlockedIPs()
	fmt.Printf("当前被阻断的IP数量: %d\n", len(blockedIPs))

	fmt.Println("\n=== 行为分析演示 ===")
	// 获取行为画像
	behaviorProfiles := securityManager.GetBehaviorProfiles()
	fmt.Printf("当前行为画像数量: %d\n", len(behaviorProfiles))

	for ip, profile := range behaviorProfiles {
		fmt.Printf("IP: %s, 异常分数: %.2f, 请求速率: %.2f\n", 
			ip, profile.AnomalyScore, profile.RequestRate)
	}

	fmt.Println("\n=== 安全指标演示 ===")
	// 等待一段时间让指标收集
	time.Sleep(2 * time.Second)

	// 获取安全指标
	metrics := securityManager.GetMetrics()
	fmt.Printf("总连接数: %d\n", metrics.TotalConnections)
	fmt.Printf("被阻断连接数: %d\n", metrics.BlockedConnections)
	fmt.Printf("活跃连接数: %d\n", metrics.ActiveConnections)
	fmt.Printf("总威胁数: %d\n", metrics.TotalThreats)
	fmt.Printf("被阻断威胁数: %d\n", metrics.BlockedThreats)
	fmt.Printf("认证尝试数: %d\n", metrics.AuthAttempts)
	fmt.Printf("失败认证数: %d\n", metrics.FailedAuthAttempts)
	fmt.Printf("最后更新时间: %s\n", metrics.LastUpdate.Format(time.RFC3339))

	fmt.Println("\n按威胁类型统计:")
	for threatType, count := range metrics.ThreatsByType {
		fmt.Printf("  %s: %d\n", threatType, count)
	}

	fmt.Println("\n按威胁严重程度统计:")
	for severity, count := range metrics.ThreatsBySeverity {
		fmt.Printf("  %s: %d\n", severity, count)
	}

	fmt.Println("\n=== 配置更新演示 ===")
	// 更新安全配置
	newConfig := security.DefaultOptimizedSecurityConfig()
	newConfig.SecurityLevel = security.SecurityLevelCritical
	newConfig.MaxConnectionsPerIP = 20
	newConfig.BlockDuration = 2 * time.Hour
	securityManager.UpdateConfig(newConfig)
	fmt.Printf("安全配置已更新 - 新安全级别: %v\n", newConfig.SecurityLevel)

	fmt.Println("\n=== 指标导出演示 ===")
	// 导出指标
	metricsData, err := securityManager.ExportMetrics()
	if err != nil {
		fmt.Printf("导出指标失败: %v\n", err)
	} else {
		fmt.Printf("指标数据大小: %d 字节\n", len(metricsData))
		fmt.Printf("指标数据预览: %s...\n", string(metricsData[:min(200, len(metricsData))]))
	}

	fmt.Println("\n=== 性能测试演示 ===")
	// 性能测试
	performanceTest(securityManager)

	fmt.Println("\n=== 安全优化演示完成 ===")
}

// MockConnection 模拟连接
type MockConnection struct {
	remoteAddr string
	localAddr  string
}

func (m *MockConnection) Read(b []byte) (n int, err error) {
	return 0, nil
}

func (m *MockConnection) Write(b []byte) (n int, err error) {
	return len(b), nil
}

func (m *MockConnection) Close() error {
	return nil
}

func (m *MockConnection) LocalAddr() net.Addr {
	addr, _ := net.ResolveTCPAddr("tcp", m.localAddr)
	return addr
}

func (m *MockConnection) RemoteAddr() net.Addr {
	addr, _ := net.ResolveTCPAddr("tcp", m.remoteAddr)
	return addr
}

func (m *MockConnection) SetDeadline(t time.Time) error {
	return nil
}

func (m *MockConnection) SetReadDeadline(t time.Time) error {
	return nil
}

func (m *MockConnection) SetWriteDeadline(t time.Time) error {
	return nil
}

// generateLargeData 生成大数据包
func generateLargeData(size int) string {
	data := make([]byte, size)
	for i := range data {
		data[i] = byte('A' + (i % 26))
	}
	return string(data)
}

// performanceTest 性能测试
func performanceTest(sm *security.OptimizedSecurityManager) {
	fmt.Println("开始性能测试...")

	// 测试连接检查性能
	start := time.Now()
	for i := 0; i < 1000; i++ {
		mockConn := &MockConnection{
			remoteAddr: fmt.Sprintf("192.168.1.%d:8080", i%255+1),
			localAddr:  "127.0.0.1:8080",
		}
		sm.CheckConnection(mockConn, "TestAgent/1.0", "HTTP/1.1")
	}
	connCheckDuration := time.Since(start)
	fmt.Printf("连接检查性能: 1000次检查耗时 %v (平均 %v/次)\n", 
		connCheckDuration, connCheckDuration/1000)

	// 测试数据检查性能
	testData := []byte("GET /api/test HTTP/1.1\r\nHost: example.com\r\n\r\n")
	start = time.Now()
	for i := 0; i < 1000; i++ {
		sm.CheckData(testData, fmt.Sprintf("192.168.1.%d", i%255+1), "127.0.0.1:8080")
	}
	dataCheckDuration := time.Since(start)
	fmt.Printf("数据检查性能: 1000次检查耗时 %v (平均 %v/次)\n", 
		dataCheckDuration, dataCheckDuration/1000)

	// 测试威胁检测性能
	maliciousData := []byte("GET /api/users?id=1' UNION SELECT * FROM users-- HTTP/1.1")
	start = time.Now()
	for i := 0; i < 100; i++ {
		sm.CheckData(maliciousData, fmt.Sprintf("203.0.113.%d", i%255+1), "127.0.0.1:8080")
	}
	threatDetectionDuration := time.Since(start)
	fmt.Printf("威胁检测性能: 100次检查耗时 %v (平均 %v/次)\n", 
		threatDetectionDuration, threatDetectionDuration/100)

	fmt.Println("性能测试完成")
}

// min 返回两个整数中的较小值
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}