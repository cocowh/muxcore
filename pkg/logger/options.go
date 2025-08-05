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

	EnableStdout    bool
	EnableWarnFile  bool
	EnableErrorFile bool
}
