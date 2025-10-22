// Package purefunccore provides functional bindings for Go's standard library interfaces.
//
// This package eliminates the need for mock generation in tests by replacing
// interface-based designs with functional types. Each functional type implements
// its corresponding standard library interface while providing additional methods
// for composition, transformation, and testing.
//
// # Key Benefits
//
// - Zero mock generation: Use inline functions instead of creating mock structs
// - Backwards compatible: Add methods without breaking existing code
// - Rich composition: Monoid operations, map, filter, and more
// - Better testing: Swap implementations without complex mock frameworks
// - Simpler code: Less boilerplate, more clarity
//
// # Quick Start
//
// Instead of defining interfaces and creating mocks:
//
//	type Database interface {
//	    Query(string) ([]Row, error)
//	}
//
// Use functional types directly:
//
//	type QueryFunc func(string) ([]Row, error)
//
//	// Production
//	query := QueryFunc(func(sql string) ([]Row, error) {
//	    return db.Query(sql)
//	})
//
//	// Testing - no mocks needed!
//	query := QueryFunc(func(sql string) ([]Row, error) {
//	    return []Row{{ID: 1}}, nil
//	})
//
// # Core Principles
//
// Purefunccore follows these principles:
//
// 1. Functions over interfaces - easier to test and compose
// 2. Monoid operations - Empty() and Compose() for all types
// 3. Rich combinators - Map, Filter, Chain, Retry, etc.
// 4. Zero breaking changes - add methods without breaking code
// 5. Standard library compatible - implements all stdlib interfaces
//
// # Example: HTTP Handler
//
//	handler := HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
//	    fmt.Fprintf(w, "Hello!")
//	}).
//	    WithLogging(logger).
//	    WithTimeout(5 * time.Second).
//	    WithAuth(authFunc).
//	    Recover()
//
// # Example: Testing Without Mocks
//
//	// Service using functional dependencies
//	type UserService struct {
//	    getUser func(ctx context.Context, id int) (*User, error)
//	    sendEmail func(to, subject, body string) error
//	}
//
//	// Production
//	service := &UserService{
//	    getUser: func(ctx context.Context, id int) (*User, error) {
//	        return db.QueryUser(id)
//	    },
//	    sendEmail: func(to, subject, body string) error {
//	        return emailClient.Send(to, subject, body)
//	    },
//	}
//
//	// Testing - just swap functions!
//	service := &UserService{
//	    getUser: func(ctx context.Context, id int) (*User, error) {
//	        return &User{ID: id, Name: "Test"}, nil
//	    },
//	    sendEmail: func(to, subject, body string) error {
//	        captured = append(captured, to)
//	        return nil
//	    },
//	}
package purefunccore

import (
	"context"
	"database/sql/driver"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// ============================================================================
// IO Package Bindings
// ============================================================================

// ReadFunc is a functional binding for io.Reader.
// It implements io.Reader and provides rich composition methods.
//
// Example:
//
//	reader := ReadFunc(func(p []byte) (int, error) {
//	    copy(p, []byte("hello"))
//	    return 5, io.EOF
//	})
//
//	// Compose with transformations
//	reader = reader.Map(strings.ToUpper).Take(10)
type ReadFunc func(p []byte) (n int, err error)

// Read implements io.Reader.
func (f ReadFunc) Read(p []byte) (int, error) {
	return f(p)
}

// Empty returns a reader that immediately returns EOF (Monoid identity).
func (f ReadFunc) Empty() ReadFunc {
	return func(p []byte) (int, error) {
		return 0, io.EOF
	}
}

// Compose creates a reader that reads from this reader, then the next (Monoid operation).
func (f ReadFunc) Compose(next ReadFunc) ReadFunc {
	exhausted := false
	return func(p []byte) (int, error) {
		if !exhausted {
			n, err := f(p)
			if err == io.EOF {
				exhausted = true
				return next(p)
			}
			return n, err
		}
		return next(p)
	}
}

// Map transforms bytes as they're read.
func (f ReadFunc) Map(transform func([]byte) []byte) ReadFunc {
	return func(p []byte) (int, error) {
		n, err := f(p)
		if n > 0 {
			transformed := transform(p[:n])
			copy(p, transformed)
		}
		return n, err
	}
}

// Filter keeps only bytes matching the predicate.
func (f ReadFunc) Filter(predicate func(byte) bool) ReadFunc {
	return func(p []byte) (int, error) {
		n, err := f(p)
		if n > 0 {
			j := 0
			for i := 0; i < n; i++ {
				if predicate(p[i]) {
					p[j] = p[i]
					j++
				}
			}
			n = j
		}
		return n, err
	}
}

// Take limits reading to n bytes total.
func (f ReadFunc) Take(limit int64) ReadFunc {
	remaining := limit
	return func(p []byte) (int, error) {
		if remaining <= 0 {
			return 0, io.EOF
		}
		if int64(len(p)) > remaining {
			p = p[:remaining]
		}
		n, err := f(p)
		remaining -= int64(n)
		return n, err
	}
}

// Retry retries on error up to maxRetries times.
func (f ReadFunc) Retry(maxRetries int) ReadFunc {
	return func(p []byte) (int, error) {
		var lastErr error
		for i := 0; i <= maxRetries; i++ {
			n, err := f(p)
			if err == nil || err == io.EOF {
				return n, err
			}
			lastErr = err
		}
		return 0, lastErr
	}
}

// WithTimeout adds a timeout to read operations.
func (f ReadFunc) WithTimeout(timeout time.Duration) ReadFunc {
	return func(p []byte) (int, error) {
		type result struct {
			n   int
			err error
		}
		ch := make(chan result, 1)
		go func() {
			n, err := f(p)
			ch <- result{n, err}
		}()
		select {
		case r := <-ch:
			return r.n, r.err
		case <-time.After(timeout):
			return 0, errors.New("read timeout")
		}
	}
}

// Tap allows side effects without modifying the stream.
func (f ReadFunc) Tap(fn func([]byte, int, error)) ReadFunc {
	return func(p []byte) (int, error) {
		n, err := f(p)
		fn(p[:n], n, err)
		return n, err
	}
}

// WriteFunc is a functional binding for io.Writer.
// It implements io.Writer and provides composition methods.
//
// Example:
//
//	writer := WriteFunc(func(p []byte) (int, error) {
//	    fmt.Print(string(p))
//	    return len(p), nil
//	})
//
//	// Compose multiple writers
//	writer = writer.Tee(logWriter, metricsWriter)
type WriteFunc func(p []byte) (n int, err error)

// Write implements io.Writer.
func (f WriteFunc) Write(p []byte) (int, error) {
	return f(p)
}

// Empty returns a writer that discards all writes (Monoid identity).
func (f WriteFunc) Empty() WriteFunc {
	return func(p []byte) (int, error) {
		return len(p), nil
	}
}

// Compose creates a writer that writes to both writers (Monoid operation).
func (f WriteFunc) Compose(other WriteFunc) WriteFunc {
	return func(p []byte) (int, error) {
		n1, err1 := f(p)
		n2, err2 := other(p)
		if err1 != nil {
			return n1, err1
		}
		if err2 != nil {
			return n2, err2
		}
		if n1 != len(p) {
			return n1, io.ErrShortWrite
		}
		return n2, nil
	}
}

// Tee writes to multiple writers.
func (f WriteFunc) Tee(others ...WriteFunc) WriteFunc {
	all := append([]WriteFunc{f}, others...)
	return func(p []byte) (int, error) {
		for _, w := range all {
			n, err := w(p)
			if err != nil {
				return n, err
			}
			if n != len(p) {
				return n, io.ErrShortWrite
			}
		}
		return len(p), nil
	}
}

// Map transforms bytes before writing.
func (f WriteFunc) Map(transform func([]byte) []byte) WriteFunc {
	return func(p []byte) (int, error) {
		transformed := transform(p)
		n, err := f(transformed)
		if err != nil {
			return n, err
		}
		return len(p), nil
	}
}

// Filter only writes bytes matching the predicate.
func (f WriteFunc) Filter(predicate func(byte) bool) WriteFunc {
	return func(p []byte) (int, error) {
		filtered := make([]byte, 0, len(p))
		for _, b := range p {
			if predicate(b) {
				filtered = append(filtered, b)
			}
		}
		n, err := f(filtered)
		if err != nil {
			return n, err
		}
		return len(p), nil
	}
}

// CloseFunc is a functional binding for io.Closer.
type CloseFunc func() error

// Close implements io.Closer.
func (f CloseFunc) Close() error {
	return f()
}

// SeekFunc is a functional binding for io.Seeker.
type SeekFunc func(offset int64, whence int) (int64, error)

// Seek implements io.Seeker.
func (f SeekFunc) Seek(offset int64, whence int) (int64, error) {
	return f(offset, whence)
}

// ReadAtFunc is a functional binding for io.ReaderAt.
type ReadAtFunc func(p []byte, off int64) (n int, err error)

// ReadAt implements io.ReaderAt.
func (f ReadAtFunc) ReadAt(p []byte, off int64) (int, error) {
	return f(p, off)
}

// WriteAtFunc is a functional binding for io.WriterAt.
type WriteAtFunc func(p []byte, off int64) (n int, err error)

// WriteAt implements io.WriterAt.
func (f WriteAtFunc) WriteAt(p []byte, off int64) (int, error) {
	return f(p, off)
}

// ReadWriteCloser combines Reader, Writer, and Closer.
type ReadWriteCloser struct {
	ReadFunc  ReadFunc
	WriteFunc WriteFunc
	CloseFunc CloseFunc
}

// Read implements io.Reader.
func (rwc ReadWriteCloser) Read(p []byte) (int, error) {
	return rwc.ReadFunc(p)
}

// Write implements io.Writer.
func (rwc ReadWriteCloser) Write(p []byte) (int, error) {
	return rwc.WriteFunc(p)
}

// Close implements io.Closer.
func (rwc ReadWriteCloser) Close() error {
	return rwc.CloseFunc()
}

// ============================================================================
// HTTP Package Bindings
// ============================================================================

// HandlerFunc is a functional binding for http.Handler.
// It provides middleware composition and decorators.
//
// Example:
//
//	handler := HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
//	    fmt.Fprintf(w, "Hello!")
//	}).
//	    WithLogging(logger).
//	    WithTimeout(5 * time.Second).
//	    WithAuth(authFunc)
type HandlerFunc func(w http.ResponseWriter, r *http.Request)

// ServeHTTP implements http.Handler.
func (f HandlerFunc) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f(w, r)
}

// Empty returns a handler that does nothing (identity).
func (f HandlerFunc) Empty() HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {}
}

// Compose chains handlers (useful for middleware).
func (f HandlerFunc) Compose(next HandlerFunc) HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		f(w, r)
		next(w, r)
	}
}

// Before runs a function before the handler.
func (f HandlerFunc) Before(before func(http.ResponseWriter, *http.Request)) HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		before(w, r)
		f(w, r)
	}
}

// After runs a function after the handler.
func (f HandlerFunc) After(after func(http.ResponseWriter, *http.Request)) HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		f(w, r)
		after(w, r)
	}
}

// WithLogging adds logging to the handler.
func (f HandlerFunc) WithLogging(logger func(string)) HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Use full URL with query params
		fullPath := r.URL.Path
		if r.URL.RawQuery != "" {
			fullPath = r.URL.Path + "?" + r.URL.RawQuery
		}
		logger(fmt.Sprintf("Request: %s %s", r.Method, fullPath))
		f(w, r)
		logger(fmt.Sprintf("Completed: %s %s", r.Method, fullPath))
	}
}

// timeoutWriter wraps http.ResponseWriter to prevent concurrent writes
type timeoutWriter struct {
	http.ResponseWriter
	mu          sync.Mutex
	timedOut    int32 // atomic
	wroteHeader bool
}

func (tw *timeoutWriter) Write(b []byte) (int, error) {
	if atomic.LoadInt32(&tw.timedOut) == 1 {
		return 0, http.ErrHandlerTimeout
	}
	tw.mu.Lock()
	defer tw.mu.Unlock()
	return tw.ResponseWriter.Write(b)
}

func (tw *timeoutWriter) WriteHeader(code int) {
	if atomic.LoadInt32(&tw.timedOut) == 1 {
		return
	}
	tw.mu.Lock()
	defer tw.mu.Unlock()
	if !tw.wroteHeader {
		tw.wroteHeader = true
		tw.ResponseWriter.WriteHeader(code)
	}
}

// WithTimeout adds a timeout to the handler.
func (f HandlerFunc) WithTimeout(timeout time.Duration) HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), timeout)
		defer cancel()

		tw := &timeoutWriter{ResponseWriter: w}
		done := make(chan struct{})

		go func() {
			defer close(done)
			f(tw, r.WithContext(ctx))
		}()

		select {
		case <-done:
			// Handler completed successfully
			return
		case <-ctx.Done():
			// Set timeout flag to prevent further writes
			atomic.StoreInt32(&tw.timedOut, 1)

			// Wait for handler goroutine to finish
			<-done

			// Write timeout error if handler hasn't written
			tw.mu.Lock()
			if !tw.wroteHeader {
				w.WriteHeader(http.StatusRequestTimeout)
				_, _ = w.Write([]byte("Request timeout\n"))
			}
			tw.mu.Unlock()
		}
	}
}

// WithAuth adds authentication to the handler.
func (f HandlerFunc) WithAuth(authenticate func(*http.Request) bool) HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !authenticate(r) {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		f(w, r)
	}
}

// WithCORS adds CORS headers to the handler.
func (f HandlerFunc) WithCORS(origin string) HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		f(w, r)
	}
}

// Recover adds panic recovery to the handler.
func (f HandlerFunc) Recover() HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				http.Error(w, fmt.Sprintf("Internal Server Error: %v", err), http.StatusInternalServerError)
			}
		}()
		f(w, r)
	}
}

// RoundTripperFunc is a functional binding for http.RoundTripper.
type RoundTripperFunc func(req *http.Request) (*http.Response, error)

// RoundTrip implements http.RoundTripper.
func (f RoundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

// ============================================================================
// Context Package Bindings
// ============================================================================

// ContextFunc creates a custom context implementation.
type ContextFunc struct {
	DeadlineFunc func() (deadline time.Time, ok bool)
	DoneFunc     func() <-chan struct{}
	ErrFunc      func() error
	ValueFunc    func(key any) any
}

// Deadline implements context.Context.
func (f ContextFunc) Deadline() (time.Time, bool) {
	return f.DeadlineFunc()
}

// Done implements context.Context.
func (f ContextFunc) Done() <-chan struct{} {
	return f.DoneFunc()
}

// Err implements context.Context.
func (f ContextFunc) Err() error {
	return f.ErrFunc()
}

// Value implements context.Context.
func (f ContextFunc) Value(key any) any {
	return f.ValueFunc(key)
}

// ============================================================================
// Sort Package Bindings
// ============================================================================

// SortInterface is a functional binding for sort.Interface.
type SortInterface struct {
	LenFunc  func() int
	LessFunc func(i, j int) bool
	SwapFunc func(i, j int)
}

// Len implements sort.Interface.
func (s SortInterface) Len() int {
	return s.LenFunc()
}

// Less implements sort.Interface.
func (s SortInterface) Less(i, j int) bool {
	return s.LessFunc(i, j)
}

// Swap implements sort.Interface.
func (s SortInterface) Swap(i, j int) {
	s.SwapFunc(i, j)
}

// ============================================================================
// Encoding Package Bindings
// ============================================================================

// MarshalerFunc is a functional binding for json.Marshaler.
type MarshalerFunc func() ([]byte, error)

// MarshalJSON implements json.Marshaler.
func (f MarshalerFunc) MarshalJSON() ([]byte, error) {
	return f()
}

// UnmarshalerFunc is a functional binding for json.Unmarshaler.
type UnmarshalerFunc func(data []byte) error

// UnmarshalJSON implements json.Unmarshaler.
func (f UnmarshalerFunc) UnmarshalJSON(data []byte) error {
	return f(data)
}

// TextMarshalerFunc is a functional binding for encoding.TextMarshaler.
type TextMarshalerFunc func() (text []byte, err error)

// MarshalText implements encoding.TextMarshaler.
func (f TextMarshalerFunc) MarshalText() ([]byte, error) {
	return f()
}

// TextUnmarshalerFunc is a functional binding for encoding.TextUnmarshaler.
type TextUnmarshalerFunc func(text []byte) error

// UnmarshalText implements encoding.TextUnmarshaler.
func (f TextUnmarshalerFunc) UnmarshalText(text []byte) error {
	return f(text)
}

// ============================================================================
// fmt Package Bindings
// ============================================================================

// StringerFunc is a functional binding for fmt.Stringer.
// It provides monoid operations for string composition.
//
// Example:
//
//	stringer := StringerFunc(func() string {
//	    return "Hello"
//	}).WithSuffix(", World!")
type StringerFunc func() string

// String implements fmt.Stringer.
func (f StringerFunc) String() string {
	return f()
}

// Empty returns empty string (Monoid identity).
func (f StringerFunc) Empty() StringerFunc {
	return func() string { return "" }
}

// Compose concatenates strings (Monoid operation).
func (f StringerFunc) Compose(other StringerFunc) StringerFunc {
	return func() string {
		return f() + other()
	}
}

// Join concatenates with separator.
func (f StringerFunc) Join(sep string, others ...StringerFunc) StringerFunc {
	all := append([]StringerFunc{f}, others...)
	return func() string {
		parts := make([]string, len(all))
		for i, fn := range all {
			parts[i] = fn()
		}
		return strings.Join(parts, sep)
	}
}

// Map transforms the string.
func (f StringerFunc) Map(transform func(string) string) StringerFunc {
	return func() string {
		return transform(f())
	}
}

// WithPrefix adds a prefix.
func (f StringerFunc) WithPrefix(prefix string) StringerFunc {
	return func() string {
		return prefix + f()
	}
}

// WithSuffix adds a suffix.
func (f StringerFunc) WithSuffix(suffix string) StringerFunc {
	return func() string {
		return f() + suffix
	}
}

// FormatterFunc is a functional binding for fmt.Formatter.
type FormatterFunc func(f fmt.State, verb rune)

// Format implements fmt.Formatter.
func (fn FormatterFunc) Format(f fmt.State, verb rune) {
	fn(f, verb)
}

// ScannerFunc is a functional binding for fmt.Scanner.
type ScannerFunc func(state fmt.ScanState, verb rune) error

// Scan implements fmt.Scanner.
func (f ScannerFunc) Scan(state fmt.ScanState, verb rune) error {
	return f(state, verb)
}

// ============================================================================
// Error Handling Bindings
// ============================================================================

// ErrorFunc is a functional binding for the error interface.
// It provides composition and wrapping methods.
//
// Example:
//
//	err := ErrorFunc(func() string {
//	    return "connection failed"
//	}).Wrap("database").WithCode(500)
type ErrorFunc func() string

// Error implements the error interface.
func (f ErrorFunc) Error() string {
	return f()
}

// Empty returns nil (Monoid identity).
func (f ErrorFunc) Empty() error {
	return nil
}

// Compose combines errors with a separator.
func (f ErrorFunc) Compose(other ErrorFunc) ErrorFunc {
	return func() string {
		return f() + "; " + other()
	}
}

// Wrap wraps the error with context.
func (f ErrorFunc) Wrap(context string) ErrorFunc {
	return func() string {
		return context + ": " + f()
	}
}

// WithCode adds an error code.
func (f ErrorFunc) WithCode(code int) *CodedError {
	return &CodedError{
		msg:  f(),
		code: code,
	}
}

// CodedError is an error with an associated code.
type CodedError struct {
	msg  string
	code int
}

func (e *CodedError) Error() string {
	return fmt.Sprintf("[%d] %s", e.code, e.msg)
}

// Code returns the error code.
func (e *CodedError) Code() int {
	return e.code
}

// ============================================================================
// Filesystem Package Bindings (io/fs)
// ============================================================================

// FSFunc is a functional binding for fs.FS.
type FSFunc func(name string) (fs.File, error)

// Open implements fs.FS.
func (f FSFunc) Open(name string) (fs.File, error) {
	return f(name)
}

// FileFunc is a functional binding for fs.File.
type FileFunc struct {
	StatFunc  func() (fs.FileInfo, error)
	ReadFunc  ReadFunc
	CloseFunc CloseFunc
}

// Stat implements fs.File.
func (f FileFunc) Stat() (fs.FileInfo, error) {
	return f.StatFunc()
}

// Read implements fs.File.
func (f FileFunc) Read(p []byte) (int, error) {
	return f.ReadFunc(p)
}

// Close implements fs.File.
func (f FileFunc) Close() error {
	return f.CloseFunc()
}

// FileInfoFunc is a functional binding for fs.FileInfo.
type FileInfoFunc struct {
	NameFunc    func() string
	SizeFunc    func() int64
	ModeFunc    func() fs.FileMode
	ModTimeFunc func() time.Time
	IsDirFunc   func() bool
	SysFunc     func() any
}

// Name implements fs.FileInfo.
func (f FileInfoFunc) Name() string {
	return f.NameFunc()
}

// Size implements fs.FileInfo.
func (f FileInfoFunc) Size() int64 {
	return f.SizeFunc()
}

// Mode implements fs.FileInfo.
func (f FileInfoFunc) Mode() fs.FileMode {
	return f.ModeFunc()
}

// ModTime implements fs.FileInfo.
func (f FileInfoFunc) ModTime() time.Time {
	return f.ModTimeFunc()
}

// IsDir implements fs.FileInfo.
func (f FileInfoFunc) IsDir() bool {
	return f.IsDirFunc()
}

// Sys implements fs.FileInfo.
func (f FileInfoFunc) Sys() any {
	return f.SysFunc()
}

// DirEntryFunc is a functional binding for fs.DirEntry.
type DirEntryFunc struct {
	NameFunc  func() string
	IsDirFunc func() bool
	TypeFunc  func() fs.FileMode
	InfoFunc  func() (fs.FileInfo, error)
}

// Name implements fs.DirEntry.
func (f DirEntryFunc) Name() string {
	return f.NameFunc()
}

// IsDir implements fs.DirEntry.
func (f DirEntryFunc) IsDir() bool {
	return f.IsDirFunc()
}

// Type implements fs.DirEntry.
func (f DirEntryFunc) Type() fs.FileMode {
	return f.TypeFunc()
}

// Info implements fs.DirEntry.
func (f DirEntryFunc) Info() (fs.FileInfo, error) {
	return f.InfoFunc()
}

// ============================================================================
// Database SQL Bindings
// ============================================================================

// DriverFunc is a functional binding for sql/driver.Driver.
type DriverFunc func(name string) (driver.Conn, error)

// Open implements sql/driver.Driver.
func (f DriverFunc) Open(name string) (driver.Conn, error) {
	return f(name)
}

// ConnFunc is a functional binding for sql/driver.Conn.
type ConnFunc struct {
	PrepareFunc func(query string) (driver.Stmt, error)
	CloseFunc   func() error
	BeginFunc   func() (driver.Tx, error)
}

// Prepare implements sql/driver.Conn.
func (f ConnFunc) Prepare(query string) (driver.Stmt, error) {
	return f.PrepareFunc(query)
}

// Close implements sql/driver.Conn.
func (f ConnFunc) Close() error {
	return f.CloseFunc()
}

// Begin implements sql/driver.Conn.
func (f ConnFunc) Begin() (driver.Tx, error) {
	return f.BeginFunc()
}

// StmtFunc is a functional binding for sql/driver.Stmt.
type StmtFunc struct {
	CloseFunc    func() error
	NumInputFunc func() int
	ExecFunc     func(args []driver.Value) (driver.Result, error)
	QueryFunc    func(args []driver.Value) (driver.Rows, error)
}

// Close implements sql/driver.Stmt.
func (f StmtFunc) Close() error {
	return f.CloseFunc()
}

// NumInput implements sql/driver.Stmt.
func (f StmtFunc) NumInput() int {
	return f.NumInputFunc()
}

// Exec implements sql/driver.Stmt.
func (f StmtFunc) Exec(args []driver.Value) (driver.Result, error) {
	return f.ExecFunc(args)
}

// Query implements sql/driver.Stmt.
func (f StmtFunc) Query(args []driver.Value) (driver.Rows, error) {
	return f.QueryFunc(args)
}

// ============================================================================
// Helper Functions for Common Patterns
// ============================================================================

// NewReader creates an io.Reader from a function.
func NewReader(fn func([]byte) (int, error)) io.Reader {
	return ReadFunc(fn)
}

// NewWriter creates an io.Writer from a function.
func NewWriter(fn func([]byte) (int, error)) io.Writer {
	return WriteFunc(fn)
}

// NewCloser creates an io.Closer from a function.
func NewCloser(fn func() error) io.Closer {
	return CloseFunc(fn)
}

// NewError creates an error from a function.
func NewError(fn func() string) error {
	return ErrorFunc(fn)
}

// NewStringer creates a fmt.Stringer from a function.
func NewStringer(fn func() string) fmt.Stringer {
	return StringerFunc(fn)
}

// NewHandler creates an http.Handler from a function.
func NewHandler(fn func(http.ResponseWriter, *http.Request)) http.Handler {
	return HandlerFunc(fn)
}

// NewRoundTripper creates an http.RoundTripper from a function.
func NewRoundTripper(fn func(*http.Request) (*http.Response, error)) http.RoundTripper {
	return RoundTripperFunc(fn)
}

// ComposeReaders composes multiple readers sequentially.
func ComposeReaders(readers ...io.Reader) io.Reader {
	idx := 0
	return ReadFunc(func(p []byte) (int, error) {
		if idx >= len(readers) {
			return 0, io.EOF
		}
		n, err := readers[idx].Read(p)
		if err == io.EOF {
			idx++
			if idx >= len(readers) {
				return n, io.EOF
			}
			if n == 0 {
				return readers[idx].Read(p)
			}
		}
		return n, err
	})
}

// TeeWriter creates a writer that writes to multiple writers.
func TeeWriter(writers ...io.Writer) io.Writer {
	return WriteFunc(func(p []byte) (int, error) {
		for _, w := range writers {
			n, err := w.Write(p)
			if err != nil {
				return n, err
			}
			if n != len(p) {
				return n, io.ErrShortWrite
			}
		}
		return len(p), nil
	})
}

// FilterWriter creates a writer that filters bytes before writing.
func FilterWriter(w io.Writer, filter func([]byte) []byte) io.Writer {
	return WriteFunc(func(p []byte) (int, error) {
		filtered := filter(p)
		n, err := w.Write(filtered)
		if err != nil {
			return n, err
		}
		return len(p), nil
	})
}

// FilterReader creates a reader that filters bytes after reading.
func FilterReader(r io.Reader, filter func([]byte) []byte) io.Reader {
	return ReadFunc(func(p []byte) (int, error) {
		n, err := r.Read(p)
		if n > 0 {
			filtered := filter(p[:n])
			copy(p, filtered)
		}
		return n, err
	})
}

// WriteMetrics tracks write operation metrics.
type WriteMetrics struct {
	mu            sync.Mutex
	TotalBytes    int64
	TotalWrites   int64
	TotalDuration time.Duration
	Errors        int64
}

// WithMetrics adds write metrics tracking.
func WithMetrics(w io.Writer, metrics *WriteMetrics) io.Writer {
	return WriteFunc(func(p []byte) (int, error) {
		start := time.Now()
		n, err := w.Write(p)
		metrics.mu.Lock()
		metrics.TotalBytes += int64(n)
		metrics.TotalWrites++
		metrics.TotalDuration += time.Since(start)
		if err != nil {
			metrics.Errors++
		}
		metrics.mu.Unlock()
		return n, err
	})
}
