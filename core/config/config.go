// Copyright (c) 2025 cocowh. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package config

import (
	"fmt"
	"reflect"
	"runtime/debug"
	"strings"
	"sync"

	"github.com/cocowh/muxcore/core/common"
	"github.com/cocowh/muxcore/core/iface"
	"github.com/cocowh/muxcore/core/utils"
	"github.com/cocowh/muxcore/pkg/logger"
)

type BaseConfig struct {
	configs   map[string]any
	callbacks map[string][]iface.CallbackFunc
	mu        sync.RWMutex
}

func NewBaseConfig() *BaseConfig {
	return &BaseConfig{
		configs:   make(map[string]any),
		callbacks: make(map[string][]iface.CallbackFunc),
		mu:        sync.RWMutex{},
	}
}

func (c *BaseConfig) Get(key string) (any, bool) {
	c.mu.RLock()
	value, ok := c.configs[key]
	c.mu.RUnlock()
	return value, ok
}

func (c *BaseConfig) Set(key string, value any) {
	c.mu.Lock()
	oldValue, ok := c.configs[key]
	c.configs[key] = value
	c.mu.Unlock()
	if ok {
		c.mu.RLock()
		callbacks, exists := c.callbacks[key]
		c.mu.RUnlock()
		if exists {
			for _, callback := range callbacks {
				go func() {
					defer utils.PanicHandler(func() {
						logger.Errorf("callback panic. key:%s, oldValue:%v, newValue:%v, stack:%s", key, oldValue, value, debug.Stack())
					})
					callback(oldValue, value)
				}()
			}
		}
	}
}

func (c *BaseConfig) Load() error {
	return nil
}

func (c *BaseConfig) Save() error {
	return nil
}

func (c *BaseConfig) Has(key string) bool {
	c.mu.RLock()
	_, ok := c.configs[key]
	c.mu.RUnlock()
	return ok
}

func (c *BaseConfig) Unset(key string) {
	c.mu.Lock()
	delete(c.configs, key)
	c.mu.Unlock()
}

func (c *BaseConfig) GetString(key string) (string, bool) {
	value, ok := c.Get(key)
	if !ok {
		return "", false
	}
	switch v := value.(type) {
	case string:
		return v, true
	default:
		return fmt.Sprintf("%v", v), true
	}
}

func (c *BaseConfig) GetInt(key string) (int, bool) {
	value, ok := c.Get(key)
	if !ok {
		return 0, false
	}
	switch v := value.(type) {
	case int:
		return v, true
	case int8, int16, int32, int64:
		return int(reflect.ValueOf(v).Int()), true
	case uint, uint8, uint16, uint32, uint64:
		return int(reflect.ValueOf(v).Uint()), true
	case float32, float64:
		return int(reflect.ValueOf(v).Float()), true
	case string:
		return utils.StringToInt(v)
	default:
		return 0, false
	}
}

func (c *BaseConfig) GetBool(key string) (bool, bool) {
	value, ok := c.Get(key)
	if !ok {
		return false, false
	}
	switch v := value.(type) {
	case bool:
		return v, true
	case string:
		return utils.StringToBool(v)
	default:
		return false, false
	}
}

func (c *BaseConfig) GetFloat(key string) (float64, bool) {
	value, ok := c.Get(key)
	if !ok {
		return 0, false
	}

	switch v := value.(type) {
	case float64:
		return v, true
	case float32:
		return float64(v), true
	case int, int8, int16, int32, int64:
		return float64(reflect.ValueOf(v).Int()), true
	case uint, uint8, uint16, uint32, uint64:
		return float64(reflect.ValueOf(v).Uint()), true
	case string:
		return utils.StringToFloat(v)
	default:
		return 0, false
	}
}

func (c *BaseConfig) GetStringSlice(key string) ([]string, bool) {
	value, ok := c.Get(key)
	if !ok {
		return nil, false
	}

	switch v := value.(type) {
	case []string:
		return v, true
	case []any:
		result := make([]string, 0, len(v))
		for _, item := range v {
			result = append(result, fmt.Sprintf("%v", item))
		}
		return result, true
	case string:
		return utils.StringToStringSlice(v)
	default:
		return nil, false
	}
}

func (c *BaseConfig) GetMap(key string) (map[string]any, bool) {
	value, ok := c.Get(key)
	if !ok {
		return nil, false
	}

	switch v := value.(type) {
	case map[string]any:
		return v, true
	case map[string]string:
		result := make(map[string]any, len(v))
		for k, val := range v {
			result[k] = val
		}
		return result, true
	default:
		return nil, false
	}
}

func (c *BaseConfig) OnChange(key string, callback iface.CallbackFunc) {
	c.mu.Lock()
	c.callbacks[key] = append(c.callbacks[key], callback)
	c.mu.Unlock()
}

func (c *BaseConfig) OffChange(key string, callback iface.CallbackFunc) {
	c.mu.Lock()
	c.callbacks[key] = append(c.callbacks[key], callback)
	c.mu.Unlock()
}

func (c *BaseConfig) GetNested(key string) (any, bool) {
	parts := strings.Split(key, ".")
	if len(parts) == 1 {
		return c.Get(key)
	}

	current, ok := c.Get(parts[0])
	if !ok {
		return nil, false
	}

	for i := 1; i < len(parts); i++ {
		switch m := current.(type) {
		case map[string]any:
			if val, ok := m[parts[i]]; ok {
				current = val
			} else {
				return nil, false
			}
		default:
			return nil, false
		}
	}

	return current, true
}

func (c *BaseConfig) SetNested(key string, value any) {
	parts := strings.Split(key, ".")
	if len(parts) == 1 {
		c.Set(key, value)
		return
	}
	rootKey := parts[0]
	root, ok := c.Get(rootKey)
	if !ok {
		root = make(map[string]any)
		c.Set(rootKey, root)
	}
	current := root.(map[string]any)
	for i := 1; i < len(parts)-1; i++ {
		part := parts[i]
		if val, ok := current[part]; ok {
			if m, ok := val.(map[string]any); ok {
				current = m
			} else {
				newMap := make(map[string]any)
				current[part] = newMap
				current = newMap
			}
		} else {
			newMap := make(map[string]any)
			current[part] = newMap
			current = newMap
		}
	}
	lastPart := parts[len(parts)-1]
	current[lastPart] = value
}

type ConfigManager struct {
	defaultConfig iface.Config
	configs       map[string]iface.Config
	mu            sync.RWMutex
}

func NewConfigManager(defaultConfig iface.Config) *ConfigManager {
	return &ConfigManager{
		defaultConfig: defaultConfig,
		configs:       map[string]iface.Config{},
	}
}

func (m *ConfigManager) RegisterConfig(name string, config iface.Config) {
	m.mu.Lock()
	m.configs[name] = config
	m.mu.Unlock()
}

func (m *ConfigManager) GetConfig(name string) (iface.Config, error) {
	m.mu.RLock()
	config, ok := m.configs[name]
	m.mu.RUnlock()
	if ok {
		return config, nil
	}
	if name == "default" {
		return m.defaultConfig, nil
	}
	return nil, common.ErrConfigNotFound
}
