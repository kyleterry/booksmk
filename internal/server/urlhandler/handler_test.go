package urlhandler

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

// ---- fixtures ---------------------------------------------------------------

var (
	fixtureUserID = uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
	fixtureUser   = store.User{ID: fixtureUserID, Email: "test@example.com"}

	fixtureURLID = uuid.MustParse("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb")
	fixtureURL   = store.URL{
		ID:    fixtureURLID,
		URL:   "https://example.com",
		Title: "Example",
		Tags:  []string{"go", "test"},
	}
)

// ---- mock store -------------------------------------------------------------

type mockURLStore struct {
	GetURLFn    func(context.Context, uuid.UUID, uuid.UUID) (store.URL, error)
	ListURLsFn  func(context.Context, uuid.UUID) ([]store.URL, error)
	CreateURLFn func(context.Context, uuid.UUID, string, string, string, []string) (store.URL, error)
	UpdateURLFn func(context.Context, uuid.UUID, uuid.UUID, string, string, []string) (store.URL, error)
	DeleteURLFn func(context.Context, uuid.UUID, uuid.UUID) error
}

func (m *mockURLStore) GetURL(ctx context.Context, id, userID uuid.UUID) (store.URL, error) {
	if m.GetURLFn != nil {
		return m.GetURLFn(ctx, id, userID)
	}
	return store.URL{}, store.ErrNotFound
}

func (m *mockURLStore) ListURLs(ctx context.Context, userID uuid.UUID) ([]store.URL, error) {
	if m.ListURLsFn != nil {
		return m.ListURLsFn(ctx, userID)
	}
	return nil, nil
}

func (m *mockURLStore) CreateURL(ctx context.Context, userID uuid.UUID, rawURL, title, description string, tags []string) (store.URL, error) {
	if m.CreateURLFn != nil {
		return m.CreateURLFn(ctx, userID, rawURL, title, description, tags)
	}
	return store.URL{}, errors.New("CreateURL not configured")
}

func (m *mockURLStore) UpdateURL(ctx context.Context, id, userID uuid.UUID, title, description string, tags []string) (store.URL, error) {
	if m.UpdateURLFn != nil {
		return m.UpdateURLFn(ctx, id, userID, title, description, tags)
	}
	return store.URL{}, errors.New("UpdateURL not configured")
}

func (m *mockURLStore) DeleteURL(ctx context.Context, id, userID uuid.UUID) error {
	if m.DeleteURLFn != nil {
		return m.DeleteURLFn(ctx, id, userID)
	}
	return errors.New("DeleteURL not configured")
}

// ---- helpers ----------------------------------------------------------------

func newHandler(ms *mockURLStore) *Handler {
	return New(ms, slog.New(slog.NewTextHandler(io.Discard, nil)))
}

func serve(t *testing.T, h *Handler, r *http.Request) *httptest.ResponseRecorder {
	t.Helper()
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}

// req builds a request with the fixture user injected into context.
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

// ---- tests ------------------------------------------------------------------

func TestHandleList(t *testing.T) {
	tests := []struct {
		name       string
		setup      func(*mockURLStore)
		wantStatus int
		wantBody   string
	}{
		{
			name:       "empty list shows empty state",
			setup:      func(m *mockURLStore) { m.ListURLsFn = func(_ context.Context, _ uuid.UUID) ([]store.URL, error) { return nil, nil } },
			wantStatus: http.StatusOK,
			wantBody:   "no bookmarks",
		},
		{
			name: "lists urls",
			setup: func(m *mockURLStore) {
				m.ListURLsFn = func(_ context.Context, _ uuid.UUID) ([]store.URL, error) {
					return []store.URL{fixtureURL}, nil
				}
			},
			wantStatus: http.StatusOK,
			wantBody:   "Example",
		},
		{
			name: "store error returns 500",
			setup: func(m *mockURLStore) {
				m.ListURLsFn = func(_ context.Context, _ uuid.UUID) ([]store.URL, error) {
					return nil, errors.New("db down")
				}
			},
			wantStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ms := &mockURLStore{}
			tt.setup(ms)
			w := serve(t, newHandler(ms), req(http.MethodGet, "/url", ""))
			assertStatus(t, w, tt.wantStatus)
			if tt.wantBody != "" {
				assertContains(t, w, tt.wantBody)
			}
		})
	}
}

func TestHandleNew(t *testing.T) {
	w := serve(t, newHandler(&mockURLStore{}), req(http.MethodGet, "/url/new", ""))
	assertStatus(t, w, http.StatusOK)
	assertContains(t, w, "add url")
}

func TestHandleCreate(t *testing.T) {
	tests := []struct {
		name       string
		body       string
		setup      func(*mockURLStore)
		wantStatus int
		wantLoc    string
		wantBody   string
	}{
		{
			name: "valid url redirects to detail",
			body: "url=https%3A%2F%2Fexample.com&title=Example&tags=go%2Ctest",
			setup: func(m *mockURLStore) {
				m.CreateURLFn = func(_ context.Context, _ uuid.UUID, rawURL, title, _ string, _ []string) (store.URL, error) {
					return fixtureURL, nil
				}
			},
			wantStatus: http.StatusSeeOther,
			wantLoc:    "/url/" + fixtureURLID.String(),
		},
		{
			name:       "missing url shows error",
			body:       "title=No+URL",
			setup:      func(m *mockURLStore) {},
			wantStatus: http.StatusOK,
			wantBody:   "url is required",
		},
		{
			name: "store error shows error",
			body: "url=https%3A%2F%2Fexample.com",
			setup: func(m *mockURLStore) {
				m.CreateURLFn = func(_ context.Context, _ uuid.UUID, _, _, _ string, _ []string) (store.URL, error) {
					return store.URL{}, errors.New("db error")
				}
			},
			wantStatus: http.StatusOK,
			wantBody:   "failed to save",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ms := &mockURLStore{}
			tt.setup(ms)
			w := serve(t, newHandler(ms), req(http.MethodPost, "/url", tt.body))
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
		urlID      string
		setup      func(*mockURLStore)
		wantStatus int
		wantBody   string
	}{
		{
			name:  "existing url renders detail",
			urlID: fixtureURLID.String(),
			setup: func(m *mockURLStore) {
				m.GetURLFn = func(_ context.Context, _, _ uuid.UUID) (store.URL, error) { return fixtureURL, nil }
			},
			wantStatus: http.StatusOK,
			wantBody:   "https://example.com",
		},
		{
			name:  "not found returns 404",
			urlID: fixtureURLID.String(),
			setup: func(m *mockURLStore) {
				m.GetURLFn = func(_ context.Context, _, _ uuid.UUID) (store.URL, error) { return store.URL{}, store.ErrNotFound }
			},
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "invalid uuid returns 400",
			urlID:      "not-a-uuid",
			setup:      func(m *mockURLStore) {},
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ms := &mockURLStore{}
			tt.setup(ms)
			w := serve(t, newHandler(ms), req(http.MethodGet, "/url/"+tt.urlID, ""))
			assertStatus(t, w, tt.wantStatus)
			if tt.wantBody != "" {
				assertContains(t, w, tt.wantBody)
			}
		})
	}
}

func TestHandleEdit(t *testing.T) {
	tests := []struct {
		name       string
		setup      func(*mockURLStore)
		wantStatus int
		wantBody   string
	}{
		{
			name: "renders edit form",
			setup: func(m *mockURLStore) {
				m.GetURLFn = func(_ context.Context, _, _ uuid.UUID) (store.URL, error) { return fixtureURL, nil }
			},
			wantStatus: http.StatusOK,
			wantBody:   "edit url",
		},
		{
			name: "not found returns 404",
			setup: func(m *mockURLStore) {
				m.GetURLFn = func(_ context.Context, _, _ uuid.UUID) (store.URL, error) { return store.URL{}, store.ErrNotFound }
			},
			wantStatus: http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ms := &mockURLStore{}
			tt.setup(ms)
			path := "/url/" + fixtureURLID.String() + "/edit"
			w := serve(t, newHandler(ms), req(http.MethodGet, path, ""))
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
		setup      func(*mockURLStore)
		wantStatus int
		wantLoc    string
		wantBody   string
	}{
		{
			name: "valid update redirects to detail",
			body: "title=Updated&description=New+desc&tags=updated",
			setup: func(m *mockURLStore) {
				m.GetURLFn = func(_ context.Context, _, _ uuid.UUID) (store.URL, error) { return fixtureURL, nil }
				m.UpdateURLFn = func(_ context.Context, _, _ uuid.UUID, _, _ string, _ []string) (store.URL, error) {
					return fixtureURL, nil
				}
			},
			wantStatus: http.StatusSeeOther,
			wantLoc:    "/url/" + fixtureURLID.String(),
		},
		{
			name: "store error shows error",
			body: "title=Updated",
			setup: func(m *mockURLStore) {
				m.GetURLFn = func(_ context.Context, _, _ uuid.UUID) (store.URL, error) { return fixtureURL, nil }
				m.UpdateURLFn = func(_ context.Context, _, _ uuid.UUID, _, _ string, _ []string) (store.URL, error) {
					return store.URL{}, errors.New("db error")
				}
			},
			wantStatus: http.StatusOK,
			wantBody:   "failed to save",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ms := &mockURLStore{}
			tt.setup(ms)
			path := "/url/" + fixtureURLID.String()
			w := serve(t, newHandler(ms), req(http.MethodPost, path, tt.body))
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

func TestHandleDelete(t *testing.T) {
	tests := []struct {
		name       string
		setup      func(*mockURLStore)
		wantStatus int
		wantLoc    string
	}{
		{
			name: "success redirects to list",
			setup: func(m *mockURLStore) {
				m.GetURLFn = func(_ context.Context, _, _ uuid.UUID) (store.URL, error) { return fixtureURL, nil }
				m.DeleteURLFn = func(_ context.Context, _, _ uuid.UUID) error { return nil }
			},
			wantStatus: http.StatusSeeOther,
			wantLoc:    "/url",
		},
		{
			name: "store error returns 500",
			setup: func(m *mockURLStore) {
				m.GetURLFn = func(_ context.Context, _, _ uuid.UUID) (store.URL, error) { return fixtureURL, nil }
				m.DeleteURLFn = func(_ context.Context, _, _ uuid.UUID) error { return errors.New("db error") }
			},
			wantStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ms := &mockURLStore{}
			tt.setup(ms)
			path := "/url/" + fixtureURLID.String() + "/delete"
			w := serve(t, newHandler(ms), req(http.MethodPost, path, ""))
			assertStatus(t, w, tt.wantStatus)
			if tt.wantLoc != "" {
				assertRedirect(t, w, tt.wantLoc)
			}
		})
	}
}

func TestParseTags(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want []string
	}{
		{"empty string", "", []string{}},
		{"single tag", "go", []string{"go"}},
		{"multiple tags", "go, tools, reference", []string{"go", "tools", "reference"}},
		{"trims whitespace", "  go  ,  tools  ", []string{"go", "tools"}},
		{"skips empty parts", "go,,tools", []string{"go", "tools"}},
		{"only commas", ",,,", []string{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseTags(tt.raw)
			if len(got) != len(tt.want) {
				t.Fatalf("parseTags(%q) = %v, want %v", tt.raw, got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("parseTags(%q)[%d] = %q, want %q", tt.raw, i, got[i], tt.want[i])
				}
			}
		})
	}
}
