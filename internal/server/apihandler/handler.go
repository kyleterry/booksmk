package apihandler

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"

	"go.e64ec.com/booksmk/internal/auth"
	"go.e64ec.com/booksmk/internal/store"
	"go.e64ec.com/booksmk/internal/urlfetch"
)

type urlStore interface {
	CreateURL(ctx context.Context, userID uuid.UUID, rawURL, title, description string, tags []string, isBlockedBypass bool) (store.URL, error)
	IsBlocked(ctx context.Context, rawURL string) (bool, error)
}

type feedItemStore interface {
	MarkItemRead(ctx context.Context, userID, itemID uuid.UUID) error
	MarkItemUnread(ctx context.Context, userID, itemID uuid.UUID) error
	MarkAllItemsRead(ctx context.Context, userID uuid.UUID) error
	MarkFeedItemsRead(ctx context.Context, userID, feedID uuid.UUID) error
}

// Handler handles requests under the /api prefix.
type Handler struct {
	store         urlStore
	feedItemStore feedItemStore
	logger        *slog.Logger
	mux           *http.ServeMux
}

func New(s urlStore, f feedItemStore, logger *slog.Logger) *Handler {
	h := &Handler{
		store:         s,
		feedItemStore: f,
		logger:        logger,
		mux:           http.NewServeMux(),
	}
	h.registerRoutes()
	return h
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.mux.ServeHTTP(w, r)
}

func (h *Handler) registerRoutes() {
	h.mux.HandleFunc("POST /api/urls", h.handleCreateURL)
	h.mux.HandleFunc("POST /api/feed/items/read-all", h.handleMarkAllItemsRead)
	h.mux.HandleFunc("POST /api/feed/{feedID}/read-all", h.handleMarkFeedItemsRead)
	h.mux.HandleFunc("POST /api/feed/items/{itemID}/read", h.handleMarkItemRead)
	h.mux.HandleFunc("DELETE /api/feed/items/{itemID}/read", h.handleMarkItemUnread)
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
	if body.Title == "" || len(body.Tags) == 0 {
		meta := urlfetch.Fetch(body.URL)
		if body.Title == "" {
			body.Title = meta.Title
		}
		if len(body.Tags) == 0 {
			body.Tags = meta.Tags
		}
	}
	if body.Tags == nil {
		body.Tags = []string{}
	}

	u, _ := auth.UserFromContext(r.Context())

	isBlocked, err := h.store.IsBlocked(r.Context(), body.URL)
	if err != nil {
		h.logger.Error("api: failed to check blocklist", "error", err)
	}

	if isBlocked && !u.IsAdmin {
		jsonError(w, "this URL or domain is blocked", http.StatusForbidden)
		return
	}

	created, err := h.store.CreateURL(r.Context(), u.ID, body.URL, body.Title, body.Description, body.Tags, isBlocked)
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
		CreatedAt:   created.CreatedAt.Time,
		UpdatedAt:   created.UpdatedAt.Time,
	})
}

func (h *Handler) handleMarkItemRead(w http.ResponseWriter, r *http.Request) {
	u, _ := auth.UserFromContext(r.Context())

	itemID, err := uuid.Parse(r.PathValue("itemID"))
	if err != nil {
		jsonError(w, "invalid item id", http.StatusBadRequest)
		return
	}

	if err := h.feedItemStore.MarkItemRead(r.Context(), u.ID, itemID); err != nil {
		h.logger.Error("api: failed to mark item read", "item_id", itemID, "error", err)
		jsonError(w, "internal server error", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) handleMarkItemUnread(w http.ResponseWriter, r *http.Request) {
	u, _ := auth.UserFromContext(r.Context())

	itemID, err := uuid.Parse(r.PathValue("itemID"))
	if err != nil {
		jsonError(w, "invalid item id", http.StatusBadRequest)
		return
	}

	if err := h.feedItemStore.MarkItemUnread(r.Context(), u.ID, itemID); err != nil {
		h.logger.Error("api: failed to mark item unread", "item_id", itemID, "error", err)
		jsonError(w, "internal server error", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) handleMarkAllItemsRead(w http.ResponseWriter, r *http.Request) {
	u, _ := auth.UserFromContext(r.Context())

	if err := h.feedItemStore.MarkAllItemsRead(r.Context(), u.ID); err != nil {
		h.logger.Error("api: failed to mark all items read", "error", err)
		jsonError(w, "internal server error", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) handleMarkFeedItemsRead(w http.ResponseWriter, r *http.Request) {
	u, _ := auth.UserFromContext(r.Context())

	feedID, err := uuid.Parse(r.PathValue("feedID"))
	if err != nil {
		jsonError(w, "invalid feed id", http.StatusBadRequest)
		return
	}

	if err := h.feedItemStore.MarkFeedItemsRead(r.Context(), u.ID, feedID); err != nil {
		h.logger.Error("api: failed to mark feed items read", "feed_id", feedID, "error", err)
		jsonError(w, "internal server error", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func jsonError(w http.ResponseWriter, msg string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
