package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/lotsendev/lotsen/proxy/internal/middleware"
)

func TestWAF_EnforcementBlocksCustomRuleMatch(t *testing.T) {
	waf, err := middleware.NewWAF()
	if err != nil {
		t.Fatalf("NewWAF: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "http://example.com/waf-trigger", nil)
	result, err := waf.Evaluate(req, "203.0.113.7", middleware.WAFModeEnforcement, []string{
		`SecRule REQUEST_URI "@contains waf-trigger" "id:10000,phase:1,deny,status:403,log,msg:'waf trigger'"`,
	})
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if !result.Blocked {
		t.Fatalf("want blocked result, got %#v", result)
	}
}

func TestWAF_DetectionMarksMatchWithoutBlocking(t *testing.T) {
	waf, err := middleware.NewWAF()
	if err != nil {
		t.Fatalf("NewWAF: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "http://example.com/waf-trigger", nil)
	result, err := waf.Evaluate(req, "203.0.113.7", middleware.WAFModeDetection, []string{
		`SecRule REQUEST_URI "@contains waf-trigger" "id:10001,phase:1,deny,status:403,log,msg:'waf trigger'"`,
	})
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if !result.Detected || result.Blocked {
		t.Fatalf("want detected-only result, got %#v", result)
	}
}
