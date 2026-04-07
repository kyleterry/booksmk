package feedhandler

import (
	"strings"
	"time"

	"github.com/kyleterry/booksmk/internal/store"
)

// dateLabel returns the display label for a published timestamp relative to now.
//
// Rules (all computed in UTC):
//   - nil             → "older"
//   - 0 days ago      → "today"
//   - 1 day ago       → "yesterday"
//   - 2–6 days ago    → lowercase weekday ("saturday", "friday", …)
//   - 7–13 days ago   → "last week"
//   - 14–60 days ago  → "last month"
//   - 60+ days ago    → "older"
func dateLabel(t *time.Time, now time.Time) string {
	if t == nil {
		return "older"
	}
	todayUTC := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	tUTC := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
	days := int(todayUTC.Sub(tUTC).Hours() / 24)
	switch {
	case days <= 0:
		return "today"
	case days == 1:
		return "yesterday"
	case days < 7:
		return strings.ToLower(t.Weekday().String())
	case days < 14:
		return "last week"
	case days < 60:
		return "last month"
	default:
		return "older"
	}
}

func groupTimeline(items []store.TimelineItem, now time.Time) []store.TimelineGroup {
	var groups []store.TimelineGroup
	for _, item := range items {
		label := dateLabel(item.PublishedAt, now)
		if len(groups) == 0 || groups[len(groups)-1].Label != label {
			groups = append(groups, store.TimelineGroup{Label: label})
		}
		groups[len(groups)-1].Items = append(groups[len(groups)-1].Items, item)
	}
	return groups
}

func groupFeedItems(items []store.FeedItem, now time.Time) []store.FeedItemGroup {
	var groups []store.FeedItemGroup
	for _, item := range items {
		label := dateLabel(item.PublishedAt, now)
		if len(groups) == 0 || groups[len(groups)-1].Label != label {
			groups = append(groups, store.FeedItemGroup{Label: label})
		}
		groups[len(groups)-1].Items = append(groups[len(groups)-1].Items, item)
	}
	return groups
}
