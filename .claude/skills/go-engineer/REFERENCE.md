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
