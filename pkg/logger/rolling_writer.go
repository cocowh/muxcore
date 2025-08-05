// Copyright (c) 2025 cocowh. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package logger

import (
	"compress/gzip"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

type RollingWriter struct {
	mu          sync.Mutex
	basePath    string
	timeFormat  string
	rotateEvery time.Duration
	maxSize     int64
	compress    bool
	currentMark time.Time

	maxBackups int
	maxAgeDays int

	file     *os.File
	fileSize int64
}

func NewRollingWriter(cfg Config) io.Writer {
	mark := time.Now().Truncate(time.Hour)

	basePath := filepath.Join(cfg.LogDir, cfg.BaseName+".log")
	rotate := time.Hour

	f, _ := os.OpenFile(basePath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	var size int64
	if info, err := f.Stat(); err == nil {
		size = info.Size()
	}
	if cfg.MaxSizeMB <= 0 {
		cfg.MaxSizeMB = 1024
	}

	r := &RollingWriter{
		basePath:    basePath,
		timeFormat:  cfg.TimeFormat,
		rotateEvery: rotate,
		maxSize:     int64(cfg.MaxSizeMB) * 1024 * 1024,
		compress:    cfg.Compress,
		currentMark: mark,
		maxBackups:  cfg.MaxBackups,
		maxAgeDays:  cfg.MaxAgeDays,
		file:        f,
		fileSize:    size,
	}
	go r.startRotateTicker()
	return r
}

func (r *RollingWriter) startRotateTicker() {
	go func() {
		defer func() {
			if err := recover(); err != nil {
				log.Printf("rotate logs failed. err:%v", err)
				Warnf("rotate logs failed. err:%v", err)
			}
		}()
		for {
			now := time.Now()
			next := now.Truncate(time.Hour).Add(time.Hour)
			time.Sleep(time.Until(next))
			r.mu.Lock()
			r.scheduleRotate()
			r.currentMark = next
			r.file.Close()
			r.file, _ = os.OpenFile(r.basePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
			r.fileSize = 0
			r.mu.Unlock()
		}
	}()
}

func (r *RollingWriter) Write(p []byte) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.maxSize > 0 && r.fileSize+int64(len(p)) > r.maxSize {
		r.scheduleRotate()
		r.file.Close()
		r.file, _ = os.OpenFile(r.basePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		r.fileSize = 0
	}

	n, err := r.file.Write(p)
	r.fileSize += int64(n)
	return n, err
}

func (r *RollingWriter) scheduleRotate() {
	base := r.basePath
	timeFormat := r.timeFormat
	timestamp := r.currentMark
	compress := r.compress
	maxBackups := r.maxBackups
	maxAgeDays := r.maxAgeDays

	info, err := os.Stat(base)
	if err != nil || info.Size() == 0 {
		return
	}

	formatted := timestamp.Format(timeFormat)
	ext := filepath.Ext(base)
	baseName := strings.TrimSuffix(base, ext)
	rotated := fmt.Sprintf("%s-%s%s", baseName, formatted, ext)

	_ = os.Rename(base, rotated)

	if compress {
		r.compressFile(rotated)
	}

	r.cleanupOldLogs(baseName, ext, timeFormat, maxBackups, maxAgeDays)
}

func (r *RollingWriter) openExistOrNew() {

}

func (r *RollingWriter) compressFile(path string) {
	in, err := os.Open(path)
	if err != nil {
		return
	}
	defer in.Close()

	outPath := path + ".gz"
	out, err := os.Create(outPath)
	if err != nil {
		return
	}
	defer out.Close()

	gz := gzip.NewWriter(out)
	_, _ = io.Copy(gz, in)
	gz.Close()
	_ = os.Remove(path)
}

func (r *RollingWriter) cleanupOldLogs(base, ext, timeFormat string, maxBackups, maxAgeDays int) {
	dir := filepath.Dir(base)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}

	prefix := filepath.Base(base) + "-"
	cutoff := time.Now().AddDate(0, 0, -maxAgeDays)
	var backups []os.DirEntry

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasPrefix(entry.Name(), prefix) {
			continue
		}
		name := entry.Name()
		ts := strings.TrimSuffix(strings.TrimPrefix(name, prefix), ext)
		ts = strings.TrimSuffix(ts, ".gz")
		t, err := time.Parse(timeFormat, ts)
		if err == nil {
			if t.Before(cutoff) {
				_ = os.Remove(filepath.Join(dir, name))
			} else {
				backups = append(backups, entry)
			}
		}
	}

	sort.Slice(backups, func(i, j int) bool {
		return backups[i].Name() > backups[j].Name()
	})
	if len(backups) > maxBackups {
		for _, extra := range backups[maxBackups:] {
			_ = os.Remove(filepath.Join(dir, extra.Name()))
		}
	}
}
