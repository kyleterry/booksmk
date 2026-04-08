package server

import (
	"context"
	"log/slog"
	"net"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"

	"go.e64ec.com/booksmk/internal/discuss"
	"go.e64ec.com/booksmk/internal/feedworker"
	"go.e64ec.com/booksmk/internal/server/adminhandler"
	"go.e64ec.com/booksmk/internal/server/apihandler"
	"go.e64ec.com/booksmk/internal/server/apikeyhandler"
	"go.e64ec.com/booksmk/internal/server/feedhandler"
	"go.e64ec.com/booksmk/internal/server/urlhandler"
	"go.e64ec.com/booksmk/internal/server/userhandler"
	"go.e64ec.com/booksmk/internal/store"
)

// Config holds server configuration.
type Config struct {
	Addr   string
	Pool   *pgxpool.Pool
	Logger *slog.Logger
}

// Server is the booksmk HTTP server.
type Server struct {
	cfg              Config
	store            *store.Store
	logger           *slog.Logger
	mux              *http.ServeMux
	urlHandler       *urlhandler.Handler
	userHandler      *userhandler.Handler
	apiKeyHandler    *apikeyhandler.Handler
	apiHandler       *apihandler.Handler
	adminHandler     *adminhandler.Handler
	feedHandler      *feedhandler.Handler
	discussionWorker *discuss.Worker
	feedWorker       *feedworker.Worker
}

func New(cfg Config) (*Server, error) {
	st := store.New(cfg.Pool)

	s := &Server{
		cfg:              cfg,
		store:            st,
		logger:           cfg.Logger,
		mux:              http.NewServeMux(),
		urlHandler:       urlhandler.New(st, st, cfg.Logger),
		userHandler:      userhandler.New(st, cfg.Logger),
		apiKeyHandler:    apikeyhandler.New(st, cfg.Logger),
		apiHandler:       apihandler.New(st, st, cfg.Logger),
		adminHandler:     adminhandler.New(st, cfg.Logger),
		feedHandler:      feedhandler.New(st, cfg.Logger),
		discussionWorker: discuss.New(st, cfg.Logger),
		feedWorker:       feedworker.New(st, cfg.Logger),
	}

	s.registerRoutes()

	return s, nil
}

// Run starts the HTTP server and blocks until ctx is cancelled, then shuts down gracefully.
func (s *Server) Run(ctx context.Context) error {
	srv := &http.Server{
		Addr:    s.cfg.Addr,
		Handler: methodOverride(s.mux),
		BaseContext: func(l net.Listener) context.Context {
			return ctx
		},
	}

	errCh := make(chan error, 1)
	go func() {
		s.logger.Info("listening", "addr", s.cfg.Addr)
		errCh <- srv.ListenAndServe()
	}()

	s.logger.Info("starting discussion worker")
	go s.discussionWorker.Run(ctx)

	s.logger.Info("starting feed worker")
	go s.feedWorker.Run(ctx)

	select {
	case <-ctx.Done():
		s.logger.Info("shutting down")
		return srv.Shutdown(context.Background())
	case err := <-errCh:
		return err
	}
}
