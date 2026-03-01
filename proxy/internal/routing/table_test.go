package routing_test

import (
	"testing"

	"github.com/ercadev/dirigent/proxy/internal/routing"
	"github.com/ercadev/dirigent/store"
)

func TestTable_SetAndGet(t *testing.T) {
	tbl := routing.NewTable()
	tbl.Set("example.com", "localhost:8080", false, nil, nil)

	route, ok := tbl.Get("example.com")
	if !ok {
		t.Fatal("want route to exist, got not found")
	}
	if route.Upstream != "localhost:8080" {
		t.Errorf("want upstream localhost:8080, got %s", route.Upstream)
	}
}

func TestTable_Update(t *testing.T) {
	tbl := routing.NewTable()
	tbl.Set("example.com", "localhost:8080", false, nil, nil)
	tbl.Set("example.com", "localhost:9090", false, nil, nil)

	route, ok := tbl.Get("example.com")
	if !ok {
		t.Fatal("want route to exist, got not found")
	}
	if route.Upstream != "localhost:9090" {
		t.Errorf("want updated upstream localhost:9090, got %s", route.Upstream)
	}
}

func TestTable_Delete(t *testing.T) {
	tbl := routing.NewTable()
	tbl.Set("example.com", "localhost:8080", false, nil, nil)
	tbl.Delete("example.com")

	if _, ok := tbl.Get("example.com"); ok {
		t.Error("want route deleted, but it still exists")
	}
}

func TestTable_DeleteNonExistent(t *testing.T) {
	tbl := routing.NewTable()
	// Must not panic.
	tbl.Delete("nonexistent.com")
}

func TestTable_UnknownDomain(t *testing.T) {
	tbl := routing.NewTable()

	if _, ok := tbl.Get("unknown.com"); ok {
		t.Error("want not found for unknown domain, got found")
	}
}

func TestTable_StaticRoutePersistsWhenDynamicDeleted(t *testing.T) {
	tbl := routing.NewTable()
	tbl.SetStatic("dashboard.example.com", "localhost:3000")
	tbl.Set("dashboard.example.com", "localhost:8080", false, nil, nil)

	tbl.Delete("dashboard.example.com")

	route, ok := tbl.Get("dashboard.example.com")
	if !ok {
		t.Fatal("want static route to exist after dynamic delete")
	}
	if route.Upstream != "localhost:3000" {
		t.Errorf("want static upstream localhost:3000, got %s", route.Upstream)
	}
}

func TestTable_MultipleDomains(t *testing.T) {
	tbl := routing.NewTable()
	tbl.Set("foo.com", "localhost:3000", false, nil, nil)
	tbl.Set("bar.com", "localhost:4000", false, nil, nil)

	if route, ok := tbl.Get("foo.com"); !ok || route.Upstream != "localhost:3000" {
		t.Errorf("foo.com: want localhost:3000, got %s (ok=%v)", route.Upstream, ok)
	}
	if route, ok := tbl.Get("bar.com"); !ok || route.Upstream != "localhost:4000" {
		t.Errorf("bar.com: want localhost:4000, got %s (ok=%v)", route.Upstream, ok)
	}

	tbl.Delete("foo.com")

	if _, ok := tbl.Get("foo.com"); ok {
		t.Error("want foo.com deleted")
	}
	if route, ok := tbl.Get("bar.com"); !ok || route.Upstream != "localhost:4000" {
		t.Errorf("bar.com must survive after foo.com deleted: got %s (ok=%v)", route.Upstream, ok)
	}
}

func TestTable_SetStoresSecurityConfig(t *testing.T) {
	tbl := routing.NewTable()
	tbl.Set("example.com", "localhost:8080", false, nil, &store.SecurityConfig{WAFEnabled: true})

	route, ok := tbl.Get("example.com")
	if !ok {
		t.Fatal("want route to exist, got not found")
	}
	if route.Security == nil || !route.Security.WAFEnabled {
		t.Fatalf("want security config stored, got %#v", route.Security)
	}
}
