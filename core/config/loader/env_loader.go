// Copyright (c) 2025 cocowh. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package loader

import (
	"os"
	"strings"

	"github.com/cocowh/muxcore/core/iface"
)

type EnvLoader struct {
	prefix  string
	mapping map[string]string
}

func NewEnvLoader(prefix string) *EnvLoader {
	return &EnvLoader{
		prefix:  prefix,
		mapping: make(map[string]string),
	}
}

func (l *EnvLoader) AddMapping(envKey, configKey string) *EnvLoader {
	l.mapping[envKey] = configKey
	return l
}

func (l *EnvLoader) Load(c iface.Config) error {
	for _, env := range os.Environ() {
		parts := strings.SplitN(env, "=", 2)
		if len(parts) != 2 {
			continue
		}
		envKey := parts[0]
		envValue := parts[1]

		if configKey, ok := l.mapping[envKey]; ok {
			c.Set(configKey, envValue)
		}
		if l.prefix != "" && !strings.HasPrefix(envKey, l.prefix) {
			continue
		}
		configKey := strings.ToLower(strings.TrimPrefix(envKey, l.prefix))
		configKey = strings.ReplaceAll(configKey, "_", ".")
		c.Set(configKey, envValue)
	}
	return nil
}
