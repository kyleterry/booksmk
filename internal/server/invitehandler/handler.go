package invitehandler

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/a-h/templ"
	"github.com/google/uuid"

	"go.e64ec.com/booksmk/internal/reqctx"
	"go.e64ec.com/booksmk/internal/store"
	"go.e64ec.com/booksmk/internal/ui"
	invitepages "go.e64ec.com/booksmk/internal/ui/invites"
)

type inviteStore interface {
	CreateInviteCode(ctx context.Context, createdBy uuid.UUID) (store.InviteCode, error)
	ListInviteCodes(ctx context.Context) ([]store.InviteCode, error)
	DeleteInviteCode(ctx context.Context, id uuid.UUID) error
}

// Handler handles all requests under the /invite prefix.
type Handler struct {
	store  inviteStore
	logger *slog.Logger
	mux    *http.ServeMux
}

func New(s inviteStore, logger *slog.Logger) *Handler {
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
	h.mux.HandleFunc("GET /invite/", h.handleList)
	h.mux.HandleFunc("POST /invite/", h.handleCreate)
	h.mux.HandleFunc("DELETE /invite/{id}", h.handleDelete)
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
	codes, err := h.store.ListInviteCodes(r.Context())
	if err != nil {
		h.logger.Error("failed to list invite codes", "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	h.render(w, r, ui.Base("invite codes", h.navUser(r), invitepages.ListPage(codes)))
}

func (h *Handler) handleCreate(w http.ResponseWriter, r *http.Request) {
	u, _ := reqctx.User(r.Context())
	if _, err := h.store.CreateInviteCode(r.Context(), u.ID); err != nil {
		h.logger.Error("failed to create invite code", "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/invite/", http.StatusSeeOther)
}

func (h *Handler) handleDelete(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	if err := h.store.DeleteInviteCode(r.Context(), id); err != nil {
		h.logger.Error("failed to delete invite code", "id", id, "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/invite/", http.StatusSeeOther)
}
