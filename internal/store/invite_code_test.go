package store_test

import (
	"context"
	"testing"

	"go.e64ec.com/booksmk/internal/store"
)

func TestCreateInviteCode(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	u := mustCreateUser(t, s, "invite-create@example.com")

	ic, err := s.CreateInviteCode(ctx, u.ID)
	if err != nil {
		t.Fatalf("CreateInviteCode: %v", err)
	}
	if ic.Code == "" {
		t.Error("Code is empty")
	}
	if ic.CreatedBy != u.ID {
		t.Errorf("CreatedBy = %v, want %v", ic.CreatedBy, u.ID)
	}
	if ic.UsedBy != nil {
		t.Errorf("UsedBy = %v, want nil", ic.UsedBy)
	}
	if ic.UsedAt != nil {
		t.Errorf("UsedAt = %v, want nil", ic.UsedAt)
	}
}

func TestGetInviteCodeByCode(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	u := mustCreateUser(t, s, "invite-get@example.com")

	t.Run("existing code", func(t *testing.T) {
		ic, err := s.CreateInviteCode(ctx, u.ID)
		if err != nil {
			t.Fatalf("setup CreateInviteCode: %v", err)
		}

		got, err := s.GetInviteCodeByCode(ctx, ic.Code)
		if err != nil {
			t.Fatalf("GetInviteCodeByCode: %v", err)
		}
		if got.ID != ic.ID {
			t.Errorf("ID = %v, want %v", got.ID, ic.ID)
		}
	})

	t.Run("missing code returns ErrNotFound", func(t *testing.T) {
		_, err := s.GetInviteCodeByCode(ctx, "doesnotexist")
		if err != store.ErrNotFound {
			t.Errorf("error = %v, want ErrNotFound", err)
		}
	})
}

func TestListInviteCodes(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	u := mustCreateUser(t, s, "invite-list@example.com")

	t.Run("empty list", func(t *testing.T) {
		codes, err := s.ListInviteCodes(ctx)
		if err != nil {
			t.Fatalf("ListInviteCodes: %v", err)
		}
		if len(codes) != 0 {
			t.Errorf("len = %d, want 0", len(codes))
		}
	})

	t.Run("returns all codes", func(t *testing.T) {
		if _, err := s.CreateInviteCode(ctx, u.ID); err != nil {
			t.Fatalf("setup CreateInviteCode: %v", err)
		}
		if _, err := s.CreateInviteCode(ctx, u.ID); err != nil {
			t.Fatalf("setup CreateInviteCode: %v", err)
		}

		codes, err := s.ListInviteCodes(ctx)
		if err != nil {
			t.Fatalf("ListInviteCodes: %v", err)
		}
		if len(codes) != 2 {
			t.Errorf("len = %d, want 2", len(codes))
		}
	})
}

func TestUseInviteCode(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	creator := mustCreateUser(t, s, "invite-use-creator@example.com")
	consumer := mustCreateUser(t, s, "invite-use-consumer@example.com")

	ic, err := s.CreateInviteCode(ctx, creator.ID)
	if err != nil {
		t.Fatalf("setup CreateInviteCode: %v", err)
	}

	if err := s.UseInviteCode(ctx, ic.ID, consumer.ID); err != nil {
		t.Fatalf("UseInviteCode: %v", err)
	}

	got, err := s.GetInviteCodeByCode(ctx, ic.Code)
	if err != nil {
		t.Fatalf("GetInviteCodeByCode after use: %v", err)
	}
	if got.UsedBy == nil {
		t.Fatal("UsedBy is nil after UseInviteCode")
	}
	if *got.UsedBy != consumer.ID {
		t.Errorf("UsedBy = %v, want %v", *got.UsedBy, consumer.ID)
	}
	if got.UsedAt == nil {
		t.Error("UsedAt is nil after UseInviteCode")
	}
}

func TestDeleteInviteCode(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	u := mustCreateUser(t, s, "invite-delete@example.com")

	ic, err := s.CreateInviteCode(ctx, u.ID)
	if err != nil {
		t.Fatalf("setup CreateInviteCode: %v", err)
	}

	if err := s.DeleteInviteCode(ctx, ic.ID); err != nil {
		t.Fatalf("DeleteInviteCode: %v", err)
	}

	_, err = s.GetInviteCodeByCode(ctx, ic.Code)
	if err != store.ErrNotFound {
		t.Errorf("after delete: GetInviteCodeByCode error = %v, want ErrNotFound", err)
	}
}
