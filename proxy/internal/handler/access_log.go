package handler

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const accessLogFilenamePrefix = "access-"

type AccessLogConfig struct {
	Dir             string
	Retention       time.Duration
	WhitelistedKeys []string
	Now             func() time.Time
}

type AccessLogEvent struct {
	Timestamp    time.Time
	ClientIP     string
	Host         string
	Method       string
	Path         string
	Query        string
	Status       int
	DurationMs   int64
	BytesWritten int64
	Outcome      string
	Headers      http.Header
}

type accessLogRecord struct {
	Timestamp    time.Time         `json:"timestamp"`
	ClientIP     string            `json:"clientIp"`
	Host         string            `json:"host"`
	Method       string            `json:"method"`
	Path         string            `json:"path"`
	Query        string            `json:"query,omitempty"`
	Status       int               `json:"status"`
	DurationMs   int64             `json:"durationMs"`
	BytesWritten int64             `json:"bytesWritten"`
	Outcome      string            `json:"outcome"`
	Headers      map[string]string `json:"headers,omitempty"`
}

type AccessLogger interface {
	Log(AccessLogEvent)
	Close() error
}

type fileAccessLogger struct {
	mu         sync.Mutex
	dir        string
	retention  time.Duration
	now        func() time.Time
	allowed    map[string]struct{}
	currentKey string
	current    *os.File
	writer     *bufio.Writer
}

func NewFileAccessLogger(cfg AccessLogConfig) (AccessLogger, error) {
	if strings.TrimSpace(cfg.Dir) == "" {
		return nil, fmt.Errorf("access log dir is required")
	}
	if cfg.Retention <= 0 {
		cfg.Retention = 7 * 24 * time.Hour
	}
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	allowed := make(map[string]struct{})
	for _, key := range cfg.WhitelistedKeys {
		normalized := strings.ToLower(strings.TrimSpace(key))
		if normalized == "" {
			continue
		}
		allowed[normalized] = struct{}{}
	}

	if err := os.MkdirAll(cfg.Dir, 0o755); err != nil {
		return nil, fmt.Errorf("create access log dir: %w", err)
	}

	logger := &fileAccessLogger{dir: cfg.Dir, retention: cfg.Retention, now: cfg.Now, allowed: allowed}
	if err := logger.rotateIfNeeded(cfg.Now().UTC()); err != nil {
		return nil, err
	}
	return logger, nil
}

func (l *fileAccessLogger) Log(event AccessLogEvent) {
	now := l.now().UTC()
	if event.Timestamp.IsZero() {
		event.Timestamp = now
	} else {
		event.Timestamp = event.Timestamp.UTC()
	}

	record := accessLogRecord{
		Timestamp:    event.Timestamp,
		ClientIP:     event.ClientIP,
		Host:         event.Host,
		Method:       event.Method,
		Path:         event.Path,
		Query:        event.Query,
		Status:       event.Status,
		DurationMs:   event.DurationMs,
		BytesWritten: event.BytesWritten,
		Outcome:      event.Outcome,
		Headers:      l.pickHeaders(event.Headers, event.Host),
	}

	line, err := json.Marshal(record)
	if err != nil {
		return
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	if err := l.rotateIfNeeded(now); err != nil {
		return
	}

	if _, err := l.writer.Write(line); err != nil {
		return
	}
	if err := l.writer.WriteByte('\n'); err != nil {
		return
	}
	_ = l.writer.Flush()
}

func (l *fileAccessLogger) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.writer != nil {
		if err := l.writer.Flush(); err != nil {
			return err
		}
	}
	if l.current != nil {
		if err := l.current.Close(); err != nil {
			return err
		}
	}

	l.writer = nil
	l.current = nil
	return nil
}

func (l *fileAccessLogger) pickHeaders(headers http.Header, host string) map[string]string {
	if len(l.allowed) == 0 {
		return nil
	}
	out := make(map[string]string)
	for rawKey, values := range headers {
		key := strings.ToLower(strings.TrimSpace(rawKey))
		if _, ok := l.allowed[key]; !ok {
			continue
		}
		if len(values) == 0 {
			continue
		}
		out[key] = strings.Join(values, ",")
	}
	if len(out) == 0 {
		if _, ok := l.allowed["host"]; ok && host != "" {
			return map[string]string{"host": host}
		}
		return nil
	}
	if _, ok := l.allowed["host"]; ok && host != "" {
		if _, exists := out["host"]; !exists {
			out["host"] = host
		}
	}
	return out
}

func (l *fileAccessLogger) rotateIfNeeded(now time.Time) error {
	key := now.Format("2006-01-02-15")
	if l.current != nil && l.currentKey == key {
		return nil
	}

	if l.writer != nil {
		if err := l.writer.Flush(); err != nil {
			return err
		}
	}
	if l.current != nil {
		if err := l.current.Close(); err != nil {
			return err
		}
	}

	path := filepath.Join(l.dir, fmt.Sprintf("%s%s.log", accessLogFilenamePrefix, key))
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("open access log file: %w", err)
	}

	l.currentKey = key
	l.current = f
	l.writer = bufio.NewWriterSize(f, 64*1024)
	l.cleanupExpired(now)
	return nil
}

func (l *fileAccessLogger) cleanupExpired(now time.Time) {
	entries, err := os.ReadDir(l.dir)
	if err != nil {
		return
	}
	cutoff := now.Add(-l.retention)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasPrefix(name, accessLogFilenamePrefix) || !strings.HasSuffix(name, ".log") {
			continue
		}
		if l.current != nil && filepath.Base(l.current.Name()) == name {
			continue
		}
		info, infoErr := entry.Info()
		if infoErr != nil {
			continue
		}
		if info.ModTime().Before(cutoff) {
			_ = os.Remove(filepath.Join(l.dir, name))
		}
	}
}
