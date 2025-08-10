package performance

import (
	"github.com/cocowh/muxcore/pkg/buffer"
	"github.com/cocowh/muxcore/pkg/logger"
)

// BufferPool 高性能缓冲区池（基于统一缓冲区池）
type BufferPool struct {
	unifiedPool *buffer.UnifiedBufferPool
}

// NewBufferPool 创建高性能缓冲区池
func NewBufferPool() *BufferPool {
	return &BufferPool{
		unifiedPool: buffer.NewUnifiedBufferPool(4096, 1000),
	}
}

// Get 获取缓冲区
func (p *BufferPool) Get() *buffer.BytesBuffer {
	buf := p.unifiedPool.Get()
	logger.Debug("Retrieved buffer from unified pool")
	return buf
}

// Put 归还缓冲区
func (p *BufferPool) Put(buf *buffer.BytesBuffer) {
	if buf == nil {
		return
	}
	p.unifiedPool.Put(buf)
}

// PreAllocate 预分配一些缓冲区到池中
func (p *BufferPool) PreAllocate(count int) {
	p.unifiedPool.PreAllocate(count)
	logger.Infof("Preallocated %d buffers in unified pool", count)
}

// Count 返回池中当前的缓冲区数量
func (p *BufferPool) Count() uint64 {
	return p.unifiedPool.Count()
}

// Stats 返回池的统计信息
func (p *BufferPool) Stats() map[string]interface{} {
	return p.unifiedPool.Stats()
}
