// Copyright (c) 2025 cocowh. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package logger

const (
	RotateFileTimeFormat = "2006-01-02"
)

type Config struct {
	LogDir   string
	BaseName string
	Format   string
	Level    Level

	Compress   bool
	TimeFormat string
	MaxSizeMB  int
	MaxBackups int
	MaxAgeDays int

	EnableWarnFile  bool
	EnableErrorFile bool
}

func (c *Config) Clone() *Config {
	return &Config{
		LogDir:          c.LogDir,
		BaseName:        c.BaseName,
		Format:          c.Format,
		Level:           c.Level,
		Compress:        c.Compress,
		TimeFormat:      c.TimeFormat,
		MaxSizeMB:       c.MaxSizeMB,
		MaxBackups:      c.MaxBackups,
		MaxAgeDays:      c.MaxAgeDays,
		EnableWarnFile:  c.EnableWarnFile,
		EnableErrorFile: c.EnableErrorFile,
	}
}
