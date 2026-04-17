package userhandler

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"

	"github.com/a-h/templ"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"go.e64ec.com/booksmk/internal/auth"
	"go.e64ec.com/booksmk/internal/store"
	"go.e64ec.com/booksmk/internal/ui"
	userpages "go.e64ec.com/booksmk/internal/ui/users"
)

type urlCreator interface {
	CreateURL(ctx context.Context, userID uuid.UUID, rawURL, title, description string, tags []string, isBlockedBypass bool) (store.URL, error)
	IsBlocked(ctx context.Context, rawURL string) (bool, error)
}

type userStore interface {
	CountUsers(ctx context.Context) (int64, error)
	CreateUser(ctx context.Context, email, passwordDigest string, isAdmin bool) (store.User, error)
	GetUser(ctx context.Context, id uuid.UUID) (store.User, error)
	ListUsers(ctx context.Context) ([]store.User, error)
	UpdateUser(ctx context.Context, id uuid.UUID, email string) (store.User, error)
	UpdateUserPassword(ctx context.Context, id uuid.UUID, passwordDigest string) (store.User, error)
	UpdateUserSettings(ctx context.Context, id uuid.UUID, settings store.UserSettings) (store.User, error)
	DeleteUser(ctx context.Context, id uuid.UUID) error
	GetInviteCodeByCode(ctx context.Context, code string) (store.InviteCode, error)
	UseInviteCode(ctx context.Context, id, usedBy uuid.UUID) error
	ListAPIKeys(ctx context.Context, userID uuid.UUID) ([]store.APIKey, error)
}

// Handler handles all requests under the /user prefix.
type Handler struct {
	store    userStore
	urlStore urlCreator
	logger   *slog.Logger
	mux      *http.ServeMux
}

func New(s userStore, us urlCreator, logger *slog.Logger) *Handler {
	h := &Handler{
		store:    s,
		urlStore: us,
		logger:   logger,
		mux:      http.NewServeMux(),
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
	h.mux.HandleFunc("PUT /user/{id}", h.handleUpdate)
	h.mux.HandleFunc("DELETE /user/{id}", h.handleDelete)
	h.mux.HandleFunc("GET /user/{id}/change-password", h.handleChangePasswordForm)
	h.mux.HandleFunc("POST /user/{id}/change-password", h.handleChangePassword)
	h.mux.HandleFunc("POST /user/{id}/settings", h.handleUpdateSettings)
	h.mux.HandleFunc("POST /user/{id}/import", h.handleImport)
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
	baseURL := (&url.URL{Scheme: scheme, Host: r.Host}).String()

	h.render(w, r, ui.Base("settings", h.navUser(r), userpages.UserDetailPage(user, keys, baseURL, "")))
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

func (h *Handler) handleUpdateSettings(w http.ResponseWriter, r *http.Request) {
	authedUser, _ := auth.UserFromContext(r.Context())

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
	fontSize := r.FormValue("font_size")
	resultsPerPageStr := r.FormValue("results_per_page")

	switch theme {
	case "dark", "light", "auto":
	default:
		http.Error(w, "invalid theme", http.StatusBadRequest)
		return
	}

	switch fontSize {
	case "small", "medium", "large":
	default:
		http.Error(w, "invalid font size", http.StatusBadRequest)
		return
	}

	resultsPerPage, err := strconv.Atoi(resultsPerPageStr)
	if err != nil || resultsPerPage < 1 || resultsPerPage > 500 {
		http.Error(w, "invalid results per page", http.StatusBadRequest)
		return
	}

	feedGroupingEnabled := r.FormValue("feed_grouping_enabled") != "false"

	if _, err := h.store.UpdateUserSettings(r.Context(), id, store.UserSettings{
		Theme:               theme,
		FontSize:            fontSize,
		ResultsPerPage:      int32(resultsPerPage),
		FeedGroupingEnabled: feedGroupingEnabled,
	}); errors.Is(err, store.ErrNotFound) {
		http.Error(w, "not found", http.StatusNotFound)
		return
	} else if err != nil {
		h.logger.Error("failed to update settings", "id", id, "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/user/"+id.String(), http.StatusSeeOther)
}

func pathUUID(r *http.Request) (uuid.UUID, error) {
	return uuid.Parse(r.PathValue("id"))
}
