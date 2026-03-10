package server

import (
	"context"
	"log/slog"
	"net"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/kyleterry/booksmk/internal/server/apikeyhandler"
	"github.com/kyleterry/booksmk/internal/server/urlhandler"
	"github.com/kyleterry/booksmk/internal/server/userhandler"
	"github.com/kyleterry/booksmk/internal/store"
)

// Config holds server configuration.
type Config struct {
	Addr   string
	Pool   *pgxpool.Pool
	Logger *slog.Logger
}

// Server is the booksmk HTTP server.
type Server struct {
	cfg           Config
	store         *store.Store
	logger        *slog.Logger
	mux           *http.ServeMux
	urlHandler    *urlhandler.Handler
	userHandler   *userhandler.Handler
	apiKeyHandler *apikeyhandler.Handler
}

func New(cfg Config) (*Server, error) {
	st := store.New(cfg.Pool)

	s := &Server{
		cfg:         cfg,
		store:       st,
		logger:      cfg.Logger,
		mux:         http.NewServeMux(),
		urlHandler:    urlhandler.New(st, cfg.Logger),
		userHandler:   userhandler.New(st, cfg.Logger),
		apiKeyHandler: apikeyhandler.New(st, cfg.Logger),
	}

	s.registerRoutes()

	return s, nil
}

// Run starts the HTTP server and blocks until ctx is cancelled, then shuts down gracefully.
func (s *Server) Run(ctx context.Context) error {
	srv := &http.Server{
		Addr:    s.cfg.Addr,
		Handler: s.mux,
		BaseContext: func(l net.Listener) context.Context {
			return ctx
		},
	}

	errCh := make(chan error, 1)
	go func() {
		s.logger.Info("listening", "addr", s.cfg.Addr)
		errCh <- srv.ListenAndServe()
	}()

	select {
	case <-ctx.Done():
		return srv.Shutdown(context.Background())
	case err := <-errCh:
		return err
	}
}
