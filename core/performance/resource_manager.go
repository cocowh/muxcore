package performance

import (
	"sync"

	"github.com/cocowh/muxcore/pkg/logger"
)

// ResourceType 资源类型

type ResourceType string

// 定义资源类型
const (
	ResourceTypeHTTP        ResourceType = "http"
	ResourceTypeStreaming   ResourceType = "streaming"
	ResourceTypeSystem      ResourceType = "system"
	ResourceTypeManagement  ResourceType = "management"
)

// ResourceQuota 资源配额

type ResourceQuota struct {
	CPU    int //  CPU核心数百分比
	Memory int // 内存百分比
	IO     int // IO带宽百分比
}

// ResourceManager 资源管理器
// 实现资源隔离和配额管理

type ResourceManager struct {
	mutex           sync.RWMutex
	quotas          map[ResourceType]ResourceQuota
	enabled         bool
	totalCPU        int
	totalMemory     int
	totalIO         int
}

// NewResourceManager 创建资源管理器实例
func NewResourceManager(totalCPU, totalMemory, totalIO int) *ResourceManager {
	// 初始化默认配额（根据用户提供的饼图）
	defaultQuotas := map[ResourceType]ResourceQuota{
		ResourceTypeHTTP: {CPU: 45, Memory: 45, IO: 45},
		ResourceTypeStreaming: {CPU: 30, Memory: 30, IO: 30},
		ResourceTypeSystem: {CPU: 15, Memory: 15, IO: 15},
		ResourceTypeManagement: {CPU: 10, Memory: 10, IO: 10},
	}

	rm := &ResourceManager{
		quotas:      defaultQuotas,
		enabled:     true,
		totalCPU:    totalCPU,
		totalMemory: totalMemory,
		totalIO:     totalIO,
	}

	logger.Infof("Resource manager initialized with CPU: %d cores, Memory: %d MB, IO: %d MB/s",
		totalCPU, totalMemory, totalIO)

	return rm
}

// SetQuota 设置资源配额
func (rm *ResourceManager) SetQuota(resourceType ResourceType, quota ResourceQuota) bool {
	// 验证配额总和不超过100%
	rm.mutex.RLock()
	currentCPU := 0
	currentMemory := 0
	currentIO := 0

	for rt, q := range rm.quotas {
		if rt != resourceType {
			currentCPU += q.CPU
			currentMemory += q.Memory
			currentIO += q.IO
		}
	}

	remainingCPU := 100 - currentCPU
	remainingMemory := 100 - currentMemory
	remainingIO := 100 - currentIO

	rm.mutex.RUnlock()

	// 检查新配额是否有效
	if quota.CPU > remainingCPU || quota.Memory > remainingMemory || quota.IO > remainingIO {
		logger.Warnf("Cannot set quota for %s: exceeds available resources", resourceType)
		return false
	}

	// 更新配额
	rm.mutex.Lock()
	rm.quotas[resourceType] = quota
	rm.mutex.Unlock()

	logger.Infof("Updated quota for %s: CPU=%d%%, Memory=%d%%, IO=%d%%",
		resourceType, quota.CPU, quota.Memory, quota.IO)

	return true
}

// GetQuota 获取资源配额
func (rm *ResourceManager) GetQuota(resourceType ResourceType) (ResourceQuota, bool) {
	rm.mutex.RLock()
	quota, exists := rm.quotas[resourceType]
	rm.mutex.RUnlock()

	return quota, exists
}

// GetAvailableResources 获取可用资源
func (rm *ResourceManager) GetAvailableResources(resourceType ResourceType) (int, int, int) {
	if !rm.enabled {
		// 如果未启用资源限制，返回所有资源
		return rm.totalCPU, rm.totalMemory, rm.totalIO
	}

	rm.mutex.RLock()
	quota, exists := rm.quotas[resourceType]
	rm.mutex.RUnlock()

	if !exists {
		return 0, 0, 0
	}

	// 计算实际可用资源
	cpu := (rm.totalCPU * quota.CPU) / 100
	memory := (rm.totalMemory * quota.Memory) / 100
	io := (rm.totalIO * quota.IO) / 100

	return cpu, memory, io
}

// Enable 启用资源管理
func (rm *ResourceManager) Enable() {
	rm.mutex.Lock()
	rm.enabled = true
	rm.mutex.Unlock()
}

// Disable 禁用资源管理
func (rm *ResourceManager) Disable() {
	rm.mutex.Lock()
	rm.enabled = false
	rm.mutex.Unlock()
}