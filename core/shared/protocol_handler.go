package shared

import (
	"net"

	"github.com/cocowh/muxcore/core/pool"
)

// ProtocolHandler 协议处理器接口
type ProtocolHandler interface {
	// Handle 处理连接
	Handle(connID string, conn net.Conn, initialData []byte)
}

// BaseHandler 基础处理器实现
type BaseHandler struct {
	Pool *pool.ConnectionPool
}

// NewBaseHandler 创建基础处理器
func NewBaseHandler(pool *pool.ConnectionPool) *BaseHandler {
	return &BaseHandler{
		Pool: pool,
	}
}
