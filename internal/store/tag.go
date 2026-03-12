package store

import (
	"context"

	"github.com/google/uuid"

	"github.com/kyleterry/booksmk/internal/store/sqlstore"
)

// setURLTags replaces all tags for a user's URL with the given tag names.
// Tag names are upserted globally; the user-url-tag association is per-user.
func (s *Store) setURLTags(ctx context.Context, userID, urlID uuid.UUID, tags []string) error {
	if err := s.queries.RemoveAllTagsFromURL(ctx, sqlstore.RemoveAllTagsFromURLParams{UserID: userID, URLID: urlID}); err != nil {
		return err
	}
	for _, name := range tags {
		tag, err := s.queries.UpsertTag(ctx, name)
		if err != nil {
			return err
		}
		if err := s.queries.AddTagToURL(ctx, sqlstore.AddTagToURLParams{
			UserID: userID,
			URLID:  urlID,
			TagID:  tag.ID,
		}); err != nil {
			return err
		}
	}
	return nil
}
