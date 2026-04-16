package feedhandler

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

	"go.e64ec.com/booksmk/internal/auth"
	"go.e64ec.com/booksmk/internal/store"
)

var (
	fixtureUserID = uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
	fixtureUser   = store.User{ID: fixtureUserID, Email: "test@example.com"}

	fixtureFeedID = uuid.MustParse("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb")
	fixtureFeed   = store.Feed{
		ID:      fixtureFeedID,
		FeedURL: "https://example.com/feed.xml",
		Title:   "Example Feed",
	}

	fixtureItemID = uuid.MustParse("cccccccc-cccc-cccc-cccc-cccccccccccc")
)

type mockFeedStore struct {
	SubscribeToFeedFn     func(context.Context, uuid.UUID, string, []string, bool) (store.Feed, error)
	GetFeedFn             func(context.Context, uuid.UUID, uuid.UUID) (store.Feed, error)
	ListFeedsFn           func(context.Context, uuid.UUID) ([]store.Feed, error)
	UnsubscribeFromFeedFn func(context.Context, uuid.UUID, uuid.UUID) error
	UpdateFeedFn          func(context.Context, uuid.UUID, uuid.UUID, string, []string) (store.Feed, error)
	ListFeedItemsFn       func(context.Context, uuid.UUID, uuid.UUID) ([]store.FeedItem, error)
	ListTimelineItemsFn   func(context.Context, uuid.UUID, int, int) ([]store.TimelineItem, error)
	GetTimelineItemFn     func(context.Context, uuid.UUID, uuid.UUID) (store.TimelineItem, error)
	MarkItemReadFn        func(context.Context, uuid.UUID, uuid.UUID) error
	MarkItemUnreadFn      func(context.Context, uuid.UUID, uuid.UUID) error
	MarkAllItemsReadFn    func(context.Context, uuid.UUID) error
	MarkFeedItemsReadFn   func(context.Context, uuid.UUID, uuid.UUID) error
	IsBlockedFn           func(context.Context, string) (bool, error)
}

func (m *mockFeedStore) IsBlocked(ctx context.Context, rawURL string) (bool, error) {
	if m.IsBlockedFn != nil {
		return m.IsBlockedFn(ctx, rawURL)
	}
	return false, nil
}

func (m *mockFeedStore) SubscribeToFeed(ctx context.Context, userID uuid.UUID, feedURL string, tags []string, isBlockedBypass bool) (store.Feed, error) {
	if m.SubscribeToFeedFn != nil {
		return m.SubscribeToFeedFn(ctx, userID, feedURL, tags, isBlockedBypass)
	}
	return store.Feed{}, errors.New("SubscribeToFeed not configured")
}

func (m *mockFeedStore) GetFeed(ctx context.Context, id, userID uuid.UUID) (store.Feed, error) {
	if m.GetFeedFn != nil {
		return m.GetFeedFn(ctx, id, userID)
	}
	return store.Feed{}, store.ErrNotFound
}

func (m *mockFeedStore) ListFeeds(ctx context.Context, userID uuid.UUID) ([]store.Feed, error) {
	if m.ListFeedsFn != nil {
		return m.ListFeedsFn(ctx, userID)
	}
	return nil, nil
}

func (m *mockFeedStore) UnsubscribeFromFeed(ctx context.Context, userID, feedID uuid.UUID) error {
	if m.UnsubscribeFromFeedFn != nil {
		return m.UnsubscribeFromFeedFn(ctx, userID, feedID)
	}
	return nil
}

func (m *mockFeedStore) UpdateFeed(ctx context.Context, feedID, userID uuid.UUID, customName string, tags []string) (store.Feed, error) {
	if m.UpdateFeedFn != nil {
		return m.UpdateFeedFn(ctx, feedID, userID, customName, tags)
	}
	return store.Feed{}, errors.New("UpdateFeed not configured")
}

func (m *mockFeedStore) ListFeedItems(ctx context.Context, feedID, userID uuid.UUID) ([]store.FeedItem, error) {
	if m.ListFeedItemsFn != nil {
		return m.ListFeedItemsFn(ctx, feedID, userID)
	}
	return nil, nil
}

func (m *mockFeedStore) ListTimelineItems(ctx context.Context, userID uuid.UUID, limit, offset int) ([]store.TimelineItem, error) {
	if m.ListTimelineItemsFn != nil {
		return m.ListTimelineItemsFn(ctx, userID, limit, offset)
	}
	return nil, nil
}

func (m *mockFeedStore) GetTimelineItem(ctx context.Context, userID, itemID uuid.UUID) (store.TimelineItem, error) {
	if m.GetTimelineItemFn != nil {
		return m.GetTimelineItemFn(ctx, userID, itemID)
	}
	return store.TimelineItem{}, store.ErrNotFound
}

func (m *mockFeedStore) MarkItemRead(ctx context.Context, userID, itemID uuid.UUID) error {
	if m.MarkItemReadFn != nil {
		return m.MarkItemReadFn(ctx, userID, itemID)
	}
	return nil
}

func (m *mockFeedStore) MarkItemUnread(ctx context.Context, userID, itemID uuid.UUID) error {
	if m.MarkItemUnreadFn != nil {
		return m.MarkItemUnreadFn(ctx, userID, itemID)
	}
	return nil
}

func (m *mockFeedStore) MarkAllItemsRead(ctx context.Context, userID uuid.UUID) error {
	if m.MarkAllItemsReadFn != nil {
		return m.MarkAllItemsReadFn(ctx, userID)
	}
	return nil
}

func (m *mockFeedStore) MarkFeedItemsRead(ctx context.Context, userID, feedID uuid.UUID) error {
	if m.MarkFeedItemsReadFn != nil {
		return m.MarkFeedItemsReadFn(ctx, userID, feedID)
	}
	return nil
}

func newHandler(ms *mockFeedStore) *Handler {
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
	return r.WithContext(auth.NewContextWithUser(r.Context(), fixtureUser))
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

func TestHandleTimeline(t *testing.T) {
	tests := []struct {
		name       string
		target     string
		setup      func(*mockFeedStore)
		wantStatus int
	}{
		{
			name:       "empty timeline renders page",
			target:     "/feed",
			setup:      func(m *mockFeedStore) {},
			wantStatus: http.StatusOK,
		},
		{
			name:   "with items renders page",
			target: "/feed",
			setup: func(m *mockFeedStore) {
				m.ListTimelineItemsFn = func(_ context.Context, _ uuid.UUID, _, _ int) ([]store.TimelineItem, error) {
					return []store.TimelineItem{
						{ID: fixtureItemID, FeedID: fixtureFeedID, Title: "Test Item"},
					}, nil
				}
			},
			wantStatus: http.StatusOK,
		},
		{
			name:   "store error returns 500",
			target: "/feed",
			setup: func(m *mockFeedStore) {
				m.ListTimelineItemsFn = func(_ context.Context, _ uuid.UUID, _, _ int) ([]store.TimelineItem, error) {
					return nil, errors.New("db error")
				}
			},
			wantStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ms := &mockFeedStore{}
			tt.setup(ms)
			w := serve(t, newHandler(ms), authReq(http.MethodGet, tt.target, ""))
			assertStatus(t, w, tt.wantStatus)
		})
	}
}

func TestHandleTimelinePagination(t *testing.T) {
	ms := &mockFeedStore{}

	// Return pageSize+1 items so hasMore is true.
	items := make([]store.TimelineItem, pageSize+1)
	for i := range items {
		items[i] = store.TimelineItem{ID: uuid.New(), FeedID: fixtureFeedID}
	}
	ms.ListTimelineItemsFn = func(_ context.Context, _ uuid.UUID, limit, _ int) ([]store.TimelineItem, error) {
		if limit > len(items) {
			return items, nil
		}
		return items[:limit], nil
	}

	w := serve(t, newHandler(ms), authReq(http.MethodGet, "/feed", ""))
	assertStatus(t, w, http.StatusOK)
}

func TestHandleNew(t *testing.T) {
	w := serve(t, newHandler(&mockFeedStore{}), authReq(http.MethodGet, "/feed/new", ""))
	assertStatus(t, w, http.StatusOK)
}

func TestHandleCreate_Validation(t *testing.T) {
	tests := []struct {
		name       string
		body       string
		wantStatus int
		wantBody   string
	}{
		{
			name:       "empty feed url shows error",
			body:       "feed_url=",
			wantStatus: http.StatusOK,
			wantBody:   "feed url is required",
		},
		{
			name:       "invalid url shows error",
			body:       "feed_url=not-a-url",
			wantStatus: http.StatusOK,
			wantBody:   "invalid feed url",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := serve(t, newHandler(&mockFeedStore{}), authReq(http.MethodPost, "/feed", tt.body))
			assertStatus(t, w, tt.wantStatus)
			assertContains(t, w, tt.wantBody)
		})
	}
}

func TestRequireFeedOwner(t *testing.T) {
	tests := []struct {
		name       string
		feedID     string
		setup      func(*mockFeedStore)
		authed     bool
		wantStatus int
	}{
		{
			name:   "owner gets feed injected into context",
			feedID: fixtureFeedID.String(),
			setup: func(m *mockFeedStore) {
				m.GetFeedFn = func(_ context.Context, _ uuid.UUID, _ uuid.UUID) (store.Feed, error) {
					return fixtureFeed, nil
				}
				m.ListFeedItemsFn = func(_ context.Context, _, _ uuid.UUID) ([]store.FeedItem, error) {
					return nil, nil
				}
			},
			authed:     true,
			wantStatus: http.StatusOK,
		},
		{
			name:       "invalid uuid returns 400",
			feedID:     "not-a-uuid",
			setup:      func(m *mockFeedStore) {},
			authed:     true,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:   "feed not found returns 404",
			feedID: fixtureFeedID.String(),
			setup: func(m *mockFeedStore) {
				m.GetFeedFn = func(_ context.Context, _ uuid.UUID, _ uuid.UUID) (store.Feed, error) {
					return store.Feed{}, store.ErrNotFound
				}
			},
			authed:     true,
			wantStatus: http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ms := &mockFeedStore{}
			tt.setup(ms)

			var r *http.Request
			if tt.authed {
				r = authReq(http.MethodGet, "/feed/"+tt.feedID, "")
			} else {
				r = req(http.MethodGet, "/feed/"+tt.feedID, "")
			}

			w := serve(t, newHandler(ms), r)
			assertStatus(t, w, tt.wantStatus)
		})
	}
}

func TestHandleGet(t *testing.T) {
	ms := &mockFeedStore{
		GetFeedFn: func(_ context.Context, _ uuid.UUID, _ uuid.UUID) (store.Feed, error) {
			return fixtureFeed, nil
		},
		ListFeedItemsFn: func(_ context.Context, _, _ uuid.UUID) ([]store.FeedItem, error) {
			return []store.FeedItem{{ID: fixtureItemID, FeedID: fixtureFeedID, Title: "Test Item"}}, nil
		},
	}

	w := serve(t, newHandler(ms), authReq(http.MethodGet, "/feed/"+fixtureFeedID.String(), ""))
	assertStatus(t, w, http.StatusOK)
}

func TestHandleEdit(t *testing.T) {
	ms := &mockFeedStore{
		GetFeedFn: func(_ context.Context, _ uuid.UUID, _ uuid.UUID) (store.Feed, error) {
			return fixtureFeed, nil
		},
	}

	w := serve(t, newHandler(ms), authReq(http.MethodGet, "/feed/"+fixtureFeedID.String()+"/edit", ""))
	assertStatus(t, w, http.StatusOK)
}

func TestHandleUpdate(t *testing.T) {
	tests := []struct {
		name       string
		body       string
		setup      func(*mockFeedStore)
		wantStatus int
		wantLoc    string
	}{
		{
			name: "valid update redirects to feed",
			body: "custom_name=My+Feed&tags=tech%2Cgo",
			setup: func(m *mockFeedStore) {
				m.UpdateFeedFn = func(_ context.Context, _, _ uuid.UUID, _ string, _ []string) (store.Feed, error) {
					return fixtureFeed, nil
				}
			},
			wantStatus: http.StatusSeeOther,
			wantLoc:    "/feed/" + fixtureFeedID.String(),
		},
		{
			name: "store error re-renders form",
			body: "custom_name=Bad",
			setup: func(m *mockFeedStore) {
				m.UpdateFeedFn = func(_ context.Context, _, _ uuid.UUID, _ string, _ []string) (store.Feed, error) {
					return store.Feed{}, errors.New("db error")
				}
			},
			wantStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ms := &mockFeedStore{
				GetFeedFn: func(_ context.Context, _ uuid.UUID, _ uuid.UUID) (store.Feed, error) {
					return fixtureFeed, nil
				},
			}
			tt.setup(ms)
			w := serve(t, newHandler(ms), authReq(http.MethodPut, "/feed/"+fixtureFeedID.String(), tt.body))
			assertStatus(t, w, tt.wantStatus)
			if tt.wantLoc != "" {
				assertRedirect(t, w, tt.wantLoc)
			}
		})
	}
}

func TestHandleDelete(t *testing.T) {
	tests := []struct {
		name       string
		setup      func(*mockFeedStore)
		wantStatus int
		wantLoc    string
	}{
		{
			name:       "success redirects to /feed",
			setup:      func(m *mockFeedStore) {},
			wantStatus: http.StatusSeeOther,
			wantLoc:    "/feed",
		},
		{
			name: "store error returns 500",
			setup: func(m *mockFeedStore) {
				m.UnsubscribeFromFeedFn = func(_ context.Context, _, _ uuid.UUID) error {
					return errors.New("db error")
				}
			},
			wantStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ms := &mockFeedStore{
				GetFeedFn: func(_ context.Context, _ uuid.UUID, _ uuid.UUID) (store.Feed, error) {
					return fixtureFeed, nil
				},
			}
			tt.setup(ms)
			w := serve(t, newHandler(ms), authReq(http.MethodDelete, "/feed/"+fixtureFeedID.String(), ""))
			assertStatus(t, w, tt.wantStatus)
			if tt.wantLoc != "" {
				assertRedirect(t, w, tt.wantLoc)
			}
		})
	}
}

func TestHandleMarkRead(t *testing.T) {
	tests := []struct {
		name       string
		htmx       bool
		setup      func(*mockFeedStore)
		wantStatus int
	}{
		{
			name: "success redirects",
			setup: func(m *mockFeedStore) {
				m.GetTimelineItemFn = func(_ context.Context, _, _ uuid.UUID) (store.TimelineItem, error) {
					return store.TimelineItem{ID: fixtureItemID}, nil
				}
			},
			wantStatus: http.StatusSeeOther,
		},
		{
			name: "htmx request renders fragment",
			htmx: true,
			setup: func(m *mockFeedStore) {
				m.GetTimelineItemFn = func(_ context.Context, _, _ uuid.UUID) (store.TimelineItem, error) {
					return store.TimelineItem{ID: fixtureItemID}, nil
				}
			},
			wantStatus: http.StatusOK,
		},
		{
			name:       "invalid item id returns 400",
			setup:      func(m *mockFeedStore) {},
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ms := &mockFeedStore{}
			tt.setup(ms)

			itemID := fixtureItemID.String()
			if tt.name == "invalid item id returns 400" {
				itemID = "not-a-uuid"
			}

			r := authReq(http.MethodPost, "/feed/items/"+itemID+"/read", "")
			if tt.htmx {
				r.Header.Set("HX-Request", "true")
			}

			w := serve(t, newHandler(ms), r)
			assertStatus(t, w, tt.wantStatus)
		})
	}
}

func TestHandleMarkUnread(t *testing.T) {
	tests := []struct {
		name       string
		htmx       bool
		wantStatus int
	}{
		{name: "success redirects", wantStatus: http.StatusSeeOther},
		{name: "htmx request returns 200", htmx: true, wantStatus: http.StatusOK},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ms := &mockFeedStore{
				GetTimelineItemFn: func(_ context.Context, _, _ uuid.UUID) (store.TimelineItem, error) {
					return store.TimelineItem{ID: fixtureItemID}, nil
				},
			}

			r := authReq(http.MethodDelete, "/feed/items/"+fixtureItemID.String()+"/read", "")
			if tt.htmx {
				r.Header.Set("HX-Request", "true")
			}

			w := serve(t, newHandler(ms), r)
			assertStatus(t, w, tt.wantStatus)
		})
	}
}

func TestHandleMarkAllRead(t *testing.T) {
	tests := []struct {
		name       string
		htmx       bool
		wantStatus int
		wantHeader string
	}{
		{name: "success redirects", wantStatus: http.StatusSeeOther},
		{name: "htmx request returns 204 with HX-Redirect", htmx: true, wantStatus: http.StatusNoContent, wantHeader: "/feed"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := authReq(http.MethodPost, "/feed/items/read-all", "")
			if tt.htmx {
				r.Header.Set("HX-Request", "true")
			}

			w := serve(t, newHandler(&mockFeedStore{}), r)
			assertStatus(t, w, tt.wantStatus)
			if tt.wantHeader != "" {
				if got := w.Header().Get("HX-Redirect"); got != tt.wantHeader {
					t.Errorf("HX-Redirect = %q, want %q", got, tt.wantHeader)
				}
			}
		})
	}
}

func TestHandleMarkFeedAllRead(t *testing.T) {
	redirect := "/feed/" + fixtureFeedID.String()

	tests := []struct {
		name       string
		htmx       bool
		wantStatus int
		wantHeader string
		wantLoc    string
	}{
		{name: "success redirects to feed", wantStatus: http.StatusSeeOther, wantLoc: redirect},
		{name: "htmx returns 204 with HX-Redirect", htmx: true, wantStatus: http.StatusNoContent, wantHeader: redirect},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ms := &mockFeedStore{
				GetFeedFn: func(_ context.Context, _ uuid.UUID, _ uuid.UUID) (store.Feed, error) {
					return fixtureFeed, nil
				},
			}

			r := authReq(http.MethodPost, "/feed/"+fixtureFeedID.String()+"/read-all", "")
			if tt.htmx {
				r.Header.Set("HX-Request", "true")
			}

			w := serve(t, newHandler(ms), r)
			assertStatus(t, w, tt.wantStatus)
			if tt.wantLoc != "" {
				assertRedirect(t, w, tt.wantLoc)
			}
			if tt.wantHeader != "" {
				if got := w.Header().Get("HX-Redirect"); got != tt.wantHeader {
					t.Errorf("HX-Redirect = %q, want %q", got, tt.wantHeader)
				}
			}
		})
	}
}

func TestParseTags(t *testing.T) {
	tests := []struct {
		input string
		want  []string
	}{
		{"", []string{}},
		{"go", []string{"go"}},
		{"go, rust, zig", []string{"go", "rust", "zig"}},
		{"  go , rust ", []string{"go", "rust"}},
		{"Go Lang, Rust Lang", []string{"go-lang", "rust-lang"}},
		{",,,", []string{}},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := parseTags(tt.input)
			if len(got) != len(tt.want) {
				t.Fatalf("parseTags(%q) = %v, want %v", tt.input, got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("parseTags(%q)[%d] = %q, want %q", tt.input, i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestSafeReferer(t *testing.T) {
	tests := []struct {
		referer string
		want    string
	}{
		{"", "/feed"},
		{"/feed/some-id", "/feed/some-id"},
		{"https://evil.com/steal", "/feed"},
		{"http://localhost:8080/feed", "/feed"},
		{"/feed?page=2", "/feed?page=2"},
	}

	for _, tt := range tests {
		t.Run(tt.referer, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, "/", nil)
			if tt.referer != "" {
				r.Header.Set("Referer", tt.referer)
			}
			if got := safeReferer(r); got != tt.want {
				t.Errorf("safeReferer(%q) = %q, want %q", tt.referer, got, tt.want)
			}
		})
	}
}

func TestDateLabel(t *testing.T) {
	now := time.Date(2024, 3, 15, 12, 0, 0, 0, time.UTC)

	day := func(daysAgo int) *time.Time {
		t := now.AddDate(0, 0, -daysAgo)
		return &t
	}

	tests := []struct {
		name string
		t    *time.Time
		want string
	}{
		{"nil", nil, "older"},
		{"today", day(0), "today"},
		{"yesterday", day(1), "yesterday"},
		{"2 days ago", day(2), strings.ToLower(now.AddDate(0, 0, -2).Weekday().String())},
		{"6 days ago", day(6), strings.ToLower(now.AddDate(0, 0, -6).Weekday().String())},
		{"7 days ago", day(7), "last week"},
		{"13 days ago", day(13), "last week"},
		{"14 days ago", day(14), "last month"},
		{"59 days ago", day(59), "last month"},
		{"60 days ago", day(60), "older"},
		{"100 days ago", day(100), "older"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := dateLabel(tt.t, now); got != tt.want {
				t.Errorf("dateLabel(%v) = %q, want %q", tt.t, got, tt.want)
			}
		})
	}

	// Verify timezone boundaries: now is UTC-8 evening (local Apr 7),
	// item published at 04:00 UTC (= Apr 7 20:00 local) — same local day.
	t.Run("timezone: same local day despite different UTC date", func(t *testing.T) {
		utcMinus8 := time.FixedZone("UTC-8", -8*60*60)
		// now: 2024-03-08 20:00 UTC-8 = 2024-03-09 04:00 UTC
		nowLocal := time.Date(2024, 3, 8, 20, 0, 0, 0, utcMinus8)
		// item published at 2024-03-09 02:00 UTC = 2024-03-08 18:00 UTC-8 → local today
		itemUTC := time.Date(2024, 3, 9, 2, 0, 0, 0, time.UTC)
		got := dateLabel(&itemUTC, nowLocal)
		if got != "today" {
			t.Errorf("dateLabel = %q, want %q", got, "today")
		}
	})

	// Verify timezone boundaries: now is UTC+8 morning (local Apr 8),
	// item published at 21:00 UTC previous day (= Apr 8 05:00 local) — local today.
	t.Run("timezone: local today despite previous UTC date", func(t *testing.T) {
		utcPlus8 := time.FixedZone("UTC+8", 8*60*60)
		// now: 2024-03-08 12:00 UTC+8 = 2024-03-08 04:00 UTC
		nowLocal := time.Date(2024, 3, 8, 12, 0, 0, 0, utcPlus8)
		// item published at 2024-03-07 21:00 UTC = 2024-03-08 05:00 UTC+8 → local today
		itemUTC := time.Date(2024, 3, 7, 21, 0, 0, 0, time.UTC)
		got := dateLabel(&itemUTC, nowLocal)
		if got != "today" {
			t.Errorf("dateLabel = %q, want %q", got, "today")
		}
	})

	t.Run("timezone: DST transition handles 23h day correctly", func(t *testing.T) {
		loc, err := time.LoadLocation("America/Los_Angeles")
		if err != nil {
			t.Skip("America/Los_Angeles not available")
		}
		// In 2026, DST starts on March 8. March 8 only has 23 hours.
		// now is March 9 00:00:00
		now := time.Date(2026, 3, 9, 0, 0, 0, 0, loc)
		// item is March 8 12:00:00
		item := time.Date(2026, 3, 8, 12, 0, 0, 0, loc)
		got := dateLabel(&item, now)
		if got != "yesterday" {
			t.Errorf("dateLabel(March 8) when now is March 9 = %q, want %q", got, "yesterday")
		}
	})
}

// TestGroupFeedItemsPDT verifies that groupFeedItems uses the client timezone
// (PDT, UTC-7) rather than UTC when grouping items. The key scenario: items
// published in the early hours UTC on "today" are already yesterday in PDT.
//
// Server wall clock: UTC noon on 2026-04-09.
// Client timezone:   America/Los_Angeles (PDT = UTC-7).
// "now" passed to groupFeedItems: 2026-04-09 05:00 PDT (= 2026-04-09 12:00 UTC).
//
// Items and expected PDT labels:
//
//	2026-04-09 15:00 UTC = 2026-04-09 08:00 PDT today
//	2026-04-09 04:01 UTC = 2026-04-08 21:01 PDT yesterday  (was mislabelled "today" when server used UTC)
//	2026-04-08 21:45 UTC = 2026-04-08 14:45 PDT yesterday
//	2026-04-07 23:00 UTC = 2026-04-07 16:00 PDT tuesday
func TestGroupFeedItemsPDT(t *testing.T) {
	pdt := time.FixedZone("PDT", -7*60*60)

	// now is 2026-04-09 12:00 UTC, expressed in PDT as 05:00.
	now := time.Date(2026, 4, 9, 12, 0, 0, 0, time.UTC).In(pdt)

	pt := func(year int, month time.Month, day, hour, min int) *time.Time {
		t := time.Date(year, month, day, hour, min, 0, 0, time.UTC)
		return &t
	}

	items := []store.FeedItem{
		{PublishedAt: pt(2026, 4, 9, 15, 0)},  // 08:00 PDT today
		{PublishedAt: pt(2026, 4, 9, 4, 1)},   // 21:01 PDT Apr 8 yesterday
		{PublishedAt: pt(2026, 4, 8, 21, 45)}, // 14:45 PDT Apr 8 yesterday
		{PublishedAt: pt(2026, 4, 7, 23, 0)},  // 16:00 PDT Apr 7 tuesday
	}

	groups := groupFeedItems(items, now)

	type wantGroup struct {
		label string
		count int
	}

	want := []wantGroup{
		{"today", 1},
		{"yesterday", 2},
		{"tuesday", 1},
	}

	if len(groups) != len(want) {
		t.Fatalf("got %d groups, want %d: %v", len(groups), len(want), groups)
	}

	for i, w := range want {
		g := groups[i]
		if g.Label != w.label {
			t.Errorf("group[%d].Label = %q, want %q", i, g.Label, w.label)
		}
		if len(g.Items) != w.count {
			t.Errorf("group[%d] (%q) has %d items, want %d", i, g.Label, len(g.Items), w.count)
		}
	}
}
