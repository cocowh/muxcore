// Copyright (c) 2025 cocowh. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package loader

import (
	"os"
	"path/filepath"

	"github.com/cocowh/muxcore/core/config/parser"
	"github.com/cocowh/muxcore/core/iface"
)

type FileLoader struct {
	path   string
	parser iface.Parser
}

func NewFileLoader(path string) *FileLoader {
	ext := filepath.Ext(path)
	var p iface.Parser
	switch ext {
	case ".json":
		p = parser.NewJSONParser()
	case ".yaml", ".yml":
		p = parser.NewYAMLParser()
	case ".toml":
		p = parser.NewTOMLParser()
	default:
		p = parser.NewJSONParser()
	}
	return &FileLoader{
		path:   path,
		parser: p,
	}
}

func (l *FileLoader) Load(c iface.Config) error {
	if _, err := os.Stat(l.path); os.IsNotExist(err) {
		return err
	}
	b, err := os.ReadFile(l.path)
	if err != nil {
		return err
	}
	configs, err := l.parser.Parse(b)
	if err != nil {
		return err
	}
	for k, v := range configs {
		c.Set(k, v)
	}
	return nil
}
