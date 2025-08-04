// Copyright (c) 2025 cocowh. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package loader

import (
	"flag"
	"fmt"
	"strings"

	"github.com/cocowh/muxcore/core/iface"
)

type CLILoader struct {
	prefix string
	flags  map[string]string
}

func NewCLIConfig() *CLILoader {
	return &CLILoader{
		flags: make(map[string]string),
	}
}

func (l *CLILoader) WithPrefix(prefix string) *CLILoader {
	l.prefix = prefix
	return l
}

func (l *CLILoader) AddFlag(name, defaultValue, usage string) *CLILoader {
	l.flags[name] = defaultValue
	flag.String(l.prefix+name, defaultValue, usage)
	return l
}

func (l *CLILoader) Load(c iface.Config) error {
	flag.Parse()
	for k, v := range l.flags {
		fullName := l.prefix + k
		value := flag.Lookup(fullName).Value.String()
		if value != v {
			c.Set(k, value)
		}
	}

	args := flag.Args()
	for i, arg := range args {
		if parts := strings.SplitN(arg, "=", 2); len(parts) == 2 {
			key := parts[0]
			value := parts[1]
			c.Set(key, value)
		} else {
			c.Set(fmt.Sprintf("pos%d", i), arg)
		}
	}
	return nil
}
