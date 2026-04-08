package store

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"go.e64ec.com/booksmk/internal/store/sqlstore"
)

type CategoryMemberKind string

const (
	CategoryMemberKindTag    CategoryMemberKind = "tag"
	CategoryMemberKindDomain CategoryMemberKind = "domain"
)

// CategoryMember is a single tag or domain filter within a category.
type CategoryMember struct {
	Kind  CategoryMemberKind
	Value string
}

// Category is a named aggregate of tag and domain filters belonging to a user.
type Category struct {
	ID        uuid.UUID
	UserID    uuid.UUID
	Name      string
	Members   []CategoryMember
	CreatedAt pgtype.Timestamptz
	UpdatedAt pgtype.Timestamptz
}

func (s *Store) GetCategory(ctx context.Context, id, userID uuid.UUID) (Category, error) {
	row, err := s.queries.GetCategory(ctx, sqlstore.GetCategoryParams{ID: id, UserID: userID})
	if errors.Is(err, pgx.ErrNoRows) {
		return Category{}, ErrNotFound
	}
	if err != nil {
		return Category{}, err
	}

	members, err := s.listCategoryMembers(ctx, row.ID)
	if err != nil {
		return Category{}, err
	}

	return newCategory(row, members), nil
}

func (s *Store) ListCategories(ctx context.Context, userID uuid.UUID) ([]Category, error) {
	rows, err := s.queries.ListCategories(ctx, userID)
	if err != nil {
		return nil, err
	}

	cats := make([]Category, len(rows))

	for i, row := range rows {
		members, err := s.listCategoryMembers(ctx, row.ID)
		if err != nil {
			return nil, err
		}
		cats[i] = newCategory(row, members)
	}

	return cats, nil
}

func (s *Store) CreateCategory(ctx context.Context, userID uuid.UUID, name string, members []CategoryMember) (Category, error) {
	row, err := s.queries.InsertCategory(ctx, sqlstore.InsertCategoryParams{UserID: userID, Name: name})
	if err != nil {
		return Category{}, err
	}

	if err := s.setCategoryMembers(ctx, row.ID, members); err != nil {
		return Category{}, err
	}

	return s.GetCategory(ctx, row.ID, userID)
}

func (s *Store) UpdateCategory(ctx context.Context, id, userID uuid.UUID, name string, members []CategoryMember) (Category, error) {
	row, err := s.queries.UpdateCategory(ctx, sqlstore.UpdateCategoryParams{Name: name, ID: id, UserID: userID})
	if errors.Is(err, pgx.ErrNoRows) {
		return Category{}, ErrNotFound
	}
	if err != nil {
		return Category{}, err
	}

	if err := s.setCategoryMembers(ctx, row.ID, members); err != nil {
		return Category{}, err
	}

	return s.GetCategory(ctx, row.ID, userID)
}

func (s *Store) DeleteCategory(ctx context.Context, id, userID uuid.UUID) error {
	return s.queries.DeleteCategory(ctx, sqlstore.DeleteCategoryParams{ID: id, UserID: userID})
}

func (s *Store) listCategoryMembers(ctx context.Context, categoryID uuid.UUID) ([]CategoryMember, error) {
	rows, err := s.queries.ListCategoryMembers(ctx, categoryID)
	if err != nil {
		return nil, err
	}

	members := make([]CategoryMember, len(rows))

	for i, row := range rows {
		members[i] = CategoryMember{Kind: CategoryMemberKind(row.Kind), Value: row.Value}
	}

	return members, nil
}

func (s *Store) setCategoryMembers(ctx context.Context, categoryID uuid.UUID, members []CategoryMember) error {
	if err := s.queries.DeleteAllCategoryMembers(ctx, categoryID); err != nil {
		return err
	}

	for _, m := range members {
		if err := s.queries.InsertCategoryMember(ctx, sqlstore.InsertCategoryMemberParams{
			CategoryID: categoryID,
			Kind:       string(m.Kind),
			Value:      m.Value,
		}); err != nil {
			return err
		}
	}

	return nil
}

func newCategory(row sqlstore.Category, members []CategoryMember) Category {
	if members == nil {
		members = []CategoryMember{}
	}
	return Category{
		ID:        row.ID,
		UserID:    row.UserID,
		Name:      row.Name,
		Members:   members,
		CreatedAt: row.CreatedAt,
		UpdatedAt: row.UpdatedAt,
	}
}
