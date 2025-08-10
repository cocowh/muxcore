// Copyright (c) 2025 cocowh. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package buffer

import (
	"encoding/binary"
	"sync"
	"sync/atomic"
	"unsafe"
)

// UnifiedBufferPool 统一的缓冲区池实现
// 结合了 sync.Pool 的 GC 友好特性和无锁链表的高性能特性
type UnifiedBufferPool struct {
	// 使用 sync.Pool 作为主要的缓冲区池
	syncPool sync.Pool

	// 无锁链表用于预分配的大缓冲区
	head  unsafe.Pointer // 指向空闲链表的头节点
	count uint64         // 池中预分配缓冲区的数量

	// 配置参数
	defaultSize int
	maxPoolSize int
}

// bufferNode 表示链表中的一个节点
type bufferNode struct {
	buf  *BytesBuffer
	next *bufferNode
}

// NewUnifiedBufferPool 创建统一的缓冲区池
func NewUnifiedBufferPool(defaultSize, maxPoolSize int) *UnifiedBufferPool {
	if defaultSize <= 0 {
		defaultSize = 4096
	}
	if maxPoolSize <= 0 {
		maxPoolSize = 1000
	}

	pool := &UnifiedBufferPool{
		defaultSize: defaultSize,
		maxPoolSize: maxPoolSize,
	}

	// 初始化 sync.Pool
	pool.syncPool = sync.Pool{
		New: func() interface{} {
			return &BytesBuffer{
				buf:   make([]byte, defaultSize),
				temp:  make([]byte, 8),
				order: binary.BigEndian,
			}
		},
	}

	return pool
}

// Get 获取缓冲区
// 优先从无锁链表获取预分配的缓冲区，如果没有则从 sync.Pool 获取
func (p *UnifiedBufferPool) Get() *BytesBuffer {
	// 首先尝试从无锁链表获取
	for {
		currentHead := atomic.LoadPointer(&p.head)
		if currentHead == nil {
			break // 链表为空，使用 sync.Pool
		}

		// 尝试将头节点更新为下一个节点
		nextNode := (*bufferNode)(currentHead).next
		if atomic.CompareAndSwapPointer(&p.head, currentHead, unsafe.Pointer(nextNode)) {
			// 成功获取节点
			buf := (*bufferNode)(currentHead).buf
			atomic.AddUint64(&p.count, ^uint64(0)) // 减少计数
			buf.Reset()                            // 确保缓冲区是干净的
			return buf
		}
		// 失败，重试
	}

	// 从 sync.Pool 获取
	buf := p.syncPool.Get().(*BytesBuffer)
	buf.Reset()
	return buf
}

// Put 归还缓冲区
// 根据池的大小决定是放入无锁链表还是 sync.Pool
func (p *UnifiedBufferPool) Put(buf *BytesBuffer) {
	if buf == nil {
		return
	}

	// 重置缓冲区
	buf.Reset()

	// 如果预分配池未满，优先放入无锁链表
	if atomic.LoadUint64(&p.count) < uint64(p.maxPoolSize) {
		// 创建新节点
		newNode := &bufferNode{
			buf:  buf,
			next: nil,
		}

		// 将新节点添加到链表头部
		for {
			currentHead := atomic.LoadPointer(&p.head)
			newNode.next = (*bufferNode)(currentHead)
			if atomic.CompareAndSwapPointer(&p.head, currentHead, unsafe.Pointer(newNode)) {
				atomic.AddUint64(&p.count, 1) // 增加计数
				return
			}
			// 失败，重试
		}
	}

	// 放入 sync.Pool
	p.syncPool.Put(buf)
}

// PreAllocate 预分配一些缓冲区到池中
func (p *UnifiedBufferPool) PreAllocate(count int) {
	for i := 0; i < count && int(atomic.LoadUint64(&p.count)) < p.maxPoolSize; i++ {
		buf := &BytesBuffer{
			buf:   make([]byte, p.defaultSize),
			temp:  make([]byte, 8),
			order: binary.BigEndian,
		}
		p.Put(buf)
	}
}

// Count 返回预分配池中当前的缓冲区数量
func (p *UnifiedBufferPool) Count() uint64 {
	return atomic.LoadUint64(&p.count)
}

// Stats 返回池的统计信息
func (p *UnifiedBufferPool) Stats() map[string]interface{} {
	return map[string]interface{}{
		"preallocated_count": atomic.LoadUint64(&p.count),
		"default_size":       p.defaultSize,
		"max_pool_size":      p.maxPoolSize,
	}
}

// 全局统一缓冲区池实例
var (
	globalUnifiedPool     *UnifiedBufferPool
	globalUnifiedPoolOnce sync.Once
)

// GetGlobalUnifiedPool 获取全局统一缓冲区池
func GetGlobalUnifiedPool() *UnifiedBufferPool {
	globalUnifiedPoolOnce.Do(func() {
		globalUnifiedPool = NewUnifiedBufferPool(4096, 1000)
		// 预分配一些缓冲区
		globalUnifiedPool.PreAllocate(100)
	})
	return globalUnifiedPool
}

// AcquireBuffer 从全局池获取缓冲区（兼容旧接口）
func AcquireBuffer() *BytesBuffer {
	return GetGlobalUnifiedPool().Get()
}

// ReleaseBuffer 归还缓冲区到全局池（兼容旧接口）
func ReleaseBuffer(buf *BytesBuffer) {
	GetGlobalUnifiedPool().Put(buf)
}
