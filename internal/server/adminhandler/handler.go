package adminhandler

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/a-h/templ"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"go.e64ec.com/booksmk/internal/reqctx"
	"go.e64ec.com/booksmk/internal/store"
	"go.e64ec.com/booksmk/internal/ui"
	adminpages "go.e64ec.com/booksmk/internal/ui/admin"
)

type adminStore interface {
	ListBatchRuns(ctx context.Context) ([]store.BatchRunSummary, error)
	GetNextBatchRunAt(ctx context.Context) (time.Time, error)
	ScheduleBatchRunNow(ctx context.Context) error
	ListInviteCodes(ctx context.Context) ([]store.InviteCode, error)
	CreateInviteCode(ctx context.Context, createdBy uuid.UUID) (store.InviteCode, error)
	DeleteInviteCode(ctx context.Context, id uuid.UUID) error
	ListUsers(ctx context.Context) ([]store.User, error)
	GetUser(ctx context.Context, id uuid.UUID) (store.User, error)
	UpdateUserPassword(ctx context.Context, id uuid.UUID, passwordDigest string) (store.User, error)
}

// Handler handles requests under the /admin prefix.
type Handler struct {
	store  adminStore
	logger *slog.Logger
	mux    *http.ServeMux
}

func New(s adminStore, logger *slog.Logger) *Handler {
	h := &Handler{
		store:  s,
		logger: logger,
		mux:    http.NewServeMux(),
	}
	h.mux.HandleFunc("GET /admin/", h.handleIndex)
	h.mux.HandleFunc("POST /admin/run", h.handleDispatchRun)
	h.mux.HandleFunc("POST /admin/invite/", h.handleCreateInvite)
	h.mux.HandleFunc("DELETE /admin/invite/{id}", h.handleDeleteInvite)
	h.mux.HandleFunc("GET /admin/users", h.handleListUsers)
	h.mux.HandleFunc("GET /admin/users/{id}/reset-password", h.handleResetPasswordForm)
	h.mux.HandleFunc("POST /admin/users/{id}/reset-password", h.handleResetPassword)
	return h
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.mux.ServeHTTP(w, r)
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

func (h *Handler) handleIndex(w http.ResponseWriter, r *http.Request) {
	runs, err := h.store.ListBatchRuns(r.Context())
	if err != nil {
		h.logger.Error("failed to list batch runs", "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	nextAt, err := h.store.GetNextBatchRunAt(r.Context())
	if err != nil {
		h.logger.Error("failed to get next batch run time", "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	codes, err := h.store.ListInviteCodes(r.Context())
	if err != nil {
		h.logger.Error("failed to list invite codes", "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	h.render(w, r, ui.Base("admin", h.navUser(r), adminpages.AdminPage(runs, nextAt, codes)))
}

func (h *Handler) handleDispatchRun(w http.ResponseWriter, r *http.Request) {
	if err := h.store.ScheduleBatchRunNow(r.Context()); err != nil {
		h.logger.Error("failed to schedule batch run", "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/admin/", http.StatusSeeOther)
}

func (h *Handler) handleCreateInvite(w http.ResponseWriter, r *http.Request) {
	u, _ := reqctx.User(r.Context())
	if _, err := h.store.CreateInviteCode(r.Context(), u.ID); err != nil {
		h.logger.Error("failed to create invite code", "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/admin/", http.StatusSeeOther)
}

func (h *Handler) handleDeleteInvite(w http.ResponseWriter, r *http.Request) {
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
	http.Redirect(w, r, "/admin/", http.StatusSeeOther)
}

func (h *Handler) handleListUsers(w http.ResponseWriter, r *http.Request) {
	users, err := h.store.ListUsers(r.Context())
	if err != nil {
		h.logger.Error("failed to list users", "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	h.render(w, r, ui.Base("admin - users", h.navUser(r), adminpages.UsersPage(users)))
}

func (h *Handler) handleResetPasswordForm(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	user, err := h.store.GetUser(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	if err != nil {
		h.logger.Error("failed to get user", "id", id, "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	h.render(w, r, ui.Base("reset password", h.navUser(r), adminpages.ResetPasswordPage(user, "")))
}

func (h *Handler) handleResetPassword(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	user, err := h.store.GetUser(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	if err != nil {
		h.logger.Error("failed to get user", "id", id, "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	password := r.FormValue("password")
	if password == "" {
		h.render(w, r, ui.Base("reset password", h.navUser(r), adminpages.ResetPasswordPage(user, "password is required")))
		return
	}

	digest, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		h.logger.Error("failed to hash password", "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	if _, err := h.store.UpdateUserPassword(r.Context(), id, string(digest)); err != nil {
		h.logger.Error("failed to update password", "id", id, "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/admin/users", http.StatusSeeOther)
}
