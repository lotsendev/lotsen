package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"golang.org/x/crypto/acme"
	"golang.org/x/crypto/acme/autocert"

	"github.com/ercadev/dirigent/proxy/internal/handler"
	"github.com/ercadev/dirigent/proxy/internal/middleware"
	"github.com/ercadev/dirigent/proxy/internal/poller"
	"github.com/ercadev/dirigent/proxy/internal/routing"
	"github.com/ercadev/dirigent/store"
)

const (
	defaultHTTPAddr                = ":80"
	defaultHTTPSAddr               = ":443"
	defaultCertCacheDir            = "/var/lib/lotsen/certs"
	defaultAccessLogDir            = "/var/lib/lotsen/logs/proxy"
	letsencryptProdDirectoryURL    = "https://acme-v02.api.letsencrypt.org/directory"
	letsencryptStagingDirectoryURL = "https://acme-staging-v02.api.letsencrypt.org/directory"
)

func dataPath() string {
	if p := os.Getenv("LOTSEN_DATA"); p != "" {
		return p
	}
	return "/var/lib/lotsen/deployments.json"
}

func main() {
	s, err := store.NewJSONStore(dataPath())
	if err != nil {
		log.Fatalf("proxy: open store: %v", err)
	}

	table := routing.NewTable()

	dashboardAuth, err := dashboardAuthFromEnv()
	if err != nil {
		log.Fatalf("proxy: %v", err)
	}
	dashboardAccessMode, err := dashboardAccessModeFromEnv()
	if err != nil {
		log.Fatalf("proxy: %v", err)
	}
	dashboardWAFConfig, err := dashboardWAFConfigFromEnv()
	if err != nil {
		log.Fatalf("proxy: %v", err)
	}
	profilePath := hostProfilePath(dataPath())
	dashboardAccessModeResolver := dashboardAccessModeResolver(profilePath, dashboardAccessMode)
	dashboardWAFResolver := dashboardWAFConfigResolver(profilePath, dashboardWAFConfig)
	authCookieDomain, err := authCookieDomainFromEnv()
	if err != nil {
		log.Fatalf("proxy: %v", err)
	}
	hardeningProfile, err := hardeningProfileFromEnv()
	if err != nil {
		log.Fatalf("proxy: %v", err)
	}
	ipDenylist := parseCSVList(os.Getenv("LOTSEN_IP_DENYLIST"))
	ipAllowlist := parseCSVList(os.Getenv("LOTSEN_IP_ALLOWLIST"))
	ipFilter, err := middleware.NewIPFilter(ipDenylist, ipAllowlist)
	if err != nil {
		log.Fatalf("proxy: %v", err)
	}
	uaFilter := middleware.NewUAFilter(hardeningProfile == handler.HardeningStrict, parseCSVList(os.Getenv("LOTSEN_UA_BLOCK_LIST")))
	waf, err := middleware.NewWAF()
	if err != nil {
		log.Fatalf("proxy: initialize waf: %v", err)
	}
	accessLogConfig, err := accessLogConfigFromEnv()
	if err != nil {
		log.Fatalf("proxy: %v", err)
	}
	accessLogger, err := handler.NewFileAccessLogger(accessLogConfig)
	if err != nil {
		log.Fatalf("proxy: initialize access logger: %v", err)
	}
	defer accessLogger.Close()
	if dashboardAuth != nil {
		table.SetStatic(dashboardAuth.Domain, "localhost:8080")
		log.Printf("proxy: registered dashboard domain %s -> localhost:8080", dashboardAuth.Domain)
	}

	jwtSecret, err := authFromEnv()
	if err != nil {
		log.Fatalf("proxy: %v", err)
	}

	log.Printf("proxy: hardening profile %s", hardeningProfile)
	log.Printf("proxy: waf initialized for per-deployment mode")

	interval := 5 * time.Second
	if v := os.Getenv("LOTSEN_POLL_INTERVAL"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			interval = d
		}
	}

	p := poller.New(s, table, interval)
	h := handler.New(
		table,
		dashboardAuth,
		handler.WithDashboardAccessModeResolver(dashboardAccessModeResolver),
		handler.WithDashboardWAFConfigResolver(dashboardWAFResolver),
		handler.WithHardeningProfile(hardeningProfile),
		handler.WithAccessLogger(accessLogger),
		handler.WithIPFilter(ipFilter),
		handler.WithUAFilter(uaFilter),
		handler.WithWAF(waf),
		handler.WithJWTSecret(jwtSecret),
		handler.WithAuthCookieDomain(authCookieDomain),
	)

	hostPolicy := hostPolicyFromTable(table)
	cacheDir := envOrDefault("LOTSEN_CERT_CACHE_DIR", defaultCertCacheDir)
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		log.Fatalf("proxy: create cert cache dir %s: %v", cacheDir, err)
	}
	manager := newAutocertManager(
		cacheDir,
		os.Getenv("LOTSEN_ACME_EMAIL"),
		envOrDefault("LOTSEN_ACME_DIRECTORY_URL", letsencryptProdDirectoryURL),
		hostPolicy,
	)

	httpsAddr := envOrDefault("LOTSEN_PROXY_HTTPS_ADDR", defaultHTTPSAddr)
	httpAddr := envOrDefault("LOTSEN_PROXY_HTTP_ADDR", envOrDefault("LOTSEN_PROXY_ADDR", defaultHTTPAddr))

	httpMux := newHTTPMux(h, manager.HTTPHandler(http.HandlerFunc(redirectToHTTPS(httpsAddr))))
	httpsMux := newHTTPSMux(h)

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	go p.Run(ctx)

	httpSrv := &http.Server{Addr: httpAddr, Handler: httpMux}
	httpsSrv := &http.Server{Addr: httpsAddr, Handler: httpsMux, TLSConfig: manager.TLSConfig()}

	go func() {
		<-ctx.Done()
		if err := httpSrv.Close(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Printf("proxy: http shutdown: %v", err)
		}
		if err := httpsSrv.Close(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Printf("proxy: https shutdown: %v", err)
		}
	}()

	errCh := make(chan error, 2)

	go func() {
		log.Printf("proxy: listening for HTTP on %s", httpAddr)
		if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- fmt.Errorf("http server: %w", err)
		}
	}()

	go func() {
		log.Printf("proxy: listening for HTTPS on %s", httpsAddr)
		if err := httpsSrv.ListenAndServeTLS("", ""); err != nil && err != http.ErrServerClosed {
			errCh <- fmt.Errorf("https server: %w", err)
		}
	}()

	select {
	case err := <-errCh:
		log.Fatalf("proxy: %v", err)
	case <-ctx.Done():
	}
}

func newHTTPMux(h *handler.Handler, external http.Handler) *http.ServeMux {
	mux := http.NewServeMux()
	h.RegisterInternalRoutes(mux)
	mux.Handle("/", external)
	return mux
}

func newHTTPSMux(h *handler.Handler) *http.ServeMux {
	mux := http.NewServeMux()
	h.RegisterProxyRoutes(mux)
	return mux
}

func redirectToHTTPS(httpsAddr string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		host := hostWithoutPort(r.Host)
		if host == "" {
			http.Error(w, "missing host", http.StatusBadRequest)
			return
		}
		if port := portFromAddr(httpsAddr); port != "" && port != "443" {
			host = net.JoinHostPort(host, port)
		}
		target := &url.URL{
			Scheme:   "https",
			Host:     host,
			Path:     r.URL.Path,
			RawQuery: r.URL.RawQuery,
		}
		http.Redirect(w, r, target.String(), http.StatusMovedPermanently)
	}
}

func hostPolicyFromTable(table interface {
	Get(string) (routing.Route, bool)
}) autocert.HostPolicy {
	return func(_ context.Context, host string) error {
		host = normalizeDomain(host)
		if host == "" {
			return errors.New("empty host")
		}
		if _, ok := table.Get(host); !ok {
			return fmt.Errorf("host %q not configured", host)
		}
		return nil
	}
}

func newAutocertManager(cacheDir, email, directoryURL string, hostPolicy autocert.HostPolicy) *autocert.Manager {
	if directoryURL == "" {
		directoryURL = letsencryptProdDirectoryURL
	}
	return &autocert.Manager{
		Prompt:     autocert.AcceptTOS,
		Cache:      autocert.DirCache(cacheDir),
		HostPolicy: hostPolicy,
		Email:      email,
		Client: &acme.Client{
			DirectoryURL: directoryURL,
		},
	}
}

func hostWithoutPort(host string) string {
	host = strings.TrimSpace(host)
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	return normalizeDomain(host)
}

func normalizeDomain(domain string) string {
	domain = strings.TrimSpace(domain)
	domain = strings.TrimSuffix(domain, ".")
	return strings.ToLower(domain)
}

func hostProfilePath(storePath string) string {
	return filepath.Join(filepath.Dir(storePath), "host_profile.json")
}

func portFromAddr(addr string) string {
	addr = strings.TrimSpace(addr)
	if strings.HasPrefix(addr, ":") {
		return strings.TrimPrefix(addr, ":")
	}
	if _, port, err := net.SplitHostPort(addr); err == nil {
		return port
	}
	return ""
}

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func dashboardAuthFromEnv() (*handler.DashboardAuth, error) {
	domain := normalizeDomain(os.Getenv("LOTSEN_DASHBOARD_DOMAIN"))
	user := strings.TrimSpace(os.Getenv("LOTSEN_DASHBOARD_USER"))
	password := strings.TrimSpace(os.Getenv("LOTSEN_DASHBOARD_PASSWORD"))

	if domain == "" {
		if user != "" || password != "" {
			log.Printf("proxy: ignoring LOTSEN_DASHBOARD_USER/LOTSEN_DASHBOARD_PASSWORD because LOTSEN_DASHBOARD_DOMAIN is unset")
		}
		return nil, nil
	}

	if user != "" || password != "" {
		log.Printf("proxy: LOTSEN_DASHBOARD_USER/LOTSEN_DASHBOARD_PASSWORD are deprecated and ignored; dashboard relies on app JWT auth")
	}

	return &handler.DashboardAuth{
		Domain: domain,
	}, nil
}

func dashboardAccessModeFromEnv() (handler.DashboardAccessMode, error) {
	raw := strings.ToLower(strings.TrimSpace(os.Getenv("LOTSEN_DASHBOARD_ACCESS_MODE")))
	if raw == "" {
		return handler.DashboardAccessModeLoginOnly, nil
	}

	mode := handler.DashboardAccessMode(raw)
	switch mode {
	case handler.DashboardAccessModeLoginOnly, handler.DashboardAccessModeWAFOnly, handler.DashboardAccessModeWAFAndLogin:
		return mode, nil
	default:
		return "", fmt.Errorf("LOTSEN_DASHBOARD_ACCESS_MODE must be one of: login_only, waf_only, waf_and_login")
	}
}

func dashboardWAFConfigFromEnv() (handler.DashboardWAFConfig, error) {
	modeRaw := strings.ToLower(strings.TrimSpace(envOrDefault("LOTSEN_DASHBOARD_WAF_MODE", string(middleware.WAFModeDetection))))
	mode := middleware.WAFMode(modeRaw)
	switch mode {
	case middleware.WAFModeDetection, middleware.WAFModeEnforcement:
	default:
		return handler.DashboardWAFConfig{}, fmt.Errorf("LOTSEN_DASHBOARD_WAF_MODE must be one of: detection, enforcement")
	}

	ipAllowlist := parseCSVList(os.Getenv("LOTSEN_DASHBOARD_IP_ALLOWLIST"))

	rules := []string{}
	if rawRules := strings.TrimSpace(os.Getenv("LOTSEN_DASHBOARD_WAF_RULES")); rawRules != "" {
		for _, line := range strings.Split(rawRules, "\n") {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			rules = append(rules, line)
		}
	}

	return handler.DashboardWAFConfig{Mode: mode, IPAllowlist: ipAllowlist, CustomRules: rules}, nil
}

func dashboardAccessModeResolver(profilePath string, fallback handler.DashboardAccessMode) func() handler.DashboardAccessMode {
	return func() handler.DashboardAccessMode {
		raw, err := os.ReadFile(profilePath)
		if err != nil {
			return fallback
		}

		var profile struct {
			DashboardAccessMode string `json:"dashboardAccessMode"`
		}
		if err := json.Unmarshal(raw, &profile); err != nil {
			return fallback
		}
		mode := handler.DashboardAccessMode(strings.ToLower(strings.TrimSpace(profile.DashboardAccessMode)))
		switch mode {
		case handler.DashboardAccessModeLoginOnly, handler.DashboardAccessModeWAFOnly, handler.DashboardAccessModeWAFAndLogin:
			return mode
		default:
			return fallback
		}
	}
}

func dashboardWAFConfigResolver(profilePath string, fallback handler.DashboardWAFConfig) func() handler.DashboardWAFConfig {
	return func() handler.DashboardWAFConfig {
		raw, err := os.ReadFile(profilePath)
		if err != nil {
			return fallback
		}

		var profile struct {
			DashboardWAF struct {
				Mode        string   `json:"mode"`
				IPAllowlist []string `json:"ipAllowlist"`
				CustomRules []string `json:"customRules"`
			} `json:"dashboardWaf"`
		}
		if err := json.Unmarshal(raw, &profile); err != nil {
			return fallback
		}
		if strings.TrimSpace(profile.DashboardWAF.Mode) == "" && len(profile.DashboardWAF.IPAllowlist) == 0 && len(profile.DashboardWAF.CustomRules) == 0 {
			return fallback
		}

		mode := middleware.WAFMode(strings.ToLower(strings.TrimSpace(profile.DashboardWAF.Mode)))
		switch mode {
		case middleware.WAFModeDetection, middleware.WAFModeEnforcement:
		default:
			mode = fallback.Mode
		}

		allowlist := make([]string, 0, len(profile.DashboardWAF.IPAllowlist))
		for _, entry := range profile.DashboardWAF.IPAllowlist {
			entry = strings.TrimSpace(entry)
			if entry == "" {
				continue
			}
			allowlist = append(allowlist, entry)
		}

		rules := make([]string, 0, len(profile.DashboardWAF.CustomRules))
		for _, rule := range profile.DashboardWAF.CustomRules {
			rule = strings.TrimSpace(rule)
			if rule == "" {
				continue
			}
			rules = append(rules, rule)
		}

		return handler.DashboardWAFConfig{Mode: mode, IPAllowlist: allowlist, CustomRules: rules}
	}
}

func hardeningProfileFromEnv() (handler.HardeningProfile, error) {
	profile := handler.HardeningProfile(strings.ToLower(strings.TrimSpace(envOrDefault("LOTSEN_PROXY_HARDENING_PROFILE", string(handler.HardeningStandard)))))
	switch profile {
	case handler.HardeningOff, handler.HardeningStandard, handler.HardeningStrict:
		return profile, nil
	default:
		return "", fmt.Errorf("LOTSEN_PROXY_HARDENING_PROFILE must be one of: off, standard, strict")
	}
}

func accessLogConfigFromEnv() (handler.AccessLogConfig, error) {
	retentionRaw := strings.TrimSpace(envOrDefault("LOTSEN_PROXY_ACCESS_LOG_RETENTION", "168h"))
	retention, err := time.ParseDuration(retentionRaw)
	if err != nil || retention <= 0 {
		return handler.AccessLogConfig{}, fmt.Errorf("LOTSEN_PROXY_ACCESS_LOG_RETENTION must be a positive duration")
	}

	headers := []string{"host", "user-agent", "accept", "accept-encoding", "accept-language", "referer", "x-forwarded-for", "x-real-ip"}
	if raw := strings.TrimSpace(os.Getenv("LOTSEN_PROXY_ACCESS_LOG_HEADERS")); raw != "" {
		headers = headers[:0]
		for _, part := range strings.Split(raw, ",") {
			header := strings.ToLower(strings.TrimSpace(part))
			if header != "" {
				headers = append(headers, header)
			}
		}
		if len(headers) == 0 {
			return handler.AccessLogConfig{}, fmt.Errorf("LOTSEN_PROXY_ACCESS_LOG_HEADERS must contain at least one header")
		}
	}

	return handler.AccessLogConfig{
		Dir:             strings.TrimSpace(envOrDefault("LOTSEN_PROXY_ACCESS_LOG_DIR", defaultAccessLogDir)),
		Retention:       retention,
		WhitelistedKeys: headers,
	}, nil
}

func authFromEnv() ([]byte, error) {
	secret := strings.TrimSpace(os.Getenv("LOTSEN_JWT_SECRET"))
	if secret == "" {
		return nil, nil
	}
	return []byte(secret), nil
}

func authCookieDomainFromEnv() (string, error) {
	raw := strings.TrimSpace(strings.TrimPrefix(os.Getenv("LOTSEN_AUTH_COOKIE_DOMAIN"), "."))
	if raw == "" {
		return "", nil
	}
	domain := normalizeDomain(raw)
	if !isValidCookieDomain(domain) {
		return "", fmt.Errorf("LOTSEN_AUTH_COOKIE_DOMAIN must be a valid domain")
	}
	return domain, nil
}

func isValidCookieDomain(domain string) bool {
	if domain == "" || strings.HasPrefix(domain, ".") || strings.HasSuffix(domain, ".") {
		return false
	}
	labels := strings.Split(domain, ".")
	if len(labels) < 2 {
		return false
	}
	for _, label := range labels {
		if label == "" || strings.HasPrefix(label, "-") || strings.HasSuffix(label, "-") {
			return false
		}
		for _, r := range label {
			if (r < 'a' || r > 'z') && (r < '0' || r > '9') && r != '-' {
				return false
			}
		}
	}
	return true
}

func parseCSVList(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		result = append(result, part)
	}
	return result
}
