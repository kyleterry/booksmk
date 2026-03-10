package server

import (
	"errors"
	"net/http"

	"golang.org/x/crypto/bcrypt"

	"github.com/kyleterry/booksmk/internal/store"
	"github.com/kyleterry/booksmk/internal/ui"
	"github.com/kyleterry/booksmk/internal/ui/auth"
)

func (s *Server) handleLoginForm(w http.ResponseWriter, r *http.Request) {
	ui.Base("sign in", nil, auth.LoginPage("")).Render(r.Context(), w)
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	email := r.FormValue("email")
	password := r.FormValue("password")
	if email == "" || password == "" {
		ui.Base("sign in", nil, auth.LoginPage("email and password are required")).Render(r.Context(), w)
		return
	}

	user, err := s.store.GetUserByEmail(r.Context(), email)
	if errors.Is(err, store.ErrNotFound) {
		ui.Base("sign in", nil, auth.LoginPage("invalid email or password")).Render(r.Context(), w)
		return
	}
	if err != nil {
		s.logger.Error("failed to look up user", "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordDigest), []byte(password)); err != nil {
		ui.Base("sign in", nil, auth.LoginPage("invalid email or password")).Render(r.Context(), w)
		return
	}

	session, err := s.store.CreateSession(r.Context(), user.ID)
	if err != nil {
		s.logger.Error("failed to create session", "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    session.Token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
	http.Redirect(w, r, "/url", http.StatusSeeOther)
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie(sessionCookieName)
	if err == nil {
		if err := s.store.DeleteSession(r.Context(), cookie.Value); err != nil {
			s.logger.Error("failed to delete session", "error", err)
		}
	}
	clearSessionCookie(w)
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}
