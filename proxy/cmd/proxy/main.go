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
	"strings"
	"syscall"
	"time"

	"golang.org/x/crypto/acme"
	"golang.org/x/crypto/acme/autocert"

	"github.com/ercadev/dirigent/proxy/internal/handler"
	"github.com/ercadev/dirigent/proxy/internal/poller"
	"github.com/ercadev/dirigent/proxy/internal/routing"
	"github.com/ercadev/dirigent/store"
)

const (
	defaultHTTPAddr                = ":80"
	defaultHTTPSAddr               = ":443"
	defaultCertCacheDir            = "/var/lib/dirigent/certs"
	defaultAccessLogDir            = "/var/lib/dirigent/logs/proxy"
	letsencryptProdDirectoryURL    = "https://acme-v02.api.letsencrypt.org/directory"
	letsencryptStagingDirectoryURL = "https://acme-staging-v02.api.letsencrypt.org/directory"
)

func dataPath() string {
	if p := os.Getenv("DIRIGENT_DATA"); p != "" {
		return p
	}
	return "/var/lib/dirigent/deployments.json"
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
		table.SetStatic(dashboardAuth.Domain, "localhost:3000")
		log.Printf("proxy: registered dashboard domain %s -> localhost:3000", dashboardAuth.Domain)
	}

	log.Printf("proxy: hardening profile %s", hardeningProfile)

	interval := 5 * time.Second
	if v := os.Getenv("DIRIGENT_POLL_INTERVAL"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			interval = d
		}
	}

	p := poller.New(s, table, interval)
	h := handler.New(table, dashboardAuth, handler.WithHardeningProfile(hardeningProfile), handler.WithAccessLogger(accessLogger))

	hostPolicy := hostPolicyFromTable(table)
	cacheDir := envOrDefault("DIRIGENT_CERT_CACHE_DIR", defaultCertCacheDir)
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		log.Fatalf("proxy: create cert cache dir %s: %v", cacheDir, err)
	}
	manager := newAutocertManager(
		cacheDir,
		os.Getenv("DIRIGENT_ACME_EMAIL"),
		envOrDefault("DIRIGENT_ACME_DIRECTORY_URL", letsencryptProdDirectoryURL),
		hostPolicy,
	)

	httpsAddr := envOrDefault("DIRIGENT_PROXY_HTTPS_ADDR", defaultHTTPSAddr)
	httpAddr := envOrDefault("DIRIGENT_PROXY_HTTP_ADDR", envOrDefault("DIRIGENT_PROXY_ADDR", defaultHTTPAddr))

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
	domain := normalizeDomain(os.Getenv("DIRIGENT_DASHBOARD_DOMAIN"))
	user := os.Getenv("DIRIGENT_DASHBOARD_USER")
	password := os.Getenv("DIRIGENT_DASHBOARD_PASSWORD")

	if domain == "" {
		if user != "" || password != "" {
			log.Printf("proxy: ignoring dashboard auth credentials because DIRIGENT_DASHBOARD_DOMAIN is unset")
		}
		return nil, nil
	}

	if user == "" || password == "" {
		return nil, fmt.Errorf("DIRIGENT_DASHBOARD_DOMAIN requires both DIRIGENT_DASHBOARD_USER and DIRIGENT_DASHBOARD_PASSWORD")
	}

	return &handler.DashboardAuth{
		Domain:   domain,
		Username: user,
		Password: password,
	}, nil
}

func hardeningProfileFromEnv() (handler.HardeningProfile, error) {
	profile := handler.HardeningProfile(strings.ToLower(strings.TrimSpace(envOrDefault("DIRIGENT_PROXY_HARDENING_PROFILE", string(handler.HardeningStandard)))))
	switch profile {
	case handler.HardeningOff, handler.HardeningStandard, handler.HardeningStrict:
		return profile, nil
	default:
		return "", fmt.Errorf("DIRIGENT_PROXY_HARDENING_PROFILE must be one of: off, standard, strict")
	}
}

func accessLogConfigFromEnv() (handler.AccessLogConfig, error) {
	retentionRaw := strings.TrimSpace(envOrDefault("DIRIGENT_PROXY_ACCESS_LOG_RETENTION", "168h"))
	retention, err := time.ParseDuration(retentionRaw)
	if err != nil || retention <= 0 {
		return handler.AccessLogConfig{}, fmt.Errorf("DIRIGENT_PROXY_ACCESS_LOG_RETENTION must be a positive duration")
	}

	headers := []string{"host", "user-agent", "accept", "accept-encoding", "accept-language", "referer", "x-forwarded-for", "x-real-ip"}
	if raw := strings.TrimSpace(os.Getenv("DIRIGENT_PROXY_ACCESS_LOG_HEADERS")); raw != "" {
		headers = headers[:0]
		for _, part := range strings.Split(raw, ",") {
			header := strings.ToLower(strings.TrimSpace(part))
			if header != "" {
				headers = append(headers, header)
			}
		}
		if len(headers) == 0 {
			return handler.AccessLogConfig{}, fmt.Errorf("DIRIGENT_PROXY_ACCESS_LOG_HEADERS must contain at least one header")
		}
	}

	return handler.AccessLogConfig{
		Dir:             strings.TrimSpace(envOrDefault("DIRIGENT_PROXY_ACCESS_LOG_DIR", defaultAccessLogDir)),
		Retention:       retention,
		WhitelistedKeys: headers,
	}, nil
}
