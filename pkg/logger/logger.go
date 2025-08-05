// Copyright (c) 2025 cocowh. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package logger

import (
	"fmt"
)

var (
	defaultLogger Logger
)

type Logger interface {
	Debugf(format string, args ...any)
	Debug(args ...any)
	Infof(format string, args ...any)
	Info(args ...any)
	Warnf(format string, args ...any)
	Warn(args ...any)
	Errorf(format string, args ...interface{})
	Error(args ...any)
	Fatalf(format string, args ...interface{})
	Fatal(args ...any)
}

func Debugf(msg string, fields ...any) {
	if defaultLogger == nil {
		fmt.Printf(msg, fields...)
	} else {
		defaultLogger.Debugf(msg, fields...)
	}
}

func Debugln(fields ...any) {
	if defaultLogger == nil {
		fmt.Println(fields...)
	} else {
		defaultLogger.Debug(fields...)
	}
}

func Infof(msg string, fields ...any) {
	if defaultLogger == nil {
		fmt.Printf(msg, fields...)
	} else {
		defaultLogger.Infof(msg, fields...)
	}
}

func Infoln(fields ...any) {
	if defaultLogger == nil {
		fmt.Println(fields...)
	} else {
		defaultLogger.Info(fields...)
	}
}

func Warnf(msg string, fields ...any) {
	if defaultLogger == nil {
		fmt.Printf(msg, fields...)
	} else {
		defaultLogger.Warnf(msg, fields...)
	}
}

func Warnln(fields ...any) {
	if defaultLogger == nil {
		fmt.Println(fields...)
	} else {
		defaultLogger.Warn(fields...)
	}
}

func Errorf(msg string, fields ...any) {
	if defaultLogger == nil {
		fmt.Printf(msg, fields...)
	} else {
		defaultLogger.Errorf(msg, fields...)
	}
}

func Errorln(fields ...any) {
	if defaultLogger == nil {
		fmt.Println(fields...)
	} else {
		defaultLogger.Error(fields...)
	}
}

func Fatalf(msg string, fields ...any) {
	if defaultLogger == nil {
		fmt.Printf(msg, fields...)
	} else {
		defaultLogger.Fatalf(msg, fields...)
	}
}

func Fatalln(fields ...any) {
	if defaultLogger == nil {
		fmt.Println(fields...)
	} else {
		defaultLogger.Fatal(fields...)
	}
}

func SetDefaultLogger(logger Logger) {
	defaultLogger = logger
}
