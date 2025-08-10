package performance

import (
	"runtime"
	"sync"

	"github.com/cocowh/muxcore/pkg/logger"
)

// 假设的CPU缓存线大小（实际应根据系统查询）
const CacheLineSize = 64

// CacheAligned 缓存对齐结构
// 用于确保关键数据结构不跨越缓存线

type CacheAligned struct {
	padding [CacheLineSize]byte
}

// CPUAffinityManager CPU亲和性管理器
// 负责将goroutine绑定到特定CPU核心

type CPUAffinityManager struct {
	mutex         sync.RWMutex
	coreCount     int
	assignedCores map[int]bool
	enabled       bool
}

// NewCPUAffinityManager 创建CPU亲和性管理器实例
func NewCPUAffinityManager() *CPUAffinityManager {
	coreCount := runtime.NumCPU()

	cam := &CPUAffinityManager{
		coreCount:     coreCount,
		assignedCores: make(map[int]bool),
		enabled:       true,
	}

	logger.Infof("CPU affinity manager initialized with %d cores", coreCount)

	return cam
}

// AssignCore 分配CPU核心
func (cam *CPUAffinityManager) AssignCore() (int, bool) {
	if !cam.enabled {
		return -1, false
	}

	cam.mutex.Lock()
	defer cam.mutex.Unlock()

	// 查找未分配的核心
	for i := 0; i < cam.coreCount; i++ {
		if !cam.assignedCores[i] {
			cam.assignedCores[i] = true
			return i, true
		}
	}

	// 没有可用核心
	return -1, false
}

// ReleaseCore 释放CPU核心
func (cam *CPUAffinityManager) ReleaseCore(coreID int) {
	if !cam.enabled {
		return
	}

	cam.mutex.Lock()
	defer cam.mutex.Unlock()

	if coreID >= 0 && coreID < cam.coreCount {
		cam.assignedCores[coreID] = false
	}
}

// BindToCore 绑定当前goroutine到指定核心
func (cam *CPUAffinityManager) BindToCore(coreID int) error {
	if !cam.enabled || coreID < 0 || coreID >= cam.coreCount {
		return nil
	}

	// 实际绑定代码需要根据操作系统实现
	// 这里简化处理
	logger.Debugf("Binding goroutine to CPU core %d", coreID)

	// 在Linux上，可以使用sched_setaffinity
	// 在 macOS 上，可以使用thread_policy_set
	// 这里省略具体实现

	return nil
}

// Enable 启用CPU亲和性管理
func (cam *CPUAffinityManager) Enable() {
	cam.mutex.Lock()
	cam.enabled = true
	cam.mutex.Unlock()
}

// Disable 禁用CPU亲和性管理
func (cam *CPUAffinityManager) Disable() {
	cam.mutex.Lock()
	cam.enabled = false
	// 释放所有分配的核心
	for i := range cam.assignedCores {
		cam.assignedCores[i] = false
	}
	cam.mutex.Unlock()
}

// ProtocolProcessorWrapper 协议处理器包装器
// 确保每个处理器独占缓存线

type ProtocolProcessorWrapper struct {
	CacheAligned
	Processor interface{}
}

// NewProtocolProcessorWrapper 创建协议处理器包装器
func NewProtocolProcessorWrapper(processor interface{}) *ProtocolProcessorWrapper {
	return &ProtocolProcessorWrapper{
		Processor: processor,
	}
}
