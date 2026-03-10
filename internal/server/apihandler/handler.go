package apihandler

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/kyleterry/booksmk/internal/reqctx"
	"github.com/kyleterry/booksmk/internal/store"
	"github.com/kyleterry/booksmk/internal/urlfetch"
)

type urlStore interface {
	CreateURL(ctx context.Context, userID uuid.UUID, rawURL, title, description string, tags []string) (store.URL, error)
}

// Handler handles requests under the /api prefix.
type Handler struct {
	store  urlStore
	logger *slog.Logger
	mux    *http.ServeMux
}

func New(s urlStore, logger *slog.Logger) *Handler {
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
	h.mux.HandleFunc("POST /api/urls", h.handleCreateURL)
}

type createURLRequest struct {
	URL         string   `json:"url"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Tags        []string `json:"tags"`
}

type urlResponse struct {
	ID          uuid.UUID `json:"id"`
	URL         string    `json:"url"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Tags        []string  `json:"tags"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func (h *Handler) handleCreateURL(w http.ResponseWriter, r *http.Request) {
	var body createURLRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		jsonError(w, "invalid json", http.StatusBadRequest)
		return
	}

	if body.URL == "" {
		jsonError(w, "url is required", http.StatusBadRequest)
		return
	}
	if body.Tags == nil {
		body.Tags = []string{}
	}
	if body.Title == "" {
		body.Title = urlfetch.FetchTitle(body.URL)
	}

	u, _ := reqctx.User(r.Context())
	created, err := h.store.CreateURL(r.Context(), u.ID, body.URL, body.Title, body.Description, body.Tags)
	if err != nil {
		h.logger.Error("api: failed to create url", "error", err)
		jsonError(w, "internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(urlResponse{
		ID:          created.ID,
		URL:         created.URL,
		Title:       created.Title,
		Description: created.Description,
		Tags:        created.Tags,
		CreatedAt:   created.CreatedAt,
		UpdatedAt:   created.UpdatedAt,
	})
}

func jsonError(w http.ResponseWriter, msg string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
