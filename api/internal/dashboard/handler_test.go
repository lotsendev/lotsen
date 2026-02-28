package dashboard

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNew_RoutesAPIRequestsToAPIHandler(t *testing.T) {
	apiMux := http.NewServeMux()
	apiMux.HandleFunc("GET /api/ping", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("api"))
	})

	h := New(apiMux)
	req := httptest.NewRequest(http.MethodGet, "/api/ping", nil)
	rr := httptest.NewRecorder()

	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", rr.Code)
	}
	if strings.TrimSpace(rr.Body.String()) != "api" {
		t.Fatalf("want api response body, got %q", rr.Body.String())
	}
}

func TestNew_ServesSPAIndexForRoot(t *testing.T) {
	h := New(http.NewServeMux())
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()

	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", rr.Code)
	}
	if ct := rr.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
		t.Fatalf("want text/html content type, got %q", ct)
	}
	if rr.Body.Len() == 0 {
		t.Fatal("want non-empty index body")
	}
}

func TestNew_FallsBackToSPAIndexForUnknownPath(t *testing.T) {
	h := New(http.NewServeMux())
	req := httptest.NewRequest(http.MethodGet, "/deployments/123", nil)
	rr := httptest.NewRecorder()

	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", rr.Code)
	}
	if ct := rr.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
		t.Fatalf("want text/html content type, got %q", ct)
	}
}

func TestNew_HeadReturnsHeadersWithoutBody(t *testing.T) {
	h := New(http.NewServeMux())
	req := httptest.NewRequest(http.MethodHead, "/", nil)
	rr := httptest.NewRecorder()

	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", rr.Code)
	}
	res := rr.Result()
	defer res.Body.Close()
	body, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatalf("read response body: %v", err)
	}
	if len(body) != 0 {
		t.Fatalf("want empty body for HEAD, got %q", string(body))
	}
}

func TestNew_NonGetAndNonHeadReturnsMethodNotAllowed(t *testing.T) {
	h := New(http.NewServeMux())
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	rr := httptest.NewRecorder()

	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("want 405, got %d", rr.Code)
	}
}

func TestIsAPIPath(t *testing.T) {
	cases := []struct {
		path string
		want bool
	}{
		{path: "/api", want: true},
		{path: "/api/", want: true},
		{path: "/api/deployments", want: true},
		{path: "/apix", want: false},
		{path: "/", want: false},
	}

	for _, tc := range cases {
		if got := isAPIPath(tc.path); got != tc.want {
			t.Fatalf("path %q: want %v, got %v", tc.path, tc.want, got)
		}
	}
}
