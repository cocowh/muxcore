// Copyright (c) 2025 cocowh. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package security

import (
	"bytes"
	"sync"

	"github.com/cocowh/muxcore/pkg/errors"
	"github.com/cocowh/muxcore/pkg/logger"
)

// DPIEngine 深度包检测引擎
// 用于协议解析层的安全防护

type Rule struct {
	ID          string
	Name        string
	Description string
	Pattern     []byte
	Action      string // block, alert, log
}

// DPIEngine 深度包检测引擎

type DPIEngine struct {
	mutex         sync.RWMutex
	rules         []Rule
	enabled       bool
	maxPacketSize int
}

// NewDPIEngine 创建深度包检测引擎实例
func NewDPIEngine(maxPacketSize int) *DPIEngine {
	dpi := &DPIEngine{
		rules:         make([]Rule, 0),
		enabled:       true,
		maxPacketSize: maxPacketSize,
	}

	// 默认加载一些常见规则
	dpi.LoadDefaultRules()

	return dpi
}

// LoadDefaultRules 加载默认规则
func (dpi *DPIEngine) LoadDefaultRules() {
	// 添加一些示例规则
	rules := []Rule{
		{
			ID:          "rule1",
			Name:        "SQL Injection Detection",
			Description: "Detect common SQL injection patterns",
			Pattern:     []byte("' OR '1'='1"),
			Action:      "block",
		},
		{
			ID:          "rule2",
			Name:        "XSS Detection",
			Description: "Detect common XSS patterns",
			Pattern:     []byte("<script>"),
			Action:      "block",
		},
		{
			ID:          "rule3",
			Name:        "Malware Signature",
			Description: "Detect known malware signatures",
			Pattern:     []byte("malware_signature_123"),
			Action:      "block",
		},
	}

	dpi.mutex.Lock()
	dpi.rules = append(dpi.rules, rules...)
	dpi.mutex.Unlock()
}

// AddRule 添加新规则
func (dpi *DPIEngine) AddRule(rule Rule) {
	dpi.mutex.Lock()
	dpi.rules = append(dpi.rules, rule)
	dpi.mutex.Unlock()
}

// RemoveRule 移除规则
func (dpi *DPIEngine) RemoveRule(ruleID string) error {
	dpi.mutex.Lock()
	defer dpi.mutex.Unlock()

	for i, rule := range dpi.rules {
		if rule.ID == ruleID {
			dpi.rules = append(dpi.rules[:i], dpi.rules[i+1:]...)
			return nil
		}
	}

	return errors.New(errors.ErrCodeGovernanceUnknown, errors.CategoryGovernance, errors.LevelWarn, "dpi rule not found")
}

// Detect 检测数据包（返回nil表示通过；返回错误表示阻断或告警）
func (dpi *DPIEngine) Detect(data []byte, protocol string) *errors.MuxError {
	if !dpi.enabled {
		return nil
	}

	// 限制数据包大小
	if len(data) > dpi.maxPacketSize {
		data = data[:dpi.maxPacketSize]
	}

	dpi.mutex.RLock()
	defer dpi.mutex.RUnlock()

	// 针对不同协议进行特定解析
	switch protocol {
	case "http", "https":
		return dpi.detectHTTP(data)
	case "websocket":
		return dpi.detectWebSocket(data)
	case "grpc":
		return dpi.detectGRPC(data)
	default:
		return dpi.detectGeneric(data)
	}
}

// 检测HTTP包
func (dpi *DPIEngine) detectHTTP(data []byte) *errors.MuxError {
	// 检查HTTP请求方法
	methods := [][]byte{
		[]byte("GET "),
		[]byte("POST "),
		[]byte("PUT "),
		[]byte("DELETE "),
		[]byte("HEAD "),
	}

	isHTTP := false
	for _, method := range methods {
		if bytes.HasPrefix(data, method) {
			isHTTP = true
			break
		}
	}

	if !isHTTP {
		return nil
	}

	// 应用规则
	for _, rule := range dpi.rules {
		if bytes.Contains(data, rule.Pattern) {
			logger.Warnf("DPI detected rule violation: %s (%s)", rule.Name, rule.Description)
			return errors.New(errors.ErrCodeGovernancePolicyViolation, errors.CategoryGovernance, errors.LevelWarn, "DPI rule matched: "+rule.ID)
		}
	}

	return nil
}

// 检测WebSocket包
func (dpi *DPIEngine) detectWebSocket(data []byte) *errors.MuxError {
	// 检查WebSocket帧
	if len(data) < 2 {
		return nil
	}

	// 检查WebSocket握手
	if bytes.HasPrefix(data, []byte("GET ")) && bytes.Contains(data, []byte("Upgrade: websocket")) {
		// 这是WebSocket握手请求
		// 应用规则
		for _, rule := range dpi.rules {
			if bytes.Contains(data, rule.Pattern) {
				logger.Warnf("DPI detected rule violation: %s (%s)", rule.Name, rule.Description)
				return errors.New(errors.ErrCodeGovernancePolicyViolation, errors.CategoryGovernance, errors.LevelWarn, "DPI rule matched: "+rule.ID)
			}
		}
	}

	// 处理WebSocket数据帧
	// 这里简化处理，实际应解析WebSocket帧结构

	return nil
}

// 检测GRPC包
func (dpi *DPIEngine) detectGRPC(data []byte) *errors.MuxError {
	// GRPC使用HTTP/2，这里简化处理
	if len(data) < 9 {
		return nil
	}

	// 检查HTTP/2帧头
	// 简化判断
	if data[0]&0x80 == 0x80 {
		// 可能是HTTP/2帧
		// 应用规则
		for _, rule := range dpi.rules {
			if bytes.Contains(data, rule.Pattern) {
				logger.Warnf("DPI detected rule violation: %s (%s)", rule.Name, rule.Description)
				return errors.New(errors.ErrCodeGovernancePolicyViolation, errors.CategoryGovernance, errors.LevelWarn, "DPI rule matched: "+rule.ID)
			}
		}
	}

	return nil
}

// 通用检测
func (dpi *DPIEngine) detectGeneric(data []byte) *errors.MuxError {
	// 应用规则到所有数据
	for _, rule := range dpi.rules {
		if bytes.Contains(data, rule.Pattern) {
			logger.Warnf("DPI detected rule violation: %s (%s)", rule.Name, rule.Description)
			return errors.New(errors.ErrCodeGovernancePolicyViolation, errors.CategoryGovernance, errors.LevelWarn, "DPI rule matched: "+rule.ID)
		}
	}

	return nil
}

// Enable 启用DPI
func (dpi *DPIEngine) Enable() {
	dpi.mutex.Lock()
	dpi.enabled = true
	dpi.mutex.Unlock()
}

// Disable 禁用DPI
func (dpi *DPIEngine) Disable() {
	dpi.mutex.Lock()
	dpi.enabled = false
	dpi.mutex.Unlock()
}

// GetRules 获取规则
func (dpi *DPIEngine) GetRules() []Rule {
	dpi.mutex.RLock()
	defer dpi.mutex.RUnlock()

	return dpi.rules
}