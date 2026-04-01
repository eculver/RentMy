// Package httpserver provides an HTTP server with graceful shutdown support.
package httpserver

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os/signal"
	"syscall"
	"time"
)

const shutdownTimeout = 15 * time.Second

// Server wraps an *http.Server with graceful shutdown behaviour.
type Server struct {
	srv *http.Server
}

// New creates a Server that listens on addr and delegates to handler.
func New(addr string, handler http.Handler) *Server {
	return &Server{
		srv: &http.Server{
			Addr:              addr,
			Handler:           handler,
			ReadHeaderTimeout: 10 * time.Second,
		},
	}
}

// Start begins listening for HTTP requests and blocks until the provided
// context is cancelled or an OS signal (SIGTERM, SIGINT) is received. It then
// performs a graceful shutdown, allowing in-flight requests up to 15 seconds to
// complete.
func (s *Server) Start(ctx context.Context) error {
	ctx, stop := signal.NotifyContext(ctx, syscall.SIGTERM, syscall.SIGINT)
	defer stop()

	errCh := make(chan error, 1)
	go func() {
		slog.Info("server listening", "addr", s.srv.Addr)
		if err := s.srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
		close(errCh)
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		slog.Info("shutdown signal received")
	}

	return s.Shutdown(context.Background())
}

// Shutdown gracefully shuts down the server within the configured timeout.
func (s *Server) Shutdown(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, shutdownTimeout)
	defer cancel()

	slog.Info("shutting down server", "timeout", shutdownTimeout)
	if err := s.srv.Shutdown(ctx); err != nil {
		return err
	}
	slog.Info("server stopped")
	return nil
}
