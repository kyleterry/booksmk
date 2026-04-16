package store_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"go.e64ec.com/booksmk/internal/store"
)

// mustSubscribe is a test helper that subscribes a user to a feed and fails on error.
func mustSubscribe(t *testing.T, s *store.Store, userID uuid.UUID, feedURL string, tags []string) store.Feed {
	t.Helper()
	f, err := s.SubscribeToFeed(context.Background(), userID, feedURL, tags, false)
	if err != nil {
		t.Fatalf("mustSubscribe(%q): %v", feedURL, err)
	}
	return f
}

// mustUpsertItem is a test helper that creates a feed item and fails on error.
func mustUpsertItem(t *testing.T, s *store.Store, feedID uuid.UUID, guid, title string) uuid.UUID {
	t.Helper()
	id, err := s.UpsertFeedItem(context.Background(), store.UpsertFeedItemParams{
		FeedID: feedID,
		GUID:   guid,
		URL:    "https://example.com/" + guid,
		Title:  title,
	})
	if err != nil {
		t.Fatalf("mustUpsertItem(%q): %v", guid, err)
	}
	return id
}

func TestSubscribeToFeed(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	u := mustCreateUser(t, s, "feed-subscribe@example.com")

	t.Run("creates subscription", func(t *testing.T) {
		f, err := s.SubscribeToFeed(ctx, u.ID, "https://example.com/feed.xml", []string{"tech", "go"}, false)
		if err != nil {
			t.Fatalf("SubscribeToFeed: %v", err)
		}
		if f.FeedURL != "https://example.com/feed.xml" {
			t.Errorf("FeedURL = %q, want %q", f.FeedURL, "https://example.com/feed.xml")
		}
		if len(f.Tags) != 2 {
			t.Errorf("Tags = %v, want 2 tags", f.Tags)
		}
	})

	t.Run("idempotent — subscribing twice returns same feed", func(t *testing.T) {
		f1, err := s.SubscribeToFeed(ctx, u.ID, "https://idempotent.example.com/feed.xml", nil, false)
		if err != nil {
			t.Fatalf("first SubscribeToFeed: %v", err)
		}
		f2, err := s.SubscribeToFeed(ctx, u.ID, "https://idempotent.example.com/feed.xml", nil, false)
		if err != nil {
			t.Fatalf("second SubscribeToFeed: %v", err)
		}
		if f1.ID != f2.ID {
			t.Errorf("IDs differ: %v != %v", f1.ID, f2.ID)
		}
	})

	t.Run("two users can subscribe to the same feed url", func(t *testing.T) {
		other := mustCreateUser(t, s, "feed-subscribe-other@example.com")
		const sharedURL = "https://shared.example.com/feed.xml"

		f1 := mustSubscribe(t, s, u.ID, sharedURL, nil)
		f2 := mustSubscribe(t, s, other.ID, sharedURL, nil)

		if f1.ID != f2.ID {
			t.Errorf("shared feed IDs differ: %v != %v", f1.ID, f2.ID)
		}
	})
}

func TestGetFeed(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	u := mustCreateUser(t, s, "feed-get@example.com")

	f := mustSubscribe(t, s, u.ID, "https://get.example.com/feed.xml", []string{"news"})

	t.Run("returns feed with tags", func(t *testing.T) {
		got, err := s.GetFeed(ctx, f.ID, u.ID)
		if err != nil {
			t.Fatalf("GetFeed: %v", err)
		}
		if got.FeedURL != f.FeedURL {
			t.Errorf("FeedURL = %q, want %q", got.FeedURL, f.FeedURL)
		}
		if len(got.Tags) != 1 || got.Tags[0] != "news" {
			t.Errorf("Tags = %v, want [news]", got.Tags)
		}
	})

	t.Run("wrong user returns ErrNotFound", func(t *testing.T) {
		other := mustCreateUser(t, s, "feed-get-other@example.com")
		_, err := s.GetFeed(ctx, f.ID, other.ID)
		if err != store.ErrNotFound {
			t.Errorf("error = %v, want ErrNotFound", err)
		}
	})

	t.Run("unknown id returns ErrNotFound", func(t *testing.T) {
		_, err := s.GetFeed(ctx, uuid.New(), u.ID)
		if err != store.ErrNotFound {
			t.Errorf("error = %v, want ErrNotFound", err)
		}
	})
}

func TestGetFeedByURL(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	u := mustCreateUser(t, s, "feed-byurl@example.com")

	const feedURL = "https://byurl.example.com/feed.xml"
	mustSubscribe(t, s, u.ID, feedURL, nil)

	t.Run("subscribed url returns feed", func(t *testing.T) {
		got, err := s.GetFeedByURL(ctx, u.ID, feedURL)
		if err != nil {
			t.Fatalf("GetFeedByURL: %v", err)
		}
		if got.FeedURL != feedURL {
			t.Errorf("FeedURL = %q, want %q", got.FeedURL, feedURL)
		}
	})

	t.Run("unsubscribed url returns ErrNotFound", func(t *testing.T) {
		_, err := s.GetFeedByURL(ctx, u.ID, "https://notsubscribed.example.com/feed.xml")
		if err != store.ErrNotFound {
			t.Errorf("error = %v, want ErrNotFound", err)
		}
	})
}

func TestListFeeds(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	u := mustCreateUser(t, s, "feed-list@example.com")

	t.Run("empty list", func(t *testing.T) {
		feeds, err := s.ListFeeds(ctx, u.ID)
		if err != nil {
			t.Fatalf("ListFeeds: %v", err)
		}
		if len(feeds) != 0 {
			t.Errorf("len = %d, want 0", len(feeds))
		}
	})

	t.Run("returns all subscribed feeds", func(t *testing.T) {
		mustSubscribe(t, s, u.ID, "https://list-a.example.com/feed.xml", nil)
		mustSubscribe(t, s, u.ID, "https://list-b.example.com/feed.xml", nil)

		feeds, err := s.ListFeeds(ctx, u.ID)
		if err != nil {
			t.Fatalf("ListFeeds: %v", err)
		}
		if len(feeds) != 2 {
			t.Errorf("len = %d, want 2", len(feeds))
		}
	})

	t.Run("does not return other users feeds", func(t *testing.T) {
		other := mustCreateUser(t, s, "feed-list-other@example.com")
		mustSubscribe(t, s, other.ID, "https://list-other.example.com/feed.xml", nil)

		feeds, err := s.ListFeeds(ctx, u.ID)
		if err != nil {
			t.Fatalf("ListFeeds: %v", err)
		}
		for _, f := range feeds {
			if f.FeedURL == "https://list-other.example.com/feed.xml" {
				t.Error("got feed belonging to other user")
			}
		}
	})
}

func TestUnsubscribeFromFeed(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	u := mustCreateUser(t, s, "feed-unsub@example.com")

	f := mustSubscribe(t, s, u.ID, "https://unsub.example.com/feed.xml", nil)

	if err := s.UnsubscribeFromFeed(ctx, u.ID, f.ID); err != nil {
		t.Fatalf("UnsubscribeFromFeed: %v", err)
	}

	_, err := s.GetFeed(ctx, f.ID, u.ID)
	if err != store.ErrNotFound {
		t.Errorf("after unsubscribe: GetFeed error = %v, want ErrNotFound", err)
	}
}

func TestUpdateFeed(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	u := mustCreateUser(t, s, "feed-update@example.com")

	f := mustSubscribe(t, s, u.ID, "https://update.example.com/feed.xml", []string{"old-tag"})

	t.Run("updates custom name and replaces tags", func(t *testing.T) {
		updated, err := s.UpdateFeed(ctx, f.ID, u.ID, "My Custom Name", []string{"new-tag"})
		if err != nil {
			t.Fatalf("UpdateFeed: %v", err)
		}
		if updated.CustomName != "My Custom Name" {
			t.Errorf("CustomName = %q, want %q", updated.CustomName, "My Custom Name")
		}
		if len(updated.Tags) != 1 || updated.Tags[0] != "new-tag" {
			t.Errorf("Tags = %v, want [new-tag]", updated.Tags)
		}
	})

	t.Run("clearing tags results in empty tag list", func(t *testing.T) {
		updated, err := s.UpdateFeed(ctx, f.ID, u.ID, "", nil)
		if err != nil {
			t.Fatalf("UpdateFeed: %v", err)
		}
		if len(updated.Tags) != 0 {
			t.Errorf("Tags = %v, want empty", updated.Tags)
		}
	})
}

func TestListFeedItems(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	u := mustCreateUser(t, s, "feed-items@example.com")

	f := mustSubscribe(t, s, u.ID, "https://items.example.com/feed.xml", nil)

	t.Run("empty list", func(t *testing.T) {
		items, err := s.ListFeedItems(ctx, f.ID, u.ID)
		if err != nil {
			t.Fatalf("ListFeedItems: %v", err)
		}
		if len(items) != 0 {
			t.Errorf("len = %d, want 0", len(items))
		}
	})

	t.Run("returns items with unread state", func(t *testing.T) {
		mustUpsertItem(t, s, f.ID, "guid-1", "Item One")
		mustUpsertItem(t, s, f.ID, "guid-2", "Item Two")

		items, err := s.ListFeedItems(ctx, f.ID, u.ID)
		if err != nil {
			t.Fatalf("ListFeedItems: %v", err)
		}
		if len(items) != 2 {
			t.Fatalf("len = %d, want 2", len(items))
		}
		for _, item := range items {
			if item.IsRead {
				t.Errorf("item %v: IsRead = true, want false", item.ID)
			}
		}
	})
}

func TestMarkItemRead(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	u := mustCreateUser(t, s, "feed-read@example.com")

	f := mustSubscribe(t, s, u.ID, "https://read.example.com/feed.xml", nil)
	itemID := mustUpsertItem(t, s, f.ID, "read-guid-1", "Read Test Item")

	t.Run("mark read", func(t *testing.T) {
		if err := s.MarkItemRead(ctx, u.ID, itemID); err != nil {
			t.Fatalf("MarkItemRead: %v", err)
		}

		items, err := s.ListFeedItems(ctx, f.ID, u.ID)
		if err != nil {
			t.Fatalf("ListFeedItems: %v", err)
		}
		if len(items) != 1 || !items[0].IsRead {
			t.Errorf("expected item to be read, got IsRead = %v", items[0].IsRead)
		}
	})

	t.Run("mark unread", func(t *testing.T) {
		if err := s.MarkItemUnread(ctx, u.ID, itemID); err != nil {
			t.Fatalf("MarkItemUnread: %v", err)
		}

		items, err := s.ListFeedItems(ctx, f.ID, u.ID)
		if err != nil {
			t.Fatalf("ListFeedItems: %v", err)
		}
		if len(items) != 1 || items[0].IsRead {
			t.Errorf("expected item to be unread, got IsRead = %v", items[0].IsRead)
		}
	})

	t.Run("read state is per-user", func(t *testing.T) {
		other := mustCreateUser(t, s, "feed-read-other@example.com")
		mustSubscribe(t, s, other.ID, "https://read.example.com/feed.xml", nil)

		if err := s.MarkItemRead(ctx, u.ID, itemID); err != nil {
			t.Fatalf("MarkItemRead: %v", err)
		}

		otherItems, err := s.ListFeedItems(ctx, f.ID, other.ID)
		if err != nil {
			t.Fatalf("ListFeedItems for other user: %v", err)
		}
		if len(otherItems) != 1 || otherItems[0].IsRead {
			t.Errorf("other user's item should be unread, got IsRead = %v", otherItems[0].IsRead)
		}
	})
}

func TestMarkAllItemsRead(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	u := mustCreateUser(t, s, "feed-markall@example.com")

	f := mustSubscribe(t, s, u.ID, "https://markall.example.com/feed.xml", nil)
	mustUpsertItem(t, s, f.ID, "markall-1", "Item One")
	mustUpsertItem(t, s, f.ID, "markall-2", "Item Two")

	if err := s.MarkAllItemsRead(ctx, u.ID); err != nil {
		t.Fatalf("MarkAllItemsRead: %v", err)
	}

	items, err := s.ListFeedItems(ctx, f.ID, u.ID)
	if err != nil {
		t.Fatalf("ListFeedItems: %v", err)
	}
	for _, item := range items {
		if !item.IsRead {
			t.Errorf("item %v: IsRead = false after MarkAllItemsRead", item.ID)
		}
	}
}

func TestMarkFeedItemsRead(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	u := mustCreateUser(t, s, "feed-markfeed@example.com")

	f1 := mustSubscribe(t, s, u.ID, "https://markfeed-1.example.com/feed.xml", nil)
	f2 := mustSubscribe(t, s, u.ID, "https://markfeed-2.example.com/feed.xml", nil)

	item1 := mustUpsertItem(t, s, f1.ID, "mf-1", "Feed1 Item")
	item2 := mustUpsertItem(t, s, f2.ID, "mf-2", "Feed2 Item")

	if err := s.MarkFeedItemsRead(ctx, u.ID, f1.ID); err != nil {
		t.Fatalf("MarkFeedItemsRead: %v", err)
	}

	// f1 item should be read.
	f1Items, err := s.ListFeedItems(ctx, f1.ID, u.ID)
	if err != nil {
		t.Fatalf("ListFeedItems f1: %v", err)
	}
	for _, item := range f1Items {
		if !item.IsRead {
			t.Errorf("f1 item %v: IsRead = false, want true", item.ID)
		}
	}

	// f2 item should still be unread.
	f2Items, err := s.ListFeedItems(ctx, f2.ID, u.ID)
	if err != nil {
		t.Fatalf("ListFeedItems f2: %v", err)
	}
	_ = item1
	_ = item2
	for _, item := range f2Items {
		if item.IsRead {
			t.Errorf("f2 item %v: IsRead = true, want false", item.ID)
		}
	}
}

func TestListTimelineItems(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	u := mustCreateUser(t, s, "feed-timeline@example.com")

	f := mustSubscribe(t, s, u.ID, "https://timeline.example.com/feed.xml", nil)

	t.Run("empty timeline", func(t *testing.T) {
		items, err := s.ListTimelineItems(ctx, u.ID, 50, 0)
		if err != nil {
			t.Fatalf("ListTimelineItems: %v", err)
		}
		if len(items) != 0 {
			t.Errorf("len = %d, want 0", len(items))
		}
	})

	t.Run("returns items from subscribed feeds", func(t *testing.T) {
		mustUpsertItem(t, s, f.ID, "tl-1", "Timeline Item One")
		mustUpsertItem(t, s, f.ID, "tl-2", "Timeline Item Two")
		mustUpsertItem(t, s, f.ID, "tl-3", "Timeline Item Three")

		items, err := s.ListTimelineItems(ctx, u.ID, 50, 0)
		if err != nil {
			t.Fatalf("ListTimelineItems: %v", err)
		}
		if len(items) != 3 {
			t.Errorf("len = %d, want 3", len(items))
		}
	})

	t.Run("pagination limit and offset", func(t *testing.T) {
		page1, err := s.ListTimelineItems(ctx, u.ID, 2, 0)
		if err != nil {
			t.Fatalf("ListTimelineItems page 1: %v", err)
		}
		page2, err := s.ListTimelineItems(ctx, u.ID, 2, 2)
		if err != nil {
			t.Fatalf("ListTimelineItems page 2: %v", err)
		}
		if len(page1) != 2 {
			t.Errorf("page1 len = %d, want 2", len(page1))
		}
		if len(page2) != 1 {
			t.Errorf("page2 len = %d, want 1", len(page2))
		}
		if page1[0].ID == page2[0].ID {
			t.Error("page1 and page2 returned the same item")
		}
	})

	t.Run("does not include other users feeds", func(t *testing.T) {
		other := mustCreateUser(t, s, "feed-timeline-other@example.com")
		otherFeed := mustSubscribe(t, s, other.ID, "https://timeline-other.example.com/feed.xml", nil)
		mustUpsertItem(t, s, otherFeed.ID, "tl-other-1", "Other User Item")

		items, err := s.ListTimelineItems(ctx, u.ID, 50, 0)
		if err != nil {
			t.Fatalf("ListTimelineItems: %v", err)
		}
		for _, item := range items {
			if item.FeedID == otherFeed.ID {
				t.Error("timeline contains item from other user's feed")
			}
		}
	})
}

func TestUpsertFeedItem(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	u := mustCreateUser(t, s, "feed-upsert@example.com")

	f := mustSubscribe(t, s, u.ID, "https://upsert.example.com/feed.xml", nil)

	t.Run("creates new item", func(t *testing.T) {
		id, err := s.UpsertFeedItem(ctx, store.UpsertFeedItemParams{
			FeedID: f.ID,
			GUID:   "upsert-guid-1",
			URL:    "https://upsert.example.com/1",
			Title:  "Upsert Item",
		})
		if err != nil {
			t.Fatalf("UpsertFeedItem: %v", err)
		}
		if id == uuid.Nil {
			t.Error("returned nil UUID")
		}
	})

	t.Run("upserting same guid updates in place", func(t *testing.T) {
		pub := time.Now().Add(-time.Hour)
		id1, err := s.UpsertFeedItem(ctx, store.UpsertFeedItemParams{
			FeedID:      f.ID,
			GUID:        "upsert-same-guid",
			URL:         "https://upsert.example.com/orig",
			Title:       "Original",
			PublishedAt: &pub,
		})
		if err != nil {
			t.Fatalf("UpsertFeedItem first: %v", err)
		}

		id2, err := s.UpsertFeedItem(ctx, store.UpsertFeedItemParams{
			FeedID: f.ID,
			GUID:   "upsert-same-guid",
			URL:    "https://upsert.example.com/updated",
			Title:  "Updated",
		})
		if err != nil {
			t.Fatalf("UpsertFeedItem second: %v", err)
		}
		if id1 != id2 {
			t.Errorf("IDs differ on upsert: %v != %v", id1, id2)
		}
	})
}
