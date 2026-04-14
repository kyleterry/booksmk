package store_test

import (
	"context"
	"testing"

	"go.e64ec.com/booksmk/internal/store"
)

// setupUser creates a user for use in URL tests.
func setupUser(t *testing.T, s *store.Store, email string) store.User {
	t.Helper()
	u, err := s.CreateUser(context.Background(), email, mustHashPassword(t, "pass"), false)
	if err != nil {
		t.Fatalf("setupUser(%q): %v", email, err)
	}
	return u
}

func TestCreateURL(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	u := setupUser(t, s, "urlcreate@example.com")

	tests := []struct {
		name        string
		rawURL      string
		title       string
		description string
		tags        []string
		wantErr     bool
	}{
		{
			name:        "basic url no tags",
			rawURL:      "https://example.com",
			title:       "Example",
			description: "A test URL",
			tags:        []string{},
		},
		{
			name:   "url with tags",
			rawURL: "https://go.dev",
			title:  "Go",
			tags:   []string{"go", "lang"},
		},
		{
			name:   "duplicate url for same user is a no-op",
			rawURL: "https://example.com",
			title:  "Example Again",
			tags:   []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			created, err := s.CreateURL(ctx, u.ID, tt.rawURL, tt.title, tt.description, tt.tags)
			if (err != nil) != tt.wantErr {
				t.Fatalf("CreateURL() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if created.URL != tt.rawURL {
				t.Errorf("URL = %q, want %q", created.URL, tt.rawURL)
			}
			if len(created.Tags) != len(tt.tags) {
				t.Errorf("Tags = %v, want %v", created.Tags, tt.tags)
			}
		})
	}
}

func TestGetURL(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	u := setupUser(t, s, "urlget@example.com")

	created, err := s.CreateURL(ctx, u.ID, "https://gettest.example.com", "Get Test", "desc", []string{"a", "b"})
	if err != nil {
		t.Fatalf("setup CreateURL: %v", err)
	}

	tests := []struct {
		name    string
		urlID   interface{ String() string }
		userID  interface{ String() string }
		wantErr error
		check   func(*testing.T, store.URL)
	}{
		{
			name:    "owner gets url with tags",
			urlID:   created.ID,
			userID:  u.ID,
			wantErr: nil,
			check: func(t *testing.T, got store.URL) {
				if got.URL != "https://gettest.example.com" {
					t.Errorf("URL = %q", got.URL)
				}
				if len(got.Tags) != 2 {
					t.Errorf("Tags = %v, want [a b]", got.Tags)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := s.GetURL(ctx, created.ID, u.ID)
			if err != tt.wantErr {
				t.Fatalf("GetURL() error = %v, want %v", err, tt.wantErr)
			}
			if tt.check != nil {
				tt.check(t, got)
			}
		})
	}
}

func TestGetURL_OtherUserCannotAccess(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	owner := setupUser(t, s, "owner-access@example.com")
	other := setupUser(t, s, "other-access@example.com")

	created, err := s.CreateURL(ctx, owner.ID, "https://private.example.com", "Private", "", []string{})
	if err != nil {
		t.Fatalf("setup: %v", err)
	}

	_, err = s.GetURL(ctx, created.ID, other.ID)
	if err != store.ErrNotFound {
		t.Errorf("GetURL by non-owner: error = %v, want ErrNotFound", err)
	}
}

func TestListURLs(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	u := setupUser(t, s, "list@example.com")

	urlsToCreate := []struct {
		url   string
		title string
		tags  []string
	}{
		{"https://one.example.com", "One", []string{"tag1"}},
		{"https://two.example.com", "Two", []string{"tag2"}},
		{"https://three.example.com", "Three", []string{}},
	}

	for _, c := range urlsToCreate {
		if _, err := s.CreateURL(ctx, u.ID, c.url, c.title, "", c.tags); err != nil {
			t.Fatalf("setup CreateURL(%q): %v", c.url, err)
		}
	}

	urls, err := s.ListURLs(ctx, u.ID)
	if err != nil {
		t.Fatalf("ListURLs: %v", err)
	}
	if len(urls) != len(urlsToCreate) {
		t.Errorf("len(urls) = %d, want %d", len(urls), len(urlsToCreate))
	}
}

func TestListURLs_IsolatedByUser(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	alice := setupUser(t, s, "alice-isolated@example.com")
	bob := setupUser(t, s, "bob-isolated@example.com")

	if _, err := s.CreateURL(ctx, alice.ID, "https://alice.example.com", "Alice's URL", "", []string{}); err != nil {
		t.Fatalf("setup alice: %v", err)
	}

	// Bob's list should be empty.
	urls, err := s.ListURLs(ctx, bob.ID)
	if err != nil {
		t.Fatalf("ListURLs(bob): %v", err)
	}
	if len(urls) != 0 {
		t.Errorf("bob sees %d urls, want 0", len(urls))
	}
}

func TestUpdateURL(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	u := setupUser(t, s, "update@example.com")

	created, err := s.CreateURL(ctx, u.ID, "https://update.example.com", "Original", "original desc", []string{"old"})
	if err != nil {
		t.Fatalf("setup: %v", err)
	}

	tests := []struct {
		name        string
		title       string
		description string
		tags        []string
		check       func(*testing.T, store.URL)
	}{
		{
			name:        "update title and description",
			title:       "Updated Title",
			description: "updated desc",
			tags:        []string{"old"},
			check: func(t *testing.T, u store.URL) {
				if u.Title != "Updated Title" {
					t.Errorf("Title = %q, want %q", u.Title, "Updated Title")
				}
				if u.Description != "updated desc" {
					t.Errorf("Description = %q", u.Description)
				}
			},
		},
		{
			name:  "replace tags",
			title: "Updated Title",
			tags:  []string{"new1", "new2"},
			check: func(t *testing.T, u store.URL) {
				if len(u.Tags) != 2 {
					t.Errorf("Tags = %v, want [new1 new2]", u.Tags)
				}
			},
		},
		{
			name:  "clear tags",
			title: "Updated Title",
			tags:  []string{},
			check: func(t *testing.T, u store.URL) {
				if len(u.Tags) != 0 {
					t.Errorf("Tags = %v, want []", u.Tags)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			updated, err := s.UpdateURL(ctx, created.ID, u.ID, tt.title, tt.description, tt.tags)
			if err != nil {
				t.Fatalf("UpdateURL: %v", err)
			}
			if tt.check != nil {
				tt.check(t, updated)
			}
		})
	}
}

func TestDeleteURL(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	u := setupUser(t, s, "delete@example.com")

	created, err := s.CreateURL(ctx, u.ID, "https://delete.example.com", "To Delete", "", []string{})
	if err != nil {
		t.Fatalf("setup: %v", err)
	}

	if err := s.DeleteURL(ctx, created.ID, u.ID); err != nil {
		t.Fatalf("DeleteURL: %v", err)
	}

	_, err = s.GetURL(ctx, created.ID, u.ID)
	if err != store.ErrNotFound {
		t.Errorf("after delete: GetURL error = %v, want ErrNotFound", err)
	}
}

func TestURLDeduplication(t *testing.T) {
	// The same raw URL bookmarked by two users should share one urls row
	// but each user gets independent metadata.
	s := testStore(t)
	ctx := context.Background()

	alice := setupUser(t, s, "alice-dedup@example.com")
	bob := setupUser(t, s, "bob-dedup@example.com")

	const rawURL = "https://shared.example.com"

	aliceURL, err := s.CreateURL(ctx, alice.ID, rawURL, "Alice's title", "", []string{"alice-tag"})
	if err != nil {
		t.Fatalf("alice CreateURL: %v", err)
	}

	bobURL, err := s.CreateURL(ctx, bob.ID, rawURL, "Bob's title", "", []string{"bob-tag"})
	if err != nil {
		t.Fatalf("bob CreateURL: %v", err)
	}

	// Same underlying URL id (deduplication).
	if aliceURL.ID != bobURL.ID {
		t.Errorf("URL IDs differ: alice=%v bob=%v — expected shared URL row", aliceURL.ID, bobURL.ID)
	}

	// Independent titles.
	if aliceURL.Title == bobURL.Title {
		t.Errorf("titles are the same (%q) — expected independent per-user titles", aliceURL.Title)
	}

	// Independent tags.
	if len(aliceURL.Tags) != 1 || aliceURL.Tags[0] != "alice-tag" {
		t.Errorf("alice tags = %v, want [alice-tag]", aliceURL.Tags)
	}
	if len(bobURL.Tags) != 1 || bobURL.Tags[0] != "bob-tag" {
		t.Errorf("bob tags = %v, want [bob-tag]", bobURL.Tags)
	}

	// Deleting for one user doesn't affect the other.
	if err := s.DeleteURL(ctx, aliceURL.ID, alice.ID); err != nil {
		t.Fatalf("alice DeleteURL: %v", err)
	}

	_, err = s.GetURL(ctx, bobURL.ID, bob.ID)
	if err != nil {
		t.Errorf("bob's URL gone after alice deleted: %v", err)
	}
}

func TestListURLsByTag(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	u := setupUser(t, s, "listbytag@example.com")

	if _, err := s.CreateURL(ctx, u.ID, "https://go.dev", "Go", "", []string{"programming", "go"}); err != nil {
		t.Fatalf("setup: %v", err)
	}
	if _, err := s.CreateURL(ctx, u.ID, "https://rust-lang.org", "Rust", "", []string{"programming", "rust"}); err != nil {
		t.Fatalf("setup: %v", err)
	}

	tests := []struct {
		tag       string
		wantCount int
	}{
		{"programming", 2},
		{"go", 1},
		{"rust", 1},
		{"nonexistent", 0},
	}

	for _, tt := range tests {
		t.Run(tt.tag, func(t *testing.T) {
			urls, err := s.ListURLsByTag(ctx, u.ID, tt.tag)
			if err != nil {
				t.Fatalf("ListURLsByTag: %v", err)
			}
			if len(urls) != tt.wantCount {
				t.Errorf("got %d urls, want %d", len(urls), tt.wantCount)
			}
		})
	}
}

func TestListURLsByCategory(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	u := setupUser(t, s, "listbycat@example.com")

	// URL 1: Matches by tag
	if _, err := s.CreateURL(ctx, u.ID, "https://blog.golang.org", "Go Blog", "", []string{"go"}); err != nil {
		t.Fatalf("setup: %v", err)
	}
	// URL 2: Matches by domain
	if _, err := s.CreateURL(ctx, u.ID, "https://news.ycombinator.com/item?id=1", "HN", "", []string{}); err != nil {
		t.Fatalf("setup: %v", err)
	}
	// URL 3: No match
	if _, err := s.CreateURL(ctx, u.ID, "https://example.com", "Example", "", []string{"other"}); err != nil {
		t.Fatalf("setup: %v", err)
	}

	cat, err := s.CreateCategory(ctx, u.ID, "My Category", []store.CategoryMember{
		{Kind: store.CategoryMemberKindTag, Value: "go"},
		{Kind: store.CategoryMemberKindDomain, Value: "news.ycombinator.com"},
	})
	if err != nil {
		t.Fatalf("CreateCategory: %v", err)
	}

	urls, err := s.ListURLsByCategory(ctx, u.ID, cat.ID)
	if err != nil {
		t.Fatalf("ListURLsByCategory: %v", err)
	}

	if len(urls) != 2 {
		t.Errorf("got %d urls, want 2", len(urls))
	}
}

func TestSearchURLs(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	u := setupUser(t, s, "search@example.com")

	if _, err := s.CreateURL(ctx, u.ID, "https://google.com", "Search Engine", "Google search", []string{}); err != nil {
		t.Fatalf("setup: %v", err)
	}
	if _, err := s.CreateURL(ctx, u.ID, "https://bing.com", "Bing", "Microsoft search", []string{}); err != nil {
		t.Fatalf("setup: %v", err)
	}

	tests := []struct {
		query     string
		wantCount int
	}{
		{"google", 1},
		{"search", 2},
		{"Microsoft", 1},
		{"nonexistent", 0},
	}

	for _, tt := range tests {
		t.Run(tt.query, func(t *testing.T) {
			urls, err := s.SearchURLs(ctx, u.ID, tt.query)
			if err != nil {
				t.Fatalf("SearchURLs: %v", err)
			}
			if len(urls) != tt.wantCount {
				t.Errorf("query %q: got %d urls, want %d", tt.query, len(urls), tt.wantCount)
			}
		})
	}
}
