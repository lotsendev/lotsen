package middleware_test

import (
	"testing"

	"github.com/lotsendev/lotsen/proxy/internal/middleware"
)

func TestUAFilter_BlocksScannerAndHeadless(t *testing.T) {
	filter := middleware.NewUAFilter(false, nil)

	if !filter.Blocked("sqlmap/1.7") {
		t.Fatal("want sqlmap user-agent blocked")
	}
	if !filter.Blocked("Mozilla/5.0 HeadlessChrome/120.0") {
		t.Fatal("want headlesschrome user-agent blocked")
	}
	if filter.Blocked("Mozilla/5.0 Safari/537.36") {
		t.Fatal("want normal browser user-agent allowed")
	}
}

func TestUAFilter_StrictAndCustomRules(t *testing.T) {
	filter := middleware.NewUAFilter(true, []string{"evilbot"})

	if !filter.Blocked("curl/8.9.0") {
		t.Fatal("want curl blocked under strict profile")
	}
	if !filter.Blocked("Mozilla/5.0 EvilBot/1.0") {
		t.Fatal("want custom user-agent token blocked")
	}
}
