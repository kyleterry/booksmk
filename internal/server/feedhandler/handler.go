package feedhandler

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/a-h/templ"
	"github.com/google/uuid"
	"github.com/mmcdole/gofeed"

	"go.e64ec.com/booksmk/internal/auth"
	"go.e64ec.com/booksmk/internal/store"
	"go.e64ec.com/booksmk/internal/ui"
	feedpages "go.e64ec.com/booksmk/internal/ui/feeds"
)

const pageSize = 50

type feedStore interface {
	SubscribeToFeed(ctx context.Context, userID uuid.UUID, feedURL string, tags []string, isBlockedBypass bool) (store.Feed, error)
	GetFeed(ctx context.Context, id, userID uuid.UUID) (store.Feed, error)
	ListFeeds(ctx context.Context, userID uuid.UUID) ([]store.Feed, error)
	UnsubscribeFromFeed(ctx context.Context, userID, feedID uuid.UUID) error
	UpdateFeed(ctx context.Context, feedID, userID uuid.UUID, customName string, tags []string) (store.Feed, error)
	ListFeedItems(ctx context.Context, feedID, userID uuid.UUID) ([]store.FeedItem, error)
	ListTimelineItems(ctx context.Context, userID uuid.UUID, limit, offset int) ([]store.TimelineItem, error)
	GetTimelineItem(ctx context.Context, userID, itemID uuid.UUID) (store.TimelineItem, error)
	MarkItemRead(ctx context.Context, userID, itemID uuid.UUID) error
	MarkItemUnread(ctx context.Context, userID, itemID uuid.UUID) error
	MarkAllItemsRead(ctx context.Context, userID uuid.UUID) error
	MarkFeedItemsRead(ctx context.Context, userID, feedID uuid.UUID) error
	IsBlocked(ctx context.Context, rawURL string) (bool, error)
}

type contextKey int

const feedKey contextKey = iota

func withFeed(ctx context.Context, f store.Feed) context.Context {
	return context.WithValue(ctx, feedKey, f)
}

func feedFromContext(ctx context.Context) store.Feed {
	f, _ := ctx.Value(feedKey).(store.Feed)
	return f
}

// Handler handles all requests under the /feed prefix.
type Handler struct {
	store  feedStore
	parser *gofeed.Parser
	logger *slog.Logger
	mux    *http.ServeMux
}

func New(s feedStore, logger *slog.Logger) *Handler {
	h := &Handler{
		store:  s,
		parser: gofeed.NewParser(),
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
	h.mux.HandleFunc("GET /feed", h.handleTimeline)
	h.mux.HandleFunc("GET /feed/new", h.handleNew)
	h.mux.HandleFunc("POST /feed", h.handleCreate)
	h.mux.Handle("GET /feed/{id}", h.requireFeedOwner(http.HandlerFunc(h.handleGet)))
	h.mux.Handle("GET /feed/{id}/edit", h.requireFeedOwner(http.HandlerFunc(h.handleEdit)))
	h.mux.Handle("PUT /feed/{id}", h.requireFeedOwner(http.HandlerFunc(h.handleUpdate)))
	h.mux.Handle("DELETE /feed/{id}", h.requireFeedOwner(http.HandlerFunc(h.handleDelete)))
	h.mux.HandleFunc("POST /feed/items/read-all", h.handleMarkAllRead)
	h.mux.HandleFunc("POST /feed/items/{itemID}/read", h.handleMarkRead)
	h.mux.HandleFunc("DELETE /feed/items/{itemID}/read", h.handleMarkUnread)
	h.mux.Handle("POST /feed/{id}/read-all", h.requireFeedOwner(http.HandlerFunc(h.handleMarkFeedAllRead)))
}

func (h *Handler) requireFeedOwner(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, _ := auth.UserFromContext(r.Context())

		id, err := uuid.Parse(r.PathValue("id"))
		if err != nil {
			http.Error(w, "invalid id", http.StatusBadRequest)
			return
		}

		f, err := h.store.GetFeed(r.Context(), id, user.ID)
		if errors.Is(err, store.ErrNotFound) {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		if err != nil {
			h.logger.Error("failed to get feed", "id", id, "error", err)
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}

		next.ServeHTTP(w, r.WithContext(withFeed(r.Context(), f)))
	})
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

func (h *Handler) handleTimeline(w http.ResponseWriter, r *http.Request) {
	user, _ := auth.UserFromContext(r.Context())

	page := 1
	if p, err := strconv.Atoi(r.URL.Query().Get("page")); err == nil && p > 1 {
		page = p
	}
	offset := (page - 1) * pageSize

	// fetch one extra to detect whether another page exists
	items, err := h.store.ListTimelineItems(r.Context(), user.ID, pageSize+1, offset)
	if err != nil {
		h.logger.Error("failed to list timeline items", "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	hasMore := len(items) > pageSize
	if hasMore {
		items = items[:pageSize]
	}

	feeds, err := h.store.ListFeeds(r.Context(), user.ID)
	if err != nil {
		h.logger.Error("failed to list feeds", "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	groups := groupTimeline(items, h.nowInRequestTZ(r))
	h.render(w, r, ui.Base("feeds", h.navUser(r), feedpages.TimelinePage(feeds, groups, page, hasMore)))
}

func (h *Handler) handleNew(w http.ResponseWriter, r *http.Request) {
	prefill := r.URL.Query().Get("url")
	h.render(w, r, ui.Base("add feed", h.navUser(r), feedpages.NewFeedPage("", prefill)))
}

func (h *Handler) handleCreate(w http.ResponseWriter, r *http.Request) {
	user, _ := auth.UserFromContext(r.Context())

	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	feedURL := r.FormValue("feed_url")
	if feedURL == "" {
		h.render(w, r, ui.Base("add feed", h.navUser(r), feedpages.NewFeedPage("feed url is required", "")))
		return
	}

	// Validate the URL parses correctly.
	if _, err := url.ParseRequestURI(feedURL); err != nil {
		h.render(w, r, ui.Base("add feed", h.navUser(r), feedpages.NewFeedPage("invalid feed url", feedURL)))
		return
	}

	isBlocked, err := h.store.IsBlocked(r.Context(), feedURL)
	if err != nil {
		h.logger.Error("failed to check blocklist", "error", err)
	}

	if isBlocked && !user.IsAdmin {
		h.render(w, r, ui.Base("add feed", h.navUser(r), feedpages.NewFeedPage("this URL or domain is blocked", feedURL)))
		return
	}

	// Attempt to parse the feed to confirm it's valid before subscribing.
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()

	parsed, err := h.parser.ParseURLWithContext(feedURL, ctx)
	if err != nil {
		h.logger.Info("could not parse feed", "url", feedURL, "error", err)
		h.render(w, r, ui.Base("add feed", h.navUser(r), feedpages.NewFeedPage("could not fetch or parse feed - check the url", feedURL)))
		return
	}

	tags := parseTags(r.FormValue("tags"))
	if len(tags) == 0 {
		for _, cat := range parsed.Categories {
			if t := store.Slug(cat); t != "" {
				tags = append(tags, t)
			}
		}
	}
	f, err := h.store.SubscribeToFeed(r.Context(), user.ID, feedURL, tags, isBlocked)
	if err != nil {
		h.logger.Error("failed to subscribe to feed", "url", feedURL, "error", err)
		h.render(w, r, ui.Base("add feed", h.navUser(r), feedpages.NewFeedPage("failed to subscribe to feed", feedURL)))
		return
	}

	http.Redirect(w, r, "/feed/"+f.ID.String(), http.StatusSeeOther)
}

func (h *Handler) handleGet(w http.ResponseWriter, r *http.Request) {
	f := feedFromContext(r.Context())
	user, _ := auth.UserFromContext(r.Context())

	items, err := h.store.ListFeedItems(r.Context(), f.ID, user.ID)
	if err != nil {
		h.logger.Error("failed to list feed items", "feed_id", f.ID, "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	groups := groupFeedItems(items, h.nowInRequestTZ(r))
	h.render(w, r, ui.Base(f.Title, h.navUser(r), feedpages.FeedDetailPage(f, groups)))
}

func (h *Handler) handleEdit(w http.ResponseWriter, r *http.Request) {
	f := feedFromContext(r.Context())
	h.render(w, r, ui.Base("edit feed", h.navUser(r), feedpages.EditFeedPage(f, "")))
}

func (h *Handler) handleUpdate(w http.ResponseWriter, r *http.Request) {
	f := feedFromContext(r.Context())
	user, _ := auth.UserFromContext(r.Context())

	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	customName := r.FormValue("custom_name")
	tags := parseTags(r.FormValue("tags"))

	if _, err := h.store.UpdateFeed(r.Context(), f.ID, user.ID, customName, tags); err != nil {
		h.logger.Error("failed to update feed", "id", f.ID, "error", err)
		h.render(w, r, ui.Base("edit feed", h.navUser(r), feedpages.EditFeedPage(f, "failed to save changes")))
		return
	}

	http.Redirect(w, r, "/feed/"+f.ID.String(), http.StatusSeeOther)
}

func (h *Handler) handleDelete(w http.ResponseWriter, r *http.Request) {
	f := feedFromContext(r.Context())
	user, _ := auth.UserFromContext(r.Context())

	if err := h.store.UnsubscribeFromFeed(r.Context(), user.ID, f.ID); err != nil {
		h.logger.Error("failed to unsubscribe from feed", "feed_id", f.ID, "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/feed", http.StatusSeeOther)
}

func (h *Handler) handleMarkRead(w http.ResponseWriter, r *http.Request) {
	user, _ := auth.UserFromContext(r.Context())

	itemID, err := uuid.Parse(r.PathValue("itemID"))
	if err != nil {
		http.Error(w, "invalid item id", http.StatusBadRequest)
		return
	}

	if err := h.store.MarkItemRead(r.Context(), user.ID, itemID); err != nil {
		h.logger.Error("failed to mark item read", "item_id", itemID, "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	if r.Header.Get("HX-Request") == "true" {
		h.renderItemFragment(w, r, user.ID, itemID)
		return
	}

	http.Redirect(w, r, safeReferer(r), http.StatusSeeOther)
}

func (h *Handler) handleMarkUnread(w http.ResponseWriter, r *http.Request) {
	user, _ := auth.UserFromContext(r.Context())

	itemID, err := uuid.Parse(r.PathValue("itemID"))
	if err != nil {
		http.Error(w, "invalid item id", http.StatusBadRequest)
		return
	}

	if err := h.store.MarkItemUnread(r.Context(), user.ID, itemID); err != nil {
		h.logger.Error("failed to mark item unread", "item_id", itemID, "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	if r.Header.Get("HX-Request") == "true" {
		h.renderItemFragment(w, r, user.ID, itemID)
		return
	}

	http.Redirect(w, r, safeReferer(r), http.StatusSeeOther)
}

func (h *Handler) handleMarkAllRead(w http.ResponseWriter, r *http.Request) {
	user, _ := auth.UserFromContext(r.Context())

	if err := h.store.MarkAllItemsRead(r.Context(), user.ID); err != nil {
		h.logger.Error("failed to mark all items read", "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	if r.Header.Get("HX-Request") == "true" {
		w.Header().Set("HX-Redirect", "/feed")
		w.WriteHeader(http.StatusNoContent)
		return
	}

	http.Redirect(w, r, "/feed", http.StatusSeeOther)
}

func (h *Handler) handleMarkFeedAllRead(w http.ResponseWriter, r *http.Request) {
	user, _ := auth.UserFromContext(r.Context())

	f := feedFromContext(r.Context())

	if err := h.store.MarkFeedItemsRead(r.Context(), user.ID, f.ID); err != nil {
		h.logger.Error("failed to mark feed items read", "feed_id", f.ID, "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	redirect := "/feed/" + f.ID.String()

	if r.Header.Get("HX-Request") == "true" {
		w.Header().Set("HX-Redirect", redirect)
		w.WriteHeader(http.StatusNoContent)
		return
	}

	http.Redirect(w, r, redirect, http.StatusSeeOther)
}

func (h *Handler) renderItemFragment(w http.ResponseWriter, r *http.Request, userID, itemID uuid.UUID) {
	item, err := h.store.GetTimelineItem(r.Context(), userID, itemID)
	if err != nil {
		h.logger.Error("failed to get timeline item for fragment", "item_id", itemID, "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	h.render(w, r, feedpages.TimelineItemCard(item))
}

// nowInRequestTZ returns the current time in the timezone reported by the
// browser via the "tz" cookie. Falls back to UTC if the cookie is absent or
// the timezone name is unrecognised.
func (h *Handler) nowInRequestTZ(r *http.Request) time.Time {
	c, err := r.Cookie("tz")
	if err != nil {
		return time.Now().UTC()
	}

	name, err := url.QueryUnescape(c.Value)
	if err != nil {
		h.logger.Error("failed to unescape tz cookie", "value", c.Value, "error", err)
		return time.Now().UTC()
	}

	loc, err := time.LoadLocation(name)
	if err != nil {
		h.logger.Error("failed to load location", "name", name, "error", err)
		return time.Now().UTC()
	}

	return time.Now().In(loc)
}

// safeReferer returns the request referer if it is a relative path, otherwise /feed.
func safeReferer(r *http.Request) string {
	ref := r.Referer()
	if ref == "" {
		return "/feed"
	}
	u, err := url.Parse(ref)
	if err != nil || u.Host != "" {
		return "/feed"
	}
	return ref
}

// parseTags splits a comma-separated tag string into slugified, non-empty names.
func parseTags(raw string) []string {
	if raw == "" {
		return []string{}
	}
	parts := strings.Split(raw, ",")
	var tags []string
	for _, p := range parts {
		if t := store.Slug(strings.TrimSpace(p)); t != "" {
			tags = append(tags, t)
		}
	}
	return tags
}

