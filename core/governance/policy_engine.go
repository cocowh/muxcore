package governance

import (
	"fmt"
	"encoding/json"
	"sync"
	"time"

	"github.com/cocowh/muxcore/pkg/logger"
)

// PolicyType 策略类型

type PolicyType string

// 定义策略类型
const (
	PolicyTypeProtocol      PolicyType = "protocol"
	PolicyTypeRouting       PolicyType = "routing"
	PolicyTypeLoadBalancing PolicyType = "load_balancing"
	PolicyTypeSecurity      PolicyType = "security"
	PolicyTypePerformance   PolicyType = "performance"
)

// Policy 策略定义

type Policy struct {
	ID          string                 `json:"id"`
	Type        PolicyType             `json:"type"`
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Content     map[string]interface{} `json:"content"`
	Version     int                    `json:"version"`
	Active      bool                   `json:"active"`
	CreatedAt   time.Time              `json:"created_at"`
	UpdatedAt   time.Time              `json:"updated_at"`
}

// PolicyEngine 运行时策略引擎

type PolicyEngine struct {
	mutex       sync.RWMutex
	policies    map[string]*Policy
	listeners   map[PolicyType][]PolicyChangeListener
	enabled     bool
	trustStore  TrustStore
}

// PolicyChangeListener 策略变更监听器

type PolicyChangeListener interface {
	OnPolicyChanged(policy *Policy)
}

// NewPolicyEngine 创建策略引擎实例
func NewPolicyEngine(trustStore TrustStore) *PolicyEngine {
	pe := &PolicyEngine{
		policies:    make(map[string]*Policy),
		listeners:   make(map[PolicyType][]PolicyChangeListener),
		enabled:     true,
		trustStore:  trustStore,
	}

	logger.Infof("Policy engine initialized")

	return pe
}

// AddPolicy 添加策略
func (pe *PolicyEngine) AddPolicy(policy *Policy) error {
	// 验证策略
	if err := pe.validatePolicy(policy); err != nil {
		return err
	}

	pe.mutex.Lock()
	defer pe.mutex.Unlock()

	// 检查策略是否已存在
	if existingPolicy, exists := pe.policies[policy.ID]; exists {
		// 更新策略
		existingPolicy.Type = policy.Type
		existingPolicy.Name = policy.Name
		existingPolicy.Description = policy.Description
		existingPolicy.Content = policy.Content
		existingPolicy.Version += 1
		existingPolicy.UpdatedAt = time.Now()

		// 如果策略变为活动状态或内容发生变化，则通知监听器
		if existingPolicy.Active != policy.Active || !isContentEqual(existingPolicy.Content, policy.Content) {
			existingPolicy.Active = policy.Active
			pe.notifyListeners(existingPolicy)
		}

		logger.Infof("Updated policy: %s (version %d)", existingPolicy.ID, existingPolicy.Version)
	} else {
		// 新增策略
		newPolicy := *policy
		newPolicy.Version = 1
		newPolicy.CreatedAt = time.Now()
		newPolicy.UpdatedAt = time.Now()

		pe.policies[newPolicy.ID] = &newPolicy

		// 如果策略是活动的，则通知监听器
		if newPolicy.Active {
			pe.notifyListeners(&newPolicy)
		}

		logger.Infof("Added new policy: %s", newPolicy.ID)
	}

	return nil
}

// 删除策略
func (pe *PolicyEngine) RemovePolicy(policyID string) bool {
	pe.mutex.Lock()
	defer pe.mutex.Unlock()

	policy, exists := pe.policies[policyID]
	if !exists {
		return false
	}

	// 标记策略为非活动状态
	policy.Active = false
	pe.notifyListeners(policy)

	// 删除策略
	delete(pe.policies, policyID)

	logger.Infof("Removed policy: %s", policyID)

	return true
}

// GetPolicy 获取策略
func (pe *PolicyEngine) GetPolicy(policyID string) (*Policy, bool) {
	pe.mutex.RLock()
	defer pe.mutex.RUnlock()

	policy, exists := pe.policies[policyID]
	return policy, exists
}

// GetPoliciesByType 按类型获取策略
func (pe *PolicyEngine) GetPoliciesByType(policyType PolicyType) []*Policy {
	pe.mutex.RLock()
	defer pe.mutex.RUnlock()

	policies := make([]*Policy, 0)
	for _, policy := range pe.policies {
		if policy.Type == policyType {
			policies = append(policies, policy)
		}
	}

	return policies
}

// RegisterListener 注册策略变更监听器
func (pe *PolicyEngine) RegisterListener(policyType PolicyType, listener PolicyChangeListener) {
	pe.mutex.Lock()
	defer pe.mutex.Unlock()

	if _, exists := pe.listeners[policyType]; !exists {
		pe.listeners[policyType] = make([]PolicyChangeListener, 0)
	}

	pe.listeners[policyType] = append(pe.listeners[policyType], listener)
}

// 通知策略变更
func (pe *PolicyEngine) notifyListeners(policy *Policy) {
	listeners, exists := pe.listeners[policy.Type]
	if !exists {
		return
	}

	// 在锁外通知监听器，避免死锁
	go func() {
		for _, listener := range listeners {
			listener.OnPolicyChanged(policy)
		}
	}()
}

// 验证策略
func (pe *PolicyEngine) validatePolicy(policy *Policy) error {
	if policy.ID == "" {
		logger.Errorf("Policy ID cannot be empty")
		return fmt.Errorf("policy ID cannot be empty")
	}

	if policy.Type == "" {
		logger.Errorf("Policy type cannot be empty for policy: %s", policy.ID)
		return fmt.Errorf("policy type cannot be empty for policy: %s", policy.ID)
	}

	// 检查策略内容是否有效
	if policy.Content == nil {
		logger.Errorf("Policy content cannot be nil for policy: %s", policy.ID)
		return fmt.Errorf("policy content cannot be nil for policy: %s", policy.ID)
	}

	// 检查数字签名（如果有）
	// 这里简化处理

	return nil
}

// 比较策略内容是否相等
func isContentEqual(content1, content2 map[string]interface{}) bool {
	// 转换为JSON进行比较
	json1, err1 := json.Marshal(content1)
	json2, err2 := json.Marshal(content2)

	if err1 != nil || err2 != nil {
		return false
	}

	return string(json1) == string(json2)
}

// Enable 启用策略引擎
func (pe *PolicyEngine) Enable() {
	pe.mutex.Lock()
	pe.enabled = true
	pe.mutex.Unlock()
}

// Disable 禁用策略引擎
func (pe *PolicyEngine) Disable() {
	pe.mutex.Lock()
	pe.enabled = false
	pe.mutex.Unlock()
}