// Copyright (c) 2025 cocowh. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package iface

type CallbackFunc func(oldValue, newValue any)

type Config interface {
	Get(key string) (any, bool)
	Set(key string, value any)
	Load() error
	Save() error
	Has(key string) bool
	Unset(key string)
	GetString(key string) (string, bool)
	GetInt(key string) (int, bool)
	GetFloat(key string) (float64, bool)
	GetBool(key string) (bool, bool)
	GetStringSlice(key string) ([]string, bool)
	GetMap(key string) (map[string]any, bool)
	OnChange(key string, callback CallbackFunc)
	OffChange(key string, callback CallbackFunc)
}

type Parser interface {
	Parse(data []byte) (map[string]any, error)
}

type Loader interface {
	Load(c Config) error
}
