package routing_test

import (
	"testing"

	"github.com/ercadev/dirigent/proxy/internal/routing"
)

func TestTable_SetAndGet(t *testing.T) {
	tbl := routing.NewTable()
	tbl.Set("example.com", "localhost:8080")

	upstream, ok := tbl.Get("example.com")
	if !ok {
		t.Fatal("want route to exist, got not found")
	}
	if upstream != "localhost:8080" {
		t.Errorf("want upstream localhost:8080, got %s", upstream)
	}
}

func TestTable_Update(t *testing.T) {
	tbl := routing.NewTable()
	tbl.Set("example.com", "localhost:8080")
	tbl.Set("example.com", "localhost:9090")

	upstream, ok := tbl.Get("example.com")
	if !ok {
		t.Fatal("want route to exist, got not found")
	}
	if upstream != "localhost:9090" {
		t.Errorf("want updated upstream localhost:9090, got %s", upstream)
	}
}

func TestTable_Delete(t *testing.T) {
	tbl := routing.NewTable()
	tbl.Set("example.com", "localhost:8080")
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

func TestTable_MultipleDomains(t *testing.T) {
	tbl := routing.NewTable()
	tbl.Set("foo.com", "localhost:3000")
	tbl.Set("bar.com", "localhost:4000")

	if u, ok := tbl.Get("foo.com"); !ok || u != "localhost:3000" {
		t.Errorf("foo.com: want localhost:3000, got %s (ok=%v)", u, ok)
	}
	if u, ok := tbl.Get("bar.com"); !ok || u != "localhost:4000" {
		t.Errorf("bar.com: want localhost:4000, got %s (ok=%v)", u, ok)
	}

	tbl.Delete("foo.com")

	if _, ok := tbl.Get("foo.com"); ok {
		t.Error("want foo.com deleted")
	}
	if u, ok := tbl.Get("bar.com"); !ok || u != "localhost:4000" {
		t.Errorf("bar.com must survive after foo.com deleted: got %s (ok=%v)", u, ok)
	}
}
