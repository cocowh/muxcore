// Copyright (c) 2025 cocowh. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package config

import (
	"github.com/cocowh/muxcore/core/config/loader"
	"github.com/cocowh/muxcore/core/iface"
	"github.com/cocowh/muxcore/pkg/logger"
)

type Option func(iface.Config)

func WithDefaultConfig(defaults map[string]any) Option {
	return func(c iface.Config) {
		for k, v := range defaults {
			c.Set(k, v)
		}
	}
}

func WithFileConfig(path string) Option {
	return func(c iface.Config) {
		loader := loader.NewFileLoader(path)
		err := loader.Load(c)
		if err != nil {
			logger.Errorf("load file config failed, path: %s, err: %v", path, err)
		}
	}
}

func WithEnvConfig(prefix string) Option {
	return func(c iface.Config) {
		loader := loader.NewEnvLoader(prefix)
		err := loader.Load(c)
		if err != nil {
			logger.Errorf("load env config failed, prefix: %s, err: %v", prefix, err)
		}
	}
}

func WithCLIConfig() Option {
	return func(c iface.Config) {
		loader := loader.NewCLIConfig()
		err := loader.Load(c)
		if err != nil {
			logger.Errorf("load cli config failed, err: %v", err)
		}
	}
}

func WithConfig(options ...Option) iface.Config {
	config := NewBaseConfig()
	for _, option := range options {
		option(config)
	}
	return config
}
