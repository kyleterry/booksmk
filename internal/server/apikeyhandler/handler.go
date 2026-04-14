package apikeyhandler

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/a-h/templ"
	"github.com/google/uuid"

	"go.e64ec.com/booksmk/internal/auth"
	"go.e64ec.com/booksmk/internal/store"
	"go.e64ec.com/booksmk/internal/ui"
	apikeypages "go.e64ec.com/booksmk/internal/ui/apikeys"
)

type apiKeyStore interface {
	CreateAPIKey(ctx context.Context, userID uuid.UUID, name string, expiresAt *time.Time) (store.NewAPIKeyResult, error)
	ListAPIKeys(ctx context.Context, userID uuid.UUID) ([]store.APIKey, error)
	DeleteAPIKey(ctx context.Context, id, userID uuid.UUID) error
}

// Handler handles all requests under the /apikey prefix.
type Handler struct {
	store  apiKeyStore
	logger *slog.Logger
	mux    *http.ServeMux
}

func New(s apiKeyStore, logger *slog.Logger) *Handler {
	h := &Handler{
		store:  s,
		logger: logger,
		mux:    http.NewServeMux(),
	}
	h.registerRoutes()
	return h
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.mux.ServeHTTP(w, r)
}

func (h *Handler) registerRoutes() {
	h.mux.HandleFunc("GET /apikey/", h.handleList)
	h.mux.HandleFunc("GET /apikey/new", h.handleNew)
	h.mux.HandleFunc("POST /apikey/", h.handleCreate)
	h.mux.HandleFunc("DELETE /apikey/{id}", h.handleDelete)
}

func (h *Handler) render(w http.ResponseWriter, r *http.Request, c templ.Component) {
	if err := c.Render(r.Context(), w); err != nil {
		h.logger.Error("render failed", "error", err)
	}
}

func (h *Handler) navUser(r *http.Request) *ui.NavUser {
	u, ok := auth.UserFromContext(r.Context())
	if !ok {
		return nil
	}
	return &ui.NavUser{ID: u.ID.String(), Email: u.Email, IsAdmin: u.IsAdmin, Theme: u.Theme, FontSize: u.FontSize, ResultsPerPage: u.ResultsPerPage}
}

func (h *Handler) handleList(w http.ResponseWriter, r *http.Request) {
	u, _ := auth.UserFromContext(r.Context())
	keys, err := h.store.ListAPIKeys(r.Context(), u.ID)
	if err != nil {
		h.logger.Error("failed to list api keys", "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	h.render(w, r, ui.Base("api keys", h.navUser(r), apikeypages.ListPage(keys, len(keys) >= 5)))
}

func (h *Handler) handleNew(w http.ResponseWriter, r *http.Request) {
	h.render(w, r, ui.Base("new api key", h.navUser(r), apikeypages.NewPage("")))
}

func (h *Handler) handleCreate(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	name := r.FormValue("name")
	if name == "" {
		h.render(w, r, ui.Base("new api key", h.navUser(r), apikeypages.NewPage("name is required")))
		return
	}

	expiresAt := parseExpiresIn(r.FormValue("expires_in"))

	u, _ := auth.UserFromContext(r.Context())
	result, err := h.store.CreateAPIKey(r.Context(), u.ID, name, expiresAt)
	if errors.Is(err, store.ErrAPIKeyLimitReached) {
		h.render(w, r, ui.Base("new api key", h.navUser(r), apikeypages.NewPage("api key limit reached (max 5)")))
		return
	}
	if err != nil {
		h.logger.Error("failed to create api key", "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	h.render(w, r, ui.Base("api key created", h.navUser(r), apikeypages.CreatedPage(result.APIKey, result.Token, u.ID.String())))
}

func (h *Handler) handleDelete(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	u, _ := auth.UserFromContext(r.Context())
	if err := h.store.DeleteAPIKey(r.Context(), id, u.ID); err != nil {
		h.logger.Error("failed to delete api key", "id", id, "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/user/"+u.ID.String(), http.StatusSeeOther)
}

// parseExpiresIn converts a duration string ("30d", "90d", "1y", "") to a *time.Time.
func parseExpiresIn(s string) *time.Time {
	var d time.Duration
	switch s {
	case "30d":
		d = 30 * 24 * time.Hour
	case "90d":
		d = 90 * 24 * time.Hour
	case "1y":
		d = 365 * 24 * time.Hour
	default:
		return nil
	}
	t := time.Now().Add(d)
	return &t
}
