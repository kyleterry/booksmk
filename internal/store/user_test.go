package store_test

import (
	"context"
	"testing"

	"github.com/kyleterry/booksmk/internal/store"
)

func TestCreateUser(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	tests := []struct {
		name    string
		email   string
		wantErr bool
	}{
		{name: "valid user", email: "alice@example.com", wantErr: false},
		{name: "duplicate email", email: "alice@example.com", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			digest := mustHashPassword(t, "secret")
			u, err := s.CreateUser(ctx, tt.email, digest)
			if (err != nil) != tt.wantErr {
				t.Fatalf("CreateUser() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr {
				if u.Email != tt.email {
					t.Errorf("Email = %q, want %q", u.Email, tt.email)
				}
				if u.ID.String() == "" {
					t.Error("ID is empty")
				}
			}
		})
	}
}

func TestGetUser(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	created, err := s.CreateUser(ctx, "bob@example.com", mustHashPassword(t, "secret"))
	if err != nil {
		t.Fatalf("setup: CreateUser: %v", err)
	}

	tests := []struct {
		name    string
		id      interface{ String() string }
		wantErr error
	}{
		{name: "existing user", id: created.ID, wantErr: nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u, err := s.GetUser(ctx, created.ID)
			if err != tt.wantErr {
				t.Fatalf("GetUser() error = %v, want %v", err, tt.wantErr)
			}
			if err == nil && u.Email != created.Email {
				t.Errorf("Email = %q, want %q", u.Email, created.Email)
			}
		})
	}
}

func TestGetUser_NotFound(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	u, err := s.CreateUser(ctx, "gettest@example.com", mustHashPassword(t, "pass"))
	if err != nil {
		t.Fatalf("setup: %v", err)
	}

	// Delete the user then try to fetch it.
	if err := s.DeleteUser(ctx, u.ID); err != nil {
		t.Fatalf("setup DeleteUser: %v", err)
	}

	_, err = s.GetUser(ctx, u.ID)
	if err != store.ErrNotFound {
		t.Errorf("GetUser() error = %v, want ErrNotFound", err)
	}
}

func TestGetUserByEmail(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	digest := mustHashPassword(t, "secret")
	created, err := s.CreateUser(ctx, "carol@example.com", digest)
	if err != nil {
		t.Fatalf("setup: %v", err)
	}

	tests := []struct {
		name    string
		email   string
		wantErr error
	}{
		{name: "existing email", email: "carol@example.com", wantErr: nil},
		{name: "missing email", email: "nobody@example.com", wantErr: store.ErrNotFound},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u, err := s.GetUserByEmail(ctx, tt.email)
			if err != tt.wantErr {
				t.Fatalf("GetUserByEmail() error = %v, want %v", err, tt.wantErr)
			}
			if err == nil && u.ID != created.ID {
				t.Errorf("ID = %v, want %v", u.ID, created.ID)
			}
		})
	}
}

func TestUpdateUser(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	u, err := s.CreateUser(ctx, "dave@example.com", mustHashPassword(t, "pass"))
	if err != nil {
		t.Fatalf("setup: %v", err)
	}

	tests := []struct {
		name     string
		newEmail string
		wantErr  bool
	}{
		{name: "update email", newEmail: "dave-updated@example.com", wantErr: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			updated, err := s.UpdateUser(ctx, u.ID, tt.newEmail)
			if (err != nil) != tt.wantErr {
				t.Fatalf("UpdateUser() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && updated.Email != tt.newEmail {
				t.Errorf("Email = %q, want %q", updated.Email, tt.newEmail)
			}
		})
	}
}

func TestUpdateUserPassword(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	u, err := s.CreateUser(ctx, "eve@example.com", mustHashPassword(t, "oldpass"))
	if err != nil {
		t.Fatalf("setup: %v", err)
	}

	newDigest := mustHashPassword(t, "newpass")
	updated, err := s.UpdateUserPassword(ctx, u.ID, newDigest)
	if err != nil {
		t.Fatalf("UpdateUserPassword: %v", err)
	}
	if updated.PasswordDigest != newDigest {
		t.Errorf("PasswordDigest not updated")
	}
}

func TestDeleteUser(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	u, err := s.CreateUser(ctx, "frank@example.com", mustHashPassword(t, "pass"))
	if err != nil {
		t.Fatalf("setup: %v", err)
	}

	if err := s.DeleteUser(ctx, u.ID); err != nil {
		t.Fatalf("DeleteUser: %v", err)
	}

	_, err = s.GetUser(ctx, u.ID)
	if err != store.ErrNotFound {
		t.Errorf("after delete: GetUser error = %v, want ErrNotFound", err)
	}
}
