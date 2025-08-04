// Copyright (c) 2025 cocowh. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package parser

import "github.com/pelletier/go-toml/v2"

type TOMLParser struct{}

func NewTOMLParser() *TOMLParser {
	return &TOMLParser{}
}

func (p *TOMLParser) Parse(b []byte) (map[string]interface{}, error) {
	var m map[string]interface{}
	err := toml.Unmarshal(b, &m)
	if err != nil {
		return nil, err
	}
	return m, nil
}
