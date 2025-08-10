package pool

import (
    "sync"
    "context"
    "github.com/cocowh/muxcore/pkg/logger"
)

// GoroutinePool 实现一个可复用的goroutine池
type GoroutinePool struct {
    tasks     chan func()
    workers   int
    wg        sync.WaitGroup
    ctx       context.Context
    cancel    context.CancelFunc
}

// NewGoroutinePool 创建goroutine池
func NewGoroutinePool(workers int, queueSize int) *GoroutinePool {
    if workers <= 0 {
        workers = 1
    }
    if queueSize <= 0 {
        queueSize = 100
    }

    ctx, cancel := context.WithCancel(context.Background())
    pool := &GoroutinePool{
        tasks:   make(chan func(), queueSize),
        workers: workers,
        ctx:     ctx,
        cancel:  cancel,
    }

    // 启动工作goroutine
    pool.wg.Add(workers)
    for i := 0; i < workers; i++ {
        go pool.worker()
    }

    logger.Infof("Created goroutine pool with %d workers and queue size %d", workers, queueSize)
    return pool
}

// worker 工作goroutine
func (p *GoroutinePool) worker() {
    defer p.wg.Done()
    for {
        select {
        case task := <-p.tasks:
            task()
        case <-p.ctx.Done():
            logger.Debug("Goroutine pool worker exiting")
            return
        }
    }
}

// Submit 提交任务到池中
func (p *GoroutinePool) Submit(task func()) {
    select {
    case p.tasks <- task:
        // 任务已提交
    case <-p.ctx.Done():
        logger.Warn("Cannot submit task to closed goroutine pool")
    default:
        // 队列已满，创建新的goroutine执行
        logger.Debug("Goroutine pool queue full, creating temporary worker")
        go task()
    }
}

// Shutdown 关闭池
func (p *GoroutinePool) Shutdown() {
    p.cancel()
    p.wg.Wait()
    logger.Info("Goroutine pool shutdown completed")
}

// GetQueueSize 返回当前队列大小
func (p *GoroutinePool) GetQueueSize() int {
    return len(p.tasks)
}