package handler

import (
	"bufio"
	"encoding/json"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ercadev/dirigent/auth"
	"github.com/ercadev/dirigent/proxy/internal/middleware"
	"github.com/ercadev/dirigent/proxy/internal/routing"
	"github.com/ercadev/dirigent/store"
)

// RoutingTable is the interface the handler reads from when proxying requests
// and the control API writes to when swapping upstreams.
type RoutingTable interface {
	Get(domain string) (routing.Route, bool)
	Set(domain, upstream string, public bool, basicAuth *store.BasicAuthConfig, security *store.SecurityConfig)
}

// Handler serves inbound HTTP requests by routing them to the upstream
// registered for the request's Host header, and exposes a small control API
// so the orchestrator can trigger upstream swaps during zero-downtime redeploys.
type Handler struct {
	table         RoutingTable
	dashboardAuth *DashboardAuth
	hardening     HardeningProfile
	scanner       *scannerLimiter
	accessLogs    AccessLogger
	ipFilter      *middleware.IPFilter
	uaFilter      *middleware.UAFilter
	waf           *middleware.WAF
	wafBlocked    atomic.Int64
	uaBlocked     atomic.Int64
	authStore     *auth.UserStore
	jwtSecret     []byte
}

// HardeningProfile controls request filtering and anti-scan behavior.
type HardeningProfile string

const (
	HardeningOff      HardeningProfile = "off"
	HardeningStandard HardeningProfile = "standard"
	HardeningStrict   HardeningProfile = "strict"
)

// Option customizes handler behavior.
type Option func(*Handler)

// DashboardAuth configures host-scoped authentication for the dashboard domain.
type DashboardAuth struct {
	Domain   string
	Username string
	Password string
}

// New creates a Handler backed by the given routing table.
func New(table RoutingTable, dashboardAuth *DashboardAuth, options ...Option) *Handler {
	h := &Handler{table: table, dashboardAuth: dashboardAuth, hardening: HardeningStandard}
	for _, apply := range options {
		if apply != nil {
			apply(h)
		}
	}
	h.hardening = normalizeHardeningProfile(h.hardening)
	h.scanner = newScannerLimiter(h.hardening)
	return h
}

// WithHardeningProfile applies proxy hardening rules.
func WithHardeningProfile(profile HardeningProfile) Option {
	return func(h *Handler) {
		h.hardening = profile
	}
}

// WithAccessLogger enables per-request access log writing.
func WithAccessLogger(logger AccessLogger) Option {
	return func(h *Handler) {
		h.accessLogs = logger
	}
}

// RegisterRoutes wires the proxy catch-all and the internal control API into mux.
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	h.RegisterInternalRoutes(mux)
	h.RegisterProxyRoutes(mux)
}

// RegisterInternalRoutes wires the internal health/control API into mux.
func (h *Handler) RegisterInternalRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /internal/health", h.health)
	mux.HandleFunc("GET /internal/traffic", h.traffic)
	mux.HandleFunc("POST /internal/routes", h.setRoute)
	mux.HandleFunc("GET /internal/access-logs", h.listAccessLogs)
	mux.HandleFunc("GET /internal/security-config", h.securityConfig)
}

// RegisterProxyRoutes wires the proxy catch-all route into mux.
func (h *Handler) RegisterProxyRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /__dirigent/login", h.proxyLogin)
	mux.HandleFunc("/", h.proxy)
}

func (h *Handler) health(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
}

type blockedIPTrafficStatus struct {
	IP           string     `json:"ip"`
	BlockedUntil *time.Time `json:"blockedUntil,omitempty"`
}

type trafficStatus struct {
	TotalRequests      int64                    `json:"totalRequests"`
	SuspiciousRequests int64                    `json:"suspiciousRequests"`
	BlockedRequests    int64                    `json:"blockedRequests"`
	WAFBlockedRequests int64                    `json:"wafBlockedRequests"`
	UABlockedRequests  int64                    `json:"uaBlockedRequests"`
	ActiveBlockedIPs   int                      `json:"activeBlockedIps"`
	BlockedIPs         []blockedIPTrafficStatus `json:"blockedIps,omitempty"`
}

type HardeningSettings struct {
	Profile                   HardeningProfile `json:"profile"`
	SuspiciousWindowSeconds   int64            `json:"suspiciousWindowSeconds"`
	SuspiciousThreshold       int              `json:"suspiciousThreshold"`
	SuspiciousBlockForSeconds int64            `json:"suspiciousBlockForSeconds"`
	GlobalIPDenylist          []string         `json:"globalIpDenylist,omitempty"`
	GlobalIPAllowlist         []string         `json:"globalIpAllowlist,omitempty"`
}

func (h *Handler) traffic(w http.ResponseWriter, _ *http.Request) {
	status := trafficStatus{}
	if h.scanner != nil {
		status = h.scanner.Snapshot(time.Now())
	}
	status.WAFBlockedRequests = h.wafBlocked.Load()
	status.UABlockedRequests = h.uaBlocked.Load()

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(status); err != nil {
		log.Printf("traffic: encode: %v", err)
	}
}

// proxy routes inbound requests to the upstream registered for the Host header.
// Returns 404 for unknown domains and 502 when the upstream is unreachable.
func (h *Handler) proxy(w http.ResponseWriter, r *http.Request) {
	now := time.Now()
	start := now
	client := clientIP(r)
	outcome := "proxied"
	rw := &accessLogResponseWriter{ResponseWriter: w}
	defer func() {
		if h.accessLogs == nil {
			return
		}
		h.accessLogs.Log(AccessLogEvent{
			Timestamp:    start.UTC(),
			ClientIP:     client,
			Host:         normalizeDomain(r.Host),
			Method:       r.Method,
			Path:         r.URL.Path,
			Query:        r.URL.RawQuery,
			Status:       rw.Status(),
			DurationMs:   time.Since(start).Milliseconds(),
			BytesWritten: rw.BytesWritten(),
			Outcome:      outcome,
			Headers:      r.Header,
		})
	}()

	if h.scanner != nil {
		h.scanner.RecordRequest()
	}
	if h.scanner != nil && client != "" && h.scanner.IsBlocked(client, now) {
		h.scanner.RecordBlockedRequest()
		outcome = "rate_limited"
		http.Error(rw, "too many requests", http.StatusTooManyRequests)
		return
	}
	if h.ipFilter != nil {
		switch h.ipFilter.EvaluateGlobal(client) {
		case middleware.IPFilterDenied:
			outcome = "ip_denied"
			http.Error(rw, "forbidden", http.StatusForbidden)
			return
		case middleware.IPFilterNotAllowed:
			outcome = "ip_not_allowed"
			http.Error(rw, "forbidden", http.StatusForbidden)
			return
		}
	}

	if shouldBlockPath(r.URL.Path, h.hardening) {
		if h.scanner != nil {
			h.scanner.RecordSuspicious(client, now)
		}
		outcome = "blocked_path"
		http.NotFound(rw, r)
		return
	}

	host := r.Host
	// Strip port if present (e.g. "example.com:80" → "example.com").
	if bare, _, err := net.SplitHostPort(host); err == nil {
		host = bare
	}
	host = normalizeDomain(host)

	route, ok := h.table.Get(host)
	if !ok {
		outcome = "unknown_domain"
		http.Error(rw, "unknown domain", http.StatusNotFound)
		return
	}

	if h.ipFilter != nil {
		switch h.ipFilter.EvaluateDeployment(client, route.Security) {
		case middleware.IPFilterDenied:
			outcome = "ip_denied"
			http.Error(rw, "forbidden", http.StatusForbidden)
			return
		case middleware.IPFilterNotAllowed:
			outcome = "ip_not_allowed"
			http.Error(rw, "forbidden", http.StatusForbidden)
			return
		}
	}

	if h.uaFilter != nil && h.uaFilter.Blocked(r.UserAgent()) {
		h.uaBlocked.Add(1)
		outcome = "ua_blocked"
		http.Error(rw, "forbidden", http.StatusForbidden)
		return
	}

	applyWAF := true
	if h.dashboardAuth != nil && host == h.dashboardAuth.Domain {
		applyWAF = false
	} else if route.Security != nil {
		applyWAF = route.Security.WAFEnabled
	}
	if h.waf != nil && applyWAF {
		customRules := []string(nil)
		mode := middleware.WAFModeDetection
		if route.Security != nil {
			customRules = route.Security.CustomRules
			mode = middleware.WAFMode(route.Security.WAFMode)
		}
		result, err := h.waf.Evaluate(r, client, mode, customRules)
		if err != nil {
			log.Printf("proxy: waf evaluate: %v", err)
		} else {
			if result.Detected {
				outcome = "waf_detected"
			}
			if result.Blocked {
				h.wafBlocked.Add(1)
				outcome = "waf_blocked"
				http.Error(rw, "forbidden", result.Status)
				return
			}
		}
	}

	if h.dashboardAuth != nil && host == h.dashboardAuth.Domain {
		if !middleware.ValidBasicAuth(r, h.dashboardAuth.Username, h.dashboardAuth.Password) {
			outcome = "unauthorized"
			middleware.WriteBasicAuthChallenge(rw, "Dirigent")
			return
		}
	}

	if !middleware.ValidBasicAuthUsers(r, route.BasicAuth) {
		outcome = "unauthorized"
		middleware.WriteBasicAuthChallenge(rw, "Dirigent")
		return
	}

	if !route.Public && h.jwtSecret != nil {
		if !h.validProxyToken(r) {
			outcome = "unauthorized"
			h.serveLoginPage(rw, r, "")
			return
		}
	}

	target := &url.URL{
		Scheme: "http",
		Host:   route.Upstream,
	}

	rp := &httputil.ReverseProxy{
		Director: func(req *http.Request) {
			req.URL.Scheme = target.Scheme
			req.URL.Host = target.Host
			req.Header.Set("X-Forwarded-Host", req.Host)
		},
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
			outcome = "upstream_error"
			log.Printf("proxy: upstream %s: %v", route.Upstream, err)
			http.Error(w, "upstream unavailable", http.StatusBadGateway)
		},
	}

	rp.ServeHTTP(rw, r)
}

func (h *Handler) listAccessLogs(w http.ResponseWriter, r *http.Request) {
	if h.accessLogs == nil {
		writeJSON(w, http.StatusOK, []AccessLogEvent{})
		return
	}
	provider, ok := h.accessLogs.(interface{ List(int) []AccessLogEvent })
	if !ok {
		writeJSON(w, http.StatusOK, []AccessLogEvent{})
		return
	}
	limit := 200
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 && parsed <= 1000 {
			limit = parsed
		}
	}
	writeJSON(w, http.StatusOK, provider.List(limit))
}

func (h *Handler) securityConfig(w http.ResponseWriter, _ *http.Request) {
	cfg := HardeningSettings{Profile: h.hardening}
	if h.scanner != nil {
		cfg.SuspiciousWindowSeconds = int64(h.scanner.window.Seconds())
		cfg.SuspiciousThreshold = h.scanner.threshold
		cfg.SuspiciousBlockForSeconds = int64(h.scanner.blockFor.Seconds())
	}
	if h.ipFilter != nil {
		cfg.GlobalIPDenylist, cfg.GlobalIPAllowlist = h.ipFilter.GlobalConfig()
	}
	writeJSON(w, http.StatusOK, cfg)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("proxy: writeJSON: %v", err)
	}
}

type accessLogResponseWriter struct {
	http.ResponseWriter
	status       int
	bytesWritten int64
}

func (w *accessLogResponseWriter) WriteHeader(statusCode int) {
	w.status = statusCode
	w.ResponseWriter.WriteHeader(statusCode)
}

func (w *accessLogResponseWriter) Write(b []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	n, err := w.ResponseWriter.Write(b)
	w.bytesWritten += int64(n)
	return n, err
}

func (w *accessLogResponseWriter) Status() int {
	if w.status == 0 {
		return http.StatusOK
	}
	return w.status
}

func (w *accessLogResponseWriter) BytesWritten() int64 {
	return w.bytesWritten
}

func (w *accessLogResponseWriter) Flush() {
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func (w *accessLogResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	h, ok := w.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, http.ErrNotSupported
	}
	return h.Hijack()
}

func (w *accessLogResponseWriter) ReadFrom(r io.Reader) (int64, error) {
	rf, ok := w.ResponseWriter.(io.ReaderFrom)
	if !ok {
		n, err := io.Copy(w.ResponseWriter, r)
		w.bytesWritten += n
		if w.status == 0 {
			w.status = http.StatusOK
		}
		return n, err
	}
	n, err := rf.ReadFrom(r)
	w.bytesWritten += n
	if w.status == 0 {
		w.status = http.StatusOK
	}
	return n, err
}

func (w *accessLogResponseWriter) Push(target string, opts *http.PushOptions) error {
	p, ok := w.ResponseWriter.(http.Pusher)
	if !ok {
		return http.ErrNotSupported
	}
	return p.Push(target, opts)
}

type routeRequest struct {
	Domain   string `json:"domain"`
	Upstream string `json:"upstream"`
}

// setRoute handles POST /internal/routes. The orchestrator calls this to swap
// an upstream immediately during a zero-downtime redeploy without waiting for
// the next store-poll cycle.
func (h *Handler) setRoute(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)

	var body routeRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if body.Domain == "" || body.Upstream == "" {
		http.Error(w, "domain and upstream are required", http.StatusBadRequest)
		return
	}

	h.table.Set(normalizeDomain(body.Domain), body.Upstream, false, nil, nil)
	w.WriteHeader(http.StatusNoContent)
}

func WithIPFilter(filter *middleware.IPFilter) Option {
	return func(h *Handler) {
		h.ipFilter = filter
	}
}

// WithAuthStore provides the user store used for proxy-level login on private deployments.
func WithAuthStore(s *auth.UserStore) Option {
	return func(h *Handler) {
		h.authStore = s
	}
}

// WithJWTSecret sets the HMAC secret used to sign and verify session tokens.
func WithJWTSecret(secret []byte) Option {
	return func(h *Handler) {
		h.jwtSecret = secret
	}
}

func WithUAFilter(filter *middleware.UAFilter) Option {
	return func(h *Handler) {
		h.uaFilter = filter
	}
}

func WithWAF(waf *middleware.WAF) Option {
	return func(h *Handler) {
		h.waf = waf
	}
}

func normalizeDomain(domain string) string {
	domain = strings.TrimSpace(domain)
	domain = strings.TrimSuffix(domain, ".")
	return strings.ToLower(domain)
}

func normalizeHardeningProfile(profile HardeningProfile) HardeningProfile {
	switch HardeningProfile(strings.ToLower(strings.TrimSpace(string(profile)))) {
	case HardeningOff:
		return HardeningOff
	case HardeningStrict:
		return HardeningStrict
	default:
		return HardeningStandard
	}
}

func shouldBlockPath(path string, profile HardeningProfile) bool {
	if normalizeHardeningProfile(profile) == HardeningOff {
		return false
	}

	p := strings.ToLower(strings.TrimSpace(path))
	if p == "" {
		p = "/"
	}

	for _, prefix := range []string{"/.git", "/.env", "/.vscode", "/.idea", "/.aws", "/.ssh", "/.svn", "/.hg"} {
		if hasPathPrefix(p, prefix) {
			return true
		}
	}

	for _, exact := range []string{"/.ds_store", "/docker-compose.yml", "/docker-compose.yaml"} {
		if p == exact {
			return true
		}
	}

	if normalizeHardeningProfile(profile) != HardeningStrict {
		return false
	}

	for _, prefix := range []string{"/wp-admin", "/wp-includes", "/actuator", "/swagger", "/api-docs", "/debug", "/telescope", "/server-status", "/phpmyadmin"} {
		if hasPathPrefix(p, prefix) {
			return true
		}
	}

	for _, exact := range []string{"/wp-login.php", "/xmlrpc.php", "/info.php", "/phpinfo.php", "/v2/_catalog"} {
		if p == exact {
			return true
		}
	}

	for _, prefix := range []string{"/swagger-", "/v2/api-docs", "/v3/api-docs"} {
		if strings.HasPrefix(p, prefix) {
			return true
		}
	}

	return false
}

func hasPathPrefix(path, prefix string) bool {
	if path == prefix {
		return true
	}
	if strings.HasPrefix(path, prefix+"/") {
		return true
	}
	if prefix == "/.env" && strings.HasPrefix(path, "/.env.") {
		return true
	}
	return false
}

type scannerLimiter struct {
	mu                 sync.Mutex
	entries            map[string]scannerEntry
	window             time.Duration
	threshold          int
	blockFor           time.Duration
	totalRequests      int64
	suspiciousRequests int64
	blockedRequests    int64
}

type scannerEntry struct {
	windowStart  time.Time
	count        int
	blockedUntil time.Time
}

func newScannerLimiter(profile HardeningProfile) *scannerLimiter {
	switch normalizeHardeningProfile(profile) {
	case HardeningOff:
		return nil
	case HardeningStrict:
		return &scannerLimiter{entries: make(map[string]scannerEntry), window: time.Minute, threshold: 6, blockFor: 15 * time.Minute}
	default:
		return &scannerLimiter{entries: make(map[string]scannerEntry), window: time.Minute, threshold: 12, blockFor: 10 * time.Minute}
	}
}

func (l *scannerLimiter) IsBlocked(ip string, now time.Time) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	e, ok := l.entries[ip]
	if !ok {
		return false
	}
	if now.Before(e.blockedUntil) {
		return true
	}
	if !e.blockedUntil.IsZero() {
		e.blockedUntil = time.Time{}
		e.windowStart = now
		e.count = 0
		l.entries[ip] = e
	}
	return false
}

func (l *scannerLimiter) RecordSuspicious(ip string, now time.Time) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.suspiciousRequests++

	if ip == "" {
		return
	}

	e := l.entries[ip]
	if e.windowStart.IsZero() || now.Sub(e.windowStart) > l.window {
		e.windowStart = now
		e.count = 0
	}
	e.count++
	if e.count >= l.threshold {
		e.blockedUntil = now.Add(l.blockFor)
		e.windowStart = now
		e.count = 0
	}
	l.entries[ip] = e
}

func (l *scannerLimiter) RecordRequest() {
	l.mu.Lock()
	l.totalRequests++
	l.mu.Unlock()
}

func (l *scannerLimiter) RecordBlockedRequest() {
	l.mu.Lock()
	l.blockedRequests++
	l.mu.Unlock()
}

func (l *scannerLimiter) Snapshot(now time.Time) trafficStatus {
	l.mu.Lock()
	defer l.mu.Unlock()

	blocked := make([]blockedIPTrafficStatus, 0, len(l.entries))
	for ip, entry := range l.entries {
		if now.Before(entry.blockedUntil) {
			blockedUntil := entry.blockedUntil
			blocked = append(blocked, blockedIPTrafficStatus{IP: ip, BlockedUntil: &blockedUntil})
		}
	}

	slices.SortFunc(blocked, func(a, b blockedIPTrafficStatus) int {
		if a.BlockedUntil == nil || b.BlockedUntil == nil {
			return strings.Compare(a.IP, b.IP)
		}
		if a.BlockedUntil.Equal(*b.BlockedUntil) {
			return strings.Compare(a.IP, b.IP)
		}
		if a.BlockedUntil.Before(*b.BlockedUntil) {
			return 1
		}
		return -1
	})

	return trafficStatus{
		TotalRequests:      l.totalRequests,
		SuspiciousRequests: l.suspiciousRequests,
		BlockedRequests:    l.blockedRequests,
		ActiveBlockedIPs:   len(blocked),
		BlockedIPs:         blocked,
	}
}

func clientIP(r *http.Request) string {
	forwarded := strings.TrimSpace(r.Header.Get("X-Forwarded-For"))
	if forwarded != "" {
		parts := strings.Split(forwarded, ",")
		if len(parts) > 0 {
			ip := strings.TrimSpace(parts[0])
			if net.ParseIP(ip) != nil {
				return ip
			}
		}
	}

	host, _, err := net.SplitHostPort(strings.TrimSpace(r.RemoteAddr))
	if err == nil && net.ParseIP(host) != nil {
		return host
	}
	if net.ParseIP(strings.TrimSpace(r.RemoteAddr)) != nil {
		return strings.TrimSpace(r.RemoteAddr)
	}
	return ""
}
