# purefunccore

[![Go Reference](https://pkg.go.dev/badge/github.com/Pure-Company/purefunccore.svg)](https://pkg.go.dev/github.com/Pure-Company/purefunccore)
[![Go Report Card](https://goreportcard.com/badge/github.com/Pure-Company/purefunccore)](https://goreportcard.com/report/github.com/Pure-Company/purefunccore)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

**Functional bindings for Go's standard library interfaces - Eliminate mocks, embrace functions.**

Purefunccore replaces interface-based designs with functional types, enabling:
- ✅ **Zero mock generation** - Use inline functions instead of mock frameworks
- ✅ **Backwards compatible** - Add methods without breaking existing code
- ✅ **Rich composition** - Monoid operations, map, filter, retry, and more
- ✅ **Better testing** - Swap implementations without complex setup
- ✅ **Simpler code** - Less boilerplate, more clarity

## Installation
```bash
go get github.com/Pure-Company/purefunccore
```

## Quick Start

### Instead of this (traditional approach):
```go
// Define interface
type Database interface {
    Query(string) ([]Row, error)
}

// Create mock struct
type mockDB struct {
    queryFunc func(string) ([]Row, error)
}

func (m *mockDB) Query(sql string) ([]Row, error) {
    return m.queryFunc(sql)
}

// Use in test
db := &mockDB{
    queryFunc: func(sql string) ([]Row, error) {
        return []Row{{ID: 1}}, nil
    },
}
```

### Do this (purefunccore approach):
```go
import "github.com/Pure-Company/purefunccore"

// Just use a function type
type QueryFunc func(string) ([]Row, error)

// Use directly in test - NO MOCK STRUCT!
query := QueryFunc(func(sql string) ([]Row, error) {
    return []Row{{ID: 1}}, nil
})
```

## Examples

### HTTP Handler with Middleware
```go
import pfc "github.com/Pure-Company/purefunccore"

handler := pfc.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
    fmt.Fprintf(w, "Hello, World!")
}).
    WithLogging(logger).
    WithTimeout(5 * time.Second).
    WithAuth(authFunc).
    WithCORS("*").
    Recover()

http.Handle("/", handler)
```

### Testing Without Mocks
```go
// Service with functional dependencies
type UserService struct {
    getUser   func(ctx context.Context, id int) (*User, error)
    sendEmail func(to, subject, body string) error
}

// Production
service := &UserService{
    getUser: func(ctx context.Context, id int) (*User, error) {
        return db.QueryUser(id)
    },
    sendEmail: func(to, subject, body string) error {
        return emailClient.Send(to, subject, body)
    },
}

// Testing - just swap functions!
service := &UserService{
    getUser: func(ctx context.Context, id int) (*User, error) {
        return &User{ID: id, Name: "Test User"}, nil
    },
    sendEmail: func(to, subject, body string) error {
        captured = append(captured, to) // capture for verification
        return nil
    },
}
```

### IO Composition
```go
import pfc "github.com/Pure-Company/purefunccore"

// Compose readers with transformations
reader := pfc.ReadFunc(func(p []byte) (int, error) {
    copy(p, []byte("hello world"))
    return 11, io.EOF
}).
    Map(bytes.ToUpper).
    Filter(func(b byte) bool { return b != ' ' }).
    Take(5)

// Tee writers
writer := pfc.WriteFunc(os.Stdout.Write).
    Tee(
        pfc.WriteFunc(logFile.Write),
        pfc.WriteFunc(metricsWriter.Write),
    )
```

## Core Principles

1. **Functions over interfaces** - Easier to test and compose
2. **Monoid operations** - Every type has `Empty()` and `Compose()`
3. **Rich combinators** - `Map`, `Filter`, `Chain`, `Retry`, etc.
4. **Zero breaking changes** - Add methods without breaking code
5. **Standard library compatible** - Implements all stdlib interfaces

## Available Bindings

- **IO**: `ReadFunc`, `WriteFunc`, `CloseFunc`, `SeekFunc`, `ReadAtFunc`, `WriteAtFunc`
- **HTTP**: `HandlerFunc`, `RoundTripperFunc`
- **Context**: `ContextFunc`
- **Encoding**: `MarshalerFunc`, `UnmarshalerFunc`, `TextMarshalerFunc`
- **Formatting**: `StringerFunc`, `FormatterFunc`, `ScannerFunc`
- **Errors**: `ErrorFunc` with composition and wrapping
- **Filesystem**: `FSFunc`, `FileFunc`, `FileInfoFunc`, `DirEntryFunc`
- **Database**: `DriverFunc`, `ConnFunc`, `StmtFunc`
- **Sorting**: `SortInterface`

## Documentation

Full API documentation is available on [pkg.go.dev](https://pkg.go.dev/github.com/Pure-Company/purefunccore).

Run locally:
```bash
make doc
# Open http://localhost:6060/pkg/github.com/Pure-Company/purefunccore/
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

## Why Purefunccore?

### The Problem with Interfaces

**Adding one method breaks ALL implementations:**
```go
type Reader interface {
    Read(p []byte) (int, error)
    // ❌ Adding this breaks every Reader implementation!
    // Retry(int) Reader
}
```

### The Purefunccore Solution

**Add infinite methods without breaking anything:**
```go
type ReadFunc func(p []byte) (int, error)

// ✅ Add methods freely - existing code still works!
func (f ReadFunc) Retry(n int) ReadFunc { ... }
func (f ReadFunc) Map(fn func([]byte) []byte) ReadFunc { ... }
func (f ReadFunc) Filter(fn func(byte) bool) ReadFunc { ... }
// ... add as many as you want!
```

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## License

MIT License - see [LICENSE](LICENSE) file for details.

## Author

Vinod Halaharvi ([@vinodhalaharvi](https://github.com/vinodhalaharvi))
