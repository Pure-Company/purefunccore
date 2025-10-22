# purefunccore

[![Go Reference](https://pkg.go.dev/badge/github.com/Pure-Company/purefunccore.svg)](https://pkg.go.dev/github.com/Pure-Company/purefunccore)
[![Go Report Card](https://goreportcard.com/badge/github.com/Pure-Company/purefunccore)](https://goreportcard.com/report/github.com/Pure-Company/purefunccore)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

**Functional bindings for Go's standard library - Eliminate mocks, embrace composition.**

Purefunccore transforms Go's standard library interfaces into composable functional types. Write cleaner code, eliminate mock frameworks, and compose behavior like LEGO blocks.

## The Problem

Traditional Go middleware is nested and hard to read:

```go
handler := recoverMiddleware(
    corsMiddleware(
        timeoutMiddleware(
            authMiddleware(
                loggingMiddleware(
                    http.HandlerFunc(myHandler),
                ),
            ),
        ),
    ),
)
```

Testing requires mock frameworks and boilerplate:

```go
type Database interface {
    Query(string) ([]Row, error)
}

// Need mock generation, mock setup, mock verification...
```

## The Solution

### Clean Middleware Composition

```go
import pfc "github.com/Pure-Company/purefunccore"

handler := pfc.HandlerFunc(myHandler).
    WithLogging(log.Println).
    WithAuth(authenticate).
    WithTimeout(5 * time.Second).
    WithCORS("*").
    Recover()
```

### Zero-Mock Testing

```go
// Production
query := QueryFunc(func(sql string) ([]Row, error) {
    return db.Query(sql)
})

// Testing - NO MOCKS NEEDED!
query := QueryFunc(func(sql string) ([]Row, error) {
    return []Row{{ID: 1}}, nil
})
```

## Installation

```bash
go get github.com/Pure-Company/purefunccore
```

## Quick Start: HTTP API with Different Security Levels

```go
package main

import (
    "fmt"
    "log"
    "net/http"
    "time"
    
    pfc "github.com/Pure-Company/purefunccore"
)

func main() {
    // Public endpoint - just logging
    http.Handle("/health", 
        pfc.HandlerFunc(handleHealth).
            WithLogging(log.Println))

    // User endpoint - auth + timeout
    http.Handle("/api/users",
        pfc.HandlerFunc(handleUsers).
            WithLogging(log.Println).
            WithAuth(isAuthenticated).
            WithTimeout(10 * time.Second).
            WithCORS("*"))

    // Admin endpoint - stricter security
    http.Handle("/admin",
        pfc.HandlerFunc(handleAdmin).
            WithLogging(log.Println).
            WithAuth(isAdmin).
            WithTimeout(2 * time.Second).
            WithCORS("https://admin.example.com"))

    log.Fatal(http.ListenAndServe(":8080", nil))
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
    fmt.Fprintln(w, "OK")
}

func handleUsers(w http.ResponseWriter, r *http.Request) {
    fmt.Fprintln(w, "User data")
}

func handleAdmin(w http.ResponseWriter, r *http.Request) {
    fmt.Fprintln(w, "Admin panel")
}

func isAuthenticated(r *http.Request) bool {
    return r.Header.Get("Authorization") != ""
}

func isAdmin(r *http.Request) bool {
    return r.Header.Get("Authorization") == "Bearer admin-token"
}
```

**Each route gets exactly the middleware it needs!** 🎯

## Features

### 🔗 Compose Like Functions

```go
// Create reusable middleware chains
standard := func(h pfc.HandlerFunc) pfc.HandlerFunc {
    return h.WithLogging(log.Println).WithCORS("*").Recover()
}

secure := func(h pfc.HandlerFunc) pfc.HandlerFunc {
    return standard(h).WithAuth(auth).WithTimeout(10 * time.Second)
}

// Apply to routes
http.Handle("/public", standard(pfc.HandlerFunc(handlePublic)))
http.Handle("/secure", secure(pfc.HandlerFunc(handleSecure)))
```

### 📖 Reader/Writer Composition

```go
// Transform data as you read
reader := pfc.ReadFunc(file.Read).
    Map(bytes.ToUpper).
    Filter(isAlphanumeric).
    Take(1000)

// Write to multiple destinations (tee)
writer := pfc.WriteFunc(file.Write).
    Tee(pfc.WriteFunc(logFile.Write)).
    Tee(pfc.WriteFunc(metricsWriter.Write))
```

### ✅ Testing Without Mocks

```go
func TestOrderService(t *testing.T) {
    var emailsSent []string
    
    service := &OrderService{
        // Real database
        saveOrder: db.SaveOrder,
        
        // Mock email - just a function!
        sendEmail: func(to, subject, body string) error {
            emailsSent = append(emailsSent, to)
            return nil
        },
    }
    
    service.PlaceOrder(ctx, order)
    
    if len(emailsSent) != 1 {
        t.Error("expected 1 email")
    }
}
```

### 🔄 Monoid Operations

```go
// Compose readers sequentially
combined := reader1.Compose(reader2).Compose(reader3)

// Broadcast writes
broadcast := writer1.Tee(writer2, writer3)

// Empty identity
empty := reader.Empty() // Returns EOF immediately
```

### 🎨 Functor Transformations

```go
// Map over data
uppercased := reader.Map(bytes.ToUpper)
timestamped := writer.Map(addTimestamp)

// Chain transformations
processed := reader.
    Map(bytes.TrimSpace).
    Map(bytes.ToUpper).
    Map(addPrefix)
```

## Available Types

- **IO**: `ReadFunc`, `WriteFunc`, `CloseFunc`, `SeekFunc`, `ReadAtFunc`, `WriteAtFunc`
- **HTTP**: `HandlerFunc`, `RoundTripperFunc`
- **Context**: `ContextFunc`
- **Errors**: `ErrorFunc` with composition and wrapping
- **Formatting**: `StringerFunc`, `FormatterFunc`, `ScannerFunc`
- **Encoding**: `MarshalerFunc`, `UnmarshalerFunc`, `TextMarshalerFunc`
- **Database**: `DriverFunc`, `ConnFunc`, `StmtFunc`
- **Filesystem**: `FSFunc`, `FileFunc`, `FileInfoFunc`, `DirEntryFunc`
- **Sorting**: `SortInterface`

## Why Purefunccore?

✅ **No mock frameworks** - Just swap functions  
✅ **No interfaces** - Functions are enough  
✅ **No boilerplate** - Compose, don't nest  
✅ **Type safe** - Compiler catches errors  
✅ **Zero dependencies** - Pure stdlib  
✅ **Backwards compatible** - It's just `http.Handler`  
✅ **Testable** - Each piece in isolation

### The Expression Problem Solved

**Traditional interfaces break when you add methods:**

```go
type Reader interface {
    Read(p []byte) (int, error)
    // ❌ Adding this breaks ALL implementations!
    // Retry(int) Reader
}
```

**Purefunccore lets you add infinite methods:**

```go
type ReadFunc func(p []byte) (int, error)

// ✅ Add methods freely - existing code still works!
func (f ReadFunc) Retry(n int) ReadFunc { ... }
func (f ReadFunc) Map(fn func([]byte) []byte) ReadFunc { ... }
func (f ReadFunc) Filter(fn func(byte) bool) ReadFunc { ... }
// ... add as many as you want!
```

## Philosophy

> "It's the same old `http.Handler`, but look at the composition now!"

Purefunccore doesn't replace Go's standard library - it enhances it with functional composition patterns. You still use `http.Handler`, `io.Reader`, and all the interfaces you know. But now they're **composable**.

## Real-World Example

```go
// Different security levels per route
func setupRoutes() {
    // Health check - public
    http.Handle("/health", public(handleHealth))
    
    // User API - authenticated
    http.Handle("/api/users", authenticated(handleUsers))
    
    // Admin - authenticated + admin role
    http.Handle("/admin", admin(handleAdmin))
}

func public(h pfc.HandlerFunc) http.Handler {
    return h.WithLogging(log.Println).WithCORS("*")
}

func authenticated(h pfc.HandlerFunc) http.Handler {
    return public(h).
        WithAuth(checkAuth).
        WithTimeout(10 * time.Second)
}

func admin(h pfc.HandlerFunc) http.Handler {
    return authenticated(h).
        WithAuth(checkAdmin) // Extra security layer
}
```

## Development

```bash
# Run tests
make test

# Run tests with coverage
make test-coverage

# Run all checks (fmt, vet, lint, test)
make check

# Run benchmarks
make bench

# View all commands
make help
```

## Documentation

Full documentation available at [pkg.go.dev](https://pkg.go.dev/github.com/Pure-Company/purefunccore)

## Examples

See [examples_test.go](examples_test.go) for comprehensive examples including:
- HTTP middleware composition
- Reader/Writer transformations
- Error handling patterns
- Testing without mocks
- Table-driven tests
- Real-world API servers

## Core Principles

1. **Functions over interfaces** - Easier to test and compose
2. **Monoid operations** - Every type has `Empty()` and `Compose()`
3. **Rich combinators** - `Map`, `Filter`, `Chain`, `Retry`, etc.
4. **Zero breaking changes** - Add methods without breaking code
5. **Standard library compatible** - Implements all stdlib interfaces

## License

MIT License - see [LICENSE](LICENSE) file for details.

For a more entertaining version, check out [LICENSE.creative](LICENSE.creative) 😄

## Author

Vinod Halaharvi ([@vinodhalaharvi](https://github.com/vinodhalaharvi))

## Contributing

Contributions welcome! Please open an issue or PR.

---

**Star ⭐ this repo if you find it useful!**