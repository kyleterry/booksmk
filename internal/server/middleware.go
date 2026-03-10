package server

import (
	"errors"
	"net/http"
	"strings"

	"github.com/kyleterry/booksmk/internal/reqctx"
	"github.com/kyleterry/booksmk/internal/store"
)

const sessionCookieName = "session"

// requireAuth validates the session cookie and injects the authenticated user
// into the request context. Redirects to /login if the session is missing or expired.
func (s *Server) requireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(sessionCookieName)
		if err != nil {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}

		user, err := s.store.GetSessionUser(r.Context(), cookie.Value)
		if errors.Is(err, store.ErrNotFound) {
			clearSessionCookie(w)
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}
		if err != nil {
			s.logger.Error("failed to look up session", "error", err)
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}

		next.ServeHTTP(w, r.WithContext(reqctx.WithUser(r.Context(), user)))
	})
}

// requireAPIKeyAuth validates a Bearer token from the Authorization header,
// looks up the associated user, and injects them into the request context.
func (s *Server) requireAPIKeyAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
		if !ok || strings.TrimSpace(token) == "" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		apiKey, err := s.store.GetAPIKeyByToken(r.Context(), strings.TrimSpace(token))
		if errors.Is(err, store.ErrNotFound) {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		if err != nil {
			s.logger.Error("failed to look up api key", "error", err)
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}

		user, err := s.store.GetUser(r.Context(), apiKey.UserID)
		if err != nil {
			s.logger.Error("failed to get user for api key", "user_id", apiKey.UserID, "error", err)
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}

		next.ServeHTTP(w, r.WithContext(reqctx.WithUser(r.Context(), user)))
	})
}

func clearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:   sessionCookieName,
		MaxAge: -1,
		Path:   "/",
	})
}
