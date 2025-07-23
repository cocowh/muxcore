// Copyright (c) 2025 cocowh. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package logger

import (
	"fmt"
	"os"
)

var (
	defaultLogger Logger
)

type Logger interface {
	Debugf(format string, args ...any)
	Debugln(args ...any)
	Infof(format string, args ...any)
	Infoln(args ...any)
	Warnf(format string, args ...any)
	Warnln(args ...any)
	Errorf(format string, args ...interface{})
	Errorln(args ...any)
	Fatalf(format string, args ...interface{})
	Fatalln(args ...any)
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
		fmt.Fprintln(os.Stdout, fields...)
	} else {
		defaultLogger.Debugln(fields...)
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
		defaultLogger.Infoln(fields...)
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
		defaultLogger.Warnln(fields...)
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
		defaultLogger.Errorln(fields...)
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
		defaultLogger.Fatalln(fields...)
	}
}

func SetDefaultLogger(logger Logger) {
	defaultLogger = logger
}
