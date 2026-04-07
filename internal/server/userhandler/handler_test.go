package userhandler

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/kyleterry/booksmk/internal/reqctx"
	"github.com/kyleterry/booksmk/internal/store"
)

var (
	fixtureUserID = uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
	fixtureUser   = store.User{ID: fixtureUserID, Email: "test@example.com"}
)

type mockUserStore struct {
	CountUsersFn          func(context.Context) (int64, error)
	CreateUserFn          func(context.Context, string, string, bool) (store.User, error)
	GetUserFn             func(context.Context, uuid.UUID) (store.User, error)
	ListUsersFn           func(context.Context) ([]store.User, error)
	UpdateUserFn          func(context.Context, uuid.UUID, string) (store.User, error)
	UpdateUserPasswordFn  func(context.Context, uuid.UUID, string) (store.User, error)
	UpdateUserThemeFn     func(context.Context, uuid.UUID, string) (store.User, error)
	UpdateUserFontSizeFn  func(context.Context, uuid.UUID, string) (store.User, error)
	DeleteUserFn          func(context.Context, uuid.UUID) error
	GetInviteCodeByCodeFn func(context.Context, string) (store.InviteCode, error)
	UseInviteCodeFn       func(context.Context, uuid.UUID, uuid.UUID) error
	ListAPIKeysFn         func(context.Context, uuid.UUID) ([]store.APIKey, error)
}

func (m *mockUserStore) CountUsers(ctx context.Context) (int64, error) {
	if m.CountUsersFn != nil {
		return m.CountUsersFn(ctx)
	}
	return 0, nil
}

func (m *mockUserStore) CreateUser(ctx context.Context, email, digest string, isAdmin bool) (store.User, error) {
	if m.CreateUserFn != nil {
		return m.CreateUserFn(ctx, email, digest, isAdmin)
	}
	return store.User{}, errors.New("CreateUser not configured")
}

func (m *mockUserStore) GetInviteCodeByCode(ctx context.Context, code string) (store.InviteCode, error) {
	if m.GetInviteCodeByCodeFn != nil {
		return m.GetInviteCodeByCodeFn(ctx, code)
	}
	return store.InviteCode{}, store.ErrNotFound
}

func (m *mockUserStore) UseInviteCode(ctx context.Context, id, usedBy uuid.UUID) error {
	if m.UseInviteCodeFn != nil {
		return m.UseInviteCodeFn(ctx, id, usedBy)
	}
	return nil
}

func (m *mockUserStore) GetUser(ctx context.Context, id uuid.UUID) (store.User, error) {
	if m.GetUserFn != nil {
		return m.GetUserFn(ctx, id)
	}
	return store.User{}, store.ErrNotFound
}

func (m *mockUserStore) ListUsers(ctx context.Context) ([]store.User, error) {
	if m.ListUsersFn != nil {
		return m.ListUsersFn(ctx)
	}
	return nil, nil
}

func (m *mockUserStore) UpdateUser(ctx context.Context, id uuid.UUID, email string) (store.User, error) {
	if m.UpdateUserFn != nil {
		return m.UpdateUserFn(ctx, id, email)
	}
	return store.User{}, store.ErrNotFound
}

func (m *mockUserStore) UpdateUserPassword(ctx context.Context, id uuid.UUID, digest string) (store.User, error) {
	if m.UpdateUserPasswordFn != nil {
		return m.UpdateUserPasswordFn(ctx, id, digest)
	}
	return store.User{}, nil
}

func (m *mockUserStore) UpdateUserTheme(ctx context.Context, id uuid.UUID, theme string) (store.User, error) {
	if m.UpdateUserThemeFn != nil {
		return m.UpdateUserThemeFn(ctx, id, theme)
	}
	return store.User{}, nil
}

func (m *mockUserStore) UpdateUserFontSize(ctx context.Context, id uuid.UUID, fontSize string) (store.User, error) {
	if m.UpdateUserFontSizeFn != nil {
		return m.UpdateUserFontSizeFn(ctx, id, fontSize)
	}
	return store.User{}, nil
}

func (m *mockUserStore) DeleteUser(ctx context.Context, id uuid.UUID) error {
	if m.DeleteUserFn != nil {
		return m.DeleteUserFn(ctx, id)
	}
	return nil
}

func (m *mockUserStore) ListAPIKeys(ctx context.Context, userID uuid.UUID) ([]store.APIKey, error) {
	if m.ListAPIKeysFn != nil {
		return m.ListAPIKeysFn(ctx, userID)
	}
	return nil, nil
}

func newHandler(ms *mockUserStore) *Handler {
	return New(ms, slog.New(slog.NewTextHandler(io.Discard, nil)))
}

func serve(t *testing.T, h *Handler, r *http.Request) *httptest.ResponseRecorder {
	t.Helper()
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}

func req(method, target, body string) *http.Request {
	var br io.Reader
	if body != "" {
		br = strings.NewReader(body)
	}
	r := httptest.NewRequest(method, target, br)
	if body != "" {
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	return r
}

func authReq(method, target, body string) *http.Request {
	r := req(method, target, body)
	return r.WithContext(reqctx.WithUser(r.Context(), fixtureUser))
}

func assertStatus(t *testing.T, w *httptest.ResponseRecorder, want int) {
	t.Helper()
	if w.Code != want {
		t.Errorf("status = %d, want %d", w.Code, want)
	}
}

func assertContains(t *testing.T, w *httptest.ResponseRecorder, sub string) {
	t.Helper()
	if !strings.Contains(w.Body.String(), sub) {
		t.Errorf("body does not contain %q\nbody: %s", sub, w.Body.String())
	}
}

func assertRedirect(t *testing.T, w *httptest.ResponseRecorder, loc string) {
	t.Helper()
	assertStatus(t, w, http.StatusSeeOther)
	if got := w.Header().Get("Location"); got != loc {
		t.Errorf("Location = %q, want %q", got, loc)
	}
}

func TestHandleNew(t *testing.T) {
	w := serve(t, newHandler(&mockUserStore{}), req(http.MethodGet, "/user/new", ""))
	assertStatus(t, w, http.StatusOK)
	assertContains(t, w, "create account")
}

func TestHandleCreate(t *testing.T) {
	tests := []struct {
		name       string
		body       string
		setup      func(*mockUserStore)
		wantStatus int
		wantLoc    string
		wantBody   string
	}{
		{
			name: "valid first user registration redirects to login",
			body: "email=new%40example.com&password=secret",
			setup: func(m *mockUserStore) {
				m.CreateUserFn = func(_ context.Context, email, _ string, _ bool) (store.User, error) {
					return store.User{ID: fixtureUserID, Email: email}, nil
				}
			},
			wantStatus: http.StatusSeeOther,
			wantLoc:    "/login",
		},
		{
			name:       "missing email shows error",
			body:       "password=secret",
			setup:      func(m *mockUserStore) {},
			wantStatus: http.StatusOK,
			wantBody:   "email and password are required",
		},
		{
			name:       "missing password shows error",
			body:       "email=new%40example.com",
			setup:      func(m *mockUserStore) {},
			wantStatus: http.StatusOK,
			wantBody:   "email and password are required",
		},
		{
			name: "store error shows error",
			body: "email=dupe%40example.com&password=secret",
			setup: func(m *mockUserStore) {
				m.CreateUserFn = func(_ context.Context, _, _ string, _ bool) (store.User, error) {
					return store.User{}, errors.New("duplicate key")
				}
			},
			wantStatus: http.StatusOK,
			wantBody:   "failed to create account",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ms := &mockUserStore{}
			tt.setup(ms)
			w := serve(t, newHandler(ms), req(http.MethodPost, "/user", tt.body))
			assertStatus(t, w, tt.wantStatus)
			if tt.wantLoc != "" {
				assertRedirect(t, w, tt.wantLoc)
			}
			if tt.wantBody != "" {
				assertContains(t, w, tt.wantBody)
			}
		})
	}
}

func TestHandleGet(t *testing.T) {
	tests := []struct {
		name       string
		userID     string
		setup      func(*mockUserStore)
		wantStatus int
		wantBody   string
	}{
		{
			name:   "existing user renders profile",
			userID: fixtureUserID.String(),
			setup: func(m *mockUserStore) {
				m.GetUserFn = func(_ context.Context, _ uuid.UUID) (store.User, error) { return fixtureUser, nil }
			},
			wantStatus: http.StatusOK,
			wantBody:   fixtureUser.Email,
		},
		{
			name:   "not found returns 404",
			userID: fixtureUserID.String(),
			setup: func(m *mockUserStore) {
				m.GetUserFn = func(_ context.Context, _ uuid.UUID) (store.User, error) { return store.User{}, store.ErrNotFound }
			},
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "invalid uuid returns 400",
			userID:     "not-a-uuid",
			setup:      func(m *mockUserStore) {},
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ms := &mockUserStore{}
			tt.setup(ms)
			w := serve(t, newHandler(ms), authReq(http.MethodGet, "/user/"+tt.userID, ""))
			assertStatus(t, w, tt.wantStatus)
			if tt.wantBody != "" {
				assertContains(t, w, tt.wantBody)
			}
		})
	}
}

func TestHandleUpdate(t *testing.T) {
	tests := []struct {
		name       string
		body       string
		setup      func(*mockUserStore)
		wantStatus int
		wantLoc    string
		wantBody   string
	}{
		{
			name: "email update only redirects to profile",
			body: "email=updated%40example.com",
			setup: func(m *mockUserStore) {
				m.UpdateUserFn = func(_ context.Context, _ uuid.UUID, email string) (store.User, error) {
					return store.User{ID: fixtureUserID, Email: email}, nil
				}
			},
			wantStatus: http.StatusSeeOther,
			wantLoc:    "/user/" + fixtureUserID.String(),
		},
		{
			name:       "missing email shows error",
			body:       "email=",
			setup:      func(m *mockUserStore) {},
			wantStatus: http.StatusOK,
			wantBody:   "email is required",
		},
		{
			name: "not found returns 404",
			body: "email=updated%40example.com",
			setup: func(m *mockUserStore) {
				m.UpdateUserFn = func(_ context.Context, _ uuid.UUID, _ string) (store.User, error) {
					return store.User{}, store.ErrNotFound
				}
			},
			wantStatus: http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ms := &mockUserStore{}
			tt.setup(ms)
			path := "/user/" + fixtureUserID.String()
			w := serve(t, newHandler(ms), authReq(http.MethodPost, path, tt.body))
			assertStatus(t, w, tt.wantStatus)
			if tt.wantLoc != "" {
				assertRedirect(t, w, tt.wantLoc)
			}
			if tt.wantBody != "" {
				assertContains(t, w, tt.wantBody)
			}
		})
	}
}
