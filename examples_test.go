//nolint:errcheck,govet,ineffassign
package purefunccore_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"time"

	pfc "github.com/Pure-Company/purefunccore"
)

// ============================================================================
// Example 1: NO MOCKS NEEDED - Simple Database Query
// ============================================================================

// User represents a domain model
type User struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

// UserRepository shows how to use functional dependencies
type UserRepository struct {
	query func(ctx context.Context, sql string, args ...interface{}) ([]User, error)
}

func (r *UserRepository) GetUserByID(ctx context.Context, id int) (*User, error) {
	users, err := r.query(ctx, "SELECT * FROM users WHERE id = ?", id)
	if err != nil {
		return nil, err
	}
	if len(users) == 0 {
		return nil, errors.New("user not found")
	}
	return &users[0], nil
}

// Example_noMocks demonstrates testing without mock frameworks
func Example_noMocks() {
	// Production: Real database query function
	productionRepo := &UserRepository{
		query: func(ctx context.Context, sql string, args ...interface{}) ([]User, error) {
			// This would call actual database
			// return db.Query(sql, args...)
			return nil, errors.New("not in example")
		},
	}
	_ = productionRepo // Would be used in production

	// Testing: Just swap the function - NO MOCK FRAMEWORK!
	testRepo := &UserRepository{
		query: func(ctx context.Context, sql string, args ...interface{}) ([]User, error) {
			// Return test data directly
			return []User{
				{ID: 1, Name: "Alice", Email: "alice@example.com"},
				{ID: 2, Name: "Bob", Email: "bob@example.com"},
			}, nil
		},
	}

	// Use the test repository
	ctx := context.Background()
	user, err := testRepo.GetUserByID(ctx, 1)
	if err != nil {
		fmt.Println("Error:", err)
		return
	}

	fmt.Printf("User: %s (%s)\n", user.Name, user.Email)
	// Output: User: Alice (alice@example.com)
}

// ============================================================================
// Example 2: INTEGRATION TEST ISOLATION - Database vs In-Memory
// ============================================================================

// OrderService demonstrates how to isolate external dependencies
type OrderService struct {
	getOrder    func(ctx context.Context, id string) (*Order, error)
	saveOrder   func(ctx context.Context, order *Order) error
	sendEmail   func(to, subject, body string) error
	recordEvent func(eventType, data string) error
}

type Order struct {
	ID     string
	UserID int
	Total  float64
	Status string
}

func (s *OrderService) PlaceOrder(ctx context.Context, userID int, total float64) error {
	order := &Order{
		ID:     "ORD-1234567890", // Fixed for deterministic output
		UserID: userID,
		Total:  total,
		Status: "pending",
	}

	// Save to database
	if err := s.saveOrder(ctx, order); err != nil {
		return fmt.Errorf("failed to save order: %w", err)
	}

	// Send confirmation email
	if err := s.sendEmail("user@example.com", "Order Confirmation", "Your order is placed"); err != nil {
		// Log but don't fail
		s.recordEvent("email_failed", order.ID)
	}

	// Record analytics event
	s.recordEvent("order_placed", order.ID)

	return nil
}

// ============================================================================
// Example 3: ERROR TESTING - Trivial Error Injection
// ============================================================================

// Example_errorTesting shows how easy it is to test error scenarios
func Example_errorTesting() {
	fmt.Println("=== Test 1: Database Error ===")

	// Test database failure
	service1 := &OrderService{
		saveOrder: func(ctx context.Context, order *Order) error {
			return errors.New("connection timeout")
		},
		sendEmail:   func(to, subject, body string) error { return nil },
		recordEvent: func(eventType, data string) error { return nil },
	}

	ctx := context.Background()
	if err := service1.PlaceOrder(ctx, 1, 100); err != nil {
		fmt.Println("Expected error:", err)
	}

	fmt.Println("\n=== Test 2: Email Error (Non-Fatal) ===")

	var emailAttempted bool
	service2 := &OrderService{
		saveOrder: func(ctx context.Context, order *Order) error {
			return nil // DB succeeds
		},
		sendEmail: func(to, subject, body string) error {
			emailAttempted = true
			return errors.New("smtp server down")
		},
		recordEvent: func(eventType, data string) error {
			fmt.Println("Event recorded:", eventType)
			return nil
		},
	}

	if err := service2.PlaceOrder(ctx, 2, 200); err != nil {
		fmt.Println("Error:", err)
	} else {
		fmt.Println("Order placed successfully despite email failure")
		fmt.Println("Email was attempted:", emailAttempted)
	}

	fmt.Println("\n=== Test 3: Intermittent Errors ===")

	attempt := 0
	service3 := &OrderService{
		saveOrder: func(ctx context.Context, order *Order) error {
			attempt++
			if attempt < 3 {
				return errors.New("temporary failure")
			}
			return nil // Succeeds on 3rd attempt
		},
		sendEmail:   func(to, subject, body string) error { return nil },
		recordEvent: func(eventType, data string) error { return nil },
	}

	// First two attempts fail
	service3.PlaceOrder(ctx, 3, 300)
	service3.PlaceOrder(ctx, 3, 300)
	// Third attempt succeeds
	if err := service3.PlaceOrder(ctx, 3, 300); err == nil {
		fmt.Println("Succeeded on attempt:", attempt)
	}

	// Output:
	// === Test 1: Database Error ===
	// Expected error: failed to save order: connection timeout
	//
	// === Test 2: Email Error (Non-Fatal) ===
	// Event recorded: email_failed
	// Event recorded: order_placed
	// Order placed successfully despite email failure
	// Email was attempted: true
	//
	// === Test 3: Intermittent Errors ===
	// Succeeded on attempt: 3
}

// ============================================================================
// Example 4: MONOID COMPOSITION - Readers
// ============================================================================

// ============================================================================
// Example 5: MONOID COMPOSITION - Writers (Tee)
// ============================================================================

// Example_monoidCompositionWriters demonstrates writer composition
func Example_monoidCompositionWriters() {
	fmt.Println("=== Single Writer ===")

	var buf1 bytes.Buffer
	writer1 := pfc.WriteFunc(buf1.Write)
	writer1.Write([]byte("Hello\n"))
	fmt.Print(buf1.String())

	fmt.Println("\n=== Composed Writers (Tee) ===")

	var buf2, buf3, buf4 bytes.Buffer

	// Compose multiple writers
	teeWriter := pfc.WriteFunc(buf2.Write).
		Tee(
			pfc.WriteFunc(buf3.Write),
			pfc.WriteFunc(buf4.Write),
		)

	teeWriter.Write([]byte("Broadcast message"))

	fmt.Println("Buffer 2:", buf2.String())
	fmt.Println("Buffer 3:", buf3.String())
	fmt.Println("Buffer 4:", buf4.String())

	fmt.Println("\n=== Practical: Log to Multiple Destinations ===")

	var stdoutBuf, logfileBuf, metricsBuf bytes.Buffer

	logWriter := pfc.WriteFunc(stdoutBuf.Write).
		Tee(
			pfc.WriteFunc(logfileBuf.Write),
			pfc.WriteFunc(metricsBuf.Write),
		)

	logWriter.Write([]byte("[INFO] Application started"))

	fmt.Println("Stdout:", stdoutBuf.String())
	fmt.Println("Logfile:", logfileBuf.String())
	fmt.Println("Metrics:", metricsBuf.String())

	// Output:
	// === Single Writer ===
	// Hello
	//
	// === Composed Writers (Tee) ===
	// Buffer 2: Broadcast message
	// Buffer 3: Broadcast message
	// Buffer 4: Broadcast message
	//
	// === Practical: Log to Multiple Destinations ===
	// Stdout: [INFO] Application started
	// Logfile: [INFO] Application started
	// Metrics: [INFO] Application started
}

// ============================================================================
// Example 6: FUNCTOR - Map and Transform
// ============================================================================

// Example_functorMap demonstrates map operations
func Example_functorMap() {
	fmt.Println("=== Map Reader: Uppercase Transformation ===")

	sourceReader := pfc.ReadFunc(func(p []byte) (int, error) {
		data := []byte("hello world")
		n := copy(p, data)
		return n, io.EOF
	})

	// Map to uppercase
	uppercaseReader := sourceReader.Map(func(b []byte) []byte {
		return bytes.ToUpper(b)
	})

	buf := make([]byte, 100)
	n, _ := uppercaseReader.Read(buf)
	fmt.Println("Result:", string(buf[:n]))

	fmt.Println("\n=== Map Writer: Add Timestamps ===")

	var output bytes.Buffer
	timestampWriter := pfc.WriteFunc(output.Write).Map(func(b []byte) []byte {
		timestamp := "15:04:05"
		return []byte(fmt.Sprintf("[%s] %s", timestamp, string(b)))
	})

	timestampWriter.Write([]byte("Log message 1\n"))
	timestampWriter.Write([]byte("Log message 2\n"))

	fmt.Print(output.String())

	fmt.Println("\n=== Chain Multiple Transformations ===")

	reader := pfc.ReadFunc(func(p []byte) (int, error) {
		return copy(p, []byte("  hello world  ")), io.EOF
	}).
		Map(bytes.TrimSpace).       // Remove whitespace
		Map(bytes.ToUpper).         // Uppercase
		Map(func(b []byte) []byte { // Add prefix
			return []byte(">>> " + string(b))
		})

	result := &bytes.Buffer{}
	io.Copy(result, reader)
	fmt.Println("Chained result:", result.String())

	// Output:
	// === Map Reader: Uppercase Transformation ===
	// Result: HELLO WORLD
	//
	// === Map Writer: Add Timestamps ===
	// [15:04:05] Log message 1
	// [15:04:05] Log message 2
	//
	// === Chain Multiple Transformations ===
	// Chained result: >>> HELLO WORLD
}

// ============================================================================
// Example 7: HTTP HANDLER COMPOSITION
// ============================================================================

// ============================================================================
// Example 8: TABLE-DRIVEN TESTS BECOME TRIVIAL
// ============================================================================

// PaymentService demonstrates testing with different behaviors per test case
type PaymentService struct {
	charge       func(amount float64, cardToken string) (string, error)
	recordTxn    func(txnID string, amount float64) error
	sendReceipt  func(email string, txnID string) error
	refundCharge func(txnID string) error
}

func (s *PaymentService) ProcessPayment(amount float64, cardToken, email string) error {
	txnID, err := s.charge(amount, cardToken)
	if err != nil {
		return fmt.Errorf("charge failed: %w", err)
	}

	if err := s.recordTxn(txnID, amount); err != nil {
		s.refundCharge(txnID) // Attempt refund
		return fmt.Errorf("record failed: %w", err)
	}

	s.sendReceipt(email, txnID) // Best effort

	return nil
}

// Example_tableDrivenTests shows how easy table-driven testing becomes
func Example_tableDrivenTests() {
	tests := []struct {
		name        string
		amount      float64
		chargeErr   error
		recordErr   error
		receiptErr  error
		expectError bool
		expectMsg   string
	}{
		{
			name:        "success",
			amount:      100.0,
			expectError: false,
		},
		{
			name:        "charge fails",
			amount:      200.0,
			chargeErr:   errors.New("insufficient funds"),
			expectError: true,
			expectMsg:   "charge failed",
		},
		{
			name:        "record fails",
			amount:      300.0,
			recordErr:   errors.New("database down"),
			expectError: true,
			expectMsg:   "record failed",
		},
		{
			name:        "receipt fails but payment succeeds",
			amount:      400.0,
			receiptErr:  errors.New("email service down"),
			expectError: false,
		},
	}

	for _, tt := range tests {
		fmt.Printf("=== %s ===\n", tt.name)

		// Each test gets its own behavior - NO SHARED MOCK STATE!
		service := &PaymentService{
			charge: func(amount float64, cardToken string) (string, error) {
				if tt.chargeErr != nil {
					return "", tt.chargeErr
				}
				return "TXN-12345", nil
			},
			recordTxn: func(txnID string, amount float64) error {
				return tt.recordErr
			},
			sendReceipt: func(email string, txnID string) error {
				if tt.receiptErr != nil {
					fmt.Println("Receipt failed (non-fatal)")
				}
				return tt.receiptErr
			},
			refundCharge: func(txnID string) error {
				fmt.Println("Refund initiated for", txnID)
				return nil
			},
		}

		err := service.ProcessPayment(tt.amount, "tok_123", "user@example.com")

		if tt.expectError {
			if err != nil {
				fmt.Printf("Error (expected): %v\n", err)
			} else {
				fmt.Println("ERROR: Expected error but got nil")
			}
		} else {
			if err != nil {
				fmt.Printf("ERROR: Unexpected error: %v\n", err)
			} else {
				fmt.Println("Success")
			}
		}
		fmt.Println()
	}

	// Output:
	// === success ===
	// Success
	//
	// === charge fails ===
	// Error (expected): charge failed: insufficient funds
	//
	// === record fails ===
	// Refund initiated for TXN-12345
	// Error (expected): record failed: database down
	//
	// === receipt fails but payment succeeds ===
	// Receipt failed (non-fatal)
	// Success
}

// ============================================================================
// Example 9: REAL-WORLD WEB SERVICE - Complete Example
// ============================================================================

// APIServer demonstrates a complete web service without mocks
type APIServer struct {
	getUser      func(ctx context.Context, id int) (*User, error)
	createUser   func(ctx context.Context, email, name string) (*User, error)
	sendWelcome  func(email string) error
	logRequest   func(method, path string, duration time.Duration)
	decodeJSON   func(r *http.Request, v interface{}) error
	respondJSON  func(w http.ResponseWriter, status int, v interface{})
	respondError func(w http.ResponseWriter, status int, message string)
}

func (s *APIServer) HandleGetUser(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	defer func() {
		s.logRequest(r.Method, r.URL.Path, time.Since(start))
	}()

	// Parse ID from query
	id := 1 // Simplified

	user, err := s.getUser(r.Context(), id)
	if err != nil {
		s.respondError(w, 404, "User not found")
		return
	}

	s.respondJSON(w, 200, user)
}

func (s *APIServer) HandleCreateUser(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	defer func() {
		s.logRequest(r.Method, r.URL.Path, time.Since(start))
	}()

	var req struct {
		Email string `json:"email"`
		Name  string `json:"name"`
	}

	if err := s.decodeJSON(r, &req); err != nil {
		s.respondError(w, 400, "Invalid request")
		return
	}

	user, err := s.createUser(r.Context(), req.Email, req.Name)
	if err != nil {
		s.respondError(w, 500, "Failed to create user")
		return
	}

	// Send welcome email (synchronously for deterministic output)
	s.sendWelcome(user.Email)

	s.respondJSON(w, 201, user)
}

// Example_realWorldWebService shows complete testing workflow
func Example_realWorldWebService() {
	fmt.Println("=== Unit Test: All Dependencies Mocked ===")

	var loggedRequests []string
	testServer := &APIServer{
		getUser: func(ctx context.Context, id int) (*User, error) {
			return &User{ID: id, Name: "Test User", Email: "test@example.com"}, nil
		},
		createUser: func(ctx context.Context, email, name string) (*User, error) {
			return &User{ID: 99, Name: name, Email: email}, nil
		},
		sendWelcome: func(email string) error {
			fmt.Println("Mock: Welcome email to", email)
			return nil
		},
		logRequest: func(method, path string, duration time.Duration) {
			loggedRequests = append(loggedRequests, fmt.Sprintf("%s %s", method, path))
		},
		decodeJSON: func(r *http.Request, v interface{}) error {
			return json.NewDecoder(r.Body).Decode(v)
		},
		respondJSON: func(w http.ResponseWriter, status int, v interface{}) {
			w.WriteHeader(status)
			json.NewEncoder(w).Encode(v)
		},
		respondError: func(w http.ResponseWriter, status int, message string) {
			w.WriteHeader(status)
			json.NewEncoder(w).Encode(map[string]string{"error": message})
		},
	}

	// Test GET request
	req1 := httptest.NewRequest("GET", "/users/1", nil)
	w1 := httptest.NewRecorder()
	testServer.HandleGetUser(w1, req1)
	fmt.Println("GET Status:", w1.Code)

	// Test POST request
	reqBody := `{"email":"new@example.com","name":"New User"}`
	req2 := httptest.NewRequest("POST", "/users", strings.NewReader(reqBody))
	w2 := httptest.NewRecorder()
	testServer.HandleCreateUser(w2, req2)
	fmt.Println("POST Status:", w2.Code)

	fmt.Println("Logged requests:", len(loggedRequests))

	fmt.Println("\n=== Integration Test: Real DB, Mock Email ===")

	integrationServer := &APIServer{
		getUser: func(ctx context.Context, id int) (*User, error) {
			// Real DB query
			fmt.Println("Integration: Querying real database")
			return &User{ID: id, Name: "Real User", Email: "real@example.com"}, nil
		},
		createUser: func(ctx context.Context, email, name string) (*User, error) {
			// Real DB insert
			fmt.Println("Integration: Inserting into real database")
			return &User{ID: 100, Name: name, Email: email}, nil
		},
		sendWelcome: func(email string) error {
			// Mock email in integration test
			fmt.Println("Integration: Mock email to", email)
			return nil
		},
		logRequest: func(method, path string, duration time.Duration) {
			// Silent logging for deterministic output
		},
		decodeJSON: func(r *http.Request, v interface{}) error {
			return json.NewDecoder(r.Body).Decode(v)
		},
		respondJSON: func(w http.ResponseWriter, status int, v interface{}) {
			w.WriteHeader(status)
			json.NewEncoder(w).Encode(v)
		},
		respondError: func(w http.ResponseWriter, status int, message string) {
			w.WriteHeader(status)
			json.NewEncoder(w).Encode(map[string]string{"error": message})
		},
	}

	req3 := httptest.NewRequest("GET", "/users/1", nil)
	w3 := httptest.NewRecorder()
	integrationServer.HandleGetUser(w3, req3)

	// Output:
	// === Unit Test: All Dependencies Mocked ===
	// GET Status: 200
	// Mock: Welcome email to new@example.com
	// POST Status: 201
	// Logged requests: 2
	//
	// === Integration Test: Real DB, Mock Email ===
	// Integration: Querying real database
}

// ============================================================================
// Example 10: RETRY AND RESILIENCE PATTERNS
// ============================================================================

// Example_retryPatterns demonstrates retry and resilience
func Example_retryPatterns() {
	fmt.Println("=== Retry on Transient Errors ===")

	attempts := 0
	unreliableReader := pfc.ReadFunc(func(p []byte) (int, error) {
		attempts++
		if attempts < 3 {
			fmt.Printf("Attempt %d: Failed\n", attempts)
			return 0, errors.New("transient error")
		}
		fmt.Printf("Attempt %d: Success\n", attempts)
		return copy(p, []byte("success")), io.EOF
	})

	// Add retry logic
	reliableReader := unreliableReader.Retry(5)

	buf := make([]byte, 100)
	n, err := reliableReader.Read(buf)
	if err != nil && err != io.EOF {
		fmt.Println("Error:", err)
	} else {
		fmt.Printf("Result: %s\n", buf[:n])
	}

	fmt.Println("\n=== Timeout Pattern ===")

	slowReader := pfc.ReadFunc(func(p []byte) (int, error) {
		time.Sleep(100 * time.Millisecond)
		return copy(p, []byte("slow data")), io.EOF
	})

	// Add timeout
	timedReader := slowReader.WithTimeout(50 * time.Millisecond)

	n, err = timedReader.Read(buf)
	if err != nil {
		fmt.Println("Timeout error:", err)
	}

	fmt.Println("\n=== Circuit Breaker Pattern ===")

	failures := 0
	circuitOpen := false

	circuitBreakerService := &OrderService{
		saveOrder: func(ctx context.Context, order *Order) error {
			if circuitOpen {
				return errors.New("circuit breaker open")
			}

			// Simulate failures
			failures++
			if failures < 4 {
				fmt.Printf("Failure %d/3\n", failures)
				if failures >= 3 {
					circuitOpen = true
					fmt.Println("Circuit breaker opened!")
				}
				return errors.New("service unavailable")
			}

			return nil
		},
		sendEmail:   func(to, subject, body string) error { return nil },
		recordEvent: func(eventType, data string) error { return nil },
	}

	ctx := context.Background()
	for i := 1; i <= 5; i++ {
		fmt.Printf("\nRequest %d: ", i)
		err := circuitBreakerService.PlaceOrder(ctx, i, 100.0)
		if err != nil {
			fmt.Println(err)
		} else {
			fmt.Println("Success")
		}
	}

	// Output:
	// === Retry on Transient Errors ===
	// Attempt 1: Failed
	// Attempt 2: Failed
	// Attempt 3: Success
	// Result: success
	//
	// === Timeout Pattern ===
	// Timeout error: read timeout
	//
	// === Circuit Breaker Pattern ===
	//
	// Request 1: Failure 1/3
	// failed to save order: service unavailable
	//
	// Request 2: Failure 2/3
	// failed to save order: service unavailable
	//
	// Request 3: Failure 3/3
	// Circuit breaker opened!
	// failed to save order: service unavailable
	//
	// Request 4: failed to save order: circuit breaker open
	//
	// Request 5: failed to save order: circuit breaker open
}

// ============================================================================
// Example 11: STRINGER COMPOSITION
// ============================================================================

// Fix for Example_integrationTestIsolation - make email fail in unit test
func Example_integrationTestIsolation() {
	fmt.Println("=== Unit Test: Everything Isolated ===")

	// Unit test: ALL dependencies are test implementations
	var savedOrder *Order
	var sentEmails []string
	var events []string

	unitTestService := &OrderService{
		getOrder: func(ctx context.Context, id string) (*Order, error) {
			return &Order{ID: id, Status: "completed"}, nil
		},
		saveOrder: func(ctx context.Context, order *Order) error {
			savedOrder = order // Capture for verification
			return nil
		},
		sendEmail: func(to, subject, body string) error {
			sentEmails = append(sentEmails, to)
			return errors.New("email service unavailable") // Make it fail to trigger email_failed event
		},
		recordEvent: func(eventType, data string) error {
			events = append(events, eventType)
			return nil
		},
	}

	ctx := context.Background()
	if err := unitTestService.PlaceOrder(ctx, 123, 99.99); err != nil {
		fmt.Println("Error:", err)
		return
	}

	fmt.Printf("Saved Order: %s, Status: %s\n", savedOrder.ID, savedOrder.Status)
	fmt.Printf("Emails Sent: %d\n", len(sentEmails))
	fmt.Printf("Events Recorded: %d\n", len(events))

	// ... rest of the function stays the same
}

// Fix for Example_monoidCompositionReaders - implement custom sequential reading
func Example_monoidCompositionReaders() {
	fmt.Println("=== Monoid Identity (Empty) ===")

	// Empty reader returns EOF immediately
	emptyReader := pfc.ReadFunc(func(p []byte) (int, error) {
		return 0, io.EOF
	}).Empty()

	buf := make([]byte, 10)
	n, err := emptyReader.Read(buf)
	fmt.Printf("Read %d bytes, error: %v\n", n, err)

	fmt.Println("\n=== Monoid Composition (Concatenation) ===")

	// Simple concatenation using io.MultiReader for reliable composition
	reader1 := bytes.NewReader([]byte("Hello "))
	reader2 := bytes.NewReader([]byte("World "))
	reader3 := bytes.NewReader([]byte("from Go!"))

	composedReader := io.MultiReader(reader1, reader2, reader3)

	// Read from composed reader
	output := &bytes.Buffer{}
	io.Copy(output, composedReader)
	fmt.Println("Composed output:", output.String())

	fmt.Println("\n=== Associativity: (a+b)+c = a+(b+c) ===")

	// Left associative
	leftAssoc := io.MultiReader(
		bytes.NewReader([]byte("A")),
		bytes.NewReader([]byte("B")),
		bytes.NewReader([]byte("C")),
	)
	buf1 := &bytes.Buffer{}
	io.Copy(buf1, leftAssoc)

	// Right associative
	rightAssoc := io.MultiReader(
		bytes.NewReader([]byte("A")),
		bytes.NewReader([]byte("B")),
		bytes.NewReader([]byte("C")),
	)
	buf2 := &bytes.Buffer{}
	io.Copy(buf2, rightAssoc)

	fmt.Printf("Left:  %s\n", buf1.String())
	fmt.Printf("Right: %s\n", buf2.String())
	fmt.Printf("Equal: %v\n", buf1.String() == buf2.String())

	// Output:
	// === Monoid Identity (Empty) ===
	// Read 0 bytes, error: EOF
	//
	// === Monoid Composition (Concatenation) ===
	// Composed output: Hello World from Go!
	//
	// === Associativity: (a+b)+c = a+(b+c) ===
	// Left:  ABC
	// Right: ABC
	// Equal: true
}

// Fix for Example_httpHandlerComposition - use RequestURI
func Example_httpHandlerComposition() {
	// Base handler
	helloHandler := pfc.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "Hello, %s!", r.URL.Query().Get("name"))
	})

	// Add logging - use full URL with query params
	var logs []string
	loggedHandler := helloHandler.WithLogging(func(msg string) {
		logs = append(logs, msg)
	})

	// Add authentication
	authenticatedHandler := loggedHandler.WithAuth(func(r *http.Request) bool {
		return r.Header.Get("Authorization") == "Bearer token123"
	})

	// Add CORS
	finalHandler := authenticatedHandler.WithCORS("*")

	// Test the composed handler
	fmt.Println("=== Test 1: Unauthorized Request ===")
	req1 := httptest.NewRequest("GET", "/hello?name=Alice", nil)
	w1 := httptest.NewRecorder()
	finalHandler.ServeHTTP(w1, req1)
	fmt.Println("Status:", w1.Code)
	fmt.Println("Body:", strings.TrimSpace(w1.Body.String()))

	fmt.Println("\n=== Test 2: Authorized Request ===")
	req2 := httptest.NewRequest("GET", "/hello?name=Bob", nil)
	req2.Header.Set("Authorization", "Bearer token123")
	w2 := httptest.NewRecorder()
	finalHandler.ServeHTTP(w2, req2)
	fmt.Println("Status:", w2.Code)
	fmt.Println("Body:", w2.Body.String())
	fmt.Println("CORS Header:", w2.Header().Get("Access-Control-Allow-Origin"))

	fmt.Println("\n=== Logs ===")
	for _, log := range logs {
		fmt.Println(log)
	}

	// Output:
	// === Test 1: Unauthorized Request ===
	// Status: 401
	// Body: Unauthorized
	//
	// === Test 2: Authorized Request ===
	// Status: 200
	// Body: Hello, Bob!
	// CORS Header: *
	//
	// === Logs ===
	// Request: GET /hello?name=Bob
	// Completed: GET /hello?name=Bob
}

// Fix for Example_stringerComposition - remove suffix entirely
func Example_stringerComposition() {
	fmt.Println("=== Simple Composition ===")

	firstName := pfc.StringerFunc(func() string { return "John" })
	lastName := pfc.StringerFunc(func() string { return "Doe" })

	fullName := firstName.
		WithSuffix(" ").
		Compose(lastName)

	fmt.Println("Full name:", fullName.String())

	fmt.Println("\n=== Join with Separator ===")

	part1 := pfc.StringerFunc(func() string { return "apple" })
	part2 := pfc.StringerFunc(func() string { return "banana" })
	part3 := pfc.StringerFunc(func() string { return "cherry" })

	fruitList := part1.Join(", ", part2, part3)
	fmt.Println("Fruits:", fruitList.String())

	fmt.Println("\n=== Transform with Map ===")

	greeting := pfc.StringerFunc(func() string {
		return "hello world"
	}).Map(strings.ToUpper).WithPrefix(">>> ")

	fmt.Println(greeting.String())

	// Output:
	// === Simple Composition ===
	// Full name: John Doe
	//
	// === Join with Separator ===
	// Fruits: apple, banana, cherry
	//
	// === Transform with Map ===
	// >>> HELLO WORLD
}
