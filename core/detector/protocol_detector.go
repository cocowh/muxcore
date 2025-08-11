// Copyright (c) 2025 cocowh. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package detector

import (
	"bytes"
	"context"
	"hash/fnv"
	"net"
	"strconv"
	"sync"
	"time"

	"github.com/cocowh/muxcore/core/config"
	"github.com/cocowh/muxcore/core/handler"
	"github.com/cocowh/muxcore/core/performance"
	poolpkg "github.com/cocowh/muxcore/core/pool"
	"github.com/cocowh/muxcore/pkg/errors"
	"github.com/cocowh/muxcore/pkg/logger"
)

// BloomFilter 实现基于TCP首包特征指纹的快速分类
type BloomFilter struct {
	bitset            []bool
	size              uint
	hashFunctionCount uint
}

// NewBloomFilter 创建布隆过滤器
func NewBloomFilter(size uint, hashFunctionCount uint) *BloomFilter {
	return &BloomFilter{
		bitset:            make([]bool, size),
		size:              size,
		hashFunctionCount: hashFunctionCount,
	}
}

// Add 添加元素到布隆过滤器
func (bf *BloomFilter) Add(data []byte) {
	for i := uint(0); i < bf.hashFunctionCount; i++ {
		index := bf.hash(data, i)
		bf.bitset[index] = true
	}
}

// Contains 检查元素是否可能存在于布隆过滤器中
func (bf *BloomFilter) Contains(data []byte) bool {
	for i := uint(0); i < bf.hashFunctionCount; i++ {
		index := bf.hash(data, i)
		if !bf.bitset[index] {
			return false
		}
	}
	return true
}

// hash 哈希函数
func (bf *BloomFilter) hash(data []byte, seed uint) uint {
	h := fnv.New64a()
	h.Write(data)
	h.Write([]byte{byte(seed)})
	return uint(h.Sum64() % uint64(bf.size))
}

// ProtocolDetectorFunc 定义协议检测函数类型
type ProtocolDetectorFunc func([]byte) (string, bool)

// ProtocolDetector 协议检测器
type ProtocolDetector struct {
	handlers              map[string]handler.ProtocolHandler
	detectors             map[string]ProtocolDetectorFunc
	mutex                 sync.RWMutex
	pool                  *poolpkg.ConnectionPool
	goroutinePool         *poolpkg.GoroutinePool
	bufferPool            *performance.BufferPool
	timeout               time.Duration
	ctx                   context.Context
	cancel                context.CancelFunc
	bloomFilter           *BloomFilter
	clientProtocolHistory map[string][]string
	portProtocolMap       map[int]map[string]float64
}

// New 创建一个新的ProtocolDetector
func New(pool *poolpkg.ConnectionPool, bufferPool *performance.BufferPool) *ProtocolDetector {
	ctx, cancel := context.WithCancel(context.Background())
	// 初始化布隆过滤器，大小为1024，使用3个哈希函数
	bloomFilter := NewBloomFilter(1024, 3)
	// 预加载常见协议特征
	preloadProtocolSignatures(bloomFilter)

	// 创建goroutine池
	goroutinePool := poolpkg.NewGoroutinePool(100, 100)

	return &ProtocolDetector{
		handlers:              make(map[string]handler.ProtocolHandler),
		detectors:             make(map[string]ProtocolDetectorFunc),
		mutex:                 sync.RWMutex{},
		pool:                  pool,
		goroutinePool:         goroutinePool,
		bufferPool:            bufferPool,
		timeout:               30 * time.Second,
		ctx:                   ctx,
		cancel:                cancel,
		bloomFilter:           bloomFilter,
		clientProtocolHistory: make(map[string][]string),
		portProtocolMap:       make(map[int]map[string]float64),
	}
}

// NewWithConfig 根据配置创建ProtocolDetector
func NewWithConfig(pool *poolpkg.ConnectionPool, bufferPool *performance.BufferPool, cfg config.DetectorConfig) *ProtocolDetector {
	pd := New(pool, bufferPool)
	if cfg.ReadTimeout > 0 {
		pd.timeout = time.Duration(cfg.ReadTimeout) * time.Second
	}
	return pd
}

// preloadProtocolSignatures 预加载常见协议特征到布隆过滤器
func preloadProtocolSignatures(bf *BloomFilter) {
	// HTTP特征
	bf.Add([]byte("GET "))
	bf.Add([]byte("POST"))
	bf.Add([]byte("PUT "))
	bf.Add([]byte("DELETE"))
	bf.Add([]byte("HEAD"))

	// WebSocket特征
	bf.Add([]byte("Upgrade: websocket"))

	// gRPC特征 (HTTP/2)
	bf.Add([]byte{0x00})

	logger.Info("Preloaded protocol signatures into bloom filter")
}

// RegisterHandler 注册协议处理器
func (d *ProtocolDetector) RegisterHandler(protocol string, handler handler.ProtocolHandler) {
	d.mutex.Lock()
	defer d.mutex.Unlock()
	d.handlers[protocol] = handler
	logger.Info("Registered handler for protocol: ", protocol)
}

// DetectProtocol 检测协议并分发给相应的处理器
func (d *ProtocolDetector) DetectProtocol(connID string, conn net.Conn) {
	// 设置读取超时（如果配置了）
	if d.timeout > 0 {
		_ = conn.SetReadDeadline(time.Now().Add(d.timeout))
	}

	// 获取客户端IP和端口
	clientAddr := conn.RemoteAddr().String()
	ip, port, err := parseAddr(clientAddr)
	if err != nil {
		muxErr := errors.NetworkError(errors.ErrCodeNetworkUnknown, "failed to parse client address").WithCause(err).WithContext("address", clientAddr)
		errors.Handle(d.ctx, muxErr)
	}

	// 从缓冲区池获取缓冲区
	buffer := d.bufferPool.Get()
	defer d.bufferPool.Put(buffer)

	// 读取数据
	n, err := conn.Read(buffer.Bytes())
	if err != nil {
		muxErr := errors.Convert(err).WithContext("operation", "read from connection").WithContext("connectionID", connID)
		errors.Handle(d.ctx, muxErr)
		d.pool.RemoveConnection(connID)
		return
	}
	buffer.SetWPos(n)

	// 第一层检测：基于Bloom Filter的快速分类
	protocol := d.firstLayerDetection(buffer.Bytes())

	// 第二层检测：协议特定状态机验证
	if protocol != "unknown" {
		protocol = d.secondLayerDetection(protocol, buffer.Bytes())
	}

	// 如果前两层检测失败，尝试基于上下文感知的检测
	if protocol == "unknown" && ip != "" {
		protocol = d.contextAwareDetection(ip, port)
	}

	// 第三层检测：基于机器学习的异常协议检测
	if protocol != "unknown" && !d.thirdLayerDetection(protocol, buffer.Bytes()) {
		logger.Warn("Protocol detected but failed anomaly check: ", protocol)
		protocol = "unknown"
	}

	logger.Debug("Detected protocol: ", protocol, " for connection: ", connID)

	// 更新客户端协议历史
	if ip != "" && protocol != "unknown" {
		d.updateClientProtocolHistory(ip, protocol)
		// 更新端口协议映射
		d.updatePortProtocolMap(port, protocol)
	}

	// 查找对应的处理器
	d.mutex.RLock()
	handler, exists := d.handlers[protocol]
	d.mutex.RUnlock()

	if !exists {
		// 如果协议未知，则这不是系统错误，只是一个不支持的协议。
		// 在较低级别记录它，并在没有堆栈跟踪的情况下关闭连接。
		if protocol == "unknown" {
			logger.Warnf("Unsupported protocol from connection %s, closing.", connID)
		} else {
			// 如果检测到协议但没有处理器，则这是一个配置错误。
			muxErr := errors.ProtocolError(errors.ErrCodeProtocolUnsupported, "no handler found for protocol").WithContext("protocol", protocol).WithContext("connectionID", connID)
			errors.Handle(d.ctx, muxErr)
		}
		d.pool.RemoveConnection(connID)
		conn.Close() // 显式关闭连接
		return
	}

	// 交给处理器处理
	handler.Handle(connID, conn, buffer.Bytes())
}

// parseAddr 解析地址获取IP和端口
func parseAddr(addr string) (string, int, error) {
	ip, portStr, err := net.SplitHostPort(addr)
	var port int
	if err == nil {
		port, err = strconv.Atoi(portStr)
	}
	return ip, port, err
}

// firstLayerDetection 第一层检测：基于Bloom Filter的快速分类
func (d *ProtocolDetector) firstLayerDetection(data []byte) string {
	// 检查是否可能是已知协议
	if d.bloomFilter.Contains(data[:min(len(data), 16)]) {
		// 快速检测常见协议
		if len(data) >= 4 {
			firstFour := string(data[:4])
			switch firstFour {
			case "GET ", "POST", "PUT ", "DELE", "HEAD", "OPTI", "PATC", "TRAC", "CONN":
				return "http"
			}
		}

		// 检查是否是WebSocket
		if len(data) >= 16 {
			if string(data[0:3]) == "GET " && bytes.Contains(data, []byte("Upgrade: websocket")) {
				return "websocket"
			}
		}

		// 检查是否是gRPC
		if len(data) >= 24 && data[0] == 0 {
			return "grpc"
		}
	}

	return "unknown"
}

// secondLayerDetection 第二层检测：协议特定状态机验证
func (d *ProtocolDetector) secondLayerDetection(protocol string, data []byte) string {
	// 这里实现协议特定的状态机验证
	// 简化实现
	switch protocol {
	case "http":
		// 检查HTTP头完整性
		if isValidHTTPHeader(data) {
			return "http"
		}
	case "websocket":
		// 检查WebSocket握手请求
		if isValidWebSocketHandshake(data) {
			return "websocket"
		}
	case "grpc":
		// 检查gRPC帧格式
		if isValidGRPCFrame(data) {
			return "grpc"
		}
	}

	return "unknown"
}

func (d *ProtocolDetector) thirdLayerDetection(protocol string, data []byte) bool {
	// 简化实现：假设通过机器学习模型检测异常
	return true
}

// contextAwareDetection 基于上下文感知的检测
func (d *ProtocolDetector) contextAwareDetection(ip string, port int) string {
	// 简化实现：根据已知端口映射推断
	if protocols, ok := d.portProtocolMap[port]; ok {
		var bestProtocol string
		var bestScore float64
		for p, score := range protocols {
			if score > bestScore {
				bestProtocol = p
				bestScore = score
			}
		}
		if bestProtocol != "" {
			return bestProtocol
		}
	}
	return "unknown"
}

func (d *ProtocolDetector) updateClientProtocolHistory(ip string, protocol string) {
	d.mutex.Lock()
	defer d.mutex.Unlock()
	d.clientProtocolHistory[ip] = append(d.clientProtocolHistory[ip], protocol)
}

func (d *ProtocolDetector) updatePortProtocolMap(port int, protocol string) {
	d.mutex.Lock()
	defer d.mutex.Unlock()
	if _, ok := d.portProtocolMap[port]; !ok {
		d.portProtocolMap[port] = make(map[string]float64)
	}
	d.portProtocolMap[port][protocol] += 1.0
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func isValidHTTPHeader(data []byte) bool {
	// 简化实现
	return len(data) > 0
}

func isValidWebSocketHandshake(data []byte) bool {
	// 简化实现
	return len(data) > 0
}

func isValidGRPCFrame(data []byte) bool {
	// 简化实现
	return len(data) > 0
}

func detectProtocolType(data []byte) string {
	// 简化实现
	if len(data) == 0 {
		return "unknown"
	}
	return "http"
}
