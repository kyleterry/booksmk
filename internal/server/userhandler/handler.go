package userhandler

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/a-h/templ"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"go.e64ec.com/booksmk/internal/reqctx"
	"go.e64ec.com/booksmk/internal/store"
	"go.e64ec.com/booksmk/internal/ui"
	userpages "go.e64ec.com/booksmk/internal/ui/users"
)

type userStore interface {
	CountUsers(ctx context.Context) (int64, error)
	CreateUser(ctx context.Context, email, passwordDigest string, isAdmin bool) (store.User, error)
	GetUser(ctx context.Context, id uuid.UUID) (store.User, error)
	ListUsers(ctx context.Context) ([]store.User, error)
	UpdateUser(ctx context.Context, id uuid.UUID, email string) (store.User, error)
	UpdateUserPassword(ctx context.Context, id uuid.UUID, passwordDigest string) (store.User, error)
	UpdateUserTheme(ctx context.Context, id uuid.UUID, theme string) (store.User, error)
	UpdateUserFontSize(ctx context.Context, id uuid.UUID, fontSize string) (store.User, error)
	DeleteUser(ctx context.Context, id uuid.UUID) error
	GetInviteCodeByCode(ctx context.Context, code string) (store.InviteCode, error)
	UseInviteCode(ctx context.Context, id, usedBy uuid.UUID) error
	ListAPIKeys(ctx context.Context, userID uuid.UUID) ([]store.APIKey, error)
}

// Handler handles all requests under the /user prefix.
type Handler struct {
	store  userStore
	logger *slog.Logger
	mux    *http.ServeMux
}

func New(s userStore, logger *slog.Logger) *Handler {
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
	h.mux.HandleFunc("GET /user/new", h.handleNew)
	h.mux.HandleFunc("POST /user", h.handleCreate)
	h.mux.HandleFunc("GET /user/{id}", h.handleGet)
	h.mux.HandleFunc("GET /user/{id}/edit", h.handleEdit)
	h.mux.HandleFunc("POST /user/{id}", h.handleUpdate)
	h.mux.HandleFunc("DELETE /user/{id}", h.handleDelete)
	h.mux.HandleFunc("GET /user/{id}/change-password", h.handleChangePasswordForm)
	h.mux.HandleFunc("POST /user/{id}/change-password", h.handleChangePassword)
	h.mux.HandleFunc("POST /user/{id}/theme", h.handleUpdateTheme)
	h.mux.HandleFunc("POST /user/{id}/font-size", h.handleUpdateFontSize)
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

func (h *Handler) requireInviteCode(ctx context.Context) bool {
	count, err := h.store.CountUsers(ctx)
	if err != nil {
		return true
	}
	return count > 0
}

func (h *Handler) handleNew(w http.ResponseWriter, r *http.Request) {
	h.render(w, r, ui.Base("create account", nil, userpages.RegisterPage("", h.requireInviteCode(r.Context()))))
}

func (h *Handler) handleCreate(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	needsInvite := h.requireInviteCode(r.Context())
	email := r.FormValue("email")
	password := r.FormValue("password")
	inviteCode := r.FormValue("invite_code")

	if email == "" || password == "" {
		h.render(w, r, ui.Base("create account", nil, userpages.RegisterPage("email and password are required", needsInvite)))
		return
	}

	if needsInvite && inviteCode == "" {
		h.render(w, r, ui.Base("create account", nil, userpages.RegisterPage("invite code is required", needsInvite)))
		return
	}

	var invite store.InviteCode
	if needsInvite {
		var err error
		invite, err = h.store.GetInviteCodeByCode(r.Context(), inviteCode)
		if err != nil || invite.UsedBy != nil {
			h.render(w, r, ui.Base("create account", nil, userpages.RegisterPage("invalid or already used invite code", needsInvite)))
			return
		}
	}

	digest, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		h.logger.Error("failed to hash password", "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	// The first user is automatically an admin.
	isAdmin := !needsInvite

	user, err := h.store.CreateUser(r.Context(), email, string(digest), isAdmin)
	if err != nil {
		h.logger.Error("failed to create user", "error", err)
		h.render(w, r, ui.Base("create account", nil, userpages.RegisterPage("failed to create account", needsInvite)))
		return
	}

	if needsInvite {
		if err := h.store.UseInviteCode(r.Context(), invite.ID, user.ID); err != nil {
			h.logger.Error("failed to mark invite code as used", "error", err)
		}
	}

	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

func (h *Handler) handleGet(w http.ResponseWriter, r *http.Request) {
	id, err := pathUUID(r)
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

	keys, err := h.store.ListAPIKeys(r.Context(), id)
	if err != nil {
		h.logger.Error("failed to list api keys", "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	scheme := "https"
	if r.TLS == nil {
		scheme = "http"
	}
	baseURL := scheme + "://" + r.Host

	h.render(w, r, ui.Base("settings", h.navUser(r), userpages.UserDetailPage(user, keys, baseURL)))
}

func (h *Handler) handleEdit(w http.ResponseWriter, r *http.Request) {
	id, err := pathUUID(r)
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

	h.render(w, r, ui.Base("edit account", h.navUser(r), userpages.UserEditPage(user, "")))
}

func (h *Handler) handleUpdate(w http.ResponseWriter, r *http.Request) {
	id, err := pathUUID(r)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	email := r.FormValue("email")
	if email == "" {
		user, _ := h.store.GetUser(r.Context(), id)
		h.render(w, r, ui.Base("edit account", h.navUser(r), userpages.UserEditPage(user, "email is required")))
		return
	}

	if _, err := h.store.UpdateUser(r.Context(), id, email); errors.Is(err, store.ErrNotFound) {
		http.Error(w, "not found", http.StatusNotFound)
		return
	} else if err != nil {
		h.logger.Error("failed to update user", "id", id, "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/user/"+id.String(), http.StatusSeeOther)
}

func (h *Handler) handleDelete(w http.ResponseWriter, r *http.Request) {
	id, err := pathUUID(r)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	if err := h.store.DeleteUser(r.Context(), id); err != nil {
		h.logger.Error("failed to delete user", "id", id, "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/user", http.StatusSeeOther)
}

func (h *Handler) handleChangePasswordForm(w http.ResponseWriter, r *http.Request) {
	id, err := pathUUID(r)
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

	h.render(w, r, ui.Base("change password", h.navUser(r), userpages.ChangePasswordPage(user, "")))
}

func (h *Handler) handleChangePassword(w http.ResponseWriter, r *http.Request) {
	id, err := pathUUID(r)
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

	currentPassword := r.FormValue("current_password")
	newPassword := r.FormValue("password")

	if currentPassword == "" || newPassword == "" {
		h.render(w, r, ui.Base("change password", h.navUser(r), userpages.ChangePasswordPage(user, "current and new password are required")))
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordDigest), []byte(currentPassword)); err != nil {
		h.render(w, r, ui.Base("change password", h.navUser(r), userpages.ChangePasswordPage(user, "current password is incorrect")))
		return
	}

	digest, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
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

	http.Redirect(w, r, "/user/"+id.String(), http.StatusSeeOther)
}

func (h *Handler) handleUpdateTheme(w http.ResponseWriter, r *http.Request) {
	authedUser, ok := reqctx.User(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	id, err := pathUUID(r)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	if id != authedUser.ID {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	theme := r.FormValue("theme")

	switch theme {
	case "dark", "light", "auto":
	default:
		http.Error(w, "invalid theme", http.StatusBadRequest)
		return
	}

	if _, err := h.store.UpdateUserTheme(r.Context(), id, theme); errors.Is(err, store.ErrNotFound) {
		http.Error(w, "not found", http.StatusNotFound)
		return
	} else if err != nil {
		h.logger.Error("failed to update theme", "id", id, "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/user/"+id.String(), http.StatusSeeOther)
}

func (h *Handler) handleUpdateFontSize(w http.ResponseWriter, r *http.Request) {
	authedUser, ok := reqctx.User(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	id, err := pathUUID(r)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	if id != authedUser.ID {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	fontSize := r.FormValue("font_size")

	switch fontSize {
	case "small", "medium", "large":
	default:
		http.Error(w, "invalid font size", http.StatusBadRequest)
		return
	}

	if _, err := h.store.UpdateUserFontSize(r.Context(), id, fontSize); errors.Is(err, store.ErrNotFound) {
		http.Error(w, "not found", http.StatusNotFound)
		return
	} else if err != nil {
		h.logger.Error("failed to update font size", "id", id, "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/user/"+id.String(), http.StatusSeeOther)
}

func pathUUID(r *http.Request) (uuid.UUID, error) {
	return uuid.Parse(r.PathValue("id"))
}
