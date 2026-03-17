package store

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/kyleterry/booksmk/internal/store/sqlstore"
)

type InviteCode struct {
	ID        uuid.UUID
	Code      string
	CreatedBy uuid.UUID
	UsedBy    *uuid.UUID
	UsedAt    *time.Time
	CreatedAt time.Time
}

func inviteCodeFromSQL(ic sqlstore.InviteCode) InviteCode {
	code := InviteCode{
		ID:        ic.ID,
		Code:      ic.Code,
		CreatedBy: ic.CreatedBy,
		CreatedAt: ic.CreatedAt.Time,
	}
	if ic.UsedBy.Valid {
		id := uuid.UUID(ic.UsedBy.Bytes)
		code.UsedBy = &id
	}
	if ic.UsedAt.Valid {
		t := ic.UsedAt.Time
		code.UsedAt = &t
	}
	return code
}

func (s *Store) CreateInviteCode(ctx context.Context, createdBy uuid.UUID) (InviteCode, error) {
	code, err := generateInviteCode()
	if err != nil {
		return InviteCode{}, err
	}

	ic, err := s.queries.CreateInviteCode(ctx, sqlstore.CreateInviteCodeParams{
		Code:      code,
		CreatedBy: createdBy,
	})
	if err != nil {
		return InviteCode{}, err
	}
	return inviteCodeFromSQL(ic), nil
}

func (s *Store) GetInviteCodeByCode(ctx context.Context, code string) (InviteCode, error) {
	ic, err := s.queries.GetInviteCodeByCode(ctx, code)
	if errors.Is(err, pgx.ErrNoRows) {
		return InviteCode{}, ErrNotFound
	}
	if err != nil {
		return InviteCode{}, err
	}
	return inviteCodeFromSQL(ic), nil
}

func (s *Store) ListInviteCodes(ctx context.Context) ([]InviteCode, error) {
	rows, err := s.queries.ListInviteCodes(ctx)
	if err != nil {
		return nil, err
	}
	codes := make([]InviteCode, len(rows))
	for i, ic := range rows {
		codes[i] = inviteCodeFromSQL(ic)
	}
	return codes, nil
}

func (s *Store) UseInviteCode(ctx context.Context, id, usedBy uuid.UUID) error {
	return s.queries.UseInviteCode(ctx, sqlstore.UseInviteCodeParams{
		UsedBy: pgtype.UUID{Bytes: usedBy, Valid: true},
		ID:     id,
	})
}

func (s *Store) DeleteInviteCode(ctx context.Context, id uuid.UUID) error {
	return s.queries.DeleteInviteCode(ctx, id)
}

func generateInviteCode() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
