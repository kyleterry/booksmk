package categoryhandler

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/a-h/templ"
	"github.com/google/uuid"

	"go.e64ec.com/booksmk/internal/auth"
	"go.e64ec.com/booksmk/internal/store"
	"go.e64ec.com/booksmk/internal/ui"
	catpages "go.e64ec.com/booksmk/internal/ui/categories"
)

type categoryStore interface {
	GetCategory(ctx context.Context, id, userID uuid.UUID) (store.Category, error)
	ListCategories(ctx context.Context, userID uuid.UUID) ([]store.Category, error)
	CreateCategory(ctx context.Context, userID uuid.UUID, name string, members []store.CategoryMember) (store.Category, error)
	UpdateCategory(ctx context.Context, id, userID uuid.UUID, name string, members []store.CategoryMember) (store.Category, error)
	DeleteCategory(ctx context.Context, id, userID uuid.UUID) error
}

// Handler handles all requests under the /category prefix.
type Handler struct {
	store  categoryStore
	logger *slog.Logger
	mux    *http.ServeMux
}

func New(s categoryStore, logger *slog.Logger) *Handler {
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
	h.mux.HandleFunc("GET /category/new", h.handleNew)
	h.mux.HandleFunc("POST /category", h.handleCreate)
	h.mux.HandleFunc("GET /category/{id}/edit", h.handleEdit)
	h.mux.HandleFunc("PUT /category/{id}", h.handleUpdate)
	h.mux.HandleFunc("DELETE /category/{id}", h.handleDelete)
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

func (h *Handler) handleNew(w http.ResponseWriter, r *http.Request) {
	h.render(w, r, ui.Base("new category", h.navUser(r), catpages.NewPage("")))
}

func (h *Handler) handleCreate(w http.ResponseWriter, r *http.Request) {
	user, _ := auth.UserFromContext(r.Context())

	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	name := strings.TrimSpace(r.FormValue("name"))
	if name == "" {
		h.render(w, r, ui.Base("new category", h.navUser(r), catpages.NewPage("name is required")))
		return
	}

	members := parseMembers(r.FormValue("members"))
	cat, err := h.store.CreateCategory(r.Context(), user.ID, name, members)
	if err != nil {
		h.logger.Error("failed to create category", "error", err)
		h.render(w, r, ui.Base("new category", h.navUser(r), catpages.NewPage("failed to save category")))
		return
	}

	http.Redirect(w, r, "/url?category="+cat.ID.String(), http.StatusSeeOther)
}

func (h *Handler) handleEdit(w http.ResponseWriter, r *http.Request) {
	user, _ := auth.UserFromContext(r.Context())

	id, err := pathUUID(r)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	cat, err := h.store.GetCategory(r.Context(), id, user.ID)
	if errors.Is(err, store.ErrNotFound) {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	if err != nil {
		h.logger.Error("failed to get category", "id", id, "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	h.render(w, r, ui.Base("edit category", h.navUser(r), catpages.EditPage(cat, "")))
}

func (h *Handler) handleUpdate(w http.ResponseWriter, r *http.Request) {
	user, _ := auth.UserFromContext(r.Context())

	id, err := pathUUID(r)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	name := strings.TrimSpace(r.FormValue("name"))
	if name == "" {
		cat, getErr := h.store.GetCategory(r.Context(), id, user.ID)
		if getErr != nil {
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}
		h.render(w, r, ui.Base("edit category", h.navUser(r), catpages.EditPage(cat, "name is required")))
		return
	}

	members := parseMembers(r.FormValue("members"))
	if _, err := h.store.UpdateCategory(r.Context(), id, user.ID, name, members); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		h.logger.Error("failed to update category", "id", id, "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/url?category="+id.String(), http.StatusSeeOther)
}

func (h *Handler) handleDelete(w http.ResponseWriter, r *http.Request) {
	user, _ := auth.UserFromContext(r.Context())

	id, err := pathUUID(r)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	if err := h.store.DeleteCategory(r.Context(), id, user.ID); err != nil {
		h.logger.Error("failed to delete category", "id", id, "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/url", http.StatusSeeOther)
}

func pathUUID(r *http.Request) (uuid.UUID, error) {
	return uuid.Parse(r.PathValue("id"))
}

// parseMembers parses a comma-separated string of member specs like
// "tag:linux, tag:raspberry-pi, domain:github.com" into CategoryMember slices.
// Bare words and tag: prefixed values are slugified; domain: values are kept as-is.
func parseMembers(raw string) []store.CategoryMember {
	if raw == "" {
		return []store.CategoryMember{}
	}
	parts := strings.Split(raw, ",")
	members := make([]store.CategoryMember, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		var kind store.CategoryMemberKind
		var value string

		switch {
		case strings.HasPrefix(p, "tag:"):
			kind = store.CategoryMemberKindTag
			value = store.Slug(strings.TrimSpace(strings.TrimPrefix(p, "tag:")))
		case strings.HasPrefix(p, "domain:"):
			kind = store.CategoryMemberKindDomain
			value = strings.TrimSpace(strings.TrimPrefix(p, "domain:"))
		default:
			kind = store.CategoryMemberKindTag
			value = store.Slug(p)
		}
		if value != "" {
			members = append(members, store.CategoryMember{Kind: kind, Value: value})
		}
	}
	return members
}

