package store_test

import (
	"context"
	"testing"

	"go.e64ec.com/booksmk/internal/store"
)

func TestCreateSession(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	u := mustCreateUser(t, s, "session-create@example.com")

	sess, err := s.CreateSession(ctx, u.ID)
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	if sess.Token == "" {
		t.Error("Token is empty")
	}
	if sess.UserID != u.ID {
		t.Errorf("UserID = %v, want %v", sess.UserID, u.ID)
	}
	if sess.ExpiresAt.IsZero() {
		t.Error("ExpiresAt is zero")
	}
}

func TestGetSessionUser(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	u := mustCreateUser(t, s, "session-get@example.com")

	t.Run("valid token returns user", func(t *testing.T) {
		sess, err := s.CreateSession(ctx, u.ID)
		if err != nil {
			t.Fatalf("setup CreateSession: %v", err)
		}

		got, err := s.GetSessionUser(ctx, sess.Token)
		if err != nil {
			t.Fatalf("GetSessionUser: %v", err)
		}
		if got.ID != u.ID {
			t.Errorf("ID = %v, want %v", got.ID, u.ID)
		}
		if got.Email != u.Email {
			t.Errorf("Email = %q, want %q", got.Email, u.Email)
		}
	})

	t.Run("unknown token returns ErrNotFound", func(t *testing.T) {
		_, err := s.GetSessionUser(ctx, "notarealtoken")
		if err != store.ErrNotFound {
			t.Errorf("error = %v, want ErrNotFound", err)
		}
	})
}

func TestDeleteSession(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	u := mustCreateUser(t, s, "session-delete@example.com")

	sess, err := s.CreateSession(ctx, u.ID)
	if err != nil {
		t.Fatalf("setup CreateSession: %v", err)
	}

	if err := s.DeleteSession(ctx, sess.Token); err != nil {
		t.Fatalf("DeleteSession: %v", err)
	}

	_, err = s.GetSessionUser(ctx, sess.Token)
	if err != store.ErrNotFound {
		t.Errorf("after delete: GetSessionUser error = %v, want ErrNotFound", err)
	}
}

func TestDeleteUserSessions(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	u := mustCreateUser(t, s, "session-delete-all@example.com")

	sess1, err := s.CreateSession(ctx, u.ID)
	if err != nil {
		t.Fatalf("setup CreateSession 1: %v", err)
	}
	sess2, err := s.CreateSession(ctx, u.ID)
	if err != nil {
		t.Fatalf("setup CreateSession 2: %v", err)
	}

	if err := s.DeleteUserSessions(ctx, u.ID); err != nil {
		t.Fatalf("DeleteUserSessions: %v", err)
	}

	for _, token := range []string{sess1.Token, sess2.Token} {
		_, err := s.GetSessionUser(ctx, token)
		if err != store.ErrNotFound {
			t.Errorf("after DeleteUserSessions: GetSessionUser(%q) = %v, want ErrNotFound", token, err)
		}
	}
}
