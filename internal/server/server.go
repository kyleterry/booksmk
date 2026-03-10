package server

import (
	"context"
	"log/slog"
	"net"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/kyleterry/booksmk/internal/server/urlhandler"
	"github.com/kyleterry/booksmk/internal/server/userhandler"
	"github.com/kyleterry/booksmk/internal/store"
)

// Config holds server configuration sourced from environment variables.
type Config struct {
	Addr        string
	DatabaseURL string
	Logger      *slog.Logger
}

// Server is the booksmk HTTP server.
type Server struct {
	cfg        Config
	db         *pgxpool.Pool
	store      *store.Store
	logger     *slog.Logger
	mux        *http.ServeMux
	urlHandler  *urlhandler.Handler
	userHandler *userhandler.Handler
}

func New(cfg Config) (*Server, error) {
	pool, err := pgxpool.New(context.Background(), cfg.DatabaseURL)
	if err != nil {
		return nil, err
	}

	st := store.New(pool)

	s := &Server{
		cfg:        cfg,
		db:         pool,
		store:      st,
		logger:     cfg.Logger,
		mux:        http.NewServeMux(),
		urlHandler:  urlhandler.New(st, cfg.Logger),
		userHandler: userhandler.New(st, cfg.Logger),
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
