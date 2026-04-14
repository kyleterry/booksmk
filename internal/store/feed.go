package store

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"go.e64ec.com/booksmk/internal/store/sqlstore"
)

// Feed is a subscribed RSS/Atom feed belonging to a user.
type Feed struct {
	ID              uuid.UUID
	FeedURL         string
	SiteURL         string
	Title           string
	Description     string
	ImageURL        string
	IsBlockedBypass bool
	CustomName      string
	Tags            []string
	LastFetchedAt   *time.Time
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// FeedItem is a single item from a feed.
type FeedItem struct {
	ID          uuid.UUID
	FeedID      uuid.UUID
	GUID        string
	URL         string
	Title       string
	Summary     string
	Author      string
	PublishedAt *time.Time
	CreatedAt   time.Time
	IsRead      bool
}

// TimelineItem is a feed item enriched with feed metadata for the unified timeline.
type TimelineItem struct {
	ID           uuid.UUID
	FeedID       uuid.UUID
	FeedTitle    string
	FeedImageURL string
	GUID         string
	URL          string
	Title        string
	Summary      string
	Author       string
	PublishedAt  *time.Time
	CreatedAt    time.Time
	IsRead       bool
}

// TimelineGroup is a date-labelled group of timeline items.
type TimelineGroup struct {
	Label string
	Items []TimelineItem
}

// FeedItemGroup is a date-labelled group of feed items.
type FeedItemGroup struct {
	Label string
	Items []FeedItem
}

// FeedPollJob is a feed due for a poll cycle.
type FeedPollJob struct {
	ID         uuid.UUID
	FeedID     uuid.UUID
	FeedURL    string
	FetchCount int32
	ErrorCount int32
}

// UpsertFeedItemParams holds the data for creating or updating a feed item.
type UpsertFeedItemParams struct {
	FeedID      uuid.UUID
	GUID        string
	URL         string
	Title       string
	Summary     string
	Author      string
	PublishedAt *time.Time
}

func buildFeed(id uuid.UUID, feedUrl, siteUrl, title, description, imageUrl string, isBlockedBypass bool, customName string, lastFetchedAt pgtype.Timestamptz, createdAt, updatedAt pgtype.Timestamptz, tags []string) Feed {
	if tags == nil {
		tags = []string{}
	}
	f := Feed{
		ID:              id,
		FeedURL:         feedUrl,
		SiteURL:         siteUrl,
		Title:           title,
		Description:     description,
		ImageURL:        imageUrl,
		IsBlockedBypass: isBlockedBypass,
		CustomName:      customName,
		Tags:            tags,
		CreatedAt:       createdAt.Time,
		UpdatedAt:       updatedAt.Time,
	}
	if lastFetchedAt.Valid {
		t := lastFetchedAt.Time
		f.LastFetchedAt = &t
	}

	return f
}

func newFeedItem(id, feedID uuid.UUID, guid, url, title, summary, author string, publishedAt pgtype.Timestamptz, createdAt pgtype.Timestamptz, isRead bool) FeedItem {
	item := FeedItem{
		ID:        id,
		FeedID:    feedID,
		GUID:      guid,
		URL:       url,
		Title:     title,
		Summary:   summary,
		Author:    author,
		CreatedAt: createdAt.Time,
		IsRead:    isRead,
	}
	if publishedAt.Valid {
		t := publishedAt.Time
		item.PublishedAt = &t
	}

	return item
}

// SubscribeToFeed creates or retrieves the global feed record, links it to the
// user, sets tags, and enqueues a poll job. It is idempotent: subscribing twice
// returns the same feed without error.
func (s *Store) SubscribeToFeed(ctx context.Context, userID uuid.UUID, feedURL string, tags []string, isBlockedBypass bool) (Feed, error) {
	row, err := s.queries.UpsertFeed(ctx, sqlstore.UpsertFeedParams{
		FeedUrl:         feedURL,
		IsBlockedBypass: isBlockedBypass,
	})
	if err != nil {
		return Feed{}, err
	}
	if err := s.queries.AddFeedToUser(ctx, sqlstore.AddFeedToUserParams{
		UserID: userID,
		FeedID: row.ID,
	}); err != nil {
		return Feed{}, err
	}
	if err := s.setFeedTags(ctx, userID, row.ID, tags); err != nil {
		return Feed{}, err
	}
	if err := s.queries.EnqueueFeedPollJob(ctx, row.ID); err != nil {
		return Feed{}, err
	}
	return s.GetFeed(ctx, row.ID, userID)
}

// GetFeed returns the feed with the given id scoped to userID.
func (s *Store) GetFeed(ctx context.Context, id, userID uuid.UUID) (Feed, error) {
	row, err := s.queries.GetUserFeed(ctx, sqlstore.GetUserFeedParams{ID: id, UserID: userID})
	if errors.Is(err, pgx.ErrNoRows) {
		return Feed{}, ErrNotFound
	}
	if err != nil {
		return Feed{}, err
	}
	tags, err := s.queries.ListTagNamesForFeed(ctx, sqlstore.ListTagNamesForFeedParams{UserID: userID, FeedID: id})
	if err != nil {
		return Feed{}, err
	}
	return buildFeed(row.ID, row.FeedUrl, row.SiteUrl, row.Title, row.Description, row.ImageUrl, row.IsBlockedBypass, row.CustomName, row.LastFetchedAt, row.CreatedAt, row.UpdatedAt, tags), nil
}

// GetFeedByURL returns the user's subscription to the feed with the given URL.
// Returns ErrNotFound if the user is not subscribed.
func (s *Store) GetFeedByURL(ctx context.Context, userID uuid.UUID, feedURL string) (Feed, error) {
	row, err := s.queries.GetUserFeedByFeedURL(ctx, sqlstore.GetUserFeedByFeedURLParams{FeedUrl: feedURL, UserID: userID})
	if errors.Is(err, pgx.ErrNoRows) {
		return Feed{}, ErrNotFound
	}
	if err != nil {
		return Feed{}, err
	}
	tags, err := s.queries.ListTagNamesForFeed(ctx, sqlstore.ListTagNamesForFeedParams{UserID: userID, FeedID: row.ID})
	if err != nil {
		return Feed{}, err
	}
	return buildFeed(row.ID, row.FeedUrl, row.SiteUrl, row.Title, row.Description, row.ImageUrl, row.IsBlockedBypass, row.CustomName, row.LastFetchedAt, row.CreatedAt, row.UpdatedAt, tags), nil
}

// ListFeeds returns all feeds the user is subscribed to.
func (s *Store) ListFeeds(ctx context.Context, userID uuid.UUID) ([]Feed, error) {
	rows, err := s.queries.ListUserFeeds(ctx, userID)
	if err != nil {
		return nil, err
	}
	feeds := make([]Feed, len(rows))
	for i, row := range rows {
		tags, err := s.queries.ListTagNamesForFeed(ctx, sqlstore.ListTagNamesForFeedParams{UserID: userID, FeedID: row.ID})
		if err != nil {
			return nil, err
		}
		feeds[i] = buildFeed(row.ID, row.FeedUrl, row.SiteUrl, row.Title, row.Description, row.ImageUrl, row.IsBlockedBypass, row.CustomName, row.LastFetchedAt, row.CreatedAt, row.UpdatedAt, tags)
	}
	return feeds, nil
}

// SearchFeeds returns the user's feeds matching the query.
func (s *Store) SearchFeeds(ctx context.Context, userID uuid.UUID, query string) ([]Feed, error) {
	rows, err := s.queries.SearchFeeds(ctx, sqlstore.SearchFeedsParams{
		UserID: userID,
		Query:  "%" + query + "%",
	})
	if err != nil {
		return nil, err
	}
	feeds := make([]Feed, len(rows))
	for i, row := range rows {
		tags, err := s.queries.ListTagNamesForFeed(ctx, sqlstore.ListTagNamesForFeedParams{UserID: userID, FeedID: row.ID})
		if err != nil {
			return nil, err
		}
		feeds[i] = buildFeed(row.ID, row.FeedUrl, row.SiteUrl, row.Title, row.Description, row.ImageUrl, row.IsBlockedBypass, row.CustomName, row.LastFetchedAt, row.CreatedAt, row.UpdatedAt, tags)
	}
	return feeds, nil
}

// UnsubscribeFromFeed removes the user's subscription to the feed.
func (s *Store) UnsubscribeFromFeed(ctx context.Context, userID, feedID uuid.UUID) error {
	return s.queries.RemoveFeedFromUser(ctx, sqlstore.RemoveFeedFromUserParams{UserID: userID, FeedID: feedID})
}

// UpdateFeed updates the per-user custom name and tags for a feed.
func (s *Store) UpdateFeed(ctx context.Context, feedID, userID uuid.UUID, customName string, tags []string) (Feed, error) {
	if err := s.queries.UpdateUserFeedCustomName(ctx, sqlstore.UpdateUserFeedCustomNameParams{
		UserID:     userID,
		FeedID:     feedID,
		CustomName: customName,
	}); err != nil {
		return Feed{}, err
	}
	if err := s.setFeedTags(ctx, userID, feedID, tags); err != nil {
		return Feed{}, err
	}
	return s.GetFeed(ctx, feedID, userID)
}

// ListFeedItems returns items for a single feed, with per-user read state.
func (s *Store) ListFeedItems(ctx context.Context, feedID, userID uuid.UUID) ([]FeedItem, error) {
	rows, err := s.queries.ListFeedItems(ctx, sqlstore.ListFeedItemsParams{FeedID: feedID, UserID: userID})
	if err != nil {
		return nil, err
	}
	items := make([]FeedItem, len(rows))
	for i, row := range rows {
		items[i] = newFeedItem(row.ID, row.FeedID, row.Guid, row.Url, row.Title, row.Summary, row.Author, row.PublishedAt, row.CreatedAt, row.IsRead)
	}
	return items, nil
}

// ListTimelineItems returns items from all of the user's feeds, paginated.
func (s *Store) ListTimelineItems(ctx context.Context, userID uuid.UUID, limit, offset int) ([]TimelineItem, error) {
	rows, err := s.queries.ListTimelineItems(ctx, sqlstore.ListTimelineItemsParams{
		UserID: userID,
		Limit:  int32(limit),
		Offset: int32(offset),
	})
	if err != nil {
		return nil, err
	}
	items := make([]TimelineItem, len(rows))
	for i, row := range rows {
		item := TimelineItem{
			ID:           row.ID,
			FeedID:       row.FeedID,
			FeedTitle:    row.FeedTitle,
			FeedImageURL: row.FeedImageUrl,
			GUID:         row.Guid,
			URL:          row.Url,
			Title:        row.ItemTitle,
			Summary:      row.Summary,
			Author:       row.Author,
			CreatedAt:    row.CreatedAt.Time,
			IsRead:       row.IsRead,
		}
		if row.PublishedAt.Valid {
			t := row.PublishedAt.Time
			item.PublishedAt = &t
		}
		items[i] = item
	}
	return items, nil
}

// GetTimelineItem returns a single timeline item scoped to the user.
func (s *Store) GetTimelineItem(ctx context.Context, userID, itemID uuid.UUID) (TimelineItem, error) {
	row, err := s.queries.GetTimelineItem(ctx, sqlstore.GetTimelineItemParams{UserID: userID, ID: itemID})
	if errors.Is(err, pgx.ErrNoRows) {
		return TimelineItem{}, ErrNotFound
	}
	if err != nil {
		return TimelineItem{}, err
	}

	item := TimelineItem{
		ID:           row.ID,
		FeedID:       row.FeedID,
		FeedTitle:    row.FeedTitle,
		FeedImageURL: row.FeedImageUrl,
		GUID:         row.Guid,
		URL:          row.Url,
		Title:        row.ItemTitle,
		Summary:      row.Summary,
		Author:       row.Author,
		CreatedAt:    row.CreatedAt.Time,
		IsRead:       row.IsRead,
	}
	if row.PublishedAt.Valid {
		t := row.PublishedAt.Time
		item.PublishedAt = &t
	}

	return item, nil
}

// MarkItemRead records that the user has read the item.
func (s *Store) MarkItemRead(ctx context.Context, userID, itemID uuid.UUID) error {
	return s.queries.MarkItemRead(ctx, sqlstore.MarkItemReadParams{UserID: userID, ItemID: itemID})
}

// MarkItemUnread removes the user's read record for the item.
func (s *Store) MarkItemUnread(ctx context.Context, userID, itemID uuid.UUID) error {
	return s.queries.MarkItemUnread(ctx, sqlstore.MarkItemUnreadParams{UserID: userID, ItemID: itemID})
}

// MarkAllItemsRead marks every item in all of the user's feeds as read.
func (s *Store) MarkAllItemsRead(ctx context.Context, userID uuid.UUID) error {
	return s.queries.MarkAllItemsRead(ctx, userID)
}

// MarkFeedItemsRead marks every item in a specific feed as read for the user.
func (s *Store) MarkFeedItemsRead(ctx context.Context, userID, feedID uuid.UUID) error {
	return s.queries.MarkFeedItemsRead(ctx, sqlstore.MarkFeedItemsReadParams{UserID: userID, FeedID: feedID})
}

// ListDueFeedPollJobs returns feeds whose poll is scheduled at or before now.
func (s *Store) ListDueFeedPollJobs(ctx context.Context) ([]FeedPollJob, error) {
	rows, err := s.queries.ListDueFeedPollJobs(ctx)
	if err != nil {
		return nil, err
	}
	jobs := make([]FeedPollJob, len(rows))
	for i, r := range rows {
		jobs[i] = FeedPollJob{
			ID:         r.ID,
			FeedID:     r.FeedID,
			FeedURL:    r.FeedUrl,
			FetchCount: r.FetchCount,
			ErrorCount: r.ErrorCount,
		}
	}
	return jobs, nil
}

// CompleteFeedPollJob updates the poll job after processing.
func (s *Store) CompleteFeedPollJob(ctx context.Context, jobID uuid.UUID, nextAt time.Time, fetchCount, errorCount int32, lastError string) error {
	return s.queries.CompleteFeedPollJob(ctx, sqlstore.CompleteFeedPollJobParams{
		ID:          jobID,
		ScheduledAt: pgtype.Timestamptz{Time: nextAt.UTC(), Valid: true},
		FetchCount:  fetchCount,
		ErrorCount:  errorCount,
		LastError:   lastError,
	})
}

// UpdateFeedMeta updates the feed's title, site URL, description, and image after a
// successful poll.
func (s *Store) UpdateFeedMeta(ctx context.Context, feedID uuid.UUID, siteURL, title, description, imageURL string) error {
	return s.queries.UpdateFeedMeta(ctx, sqlstore.UpdateFeedMetaParams{
		ID:          feedID,
		SiteUrl:     siteURL,
		Title:       title,
		Description: description,
		ImageUrl:    imageURL,
	})
}

// UpsertFeedItem inserts or updates a feed item, returning its ID.
func (s *Store) UpsertFeedItem(ctx context.Context, p UpsertFeedItemParams) (uuid.UUID, error) {
	var publishedAt pgtype.Timestamptz
	if p.PublishedAt != nil {
		publishedAt = pgtype.Timestamptz{Time: p.PublishedAt.UTC(), Valid: true}
	}
	return s.queries.UpsertFeedItem(ctx, sqlstore.UpsertFeedItemParams{
		FeedID:      p.FeedID,
		Guid:        p.GUID,
		Url:         p.URL,
		Title:       p.Title,
		Summary:     p.Summary,
		Author:      p.Author,
		PublishedAt: publishedAt,
	})
}

// EnqueueFeedPollJob schedules a feed for its first (or next) poll.
func (s *Store) EnqueueFeedPollJob(ctx context.Context, feedID uuid.UUID) error {
	return s.queries.EnqueueFeedPollJob(ctx, feedID)
}

// setFeedTags replaces all tags for a user's feed with the given names.
func (s *Store) setFeedTags(ctx context.Context, userID, feedID uuid.UUID, tags []string) error {
	if err := s.queries.RemoveAllTagsFromFeed(ctx, sqlstore.RemoveAllTagsFromFeedParams{UserID: userID, FeedID: feedID}); err != nil {
		return err
	}
	for _, name := range tags {
		tag, err := s.queries.UpsertTag(ctx, name)
		if err != nil {
			return err
		}
		if err := s.queries.AddTagToFeed(ctx, sqlstore.AddTagToFeedParams{
			UserID: userID,
			FeedID: feedID,
			TagID:  tag.ID,
		}); err != nil {
			return err
		}
	}
	return nil
}
