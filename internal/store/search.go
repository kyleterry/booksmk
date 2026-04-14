package store

import (
	"context"

	"github.com/google/uuid"
	"go.e64ec.com/booksmk/internal/store/sqlstore"
)

// SearchResults contains matches from both URLs and Feeds.
type SearchResults struct {
	URLs  []URL
	Feeds []Feed
}

// Search performs a combined search across URLs and Feeds.
func (s *Store) Search(ctx context.Context, userID uuid.UUID, query string) (SearchResults, error) {
	q := "%" + query + "%"
	
	urlRows, err := s.queries.SearchURLs(ctx, sqlstore.SearchURLsParams{
		UserID: userID,
		Query:  q,
	})
	if err != nil {
		return SearchResults{}, err
	}
	
	feedRows, err := s.queries.SearchFeeds(ctx, sqlstore.SearchFeedsParams{
		UserID: userID,
		Query:  q,
	})
	if err != nil {
		return SearchResults{}, err
	}

	results := SearchResults{
		URLs:  make([]URL, len(urlRows)),
		Feeds: make([]Feed, len(feedRows)),
	}

	for i, u := range urlRows {
		results.URLs[i] = newURL(u.ID, u.Url, u.FeedUrl, u.Title, u.Description, u.Tags, u.CreatedAt, u.UpdatedAt)
	}

	for i, f := range feedRows {
		// Note: SearchFeeds doesn't return tags, so they will be empty in search results
		// to avoid N+1 queries. If tags are needed in search results, we'd need a more
		// complex query or a separate fetch.
		results.Feeds[i] = buildFeed(f.ID, f.FeedUrl, f.SiteUrl, f.Title, f.Description, f.ImageUrl, f.CustomName, f.LastFetchedAt, f.CreatedAt, f.UpdatedAt, nil)
	}

	return results, nil
}
