package apikeyhandler

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"go.e64ec.com/booksmk/internal/reqctx"
	"go.e64ec.com/booksmk/internal/store"
)

var (
	fixtureUserID = uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
	fixtureUser   = store.User{ID: fixtureUserID, Email: "test@example.com"}

	fixtureKeyID = uuid.MustParse("cccccccc-cccc-cccc-cccc-cccccccccccc")
	fixtureKey   = store.APIKey{
		ID:          fixtureKeyID,
		UserID:      fixtureUserID,
		Name:        "my key",
		TokenPrefix: "bsmk_abc123",
		CreatedAt:   time.Now(),
	}
)

type mockAPIKeyStore struct {
	CreateAPIKeyFn func(context.Context, uuid.UUID, string, *time.Time) (store.NewAPIKeyResult, error)
	ListAPIKeysFn  func(context.Context, uuid.UUID) ([]store.APIKey, error)
	DeleteAPIKeyFn func(context.Context, uuid.UUID, uuid.UUID) error
}

func (m *mockAPIKeyStore) CreateAPIKey(ctx context.Context, userID uuid.UUID, name string, expiresAt *time.Time) (store.NewAPIKeyResult, error) {
	if m.CreateAPIKeyFn != nil {
		return m.CreateAPIKeyFn(ctx, userID, name, expiresAt)
	}
	return store.NewAPIKeyResult{}, errors.New("CreateAPIKey not configured")
}

func (m *mockAPIKeyStore) ListAPIKeys(ctx context.Context, userID uuid.UUID) ([]store.APIKey, error) {
	if m.ListAPIKeysFn != nil {
		return m.ListAPIKeysFn(ctx, userID)
	}
	return nil, nil
}

func (m *mockAPIKeyStore) DeleteAPIKey(ctx context.Context, id, userID uuid.UUID) error {
	if m.DeleteAPIKeyFn != nil {
		return m.DeleteAPIKeyFn(ctx, id, userID)
	}
	return nil
}

func newHandler(ms *mockAPIKeyStore) *Handler {
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

func TestHandleList(t *testing.T) {
	tests := []struct {
		name       string
		setup      func(*mockAPIKeyStore)
		wantStatus int
		wantBody   string
	}{
		{
			name: "empty list shows empty state",
			setup: func(m *mockAPIKeyStore) {
				m.ListAPIKeysFn = func(_ context.Context, _ uuid.UUID) ([]store.APIKey, error) { return nil, nil }
			},
			wantStatus: http.StatusOK,
			wantBody:   "no api keys",
		},
		{
			name: "lists key names",
			setup: func(m *mockAPIKeyStore) {
				m.ListAPIKeysFn = func(_ context.Context, _ uuid.UUID) ([]store.APIKey, error) {
					return []store.APIKey{fixtureKey}, nil
				}
			},
			wantStatus: http.StatusOK,
			wantBody:   "my key",
		},
		{
			name: "shows limit warning when at 5 keys",
			setup: func(m *mockAPIKeyStore) {
				keys := make([]store.APIKey, 5)
				for i := range keys {
					keys[i] = store.APIKey{ID: uuid.New(), UserID: fixtureUserID, Name: "key"}
				}
				m.ListAPIKeysFn = func(_ context.Context, _ uuid.UUID) ([]store.APIKey, error) { return keys, nil }
			},
			wantStatus: http.StatusOK,
			wantBody:   "limit of 5 keys reached",
		},
		{
			name: "store error returns 500",
			setup: func(m *mockAPIKeyStore) {
				m.ListAPIKeysFn = func(_ context.Context, _ uuid.UUID) ([]store.APIKey, error) {
					return nil, errors.New("db down")
				}
			},
			wantStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ms := &mockAPIKeyStore{}
			tt.setup(ms)
			w := serve(t, newHandler(ms), req(http.MethodGet, "/apikey/", ""))
			assertStatus(t, w, tt.wantStatus)
			if tt.wantBody != "" {
				assertContains(t, w, tt.wantBody)
			}
		})
	}
}

func TestHandleNew(t *testing.T) {
	w := serve(t, newHandler(&mockAPIKeyStore{}), req(http.MethodGet, "/apikey/new", ""))
	assertStatus(t, w, http.StatusOK)
	assertContains(t, w, "new api key")
}

func TestHandleCreate(t *testing.T) {
	tests := []struct {
		name       string
		body       string
		setup      func(*mockAPIKeyStore)
		wantStatus int
		wantBody   string
	}{
		{
			name: "valid create shows token",
			body: "name=cli&expires_in=30d",
			setup: func(m *mockAPIKeyStore) {
				m.CreateAPIKeyFn = func(_ context.Context, _ uuid.UUID, name string, _ *time.Time) (store.NewAPIKeyResult, error) {
					return store.NewAPIKeyResult{APIKey: fixtureKey, Token: "bsmk_testtoken"}, nil
				}
			},
			wantStatus: http.StatusOK,
			wantBody:   "bsmk_testtoken",
		},
		{
			name: "created page shows key name",
			body: "name=mykey&expires_in=",
			setup: func(m *mockAPIKeyStore) {
				m.CreateAPIKeyFn = func(_ context.Context, _ uuid.UUID, name string, _ *time.Time) (store.NewAPIKeyResult, error) {
					return store.NewAPIKeyResult{APIKey: store.APIKey{Name: name}, Token: "bsmk_tok"}, nil
				}
			},
			wantStatus: http.StatusOK,
			wantBody:   "mykey",
		},
		{
			name:       "missing name shows error",
			body:       "name=&expires_in=30d",
			setup:      func(m *mockAPIKeyStore) {},
			wantStatus: http.StatusOK,
			wantBody:   "name is required",
		},
		{
			name: "limit reached shows error",
			body: "name=extra&expires_in=30d",
			setup: func(m *mockAPIKeyStore) {
				m.CreateAPIKeyFn = func(_ context.Context, _ uuid.UUID, _ string, _ *time.Time) (store.NewAPIKeyResult, error) {
					return store.NewAPIKeyResult{}, store.ErrAPIKeyLimitReached
				}
			},
			wantStatus: http.StatusOK,
			wantBody:   "api key limit reached",
		},
		{
			name: "store error returns 500",
			body: "name=cli&expires_in=30d",
			setup: func(m *mockAPIKeyStore) {
				m.CreateAPIKeyFn = func(_ context.Context, _ uuid.UUID, _ string, _ *time.Time) (store.NewAPIKeyResult, error) {
					return store.NewAPIKeyResult{}, errors.New("db error")
				}
			},
			wantStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ms := &mockAPIKeyStore{}
			tt.setup(ms)
			w := serve(t, newHandler(ms), req(http.MethodPost, "/apikey/", tt.body))
			assertStatus(t, w, tt.wantStatus)
			if tt.wantBody != "" {
				assertContains(t, w, tt.wantBody)
			}
		})
	}
}

func TestHandleDelete(t *testing.T) {
	tests := []struct {
		name       string
		keyID      string
		setup      func(*mockAPIKeyStore)
		wantStatus int
		wantLoc    string
	}{
		{
			name:  "success redirects to settings",
			keyID: fixtureKeyID.String(),
			setup: func(m *mockAPIKeyStore) {
				m.DeleteAPIKeyFn = func(_ context.Context, _, _ uuid.UUID) error { return nil }
			},
			wantStatus: http.StatusSeeOther,
			wantLoc:    "/user/" + fixtureUserID.String(),
		},
		{
			name:       "invalid uuid returns 400",
			keyID:      "not-a-uuid",
			setup:      func(m *mockAPIKeyStore) {},
			wantStatus: http.StatusBadRequest,
		},
		{
			name:  "store error returns 500",
			keyID: fixtureKeyID.String(),
			setup: func(m *mockAPIKeyStore) {
				m.DeleteAPIKeyFn = func(_ context.Context, _, _ uuid.UUID) error { return errors.New("db error") }
			},
			wantStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ms := &mockAPIKeyStore{}
			tt.setup(ms)
			path := "/apikey/" + tt.keyID
			w := serve(t, newHandler(ms), req(http.MethodDelete, path, ""))
			assertStatus(t, w, tt.wantStatus)
			if tt.wantLoc != "" {
				assertRedirect(t, w, tt.wantLoc)
			}
		})
	}
}

func TestParseExpiresIn(t *testing.T) {
	tests := []struct {
		input    string
		wantNil  bool
		wantDays int
	}{
		{"30d", false, 30},
		{"90d", false, 90},
		{"1y", false, 365},
		{"", true, 0},
		{"invalid", true, 0},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := parseExpiresIn(tt.input)
			if tt.wantNil {
				if got != nil {
					t.Errorf("parseExpiresIn(%q) = %v, want nil", tt.input, got)
				}
				return
			}
			if got == nil {
				t.Fatalf("parseExpiresIn(%q) = nil, want non-nil", tt.input)
			}
			want := time.Now().Add(time.Duration(tt.wantDays) * 24 * time.Hour)
			diff := got.Sub(want)
			if diff < -time.Second || diff > time.Second {
				t.Errorf("parseExpiresIn(%q) = %v, want ~%v", tt.input, got, want)
			}
		})
	}
}
