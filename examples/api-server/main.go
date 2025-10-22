package main

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"

	pfc "github.com/Pure-Company/purefunccore"
)

type User struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

func main() {
	// Logging middleware for all routes
	logger := func(msg string) {
		log.Printf("[%s] %s", time.Now().Format("15:04:05"), msg)
	}

	// Authentication checker
	isAuthenticated := func(r *http.Request) bool {
		auth := r.Header.Get("Authorization")
		return strings.HasPrefix(auth, "Bearer ")
	}

	// GET /users - public, no auth needed
	listUsers := pfc.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		users := []User{
			{ID: "1", Name: "Alice", Email: "alice@example.com"},
			{ID: "2", Name: "Bob", Email: "bob@example.com"},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(users)
	}).
		WithLogging(logger).
		WithCORS("*").
		Recover()

	// POST /users - requires auth
	createUser := pfc.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var user User
		if err := json.NewDecoder(r.Body).Decode(&user); err != nil {
			http.Error(w, "Invalid JSON", http.StatusBadRequest)
			return
		}

		user.ID = "3" // Generate ID
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(user)
	}).
		WithLogging(logger).
		WithAuth(isAuthenticated).
		WithTimeout(5 * time.Second).
		WithCORS("*").
		Recover()

	// GET /users/:id - requires auth, has timeout
	getUser := pfc.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.URL.Query().Get("id")
		user := User{ID: id, Name: "Alice", Email: "alice@example.com"}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(user)
	}).
		WithLogging(logger).
		WithAuth(isAuthenticated).
		WithTimeout(3 * time.Second).
		WithCORS("*").
		Recover()

	// DELETE /users/:id - admin only (stricter auth)
	deleteUser := pfc.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.URL.Query().Get("id")
		w.WriteHeader(http.StatusNoContent)
		log.Printf("Deleted user %s", id)
	}).
		WithLogging(logger).
		WithAuth(func(r *http.Request) bool {
			return r.Header.Get("Authorization") == "Bearer admin-token"
		}).
		WithTimeout(2 * time.Second).
		WithCORS("https://admin.example.com").
		Recover()

	// Register routes
	http.Handle("/users", methodRouter(map[string]http.Handler{
		"GET":  listUsers,
		"POST": createUser,
	}))
	http.Handle("/users/", methodRouter(map[string]http.Handler{
		"GET":    getUser,
		"DELETE": deleteUser,
	}))

	log.Println("🚀 Server running on :8080")
	log.Println("Try:")
	log.Println("  curl http://localhost:8080/users")
	log.Println("  curl -H 'Authorization: Bearer token' http://localhost:8080/users?id=1")

	http.ListenAndServe(":8080", nil)
}

// Helper to route by HTTP method
func methodRouter(handlers map[string]http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if handler, ok := handlers[r.Method]; ok {
			handler.ServeHTTP(w, r)
		} else {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})
}
