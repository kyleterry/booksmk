package feedhandler

import (
	"net/http"
	"net/url"
	"strings"
	"time"

	"go.e64ec.com/booksmk/internal/store"
)

// nowInRequestTZ returns the current time in the timezone reported by the
// browser via the "tz" cookie. Falls back to UTC if the cookie is absent or
// the timezone name is unrecognised.
func nowInRequestTZ(r *http.Request) time.Time {
	c, err := r.Cookie("tz")
	if err != nil {
		return time.Now().UTC()
	}

	name, err := url.QueryUnescape(c.Value)
	if err != nil {
		return time.Now().UTC()
	}

	loc, err := time.LoadLocation(name)
	if err != nil {
		return time.Now().UTC()
	}

	return time.Now().In(loc)
}

// dateLabel returns the display label for a published timestamp relative to now.
//
// Rules (computed in now's timezone):
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
	loc := now.Location()
	tLocal := t.In(loc)
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)
	tDay := time.Date(tLocal.Year(), tLocal.Month(), tLocal.Day(), 0, 0, 0, 0, loc)
	days := int(today.Sub(tDay).Hours() / 24)
	switch {
	case days <= 0:
		return "today"
	case days == 1:
		return "yesterday"
	case days < 7:
		return strings.ToLower(tLocal.Weekday().String())
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
