// Copyright (c) 2025 cocowh. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package logger

import (
	"log"
	"os"
)

type stdLogger struct {
	logger *log.Logger
}

func NewStdLogger(output *os.File, prefix string, flag int) Logger {
	return &stdLogger{
		logger: log.New(output, prefix, flag),
	}
}

func (l *stdLogger) Debugf(format string, args ...interface{}) {
	l.logger.Printf("[DEBUG] "+format, args...)
}

func (l *stdLogger) Debug(args ...interface{}) {
	l.logger.Print(append([]interface{}{"[DEBUG]"}, args...)...)
}

func (l *stdLogger) Infof(format string, args ...interface{}) {
	l.logger.Printf("[INFO] "+format, args...)
}

func (l *stdLogger) Info(args ...interface{}) {
	l.logger.Print(append([]interface{}{"[INFO]"}, args...)...)
}

func (l *stdLogger) Warnf(format string, args ...interface{}) {
	l.logger.Printf("[WARN] "+format, args...)
}

func (l *stdLogger) Warn(args ...interface{}) {
	l.logger.Print(append([]interface{}{"[WARN]"}, args...)...)
}

func (l *stdLogger) Errorf(format string, args ...interface{}) {
	l.logger.Printf("[ERROR] "+format, args...)
}

func (l *stdLogger) Error(args ...interface{}) {
	l.logger.Print(append([]interface{}{"[ERROR]"}, args...)...)
}

func (l *stdLogger) Fatalf(format string, args ...interface{}) {
	l.logger.Fatalf("[FATAL] "+format, args...)
}

func (l *stdLogger) Fatal(args ...interface{}) {
	l.logger.Fatal(append([]interface{}{"[FATAL]"}, args...)...)
}
