package server

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/edgard/opencompat/internal/api"
	"github.com/edgard/opencompat/internal/auth"
	"github.com/edgard/opencompat/internal/config"
	"github.com/edgard/opencompat/internal/upstream"
)

// Server represents the HTTP server.
type Server struct {
	httpServer *http.Server
	handlers   *Handlers
	client     *upstream.Client
	cfg        *config.Config
}

// New creates a new server instance.
func New(store *auth.Store, cfg *config.Config) *Server {
	// Create upstream client (shared between handlers and server lifecycle)
	client := upstream.NewClient(store, cfg)
	handlers := NewHandlersWithClient(store, cfg, client)

	mux := http.NewServeMux()

	// Register routes
	mux.HandleFunc("/health", handlers.Health)
	mux.HandleFunc("/v1/models", handlers.Models)
	mux.HandleFunc("/v1/chat/completions", handlers.ChatCompletions)

	// Catch-all for unknown /v1/ endpoints - returns OpenAI-style 404
	mux.HandleFunc("/v1/", func(w http.ResponseWriter, r *http.Request) {
		// Check if this path matches a known endpoint (exact match handled above)
		path := r.URL.Path
		if path == "/v1/models" || path == "/v1/chat/completions" {
			// Shouldn't reach here due to exact match, but just in case
			return
		}
		// Return OpenAI-style 404 for unknown /v1/ paths
		endpoint := strings.TrimPrefix(path, "/v1/")
		api.WriteNotFound(w, fmt.Sprintf("Unknown endpoint: /v1/%s", endpoint))
	})

	// Apply middleware
	handler := ChainMiddleware(
		mux,
		RecoveryMiddleware,
		LoggingMiddleware(cfg.Verbose),
		CORSMiddleware,
	)

	addr := fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)

	return &Server{
		httpServer: &http.Server{
			Addr:    addr,
			Handler: handler,
		},
		handlers: handlers,
		client:   client,
		cfg:      cfg,
	}
}

// PrefetchInstructions fetches all instruction files before starting the server.
// This should be called before Start().
// Returns error if instructions cannot be loaded (no cache AND GitHub down).
func (s *Server) PrefetchInstructions() error {
	return s.client.PrefetchInstructions()
}

// Start starts the HTTP server.
// Should be called after PrefetchInstructions().
func (s *Server) Start() error {
	// Start background refresh for instructions
	s.client.StartBackgroundRefresh()

	log.Printf("OpenCompat server starting on %s", s.httpServer.Addr)
	log.Printf("OpenAI-compatible API available at http://%s/v1", s.httpServer.Addr)

	if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}

// Shutdown gracefully shuts down the server.
func (s *Server) Shutdown(ctx context.Context) error {
	// Stop background refresh
	s.client.Close()

	return s.httpServer.Shutdown(ctx)
}
