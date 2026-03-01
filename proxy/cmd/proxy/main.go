package main

import (
	"context"
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

	"github.com/ercadev/dirigent/auth"
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

	userStore, jwtSecret, err := authFromEnv(dataPath())
	if err != nil {
		log.Fatalf("proxy: %v", err)
	}
	if userStore != nil {
		defer userStore.Close()
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
		handler.WithHardeningProfile(hardeningProfile),
		handler.WithAccessLogger(accessLogger),
		handler.WithIPFilter(ipFilter),
		handler.WithUAFilter(uaFilter),
		handler.WithWAF(waf),
		handler.WithAuthStore(userStore),
		handler.WithJWTSecret(jwtSecret),
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

func authFromEnv(dataPath string) (*auth.UserStore, []byte, error) {
	secret := strings.TrimSpace(os.Getenv("LOTSEN_JWT_SECRET"))
	if secret == "" {
		return nil, nil, nil
	}

	dbPath := filepath.Join(filepath.Dir(dataPath), "users.db")
	userStore, err := auth.NewUserStore(dbPath)
	if err != nil {
		return nil, nil, fmt.Errorf("open user store: %w", err)
	}

	return userStore, []byte(secret), nil
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
