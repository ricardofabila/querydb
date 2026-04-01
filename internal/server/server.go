package server

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"querydb/internal/config"
)

// Server represents the HTTP server for the DynamoDB web UI
type Server struct {
	httpServer *http.Server
	config     *config.Config
	host       string
	port       int
}

// New creates a new Server instance with the provided configuration
// Validates: Requirements 1.2, 1.5, 1.6, 14.1, 14.2, 14.3
func New(cfg *config.Config, host string, port int) *Server {
	s := &Server{
		config: cfg,
		host:   host,
		port:   port,
	}

	// Setup router
	mux := http.NewServeMux()
	s.registerRoutes(mux)

	// Apply middleware (recovery is outermost to catch all panics)
	handler := s.recoveryMiddleware(s.loggingMiddleware(s.corsMiddleware(mux)))

	// Create HTTP server
	s.httpServer = &http.Server{
		Addr:         fmt.Sprintf("%s:%d", host, port),
		Handler:      handler,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	return s
}

// Start begins listening and serving HTTP requests
// Validates: Requirements 1.2, 1.4
func (s *Server) Start() error {
	fmt.Printf("Starting server on http://%s:%d\n", s.host, s.port)
	return s.httpServer.ListenAndServe()
}

// Shutdown gracefully shuts down the server with the provided context
// Validates: Requirements 1.6, 14.1, 14.2, 14.3
func (s *Server) Shutdown(ctx context.Context) error {
	fmt.Println("Shutting down server...")
	return s.httpServer.Shutdown(ctx)
}

// registerRoutes sets up the HTTP routes for the server
// Validates: Requirements 2.3, 10.1, 10.2, 10.3, 10.4, 10.5
func (s *Server) registerRoutes(mux *http.ServeMux) {
	// API routes
	mux.HandleFunc("/api/tables", s.handleGetTables)
	mux.HandleFunc("/api/tables/", s.handleTableOperations)

	// Serve embedded static assets
	mux.Handle("/", http.FileServer(http.FS(staticAssets)))
}
