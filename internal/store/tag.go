package store

import (
	"context"
	"strings"

	"github.com/google/uuid"

	"go.e64ec.com/booksmk/internal/store/sqlstore"
)

// Slug lowercases s and converts it to a URL-safe slug, replacing spaces and
// non-alphanumeric characters with hyphens and collapsing consecutive hyphens.
func Slug(s string) string {
	s = strings.ToLower(s)
	var b strings.Builder
	prevHyphen := false
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z' || r >= '0' && r <= '9':
			b.WriteRune(r)
			prevHyphen = false
		case r == ' ' || r == '-' || r == '_':
			if !prevHyphen {
				b.WriteRune('-')
				prevHyphen = true
			}
		}
	}
	return strings.Trim(b.String(), "-")
}

// setURLTags replaces all tags for a user's URL with the given tag names.
// Tag names are upserted globally; the user-url-tag association is per-user.
func (s *Store) setURLTags(ctx context.Context, userID, urlID uuid.UUID, tags []string) error {
	if err := s.queries.RemoveAllTagsFromURL(ctx, sqlstore.RemoveAllTagsFromURLParams{UserID: userID, URLID: urlID}); err != nil {
		return err
	}
	if len(tags) == 0 {
		return nil
	}
	return s.queries.SetURLTags(ctx, sqlstore.SetURLTagsParams{
		UserID: userID,
		URLID:  urlID,
		Names:  tags,
	})
}
