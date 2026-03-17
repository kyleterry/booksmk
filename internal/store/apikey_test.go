package store_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/kyleterry/booksmk/internal/store"
)

// mustCreateUser is a test helper that creates a user and fails the test on error.
func mustCreateUser(t *testing.T, s *store.Store, email string) store.User {
	t.Helper()
	u, err := s.CreateUser(context.Background(), email, mustHashPassword(t, "secret"), false)
	if err != nil {
		t.Fatalf("mustCreateUser(%q): %v", email, err)
	}
	return u
}

func TestCreateAPIKey(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	u := mustCreateUser(t, s, "apikey-create@example.com")

	t.Run("creates key and returns token", func(t *testing.T) {
		result, err := s.CreateAPIKey(ctx, u.ID, "test key", nil)
		if err != nil {
			t.Fatalf("CreateAPIKey: %v", err)
		}
		if result.Name != "test key" {
			t.Errorf("Name = %q, want %q", result.Name, "test key")
		}
		if !strings.HasPrefix(result.Token, "bsmk_") {
			t.Errorf("Token = %q, want bsmk_ prefix", result.Token)
		}
		if result.TokenPrefix != result.Token[:12] {
			t.Errorf("TokenPrefix = %q, want first 12 chars of token %q", result.TokenPrefix, result.Token[:12])
		}
		if result.ExpiresAt != nil {
			t.Errorf("ExpiresAt = %v, want nil", result.ExpiresAt)
		}
	})

	t.Run("creates key with expiry", func(t *testing.T) {
		exp := time.Now().Add(30 * 24 * time.Hour)
		result, err := s.CreateAPIKey(ctx, u.ID, "expiring key", &exp)
		if err != nil {
			t.Fatalf("CreateAPIKey: %v", err)
		}
		if result.ExpiresAt == nil {
			t.Fatal("ExpiresAt is nil, want non-nil")
		}
		diff := result.ExpiresAt.Sub(exp)
		if diff < -time.Second || diff > time.Second {
			t.Errorf("ExpiresAt = %v, want ~%v", result.ExpiresAt, exp)
		}
	})
}

func TestCreateAPIKey_Limit(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	u := mustCreateUser(t, s, "apikey-limit@example.com")

	for i := range 5 {
		_, err := s.CreateAPIKey(ctx, u.ID, "key", nil)
		if err != nil {
			t.Fatalf("CreateAPIKey %d: %v", i+1, err)
		}
	}

	_, err := s.CreateAPIKey(ctx, u.ID, "one too many", nil)
	if err != store.ErrAPIKeyLimitReached {
		t.Errorf("CreateAPIKey at limit: error = %v, want ErrAPIKeyLimitReached", err)
	}
}

func TestListAPIKeys(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	u := mustCreateUser(t, s, "apikey-list@example.com")

	t.Run("empty list", func(t *testing.T) {
		keys, err := s.ListAPIKeys(ctx, u.ID)
		if err != nil {
			t.Fatalf("ListAPIKeys: %v", err)
		}
		if len(keys) != 0 {
			t.Errorf("len = %d, want 0", len(keys))
		}
	})

	t.Run("returns created keys", func(t *testing.T) {
		if _, err := s.CreateAPIKey(ctx, u.ID, "alpha", nil); err != nil {
			t.Fatalf("setup CreateAPIKey: %v", err)
		}
		if _, err := s.CreateAPIKey(ctx, u.ID, "beta", nil); err != nil {
			t.Fatalf("setup CreateAPIKey: %v", err)
		}

		keys, err := s.ListAPIKeys(ctx, u.ID)
		if err != nil {
			t.Fatalf("ListAPIKeys: %v", err)
		}
		if len(keys) != 2 {
			t.Fatalf("len = %d, want 2", len(keys))
		}
		// most recently created first
		if keys[0].Name != "beta" {
			t.Errorf("keys[0].Name = %q, want %q", keys[0].Name, "beta")
		}
	})

	t.Run("does not return other users keys", func(t *testing.T) {
		other := mustCreateUser(t, s, "apikey-list-other@example.com")
		if _, err := s.CreateAPIKey(ctx, other.ID, "other key", nil); err != nil {
			t.Fatalf("setup CreateAPIKey: %v", err)
		}

		keys, err := s.ListAPIKeys(ctx, u.ID)
		if err != nil {
			t.Fatalf("ListAPIKeys: %v", err)
		}
		for _, k := range keys {
			if k.UserID != u.ID {
				t.Errorf("got key belonging to user %v, want only user %v", k.UserID, u.ID)
			}
		}
	})
}

func TestDeleteAPIKey(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	u := mustCreateUser(t, s, "apikey-delete@example.com")

	t.Run("deletes own key", func(t *testing.T) {
		result, err := s.CreateAPIKey(ctx, u.ID, "to delete", nil)
		if err != nil {
			t.Fatalf("setup CreateAPIKey: %v", err)
		}

		if err := s.DeleteAPIKey(ctx, result.ID, u.ID); err != nil {
			t.Fatalf("DeleteAPIKey: %v", err)
		}

		keys, err := s.ListAPIKeys(ctx, u.ID)
		if err != nil {
			t.Fatalf("ListAPIKeys: %v", err)
		}
		for _, k := range keys {
			if k.ID == result.ID {
				t.Errorf("key %v still present after delete", result.ID)
			}
		}
	})

	t.Run("cannot delete another users key", func(t *testing.T) {
		other := mustCreateUser(t, s, "apikey-delete-other@example.com")
		result, err := s.CreateAPIKey(ctx, other.ID, "others key", nil)
		if err != nil {
			t.Fatalf("setup CreateAPIKey: %v", err)
		}

		// Delete with u.ID instead of other.ID — should silently do nothing.
		if err := s.DeleteAPIKey(ctx, result.ID, u.ID); err != nil {
			t.Fatalf("DeleteAPIKey: %v", err)
		}

		keys, err := s.ListAPIKeys(ctx, other.ID)
		if err != nil {
			t.Fatalf("ListAPIKeys: %v", err)
		}
		if len(keys) != 1 {
			t.Errorf("other user's key was deleted by wrong user")
		}
	})
}

func TestGetAPIKeyByToken(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	u := mustCreateUser(t, s, "apikey-lookup@example.com")

	t.Run("valid token returns key", func(t *testing.T) {
		result, err := s.CreateAPIKey(ctx, u.ID, "lookup key", nil)
		if err != nil {
			t.Fatalf("setup CreateAPIKey: %v", err)
		}

		got, err := s.GetAPIKeyByToken(ctx, result.Token)
		if err != nil {
			t.Fatalf("GetAPIKeyByToken: %v", err)
		}
		if got.ID != result.ID {
			t.Errorf("ID = %v, want %v", got.ID, result.ID)
		}
		if got.UserID != u.ID {
			t.Errorf("UserID = %v, want %v", got.UserID, u.ID)
		}
	})

	t.Run("unknown token returns ErrNotFound", func(t *testing.T) {
		_, err := s.GetAPIKeyByToken(ctx, "bsmk_notarealtoken")
		if err != store.ErrNotFound {
			t.Errorf("error = %v, want ErrNotFound", err)
		}
	})

	t.Run("expired token returns ErrNotFound", func(t *testing.T) {
		past := time.Now().Add(-time.Hour)
		result, err := s.CreateAPIKey(ctx, u.ID, "expired key", &past)
		if err != nil {
			t.Fatalf("setup CreateAPIKey: %v", err)
		}

		_, err = s.GetAPIKeyByToken(ctx, result.Token)
		if err != store.ErrNotFound {
			t.Errorf("error = %v, want ErrNotFound for expired token", err)
		}
	})

	t.Run("non-expired token is found", func(t *testing.T) {
		future := time.Now().Add(24 * time.Hour)
		result, err := s.CreateAPIKey(ctx, u.ID, "future key", &future)
		if err != nil {
			t.Fatalf("setup CreateAPIKey: %v", err)
		}

		if _, err := s.GetAPIKeyByToken(ctx, result.Token); err != nil {
			t.Errorf("GetAPIKeyByToken: %v, want nil", err)
		}
	})
}
