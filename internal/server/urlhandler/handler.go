package urlhandler

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/a-h/templ"
	"github.com/google/uuid"

	"go.e64ec.com/booksmk/internal/reqctx"
	"go.e64ec.com/booksmk/internal/store"
	"go.e64ec.com/booksmk/internal/ui"
	urlpages "go.e64ec.com/booksmk/internal/ui/urls"
	"go.e64ec.com/booksmk/internal/urlfetch"
)

type urlStore interface {
	GetURL(ctx context.Context, id, userID uuid.UUID) (store.URL, error)
	ListURLs(ctx context.Context, userID uuid.UUID) ([]store.URL, error)
	ListURLsByTag(ctx context.Context, userID uuid.UUID, tag string) ([]store.URL, error)
	CreateURL(ctx context.Context, userID uuid.UUID, rawURL, title, description string, tags []string) (store.URL, error)
	UpdateURL(ctx context.Context, id, userID uuid.UUID, title, description string, tags []string) (store.URL, error)
	DeleteURL(ctx context.Context, id, userID uuid.UUID) error
	ListDiscussionsForURL(ctx context.Context, urlID uuid.UUID) ([]store.Discussion, error)
	SetURLFeedURL(ctx context.Context, id uuid.UUID, feedURL string) error
}

type feedQueryStore interface {
	GetFeedByURL(ctx context.Context, userID uuid.UUID, feedURL string) (store.Feed, error)
}

type contextKey int

const urlKey contextKey = iota

func withURL(ctx context.Context, u store.URL) context.Context {
	return context.WithValue(ctx, urlKey, u)
}

func urlFromContext(ctx context.Context) store.URL {
	u, _ := ctx.Value(urlKey).(store.URL)
	return u
}

// Handler handles all requests under the /url prefix.
type Handler struct {
	store     urlStore
	feedStore feedQueryStore
	logger    *slog.Logger
	mux       *http.ServeMux
}

func New(s urlStore, fs feedQueryStore, logger *slog.Logger) *Handler {
	h := &Handler{
		store:     s,
		feedStore: fs,
		logger:    logger,
		mux:       http.NewServeMux(),
	}
	h.registerRoutes()
	return h
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.mux.ServeHTTP(w, r)
}

func (h *Handler) registerRoutes() {
	h.mux.HandleFunc("GET /url", h.handleList)
	h.mux.HandleFunc("GET /url/new", h.handleNew)
	h.mux.HandleFunc("POST /url", h.handleCreate)
	h.mux.Handle("GET /url/{id}", h.requireURLOwner(http.HandlerFunc(h.handleGet)))
	h.mux.Handle("GET /url/{id}/edit", h.requireURLOwner(http.HandlerFunc(h.handleEdit)))
	h.mux.Handle("POST /url/{id}", h.requireURLOwner(http.HandlerFunc(h.handleUpdate)))
	h.mux.Handle("DELETE /url/{id}", h.requireURLOwner(http.HandlerFunc(h.handleDelete)))
}

// requireURLOwner fetches the URL scoped to the authenticated user (implicitly
// verifying ownership), then injects it into the request context so handlers
// don't need to re-fetch it. Returns 404 for both missing and unowned URLs.
func (h *Handler) requireURLOwner(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, ok := reqctx.User(r.Context())
		if !ok {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		id, err := pathUUID(r)
		if err != nil {
			http.Error(w, "invalid id", http.StatusBadRequest)
			return
		}

		u, err := h.store.GetURL(r.Context(), id, user.ID)
		if errors.Is(err, store.ErrNotFound) {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		if err != nil {
			h.logger.Error("failed to get url", "id", id, "error", err)
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}

		next.ServeHTTP(w, r.WithContext(withURL(r.Context(), u)))
	})
}

func (h *Handler) render(w http.ResponseWriter, r *http.Request, c templ.Component) {
	if err := c.Render(r.Context(), w); err != nil {
		h.logger.Error("render failed", "error", err)
	}
}

func (h *Handler) navUser(r *http.Request) *ui.NavUser {
	u, ok := reqctx.User(r.Context())
	if !ok {
		return nil
	}
	return &ui.NavUser{ID: u.ID.String(), Email: u.Email, IsAdmin: u.IsAdmin, Theme: u.Theme, FontSize: u.FontSize}
}

func (h *Handler) handleList(w http.ResponseWriter, r *http.Request) {
	user, ok := reqctx.User(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	if tag := r.URL.Query().Get("tag"); tag != "" {
		urls, err := h.store.ListURLsByTag(r.Context(), user.ID, tag)
		if err != nil {
			h.logger.Error("failed to list urls by tag", "tag", tag, "error", err)
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}
		h.render(w, r, ui.Base("tag: "+tag, h.navUser(r), urlpages.TagPage(tag, urls)))
		return
	}

	urls, err := h.store.ListURLs(r.Context(), user.ID)
	if err != nil {
		h.logger.Error("failed to list urls", "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	h.render(w, r, ui.Base("urls", h.navUser(r), urlpages.ListPage(urls)))
}

func (h *Handler) handleNew(w http.ResponseWriter, r *http.Request) {
	h.render(w, r, ui.Base("add url", h.navUser(r), urlpages.NewPage("")))
}

func (h *Handler) handleCreate(w http.ResponseWriter, r *http.Request) {
	user, ok := reqctx.User(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	rawURL := r.FormValue("url")
	if rawURL == "" {
		h.render(w, r, ui.Base("add url", h.navUser(r), urlpages.NewPage("url is required")))
		return
	}

	title := r.FormValue("title")
	tags := parseTags(r.FormValue("tags"))

	meta := urlfetch.Fetch(rawURL)
	if title == "" {
		title = meta.Title
	}
	if len(tags) == 0 {
		tags = meta.Tags
	}

	u, err := h.store.CreateURL(r.Context(), user.ID, rawURL, title, r.FormValue("description"), tags)
	if err != nil {
		h.logger.Error("failed to create url", "error", err)
		h.render(w, r, ui.Base("add url", h.navUser(r), urlpages.NewPage("failed to save url")))
		return
	}

	if meta.FeedURL != "" {
		if err := h.store.SetURLFeedURL(r.Context(), u.ID, meta.FeedURL); err != nil {
			h.logger.Warn("failed to set feed url", "url_id", u.ID, "error", err)
		}
	}

	http.Redirect(w, r, "/url/"+u.ID.String(), http.StatusSeeOther)
}

func (h *Handler) handleGet(w http.ResponseWriter, r *http.Request) {
	u := urlFromContext(r.Context())
	user, _ := reqctx.User(r.Context())

	discussions, err := h.store.ListDiscussionsForURL(r.Context(), u.ID)
	if err != nil {
		h.logger.Error("failed to list discussions", "url_id", u.ID, "error", err)
		discussions = nil
	}

	var subscribedFeed *store.Feed
	if u.FeedURL != "" {
		f, err := h.feedStore.GetFeedByURL(r.Context(), user.ID, u.FeedURL)
		if err == nil {
			subscribedFeed = &f
		}
	}

	h.render(w, r, ui.Base(u.Title, h.navUser(r), urlpages.DetailPage(u, discussions, subscribedFeed)))
}

func (h *Handler) handleEdit(w http.ResponseWriter, r *http.Request) {
	u := urlFromContext(r.Context())
	h.render(w, r, ui.Base("edit url", h.navUser(r), urlpages.EditPage(u, "")))
}

func (h *Handler) handleUpdate(w http.ResponseWriter, r *http.Request) {
	u := urlFromContext(r.Context())
	user, _ := reqctx.User(r.Context())

	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	if _, err := h.store.UpdateURL(r.Context(), u.ID, user.ID, r.FormValue("title"), r.FormValue("description"), parseTags(r.FormValue("tags"))); err != nil {
		h.logger.Error("failed to update url", "id", u.ID, "error", err)
		h.render(w, r, ui.Base("edit url", h.navUser(r), urlpages.EditPage(u, "failed to save changes")))
		return
	}

	http.Redirect(w, r, "/url/"+u.ID.String(), http.StatusSeeOther)
}

func (h *Handler) handleDelete(w http.ResponseWriter, r *http.Request) {
	u := urlFromContext(r.Context())
	user, _ := reqctx.User(r.Context())

	if err := h.store.DeleteURL(r.Context(), u.ID, user.ID); err != nil {
		h.logger.Error("failed to delete url", "id", u.ID, "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/url", http.StatusSeeOther)
}

func pathUUID(r *http.Request) (uuid.UUID, error) {
	return uuid.Parse(r.PathValue("id"))
}

// parseTags splits a comma-separated tag string into slugified, non-empty names.
func parseTags(raw string) []string {
	if raw == "" {
		return []string{}
	}
	parts := strings.Split(raw, ",")
	tags := make([]string, 0, len(parts))
	for _, p := range parts {
		if t := toSlug(strings.TrimSpace(p)); t != "" {
			tags = append(tags, t)
		}
	}
	return tags
}

// toSlug lowercases s and converts it to a URL-safe slug, replacing spaces and
// non-alphanumeric characters with hyphens and collapsing consecutive hyphens.
func toSlug(s string) string {
	s = strings.ToLower(s)
	var b strings.Builder
	prevHyphen := false
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z' || r >= '0' && r <= '9':
			b.WriteRune(r)
			prevHyphen = false
		case r == ' ' || r == '-' || r == '_':
			if !prevHyphen {
				b.WriteRune('-')
				prevHyphen = true
			}
		}
	}
	return strings.Trim(b.String(), "-")
}
