package handler

import (
	"encoding/json"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
)

// RoutingTable is the interface the handler reads from when proxying requests
// and the control API writes to when swapping upstreams.
type RoutingTable interface {
	Get(domain string) (string, bool)
	Set(domain, upstream string)
}

// Handler serves inbound HTTP requests by routing them to the upstream
// registered for the request's Host header, and exposes a small control API
// so the orchestrator can trigger upstream swaps during zero-downtime redeploys.
type Handler struct {
	table RoutingTable
}

// New creates a Handler backed by the given routing table.
func New(table RoutingTable) *Handler {
	return &Handler{table: table}
}

// RegisterRoutes wires the proxy catch-all and the internal control API into mux.
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /internal/routes", h.setRoute)
	mux.HandleFunc("/", h.proxy)
}

// proxy routes inbound requests to the upstream registered for the Host header.
// Returns 404 for unknown domains and 502 when the upstream is unreachable.
func (h *Handler) proxy(w http.ResponseWriter, r *http.Request) {
	host := r.Host
	// Strip port if present (e.g. "example.com:80" → "example.com").
	if bare, _, err := net.SplitHostPort(host); err == nil {
		host = bare
	}

	upstream, ok := h.table.Get(host)
	if !ok {
		http.Error(w, "unknown domain", http.StatusNotFound)
		return
	}

	target := &url.URL{
		Scheme: "http",
		Host:   upstream,
	}

	rp := &httputil.ReverseProxy{
		Director: func(req *http.Request) {
			req.URL.Scheme = target.Scheme
			req.URL.Host = target.Host
			req.Header.Set("X-Forwarded-Host", req.Host)
		},
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
			log.Printf("proxy: upstream %s: %v", upstream, err)
			http.Error(w, "upstream unavailable", http.StatusBadGateway)
		},
	}

	rp.ServeHTTP(w, r)
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

	h.table.Set(body.Domain, body.Upstream)
	w.WriteHeader(http.StatusNoContent)
}
