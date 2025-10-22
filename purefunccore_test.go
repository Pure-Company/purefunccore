// nolint:errcheck
package purefunccore

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// ============================================================================
// ReadFunc Tests
// ============================================================================

func TestReadFunc_Read(t *testing.T) {
	data := []byte("hello world")
	reader := ReadFunc(func(p []byte) (int, error) {
		n := copy(p, data)
		return n, io.EOF
	})

	buf := make([]byte, 100)
	n, err := reader.Read(buf)

	if n != len(data) {
		t.Errorf("expected %d bytes, got %d", len(data), n)
	}
	if err != io.EOF {
		t.Errorf("expected EOF, got %v", err)
	}
	if string(buf[:n]) != "hello world" {
		t.Errorf("expected 'hello world', got '%s'", buf[:n])
	}
}

func TestReadFunc_Empty(t *testing.T) {
	reader := ReadFunc(func(p []byte) (int, error) {
		return copy(p, []byte("data")), nil
	}).Empty()

	buf := make([]byte, 10)
	n, err := reader.Read(buf)

	if n != 0 {
		t.Errorf("empty reader should return 0 bytes, got %d", n)
	}
	if err != io.EOF {
		t.Errorf("empty reader should return EOF, got %v", err)
	}
}

func TestReadFunc_Map(t *testing.T) {
	reader := ReadFunc(func(p []byte) (int, error) {
		return copy(p, []byte("hello")), io.EOF
	}).Map(bytes.ToUpper)

	buf := make([]byte, 10)
	n, err := reader.Read(buf)

	if string(buf[:n]) != "HELLO" {
		t.Errorf("expected 'HELLO', got '%s'", buf[:n])
	}
	if err != io.EOF {
		t.Errorf("expected EOF, got %v", err)
	}
}

func TestReadFunc_Map_WithError(t *testing.T) {
	expectedErr := errors.New("read error")
	reader := ReadFunc(func(p []byte) (int, error) {
		return 0, expectedErr
	}).Map(bytes.ToUpper)

	buf := make([]byte, 10)
	n, err := reader.Read(buf)

	if n != 0 {
		t.Errorf("expected 0 bytes on error, got %d", n)
	}
	if err != expectedErr {
		t.Errorf("expected error %v, got %v", expectedErr, err)
	}
}

func TestReadFunc_Filter(t *testing.T) {
	reader := ReadFunc(func(p []byte) (int, error) {
		return copy(p, []byte("abc123")), io.EOF
	}).Filter(func(b byte) bool {
		return b >= 'a' && b <= 'z' // only letters
	})

	buf := make([]byte, 10)
	n, err := reader.Read(buf)

	if string(buf[:n]) != "abc" {
		t.Errorf("expected 'abc', got '%s'", buf[:n])
	}
	if err != io.EOF {
		t.Errorf("expected EOF, got %v", err)
	}
}

func TestReadFunc_Take(t *testing.T) {
	reader := ReadFunc(func(p []byte) (int, error) {
		return copy(p, []byte("hello world")), io.EOF
	}).Take(5)

	buf := make([]byte, 100)
	n, _ := reader.Read(buf)

	if n != 5 {
		t.Errorf("expected 5 bytes, got %d", n)
	}
	if string(buf[:n]) != "hello" {
		t.Errorf("expected 'hello', got '%s'", buf[:n])
	}
}

func TestReadFunc_Retry(t *testing.T) {
	attempts := 0
	reader := ReadFunc(func(p []byte) (int, error) {
		attempts++
		if attempts < 3 {
			return 0, errors.New("transient error")
		}
		return copy(p, []byte("success")), io.EOF
	}).Retry(5)

	buf := make([]byte, 10)
	n, err := reader.Read(buf)

	if attempts != 3 {
		t.Errorf("expected 3 attempts, got %d", attempts)
	}
	if string(buf[:n]) != "success" {
		t.Errorf("expected 'success', got '%s'", buf[:n])
	}
	if err != io.EOF {
		t.Errorf("expected EOF, got %v", err)
	}
}

func TestReadFunc_Retry_ExhaustsRetries(t *testing.T) {
	attempts := 0
	reader := ReadFunc(func(p []byte) (int, error) {
		attempts++
		return 0, errors.New("persistent error")
	}).Retry(2)

	buf := make([]byte, 10)
	_, err := reader.Read(buf)

	if attempts != 3 { // initial + 2 retries
		t.Errorf("expected 3 attempts, got %d", attempts)
	}
	if err == nil {
		t.Error("expected error, got nil")
	}
}

func TestReadFunc_Tap(t *testing.T) {
	var tappedData []byte
	var tappedN int
	var tappedErr error

	reader := ReadFunc(func(p []byte) (int, error) {
		return copy(p, []byte("test")), io.EOF
	}).Tap(func(data []byte, n int, err error) {
		tappedData = make([]byte, n)
		copy(tappedData, data)
		tappedN = n
		tappedErr = err
	})

	buf := make([]byte, 10)
	reader.Read(buf)

	if string(tappedData) != "test" {
		t.Errorf("expected tapped 'test', got '%s'", tappedData)
	}
	if tappedN != 4 {
		t.Errorf("expected tapped n=4, got %d", tappedN)
	}
	if tappedErr != io.EOF {
		t.Errorf("expected tapped EOF, got %v", tappedErr)
	}
}

// ============================================================================
// WriteFunc Tests
// ============================================================================

func TestWriteFunc_Write(t *testing.T) {
	var written []byte
	writer := WriteFunc(func(p []byte) (int, error) {
		written = make([]byte, len(p))
		copy(written, p)
		return len(p), nil
	})

	data := []byte("hello")
	n, err := writer.Write(data)

	if n != len(data) {
		t.Errorf("expected %d bytes written, got %d", len(data), n)
	}
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if string(written) != "hello" {
		t.Errorf("expected 'hello', got '%s'", written)
	}
}

func TestWriteFunc_Empty(t *testing.T) {
	writer := WriteFunc(func(p []byte) (int, error) {
		return 0, errors.New("should not be called")
	}).Empty()

	n, err := writer.Write([]byte("test"))

	if n != 4 {
		t.Errorf("empty writer should return len(p), got %d", n)
	}
	if err != nil {
		t.Errorf("empty writer should return nil error, got %v", err)
	}
}

func TestWriteFunc_Tee(t *testing.T) {
	var buf1, buf2, buf3 bytes.Buffer

	writer := WriteFunc(buf1.Write).Tee(
		WriteFunc(buf2.Write),
		WriteFunc(buf3.Write),
	)

	data := []byte("broadcast")
	writer.Write(data)

	if buf1.String() != "broadcast" {
		t.Errorf("buf1 expected 'broadcast', got '%s'", buf1.String())
	}
	if buf2.String() != "broadcast" {
		t.Errorf("buf2 expected 'broadcast', got '%s'", buf2.String())
	}
	if buf3.String() != "broadcast" {
		t.Errorf("buf3 expected 'broadcast', got '%s'", buf3.String())
	}
}

func TestWriteFunc_Map(t *testing.T) {
	var buf bytes.Buffer
	writer := WriteFunc(buf.Write).Map(bytes.ToUpper)

	writer.Write([]byte("hello"))

	if buf.String() != "HELLO" {
		t.Errorf("expected 'HELLO', got '%s'", buf.String())
	}
}

func TestWriteFunc_Filter(t *testing.T) {
	var buf bytes.Buffer
	writer := WriteFunc(buf.Write).Filter(func(b byte) bool {
		return b >= 'a' && b <= 'z'
	})

	writer.Write([]byte("abc123xyz"))

	if buf.String() != "abcxyz" {
		t.Errorf("expected 'abcxyz', got '%s'", buf.String())
	}
}

// ============================================================================
// HandlerFunc Tests
// ============================================================================

func TestHandlerFunc_ServeHTTP(t *testing.T) {
	called := false
	handler := HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if !called {
		t.Error("handler was not called")
	}
	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestHandlerFunc_WithLogging(t *testing.T) {
	var logs []string
	handler := HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}).WithLogging(func(msg string) {
		logs = append(logs, msg)
	})

	req := httptest.NewRequest("GET", "/test?foo=bar", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if len(logs) != 2 {
		t.Errorf("expected 2 log entries, got %d", len(logs))
	}
	if !strings.Contains(logs[0], "GET /test?foo=bar") {
		t.Errorf("expected log to contain path with query, got '%s'", logs[0])
	}
}

func TestHandlerFunc_WithAuth(t *testing.T) {
	handler := HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}).WithAuth(func(r *http.Request) bool {
		return r.Header.Get("Authorization") == "Bearer secret"
	})

	// Test unauthorized
	req1 := httptest.NewRequest("GET", "/test", nil)
	w1 := httptest.NewRecorder()
	handler.ServeHTTP(w1, req1)

	if w1.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w1.Code)
	}

	// Test authorized
	req2 := httptest.NewRequest("GET", "/test", nil)
	req2.Header.Set("Authorization", "Bearer secret")
	w2 := httptest.NewRecorder()
	handler.ServeHTTP(w2, req2)

	if w2.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w2.Code)
	}
}

func TestHandlerFunc_WithTimeout(t *testing.T) {
	handler := HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}).WithTimeout(50 * time.Millisecond)

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusRequestTimeout {
		t.Errorf("expected timeout status 408, got %d", w.Code)
	}
}

func TestHandlerFunc_WithCORS(t *testing.T) {
	handler := HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}).WithCORS("https://example.com")

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	origin := w.Header().Get("Access-Control-Allow-Origin")
	if origin != "https://example.com" {
		t.Errorf("expected CORS origin 'https://example.com', got '%s'", origin)
	}
}

func TestHandlerFunc_Recover(t *testing.T) {
	handler := HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("something went wrong")
	}).Recover()

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	// Should not panic
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 500, got %d", w.Code)
	}
}

// ============================================================================
// StringerFunc Tests
// ============================================================================

func TestStringerFunc_String(t *testing.T) {
	stringer := StringerFunc(func() string {
		return "hello"
	})

	if stringer.String() != "hello" {
		t.Errorf("expected 'hello', got '%s'", stringer.String())
	}
}

func TestStringerFunc_Empty(t *testing.T) {
	stringer := StringerFunc(func() string {
		return "data"
	}).Empty()

	if stringer.String() != "" {
		t.Errorf("expected empty string, got '%s'", stringer.String())
	}
}

func TestStringerFunc_Compose(t *testing.T) {
	s1 := StringerFunc(func() string { return "hello" })
	s2 := StringerFunc(func() string { return " world" })

	composed := s1.Compose(s2)

	if composed.String() != "hello world" {
		t.Errorf("expected 'hello world', got '%s'", composed.String())
	}
}

func TestStringerFunc_Join(t *testing.T) {
	s1 := StringerFunc(func() string { return "a" })
	s2 := StringerFunc(func() string { return "b" })
	s3 := StringerFunc(func() string { return "c" })

	joined := s1.Join(", ", s2, s3)

	if joined.String() != "a, b, c" {
		t.Errorf("expected 'a, b, c', got '%s'", joined.String())
	}
}

func TestStringerFunc_Map(t *testing.T) {
	stringer := StringerFunc(func() string {
		return "hello"
	}).Map(strings.ToUpper)

	if stringer.String() != "HELLO" {
		t.Errorf("expected 'HELLO', got '%s'", stringer.String())
	}
}

func TestStringerFunc_WithPrefix(t *testing.T) {
	stringer := StringerFunc(func() string {
		return "world"
	}).WithPrefix("hello ")

	if stringer.String() != "hello world" {
		t.Errorf("expected 'hello world', got '%s'", stringer.String())
	}
}

func TestStringerFunc_WithSuffix(t *testing.T) {
	stringer := StringerFunc(func() string {
		return "hello"
	}).WithSuffix(" world")

	if stringer.String() != "hello world" {
		t.Errorf("expected 'hello world', got '%s'", stringer.String())
	}
}

// ============================================================================
// ErrorFunc Tests
// ============================================================================

func TestErrorFunc_Error(t *testing.T) {
	err := ErrorFunc(func() string {
		return "something failed"
	})

	if err.Error() != "something failed" {
		t.Errorf("expected 'something failed', got '%s'", err.Error())
	}
}

func TestErrorFunc_Wrap(t *testing.T) {
	err := ErrorFunc(func() string {
		return "connection failed"
	}).Wrap("database")

	if err.Error() != "database: connection failed" {
		t.Errorf("expected 'database: connection failed', got '%s'", err.Error())
	}
}

func TestErrorFunc_Compose(t *testing.T) {
	err1 := ErrorFunc(func() string { return "error 1" })
	err2 := ErrorFunc(func() string { return "error 2" })

	composed := err1.Compose(err2)

	if composed.Error() != "error 1; error 2" {
		t.Errorf("expected 'error 1; error 2', got '%s'", composed.Error())
	}
}

func TestErrorFunc_WithCode(t *testing.T) {
	err := ErrorFunc(func() string {
		return "not found"
	}).WithCode(404)

	if err.Code() != 404 {
		t.Errorf("expected code 404, got %d", err.Code())
	}
	if err.Error() != "[404] not found" {
		t.Errorf("expected '[404] not found', got '%s'", err.Error())
	}
}

// ============================================================================
// Helper Function Tests
// ============================================================================

func TestComposeReaders(t *testing.T) {
	r1 := bytes.NewReader([]byte("hello "))
	r2 := bytes.NewReader([]byte("world"))

	composed := ComposeReaders(r1, r2)

	result := &bytes.Buffer{}
	io.Copy(result, composed)

	if result.String() != "hello world" {
		t.Errorf("expected 'hello world', got '%s'", result.String())
	}
}

func TestTeeWriter(t *testing.T) {
	var buf1, buf2 bytes.Buffer

	tee := TeeWriter(
		WriteFunc(buf1.Write),
		WriteFunc(buf2.Write),
	)

	tee.Write([]byte("test"))

	if buf1.String() != "test" {
		t.Errorf("buf1: expected 'test', got '%s'", buf1.String())
	}
	if buf2.String() != "test" {
		t.Errorf("buf2: expected 'test', got '%s'", buf2.String())
	}
}

// ============================================================================
// Benchmarks
// ============================================================================

func BenchmarkReadFunc_Map(b *testing.B) {
	reader := ReadFunc(func(p []byte) (int, error) {
		return copy(p, []byte("hello world")), io.EOF
	}).Map(bytes.ToUpper)

	buf := make([]byte, 100)
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		reader.Read(buf)
	}
}

func BenchmarkWriteFunc_Tee(b *testing.B) {
	var buf1, buf2 bytes.Buffer
	writer := WriteFunc(buf1.Write).Tee(WriteFunc(buf2.Write))

	data := []byte("test data")
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		writer.Write(data)
	}
}

func BenchmarkHandlerFunc_Middleware(b *testing.B) {
	handler := HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}).
		WithLogging(func(msg string) {}).
		WithAuth(func(r *http.Request) bool { return true }).
		WithCORS("*")

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer token")

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
	}
}
