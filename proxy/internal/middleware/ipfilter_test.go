package middleware_test

import (
	"testing"

	"github.com/lotsendev/lotsen/proxy/internal/middleware"
	"github.com/lotsendev/lotsen/store"
)

func TestIPFilter_EvaluateGlobal(t *testing.T) {
	filter, err := middleware.NewIPFilter([]string{"10.0.0.0/8"}, []string{"203.0.113.0/24"})
	if err != nil {
		t.Fatalf("NewIPFilter: %v", err)
	}

	if got := filter.EvaluateGlobal("10.1.2.3"); got != middleware.IPFilterDenied {
		t.Fatalf("want %q, got %q", middleware.IPFilterDenied, got)
	}
	if got := filter.EvaluateGlobal("198.51.100.10"); got != middleware.IPFilterNotAllowed {
		t.Fatalf("want %q, got %q", middleware.IPFilterNotAllowed, got)
	}
	if got := filter.EvaluateGlobal("203.0.113.9"); got != middleware.IPFilterAllowed {
		t.Fatalf("want allowed, got %q", got)
	}
}

func TestIPFilter_EvaluateDeployment(t *testing.T) {
	filter, err := middleware.NewIPFilter(nil, nil)
	if err != nil {
		t.Fatalf("NewIPFilter: %v", err)
	}

	security := &store.SecurityConfig{IPDenylist: []string{"192.0.2.0/24"}, IPAllowlist: []string{"198.51.100.0/24"}}
	if got := filter.EvaluateDeployment("192.0.2.7", security); got != middleware.IPFilterDenied {
		t.Fatalf("want %q, got %q", middleware.IPFilterDenied, got)
	}
	if got := filter.EvaluateDeployment("203.0.113.10", security); got != middleware.IPFilterNotAllowed {
		t.Fatalf("want %q, got %q", middleware.IPFilterNotAllowed, got)
	}
	if got := filter.EvaluateDeployment("198.51.100.42", security); got != middleware.IPFilterAllowed {
		t.Fatalf("want allowed, got %q", got)
	}
}
