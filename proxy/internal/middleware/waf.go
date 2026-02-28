package middleware

import (
	"bytes"
	"fmt"
	"io"
	"net"
	"net/http"
	"sort"
	"strings"
	"sync"

	coreruleset "github.com/corazawaf/coraza-coreruleset"
	"github.com/corazawaf/coraza/v3"
)

type WAFMode string

const (
	WAFModeDetection   WAFMode = "detection"
	WAFModeEnforcement WAFMode = "enforcement"
)

type WAFResult struct {
	Detected bool
	Blocked  bool
	Status   int
}

type WAF struct {
	enabled bool
	mode    WAFMode
	baseWAF coraza.WAF

	mu    sync.RWMutex
	cache map[string]coraza.WAF
}

func NewWAF(enabled bool, mode WAFMode) (*WAF, error) {
	mode = normalizeWAFMode(mode)
	w := &WAF{enabled: enabled, mode: mode, cache: make(map[string]coraza.WAF)}
	if !enabled {
		return w, nil
	}
	base, err := buildCorazaWAF(mode, nil)
	if err != nil {
		return nil, err
	}
	w.baseWAF = base
	return w, nil
}

func (w *WAF) Enabled() bool {
	return w != nil && w.enabled
}

func (w *WAF) Mode() WAFMode {
	if w == nil {
		return WAFModeDetection
	}
	return normalizeWAFMode(w.mode)
}

func (w *WAF) Evaluate(r *http.Request, clientIP string, customRules []string) (WAFResult, error) {
	if w == nil || !w.enabled || w.baseWAF == nil {
		return WAFResult{}, nil
	}

	wafInstance, err := w.wafForRules(customRules)
	if err != nil {
		return WAFResult{}, err
	}

	tx := wafInstance.NewTransaction()
	defer tx.Close()
	defer tx.ProcessLogging()

	clientHost := strings.TrimSpace(clientIP)
	if clientHost == "" {
		clientHost = "0.0.0.0"
	}
	tx.ProcessConnection(clientHost, 0, hostOnly(r.Host), 0)
	tx.ProcessURI(r.URL.RequestURI(), r.Method, r.Proto)
	tx.SetServerName(hostOnly(r.Host))
	tx.AddRequestHeader("Host", r.Host)
	for key, values := range r.Header {
		for _, value := range values {
			tx.AddRequestHeader(key, value)
		}
	}
	for key, values := range r.URL.Query() {
		for _, value := range values {
			tx.AddGetRequestArgument(key, value)
		}
	}

	if interruption := tx.ProcessRequestHeaders(); interruption != nil {
		if w.mode == WAFModeEnforcement {
			return WAFResult{Blocked: true, Status: interruptionStatus(interruption.Status)}, nil
		}
		return WAFResult{Detected: true}, nil
	}

	if r.Body != nil {
		body, readErr := io.ReadAll(r.Body)
		if readErr != nil {
			return WAFResult{}, readErr
		}
		r.Body = io.NopCloser(bytes.NewReader(body))
		if tx.IsRequestBodyAccessible() && len(body) > 0 {
			if interruption, _, writeErr := tx.WriteRequestBody(body); writeErr != nil {
				return WAFResult{}, writeErr
			} else if interruption != nil {
				if w.mode == WAFModeEnforcement {
					return WAFResult{Blocked: true, Status: interruptionStatus(interruption.Status)}, nil
				}
				return WAFResult{Detected: true}, nil
			}
		}
	}

	if interruption, err := tx.ProcessRequestBody(); err != nil {
		return WAFResult{}, err
	} else if interruption != nil {
		if w.mode == WAFModeEnforcement {
			return WAFResult{Blocked: true, Status: interruptionStatus(interruption.Status)}, nil
		}
		return WAFResult{Detected: true}, nil
	}

	if w.mode == WAFModeDetection && len(tx.MatchedRules()) > 0 {
		return WAFResult{Detected: true}, nil
	}

	return WAFResult{}, nil
}

func (w *WAF) wafForRules(customRules []string) (coraza.WAF, error) {
	rules := normalizeRules(customRules)
	if len(rules) == 0 {
		return w.baseWAF, nil
	}

	key := strings.Join(rules, "\n")
	w.mu.RLock()
	cached := w.cache[key]
	w.mu.RUnlock()
	if cached != nil {
		return cached, nil
	}

	built, err := buildCorazaWAF(w.mode, rules)
	if err != nil {
		return nil, err
	}

	w.mu.Lock()
	if existing := w.cache[key]; existing != nil {
		w.mu.Unlock()
		return existing, nil
	}
	w.cache[key] = built
	w.mu.Unlock()
	return built, nil
}

func buildCorazaWAF(mode WAFMode, customRules []string) (coraza.WAF, error) {
	ruleEngine := "On"
	if normalizeWAFMode(mode) == WAFModeDetection {
		ruleEngine = "DetectionOnly"
	}

	directives := strings.Builder{}
	directives.WriteString("Include @coraza.conf-recommended\n")
	directives.WriteString("Include @crs-setup.conf.example\n")
	directives.WriteString(fmt.Sprintf("SecRuleEngine %s\n", ruleEngine))
	directives.WriteString("Include @owasp_crs/*.conf\n")
	for _, rule := range normalizeRules(customRules) {
		directives.WriteString(rule)
		directives.WriteString("\n")
	}

	return coraza.NewWAF(
		coraza.NewWAFConfig().
			WithRootFS(coreruleset.FS).
			WithRequestBodyAccess().
			WithDirectives(directives.String()),
	)
}

func normalizeWAFMode(mode WAFMode) WAFMode {
	switch WAFMode(strings.ToLower(strings.TrimSpace(string(mode)))) {
	case WAFModeEnforcement:
		return WAFModeEnforcement
	default:
		return WAFModeDetection
	}
}

func normalizeRules(customRules []string) []string {
	out := make([]string, 0, len(customRules))
	for _, rule := range customRules {
		rule = strings.TrimSpace(rule)
		if rule == "" {
			continue
		}
		out = append(out, rule)
	}
	sort.Strings(out)
	return out
}

func interruptionStatus(status int) int {
	if status >= 400 {
		return status
	}
	return http.StatusForbidden
}

func hostOnly(hostport string) string {
	hostport = strings.TrimSpace(hostport)
	if host, _, err := net.SplitHostPort(hostport); err == nil {
		return host
	}
	return hostport
}
