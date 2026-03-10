package userhandler

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"github.com/kyleterry/booksmk/internal/reqctx"
	"github.com/kyleterry/booksmk/internal/store"
	"github.com/kyleterry/booksmk/internal/ui"
	userpages "github.com/kyleterry/booksmk/internal/ui/users"
)

type userStore interface {
	CreateUser(ctx context.Context, email, passwordDigest string) (store.User, error)
	GetUser(ctx context.Context, id uuid.UUID) (store.User, error)
	ListUsers(ctx context.Context) ([]store.User, error)
	UpdateUser(ctx context.Context, id uuid.UUID, email string) (store.User, error)
	UpdateUserPassword(ctx context.Context, id uuid.UUID, passwordDigest string) (store.User, error)
	DeleteUser(ctx context.Context, id uuid.UUID) error
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
}

func (h *Handler) navUser(r *http.Request) *ui.NavUser {
	u, ok := reqctx.User(r.Context())
	if !ok {
		return nil
	}
	return &ui.NavUser{Email: u.Email}
}

func (h *Handler) handleNew(w http.ResponseWriter, r *http.Request) {
	ui.Base("create account", nil, userpages.RegisterPage("")).Render(r.Context(), w)
}

func (h *Handler) handleCreate(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	email := r.FormValue("email")
	password := r.FormValue("password")

	if email == "" || password == "" {
		ui.Base("create account", nil, userpages.RegisterPage("email and password are required")).Render(r.Context(), w)
		return
	}

	digest, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		h.logger.Error("failed to hash password", "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	user, err := h.store.CreateUser(r.Context(), email, string(digest))
	if err != nil {
		h.logger.Error("failed to create user", "error", err)
		ui.Base("create account", nil, userpages.RegisterPage("failed to create account")).Render(r.Context(), w)
		return
	}

	http.Redirect(w, r, "/user/"+user.ID.String(), http.StatusSeeOther)
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

	ui.Base("account", h.navUser(r), userpages.UserDetailPage(user)).Render(r.Context(), w)
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

	ui.Base("edit account", h.navUser(r), userpages.UserEditPage(user, "")).Render(r.Context(), w)
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
		ui.Base("edit account", h.navUser(r), userpages.UserEditPage(user, "email is required")).Render(r.Context(), w)
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

	if password := r.FormValue("password"); password != "" {
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

func pathUUID(r *http.Request) (uuid.UUID, error) {
	return uuid.Parse(r.PathValue("id"))
}
