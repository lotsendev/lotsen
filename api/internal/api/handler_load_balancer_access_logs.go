package api

import (
	"bufio"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"
)

const (
	defaultAccessLogsPageSize = 200
	maxAccessLogsPageSize     = 500
)

type loadBalancerAccessLogEntry struct {
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

type accessLogsCursor struct {
	File          string `json:"file"`
	OffsetFromEnd int    `json:"offsetFromEnd"`
}

type loadBalancerAccessLogsResponse struct {
	Items      []loadBalancerAccessLogEntry `json:"items"`
	HasMore    bool                         `json:"hasMore"`
	NextCursor string                       `json:"nextCursor,omitempty"`
}

type accessLogsFilters struct {
	Method string
	Status int
	Host   string
	IP     string
}

func (h *Handler) loadBalancerAccessLogs(w http.ResponseWriter, r *http.Request) {
	limit, err := parseAccessLogsLimit(r.URL.Query().Get("limit"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	filters, err := parseAccessLogsFilters(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	cursor, err := decodeAccessLogsCursor(r.URL.Query().Get("cursor"))
	if err != nil {
		http.Error(w, "invalid cursor", http.StatusBadRequest)
		return
	}

	items, next, hasMore, err := readAccessLogsPage(h.accessLogDir, cursor, filters, limit)
	if err != nil {
		http.Error(w, "failed to read access logs", http.StatusInternalServerError)
		return
	}

	resp := loadBalancerAccessLogsResponse{Items: items, HasMore: hasMore}
	if next != nil {
		resp.NextCursor = encodeAccessLogsCursor(*next)
	}

	writeJSON(w, http.StatusOK, resp)
}

func parseAccessLogsFilters(r *http.Request) (accessLogsFilters, error) {
	q := r.URL.Query()
	filters := accessLogsFilters{
		Method: strings.ToUpper(strings.TrimSpace(q.Get("method"))),
		Host:   strings.ToLower(strings.TrimSpace(q.Get("host"))),
		IP:     strings.TrimSpace(q.Get("ip")),
	}

	if rawStatus := strings.TrimSpace(q.Get("status")); rawStatus != "" {
		status, err := strconv.Atoi(rawStatus)
		if err != nil || status < 100 || status > 599 {
			return accessLogsFilters{}, fmt.Errorf("status must be an HTTP status code")
		}
		filters.Status = status
	}

	if filters.Method != "" {
		for _, ch := range filters.Method {
			if ch < 'A' || ch > 'Z' {
				return accessLogsFilters{}, fmt.Errorf("method must contain only letters")
			}
		}
	}

	return filters, nil
}

func parseAccessLogsLimit(raw string) (int, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return defaultAccessLogsPageSize, nil
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n <= 0 {
		return 0, fmt.Errorf("limit must be a positive integer")
	}
	if n > maxAccessLogsPageSize {
		n = maxAccessLogsPageSize
	}
	return n, nil
}

func decodeAccessLogsCursor(raw string) (*accessLogsCursor, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}

	buf, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		return nil, err
	}

	var cursor accessLogsCursor
	if err := json.Unmarshal(buf, &cursor); err != nil {
		return nil, err
	}
	if strings.TrimSpace(cursor.File) == "" || cursor.OffsetFromEnd < 0 {
		return nil, errors.New("invalid cursor")
	}
	return &cursor, nil
}

func encodeAccessLogsCursor(cursor accessLogsCursor) string {
	buf, _ := json.Marshal(cursor)
	return base64.RawURLEncoding.EncodeToString(buf)
}

func readAccessLogsPage(dir string, cursor *accessLogsCursor, filters accessLogsFilters, limit int) ([]loadBalancerAccessLogEntry, *accessLogsCursor, bool, error) {
	if limit <= 0 {
		return []loadBalancerAccessLogEntry{}, nil, false, nil
	}

	files, err := listAccessLogFiles(dir)
	if err != nil {
		return nil, nil, false, err
	}
	if len(files) == 0 {
		return []loadBalancerAccessLogEntry{}, nil, false, nil
	}

	fileIndex := 0
	offset := 0
	if cursor != nil {
		idx := slices.Index(files, cursor.File)
		if idx == -1 {
			return []loadBalancerAccessLogEntry{}, nil, false, nil
		}
		fileIndex = idx
		offset = cursor.OffsetFromEnd
	}

	items := make([]loadBalancerAccessLogEntry, 0, limit)
	for i := fileIndex; i < len(files); i++ {
		lines, err := readLogLines(filepath.Join(dir, files[i]))
		if err != nil {
			return nil, nil, false, err
		}

		startOffset := 0
		if i == fileIndex {
			startOffset = offset
		}
		for consumed := startOffset; consumed < len(lines); consumed++ {
			idx := len(lines) - 1 - consumed
			if idx < 0 {
				break
			}

			line := strings.TrimSpace(lines[idx])
			if line == "" {
				continue
			}
			var entry loadBalancerAccessLogEntry
			if err := json.Unmarshal([]byte(line), &entry); err != nil {
				continue
			}
			if !entryMatchesAccessLogFilters(entry, filters) {
				continue
			}
			items = append(items, entry)
			if len(items) == limit {
				next := accessLogsCursor{File: files[i], OffsetFromEnd: consumed + 1}
				hasMore, hasMoreErr := hasMoreFilteredAccessLogs(dir, files, next, filters)
				if hasMoreErr != nil {
					return nil, nil, false, hasMoreErr
				}
				return items, &next, hasMore, nil
			}
		}
	}

	return items, nil, false, nil
}

func entryMatchesAccessLogFilters(entry loadBalancerAccessLogEntry, filters accessLogsFilters) bool {
	if filters.Method != "" && strings.ToUpper(strings.TrimSpace(entry.Method)) != filters.Method {
		return false
	}
	if filters.Status != 0 && entry.Status != filters.Status {
		return false
	}
	if filters.Host != "" && !strings.Contains(strings.ToLower(strings.TrimSpace(entry.Host)), filters.Host) {
		return false
	}
	if filters.IP != "" && !strings.Contains(strings.TrimSpace(entry.ClientIP), filters.IP) {
		return false
	}
	return true
}

func hasMoreFilteredAccessLogs(dir string, files []string, cursor accessLogsCursor, filters accessLogsFilters) (bool, error) {
	startFile := slices.Index(files, cursor.File)
	if startFile == -1 {
		return false, nil
	}

	for i := startFile; i < len(files); i++ {
		lines, err := readLogLines(filepath.Join(dir, files[i]))
		if err != nil {
			return false, err
		}

		offset := 0
		if i == startFile {
			offset = cursor.OffsetFromEnd
		}

		for consumed := offset; consumed < len(lines); consumed++ {
			idx := len(lines) - 1 - consumed
			if idx < 0 {
				break
			}
			line := strings.TrimSpace(lines[idx])
			if line == "" {
				continue
			}
			var entry loadBalancerAccessLogEntry
			if err := json.Unmarshal([]byte(line), &entry); err != nil {
				continue
			}
			if entryMatchesAccessLogFilters(entry, filters) {
				return true, nil
			}
		}
	}

	return false, nil
}

func listAccessLogFiles(logDir string) ([]string, error) {
	entries, err := os.ReadDir(logDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, err
	}

	files := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.HasPrefix(name, "access-") && strings.HasSuffix(name, ".log") {
			files = append(files, name)
		}
	}

	slices.Sort(files)
	slices.Reverse(files)
	return files, nil
}

func readLogLines(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	lines := make([]string, 0, 256)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return lines, nil
}
