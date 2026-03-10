package server

import (
	"net/http"
)

func (s *Server) registerRoutes() {
	s.mux.HandleFunc("GET /{$}", s.handleIndex)
	s.mux.HandleFunc("GET /login", s.handleLoginForm)
	s.mux.HandleFunc("POST /login", s.handleLogin)
	s.mux.HandleFunc("POST /logout", s.handleLogout)

	// Registration is public so the first user can be created.
	s.mux.HandleFunc("GET /user/new", s.userHandler.ServeHTTP)
	s.mux.HandleFunc("POST /user", s.userHandler.ServeHTTP)

	s.mux.Handle("/url", s.requireAuth(s.urlHandler))
	s.mux.Handle("/url/", s.requireAuth(s.urlHandler))

	s.mux.Handle("/user", s.requireAuth(s.userHandler))
	s.mux.Handle("/user/", s.requireAuth(s.userHandler))

	s.mux.Handle("/apikey/", s.requireAuth(s.apiKeyHandler))
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/url", http.StatusFound)
}
