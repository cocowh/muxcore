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

// NewRollingWriter create a new rolling writer
func NewRollingWriter(cfg *Config) io.Writer {
	if cfg == nil {
		return os.Stdout
	}
	mark := time.Now().Truncate(time.Hour)

	basePath := filepath.Join(cfg.LogDir, cfg.BaseName+".log")
	rotate := time.Hour

	if err := os.MkdirAll(cfg.LogDir, 0755); err != nil {
		Errorf("Failed to create log directory: %v", err)
		return os.Stdout
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
		fileSize:    0,
	}

	r.openExistOrNew()

	go r.startRotateTicker()
	return r
}

// startRotateTicker 启动滚动定时器
func (r *RollingWriter) startRotateTicker() {
	go func() {
		defer func() {
			if err := recover(); err != nil {
				log.Printf("rotate logs failed. err:%v", err)
			}
		}()
		for {
			now := time.Now()
			next := now.Truncate(r.rotateEvery).Add(r.rotateEvery)
			time.Sleep(time.Until(next))
			r.mu.Lock()

			// rotate log file
			r.scheduleRotate()

			// update current mark
			r.currentMark = next

			// close current file
			if err := r.file.Close(); err != nil {
				log.Printf("Failed to close log file: %v", err)
			}

			// update base path (ensure without timestamp)
			basePath := r.basePath
			if strings.Contains(basePath, "-") {
				baseName := filepath.Base(basePath)
				parts := strings.Split(baseName, "-")
				if len(parts) > 0 {
					basePath = filepath.Join(filepath.Dir(basePath), parts[0]+".log")
				}
			}
			r.basePath = basePath

			// open exist file or create new file
			r.openExistOrNew()

			r.mu.Unlock()
		}
	}()
}

func (r *RollingWriter) Write(p []byte) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.maxSize > 0 && r.fileSize+int64(len(p)) > r.maxSize {
		r.scheduleRotate()
		// close current file
		if err := r.file.Close(); err != nil {
			log.Printf("Failed to close log file: %v", err)
		}
		// open exist file or create new file
		r.openExistOrNew()
	}

	n, err := r.file.Write(p)
	r.fileSize += int64(n)
	return n, err
}

// openExistOrNew open exist file or create new file
func (r *RollingWriter) openExistOrNew() {
	r.mu.Lock()
	defer r.mu.Unlock()

	// check if file exist
	if _, err := os.Stat(r.basePath); err == nil {
		// file exist, open it
		newFile, err := os.OpenFile(r.basePath, os.O_APPEND|os.O_WRONLY, 0644)
		if err == nil {
			// close old file
			if r.file != os.Stdout && r.file != os.Stderr && r.file != nil {
				if err := r.file.Close(); err != nil {
					log.Printf("Failed to close old log file: %v", err)
				}
			}
			// update file and size
			r.file = newFile
			if info, err := newFile.Stat(); err == nil {
				r.fileSize = info.Size()
			}
			return
		}
	}

	// if file not exist or open failed, create new file
	newFile, err := os.OpenFile(r.basePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		log.Printf("Failed to create new log file: %v", err)
		// if create failed, try to use standard error output
		if r.file != os.Stdout && r.file != os.Stderr && r.file != nil {
			if err := r.file.Close(); err != nil {
				log.Printf("Failed to close old log file: %v", err)
			}
		}
		r.file = os.Stdout
	} else {
		// close old file
		if r.file != os.Stdout && r.file != os.Stderr && r.file != nil {
			if err := r.file.Close(); err != nil {
				log.Printf("Failed to close old log file: %v", err)
			}
		}
		// update file and size
		r.file = newFile
		r.fileSize = 0
	}
}

// scheduleRotate rotate log file
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

	// get base name
	baseName := strings.TrimSuffix(base, filepath.Ext(base))
	// if base name already contains timestamp, extract without timestamp
	if strings.Contains(baseName, "-") {
		parts := strings.Split(baseName, "-")
		if len(parts) > 0 {
			baseName = parts[0]
		}
	}

	formatted := timestamp.Format(timeFormat)
	ext := filepath.Ext(base)
	rotated := fmt.Sprintf("%s-%s%s", baseName, formatted, ext)

	// check if rotated file exist
	if _, err := os.Stat(rotated); err == nil {
		// if file exist, add a counter
		i := 1
		for {
			newRotated := fmt.Sprintf("%s-%s-%d%s", baseName, formatted, i, ext)
			if _, err := os.Stat(newRotated); err != nil {
				rotated = newRotated
				break
			}
			i++
		}
	}

	if err := os.Rename(base, rotated); err != nil {
		log.Printf("Failed to rotate log file: %v", err)
		return
	}

	if compress {
		compressFile(rotated)
	}

	cleanupOldLogs(baseName, ext, timeFormat, maxBackups, maxAgeDays)
}

// compressFile compress log file
func compressFile(path string) {
	in, err := os.Open(path)
	if err != nil {
		log.Printf("Failed to open file for compression: %v", err)
		return
	}
	defer in.Close()

	outPath := path + ".gz"
	out, err := os.Create(outPath)
	if err != nil {
		log.Printf("Failed to create compressed file: %v", err)
		return
	}
	defer out.Close()

	gz := gzip.NewWriter(out)
	_, err = io.Copy(gz, in)
	if err != nil {
		log.Printf("Failed to compress file: %v", err)
	}
	gz.Close()
	if err := os.Remove(path); err != nil {
		log.Printf("Failed to remove original file after compression: %v", err)
	}
}

// cleanupOldLogs cleanup old log files
func cleanupOldLogs(base, ext, timeFormat string, maxBackups, maxAgeDays int) {
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
