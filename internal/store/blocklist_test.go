package store_test

import (
	"context"
	"testing"
)

func TestBlocklist(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	t.Run("Create and List", func(t *testing.T) {
		_, err := s.CreateBlocklistEntry(ctx, "example.com", "domain")
		if err != nil {
			t.Fatalf("CreateBlocklistEntry: %v", err)
		}

		entries, err := s.ListBlocklistEntries(ctx)
		if err != nil {
			t.Fatalf("ListBlocklistEntries: %v", err)
		}

		found := false
		for _, e := range entries {
			if e.Pattern == "example.com" && e.Kind == "domain" {
				found = true
				break
			}
		}
		if !found {
			t.Error("example.com domain block not found in entries")
		}
	})

	t.Run("IsBlocked - Domain", func(t *testing.T) {
		_, _ = s.CreateBlocklistEntry(ctx, "badsite.com", "domain")

		tests := []struct {
			url     string
			blocked bool
		}{
			{"https://badsite.com", true},
			{"https://www.badsite.com", true},
			{"https://sub.badsite.com/path", true},
			{"https://goodsite.com", false},
			{"https://badsite.com.ok.com", false},
		}

		for _, tt := range tests {
			blocked, err := s.IsBlocked(ctx, tt.url)
			if err != nil {
				t.Fatalf("IsBlocked(%q): %v", tt.url, err)
			}
			if blocked != tt.blocked {
				t.Errorf("IsBlocked(%q) = %v, want %v", tt.url, blocked, tt.blocked)
			}
		}
	})

	t.Run("IsBlocked - URL", func(t *testing.T) {
		const targetURL = "https://specific.com/bad"
		_, _ = s.CreateBlocklistEntry(ctx, targetURL, "url")

		tests := []struct {
			url     string
			blocked bool
		}{
			{targetURL, true},
			{"https://specific.com/bad/", false},
			{"https://specific.com/good", false},
			{"https://other.com/bad", false},
		}

		for _, tt := range tests {
			blocked, err := s.IsBlocked(ctx, tt.url)
			if err != nil {
				t.Fatalf("IsBlocked(%q): %v", tt.url, err)
			}
			if blocked != tt.blocked {
				t.Errorf("IsBlocked(%q) = %v, want %v", tt.url, blocked, tt.blocked)
			}
		}
	})
}
