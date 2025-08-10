package governance

import (
	"fmt"
	"sync"
	"time"

	"github.com/cocowh/muxcore/pkg/errors"
	"github.com/cocowh/muxcore/pkg/logger"
	"github.com/tetratelabs/wazero/api"
)

// ProtocolStatus 协议状态

type ProtocolStatus string

// 定义协议状态
const (
	ProtocolStatusInactive    ProtocolStatus = "inactive"
	ProtocolStatusTesting     ProtocolStatus = "testing"
	ProtocolStatusActive      ProtocolStatus = "active"
	ProtocolStatusDeprecating ProtocolStatus = "deprecating"
	ProtocolStatusDeprecated  ProtocolStatus = "deprecated"
)

// Protocol 协议定义

type Protocol struct {
	ID          string         `json:"id"`
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Version     string         `json:"version"`
	Status      ProtocolStatus `json:"status"`
	Handler     interface{}    `json:"handler"`
	TrafficRate int            `json:"traffic_rate"` // 流量比例(0-100)
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
}

// TrustStore 可信仓库接口

type TrustStore interface {
	// UploadProtocol 上传协议包
	UploadProtocol(protocol *Protocol, signature string) error

	// DownloadProtocol 下载协议包
	DownloadProtocol(protocolID, version string) (*Protocol, error)

	// VerifySignature 验证签名
	VerifySignature(protocol *Protocol, signature string) bool

	// ListProtocols 列出所有协议
	ListProtocols() ([]*Protocol, error)
}

// WASMProtocolLoader 接口定义

type WASMProtocolLoader interface {
	LoadProtocol(protocolID, version string, wasmCode []byte) error
	GetProtocolInstance(protocolID, version string) (api.Module, bool)
	ExecuteProtocol(protocolID, version string, input []byte) ([]byte, error)
	UnloadProtocol(protocolID, version string) error
	InitAllocator(protocolID, version string) error
	Activate()
	Deactivate()
	Close() error
}

// ProtocolManager 协议管理器

type ProtocolManager struct {
	mutex           sync.RWMutex
	protocols       map[string]map[string]*Protocol // map[protocolID]map[version]*Protocol
	activeProtocols map[string]*Protocol            // map[protocolID]*Protocol
	trustStore      TrustStore
	enabled         bool
	policyEngine    *PolicyEngine
	wasmLoader      WASMProtocolLoader
}

// NewProtocolManager 创建协议管理器实例
func NewProtocolManager(trustStore TrustStore, policyEngine *PolicyEngine, wasmLoader WASMProtocolLoader) *ProtocolManager {
	pm := &ProtocolManager{
		protocols:       make(map[string]map[string]*Protocol),
		activeProtocols: make(map[string]*Protocol),
		trustStore:      trustStore,
		enabled:         true,
		policyEngine:    policyEngine,
		wasmLoader:      wasmLoader,
	}

	// 从可信仓库加载协议
	pm.loadProtocols()

	logger.Infof("Protocol manager initialized")

	return pm
}

// LoadWASMProtocol 加载WASM协议模块
func (pm *ProtocolManager) LoadWASMProtocol(protocolID, version string, wasmCode []byte) error {
	if !pm.enabled {
		return errors.GovernanceError(errors.ErrCodeGovernanceUnknown, "protocol manager is disabled")
	}

	if pm.wasmLoader == nil {
		return errors.WASMError(errors.ErrCodeWASMRuntimeError, "WASM protocol loader is not initialized")
	}

	// 加载WASM模块
	if err := pm.wasmLoader.LoadProtocol(protocolID, version, wasmCode); err != nil {
		muxErr := errors.Convert(err).WithContext("protocolID", protocolID).WithContext("version", version)
		return muxErr
	}

	// 创建协议对象
	protocol := &Protocol{
		ID:          protocolID,
		Name:        fmt.Sprintf("WASM Protocol %s", protocolID),
		Description: "Dynamically loaded WASM protocol module",
		Version:     version,
		Status:      ProtocolStatusInactive,
		Handler:     nil, // WASM协议不需要传统处理器
		TrafficRate: 0,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	// 更新本地缓存
	pm.mutex.Lock()
	defer pm.mutex.Unlock()

	if _, exists := pm.protocols[protocol.ID]; !exists {
		pm.protocols[protocol.ID] = make(map[string]*Protocol)
	}

	pm.protocols[protocol.ID][protocol.Version] = protocol

	logger.Infof("Loaded WASM protocol: %s v%s", protocolID, version)

	return nil
}

// 从可信仓库加载协议
func (pm *ProtocolManager) loadProtocols() {
	protocols, err := pm.trustStore.ListProtocols()
	if err != nil {
		logger.Errorf("Failed to load protocols from trust store: %v", err)
		return
	}

	pm.mutex.Lock()
	defer pm.mutex.Unlock()

	for _, protocol := range protocols {
		if _, exists := pm.protocols[protocol.ID]; !exists {
			pm.protocols[protocol.ID] = make(map[string]*Protocol)
		}

		pm.protocols[protocol.ID][protocol.Version] = protocol

		// 如果协议是活动的，则设置为当前活动版本
		if protocol.Status == ProtocolStatusActive {
			pm.activeProtocols[protocol.ID] = protocol
		}
	}
}

// UploadProtocol 上传新协议
func (pm *ProtocolManager) UploadProtocol(protocol *Protocol, signature string) error {
	// 验证签名
	if !pm.trustStore.VerifySignature(protocol, signature) {
		logger.Errorf("Invalid signature for protocol: %s v%s", protocol.ID, protocol.Version)
		return errors.AuthError(errors.ErrCodeAuthUnauthorized, "invalid signature for protocol").WithContext("protocolID", protocol.ID).WithContext("version", protocol.Version)
	}

	// 上传到可信仓库
	if err := pm.trustStore.UploadProtocol(protocol, signature); err != nil {
		return err
	}

	// 更新本地缓存
	pm.mutex.Lock()
	defer pm.mutex.Unlock()

	if _, exists := pm.protocols[protocol.ID]; !exists {
		pm.protocols[protocol.ID] = make(map[string]*Protocol)
	}

	protocol.Status = ProtocolStatusInactive
	protocol.CreatedAt = time.Now()
	protocol.UpdatedAt = time.Now()

	pm.protocols[protocol.ID][protocol.Version] = protocol

	logger.Infof("Uploaded new protocol: %s v%s", protocol.ID, protocol.Version)

	return nil
}

// StartProtocolTesting 开始协议测试
func (pm *ProtocolManager) StartProtocolTesting(protocolID, version string) error {
	pm.mutex.Lock()
	defer pm.mutex.Unlock()

	// 检查协议是否存在
	if _, exists := pm.protocols[protocolID]; !exists {
		logger.Errorf("Protocol not found: %s", protocolID)
		return fmt.Errorf("protocol not found: %s", protocolID)
	}

	protocol, exists := pm.protocols[protocolID][version]
	if !exists {
		logger.Errorf("Protocol version not found: %s v%s", protocolID, version)
		return fmt.Errorf("protocol version not found: %s v%s", protocolID, version)
	}

	// 设置协议状态为测试中
	protocol.Status = ProtocolStatusTesting
	protocol.TrafficRate = 0 // 初始流量为0
	protocol.UpdatedAt = time.Now()

	logger.Infof("Started testing protocol: %s v%s", protocolID, version)

	return nil
}

// UpdateProtocolTraffic 更新协议流量比例
func (pm *ProtocolManager) UpdateProtocolTraffic(protocolID, version string, trafficRate int) error {
	if trafficRate < 0 || trafficRate > 100 {
		logger.Errorf("Invalid traffic rate: %d (must be 0-100)", trafficRate)
		return fmt.Errorf("invalid traffic rate: %d (must be 0-100)", trafficRate)
	}

	pm.mutex.Lock()
	defer pm.mutex.Unlock()

	// 检查协议是否存在
	if _, exists := pm.protocols[protocolID]; !exists {
		logger.Errorf("Protocol not found: %s", protocolID)
		return fmt.Errorf("protocol not found: %s", protocolID)
	}

	protocol, exists := pm.protocols[protocolID][version]
	if !exists {
		logger.Errorf("Protocol version not found: %s v%s", protocolID, version)
		return fmt.Errorf("protocol version not found: %s v%s", protocolID, version)
	}

	// 检查协议是否在测试中或活动状态
	if protocol.Status != ProtocolStatusTesting && protocol.Status != ProtocolStatusActive {
		logger.Errorf("Cannot update traffic for protocol %s v%s: not in testing or active state", protocolID, version)
		return fmt.Errorf("cannot update traffic for protocol %s v%s: not in testing or active state", protocolID, version)
	}

	// 更新流量比例
	protocol.TrafficRate = trafficRate
	protocol.UpdatedAt = time.Now()

	// 如果流量比例为100%，则设置为活动状态
	if trafficRate == 100 {
		protocol.Status = ProtocolStatusActive
		pm.activeProtocols[protocolID] = protocol

		// 标记旧版本为废弃中
		for v, p := range pm.protocols[protocolID] {
			if v != version && p.Status == ProtocolStatusActive {
				p.Status = ProtocolStatusDeprecating
				p.TrafficRate = 0
			}
		}
	}

	logger.Infof("Updated traffic for protocol %s v%s: %d%%", protocolID, version, trafficRate)

	return nil
}

// DeprecateProtocol 废弃协议
func (pm *ProtocolManager) DeprecateProtocol(protocolID, version string) error {
	pm.mutex.Lock()
	defer pm.mutex.Unlock()

	// 检查协议是否存在
	if _, exists := pm.protocols[protocolID]; !exists {
		logger.Errorf("Protocol not found: %s", protocolID)
		return fmt.Errorf("protocol not found: %s", protocolID)
	}

	protocol, exists := pm.protocols[protocolID][version]
	if !exists {
		logger.Errorf("Protocol version not found: %s v%s", protocolID, version)
		return fmt.Errorf("protocol version not found: %s v%s", protocolID, version)
	}

	// 标记协议为废弃中
	protocol.Status = ProtocolStatusDeprecating
	protocol.TrafficRate = 0
	protocol.UpdatedAt = time.Now()

	logger.Infof("Deprecated protocol: %s v%s", protocolID, version)

	return nil
}

// GetActiveProtocol 获取活动的协议版本
func (pm *ProtocolManager) GetActiveProtocol(protocolID string) (*Protocol, bool) {
	pm.mutex.RLock()
	defer pm.mutex.RUnlock()

	protocol, exists := pm.activeProtocols[protocolID]
	return protocol, exists
}

// GetProtocol 获取特定版本的协议
func (pm *ProtocolManager) GetProtocol(protocolID, version string) (*Protocol, bool) {
	pm.mutex.RLock()
	defer pm.mutex.RUnlock()

	if _, exists := pm.protocols[protocolID]; !exists {
		return nil, false
	}

	protocol, exists := pm.protocols[protocolID][version]
	return protocol, exists
}

// ExecuteWASMProtocol 执行WASM协议处理
func (pm *ProtocolManager) ExecuteWASMProtocol(protocolID, version string, input []byte) ([]byte, error) {
	if !pm.enabled {
		return nil, fmt.Errorf("protocol manager is disabled")
	}

	if pm.wasmLoader == nil {
		return nil, fmt.Errorf("WASM protocol loader is not initialized")
	}

	// 检查协议是否存在
	protocol, exists := pm.GetProtocol(protocolID, version)
	if !exists {
		return nil, fmt.Errorf("protocol not found: %s v%s", protocolID, version)
	}

	// 检查协议是否处于活动状态
	if protocol.Status != ProtocolStatusActive && protocol.Status != ProtocolStatusTesting {
		return nil, fmt.Errorf("protocol %s v%s is not active or testing", protocolID, version)
	}

	// 执行WASM协议
	result, err := pm.wasmLoader.ExecuteProtocol(protocolID, version, input)
	if err != nil {
		return nil, fmt.Errorf("failed to execute WASM protocol: %w", err)
	}

	return result, nil
}

// Enable 启用协议管理器
func (pm *ProtocolManager) Enable() {
	pm.mutex.Lock()
	pm.enabled = true
	pm.mutex.Unlock()
}

// Disable 禁用协议管理器
func (pm *ProtocolManager) Disable() {
	pm.mutex.Lock()
	pm.enabled = false
	pm.mutex.Unlock()
}
