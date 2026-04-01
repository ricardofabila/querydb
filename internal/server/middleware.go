package server

import (
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"
)

// loggingMiddleware logs HTTP requests with method, path, timestamp, and duration
// Validates: Requirements 11.4
func (s *Server) loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Log request
		fmt.Printf("[%s] %s %s\n",
			time.Now().Format("2006-01-02 15:04:05"),
			r.Method,
			r.URL.Path)

		next.ServeHTTP(w, r)

		// Log duration
		fmt.Printf("  Completed in %v\n", time.Since(start))
	})
}

// recoveryMiddleware catches panics in request handlers and returns 500 errors
// Validates: Requirements 11.1
func (s *Server) recoveryMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				fmt.Fprintf(os.Stderr, "[PANIC] %v\n", err)
				s.sendError(w, http.StatusInternalServerError,
					"Internal server error",
					[]string{"Check server logs for details"})
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// corsMiddleware adds CORS headers to API responses
// Validates: Requirements 13.1, 13.2, 13.3
func (s *Server) corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")

		// Allow localhost origins when bound to 127.0.0.1
		// Allow any origin when bound to 0.0.0.0
		if s.host == "0.0.0.0" {
			// When bound to all interfaces, allow any origin
			if origin != "" {
				w.Header().Set("Access-Control-Allow-Origin", origin)
			} else {
				w.Header().Set("Access-Control-Allow-Origin", "*")
			}
		} else if strings.HasPrefix(origin, "http://localhost") ||
			strings.HasPrefix(origin, "http://127.0.0.1") ||
			strings.HasPrefix(origin, "https://localhost") ||
			strings.HasPrefix(origin, "https://127.0.0.1") {
			// When bound to localhost, only allow localhost origins
			w.Header().Set("Access-Control-Allow-Origin", origin)
		}

		// Set other CORS headers
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		// Handle OPTIONS preflight requests
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}
