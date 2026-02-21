# Go Reference

## Interfaces

Define interfaces at the point of use, not the point of implementation.

```go
// Good: defined in the package that needs it
package handler

type UserStore interface {
    FindByID(ctx context.Context, id string) (*User, error)
}
```

Keep interfaces small. A one-method interface is ideal. If an interface grows beyond 3 methods, consider splitting it.

Prefer accepting interfaces, returning concrete types.

## Error handling

Always wrap errors with context using `fmt.Errorf` and `%w`:

```go
user, err := store.FindByID(ctx, id)
if err != nil {
    return fmt.Errorf("find user %s: %w", id, err)
}
```

Never ignore errors. Never use `_` for an error return unless you have explicitly decided the error cannot affect correctness.

Create sentinel errors with `errors.New` for errors callers need to inspect:

```go
var ErrNotFound = errors.New("not found")

// Caller:
if errors.Is(err, store.ErrNotFound) { ... }
```

## Package structure

- Keep packages small and focused on a single concern
- Avoid `util`, `common`, or `helpers` packages — put code where it is actually used
- Use `internal/` to hide implementation details from external consumers
- Name packages after what they provide, not what they contain (`store` not `database`, `auth` not `authentication`)

## Code style

- Short variable names for short scopes (`i`, `v`, `r`, `w`)
- Descriptive names for package-level declarations and function parameters
- Avoid stuttering: `store.Store` → `store.Client`, `user.UserService` → `user.Service`
- Return early; don't nest the happy path

```go
// Good
if err != nil {
    return err
}
doThing()

// Bad
if err == nil {
    doThing()
}
```

- Prefer explicit over clever — readability beats brevity when they conflict

## Testing

Use table-driven tests for anything with more than one case:

```go
func TestFindByID(t *testing.T) {
    tests := []struct {
        name    string
        id      string
        want    *User
        wantErr error
    }{
        {"found", "abc", &User{ID: "abc"}, nil},
        {"not found", "xyz", nil, ErrNotFound},
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // ...
        })
    }
}
```

Use interfaces to keep unit tests fast and dependency-free — no real databases, HTTP servers, or filesystems.

In-memory test doubles that hold shared state must use a mutex, matching the concurrency contract of the real implementation:

```go
type memStore struct {
    mu          sync.RWMutex
    deployments map[string]Deployment
}

func (m *memStore) List() []Deployment {
    m.mu.RLock()
    defer m.mu.RUnlock()
    ...
}
```

Test all error paths, not just happy paths and not-found. For each handler or operation backed by fallible I/O, add a test that injects a store error and asserts the correct HTTP status or error value:

```go
type errStore struct{}
func (e *errStore) Create(_ Deployment) (Deployment, error) { return Deployment{}, errors.New("disk full") }
func (e *errStore) Delete(_ string) error                   { return errors.New("disk full") }

func TestDeleteDeployment_StoreError(t *testing.T) {
    srv := newTestServer(&errStore{})
    defer srv.Close()
    req, _ := http.NewRequest(http.MethodDelete, srv.URL+"/api/deployments/x", nil)
    resp, _ := http.DefaultClient.Do(req)
    if resp.StatusCode != http.StatusInternalServerError {
        t.Fatalf("want 500, got %d", resp.StatusCode)
    }
}
```

Test that constructors and loaders reject malformed input:

```go
func TestJSONStore_CorruptedFile(t *testing.T) {
    path := filepath.Join(t.TempDir(), "store.json")
    os.WriteFile(path, []byte("not valid json"), 0o644)
    if _, err := store.NewJSONStore(path); err == nil {
        t.Fatal("want error for corrupted file")
    }
}
```

## HTTP handlers

Always cap the request body before decoding to prevent memory exhaustion:

```go
r.Body = http.MaxBytesReader(w, r.Body, 1<<20) // 1 MB
if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
    http.Error(w, "invalid request body", http.StatusBadRequest)
    return
}
```

Validate required fields immediately after decoding — before any side effects:

```go
if body.Name == "" || body.Image == "" {
    http.Error(w, "name and image are required", http.StatusBadRequest)
    return
}
```

When writing a response, you cannot return an error after `WriteHeader` has been called. Log it instead:

```go
func writeJSON(w http.ResponseWriter, status int, v any) {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(status)
    if err := json.NewEncoder(w).Encode(v); err != nil {
        log.Printf("writeJSON: encode: %v", err)
    }
}
```

## File I/O

Atomic write pattern — write to a temp file, sync, close, then rename. The rename is atomic on POSIX; the sync ensures data is on disk before the rename so a crash between the two cannot leave you with a zero-length file:

```go
tmp := path + ".tmp"
f, err := os.Create(tmp)
if err != nil { ... }

if err := json.NewEncoder(f).Encode(data); err != nil {
    f.Close(); os.Remove(tmp)
    return fmt.Errorf("encode: %w", err)
}
if err := f.Sync(); err != nil {        // flush before rename
    f.Close(); os.Remove(tmp)
    return fmt.Errorf("sync: %w", err)
}
if err := f.Close(); err != nil {
    os.Remove(tmp)
    return fmt.Errorf("close: %w", err)
}
if err := os.Rename(tmp, path); err != nil {
    os.Remove(tmp)
    return fmt.Errorf("rename: %w", err)
}
```

Validate constructor arguments at the boundary — fail fast rather than producing confusing errors deep inside:

```go
func NewJSONStore(path string) (*JSONStore, error) {
    if path == "" {
        return nil, fmt.Errorf("store: path must be non-empty")
    }
    if !filepath.IsAbs(path) {
        return nil, fmt.Errorf("store: path must be absolute: %s", path)
    }
    ...
}
```

## ID generation

When generating random UUIDs without a library, set the version and variant bits to produce a valid UUID v4:

```go
func newID() (string, error) {
    b := make([]byte, 16)
    if _, err := rand.Read(b); err != nil {
        return "", fmt.Errorf("newID: %w", err)
    }
    b[6] = (b[6] & 0x0f) | 0x40 // version 4
    b[8] = (b[8] & 0x3f) | 0x80 // variant bits
    return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:]), nil
}
```

## Stdlib over third-party

Before adding a dependency, check if stdlib covers it:

| Need | Stdlib |
|------|--------|
| HTTP client/server | `net/http` |
| JSON | `encoding/json` |
| Templating | `text/template`, `html/template` |
| Scheduling | `time.Ticker`, `time.AfterFunc` |
| Concurrency | `sync`, `sync/atomic`, `context` |
| CLI flags | `flag` |

Add a dependency only when stdlib is genuinely insufficient and the package is well-maintained with a stable API.

## Concurrency

Every goroutine needs a clear owner, a clear lifetime, and a way to be stopped:

```go
ctx, cancel := context.WithCancel(ctx)
defer cancel()

go func() {
    select {
    case <-ctx.Done():
        return
    case work := <-jobs:
        process(work)
    }
}()
```

Never start a goroutine you cannot stop. Pass `context.Context` as the first argument to any function that blocks or does I/O.

## Antipatterns to avoid

- `interface{}` / `any` when a concrete type works
- Goroutines without a clear lifecycle and cancellation path
- Panicking instead of returning errors (outside `main` and `init`)
- `init()` functions with side effects or I/O
- Named return values except for deferred cleanup
- Unexported fields on structs returned across package boundaries
- Deeply nested logic — extract named functions instead
