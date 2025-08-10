package performance

import (
	"sync"
	"syscall"

	"github.com/cocowh/muxcore/pkg/logger"
)

// MemoryPageSize 内存页大小
var MemoryPageSize = syscall.Getpagesize()

// MemoryManager 内存管理器
// 负责优化内存分配和访问模式

type MemoryManager struct {
	mutex             sync.RWMutex
	sharedPages       map[string][]byte
	enabled           bool
	numaAware         bool
	preallocatedPages int
}

// NewMemoryManager 创建内存管理器实例
func NewMemoryManager(preallocatedPages int, numaAware bool) *MemoryManager {
	mm := &MemoryManager{
		sharedPages:       make(map[string][]byte),
		enabled:           true,
		numaAware:         numaAware,
		preallocatedPages: preallocatedPages,
	}

	// 预分配内存页
	mm.preallocateMemory()

	return mm
}

// 预分配内存页
func (mm *MemoryManager) preallocateMemory() {
	if mm.preallocatedPages <= 0 {
		return
	}

	pageSize := MemoryPageSize
	totalSize := mm.preallocatedPages * pageSize

	// 分配大块内存
	// 使用MAP_ANON而不是MAP_ANONYMOUS以兼容macOS
	memory, err := syscall.Mmap(-1, 0, totalSize, syscall.PROT_READ|syscall.PROT_WRITE, syscall.MAP_PRIVATE|syscall.MAP_ANON)
	if err != nil {
		logger.Errorf("Failed to preallocate memory: %v", err)
		return
	}

	// 将内存分为多个页
	for i := 0; i < mm.preallocatedPages; i++ {
		pageStart := i * pageSize
		pageEnd := pageStart + pageSize
		pageID := "prealloc_" + string(rune(i))

		mm.mutex.Lock()
		mm.sharedPages[pageID] = memory[pageStart:pageEnd]
		mm.mutex.Unlock()
	}

	logger.Infof("Preallocated %d memory pages (%d bytes total)", mm.preallocatedPages, totalSize)
}

// GetSharedPage 获取共享内存页
func (mm *MemoryManager) GetSharedPage(pageID string) ([]byte, bool) {
	if !mm.enabled {
		// 如果未启用，直接返回新分配的内存
		page := make([]byte, MemoryPageSize)
		return page, true
	}

	mm.mutex.RLock()
	page, exists := mm.sharedPages[pageID]
	mm.mutex.RUnlock()

	return page, exists
}

// AllocateAlignedMemory 分配对齐内存
func (mm *MemoryManager) AllocateAlignedMemory(size int, alignment int) ([]byte, error) {
	// 确保大小为内存页的倍数
	size = ((size + MemoryPageSize - 1) / MemoryPageSize) * MemoryPageSize

	// 分配内存
	// 使用MAP_ANON而不是MAP_ANONYMOUS以兼容macOS
	// 移除了MAP_ALIGNED_SUPER，因为它在macOS上不可用
	// 如果需要内存对齐，可以在分配后手动处理
	memory, err := syscall.Mmap(-1, 0, size, syscall.PROT_READ|syscall.PROT_WRITE, syscall.MAP_PRIVATE|syscall.MAP_ANON)

	if err != nil {
		return nil, err
	}

	// 如果启用了NUMA感知，将内存绑定到当前NUMA节点
	if mm.numaAware {
		// 获取当前线程ID
		// 在不同系统上获取线程ID的方式不同，这里针对Linux系统使用syscall.Syscall获取
		threadID, _, errno := syscall.Syscall(syscall.SYS_GETTID, 0, 0, 0)
		if errno != 0 {
			logger.Errorf("Failed to get thread ID: %v", errno)
			threadID = 0
		}

		// 获取当前NUMA节点
		// 注意：这部分代码需要根据实际系统进行调整
		// 这里简化处理
		numaNode := 0

		// 将内存绑定到NUMA节点
		// 实际实现需要特定系统调用
		logger.Debugf("Binding memory to NUMA node %d for thread %d", numaNode, threadID)
	}

	return memory, nil
}

// FreeMemory 释放内存
func (mm *MemoryManager) FreeMemory(memory []byte) error {
	return syscall.Munmap(memory)
}

// Enable 启用内存管理器
func (mm *MemoryManager) Enable() {
	mm.mutex.Lock()
	mm.enabled = true
	mm.mutex.Unlock()
}

// Disable 禁用内存管理器
func (mm *MemoryManager) Disable() {
	mm.mutex.Lock()
	mm.enabled = false
	mm.mutex.Unlock()
}

// SetNUMAAware 设置NUMA感知
func (mm *MemoryManager) SetNUMAAware(enabled bool) {
	mm.mutex.Lock()
	mm.numaAware = enabled
	mm.mutex.Unlock()
}
