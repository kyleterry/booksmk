package discuss

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// HackerNewsFetcher searches Hacker News via the Algolia API.
type HackerNewsFetcher struct {
	client *http.Client
}

// NewHackerNewsFetcher returns a new HackerNewsFetcher.
func NewHackerNewsFetcher() *HackerNewsFetcher {
	return &HackerNewsFetcher{
		client: &http.Client{Timeout: 10 * time.Second},
	}
}

func (f *HackerNewsFetcher) Name() string { return "hackernews" }

func (f *HackerNewsFetcher) Fetch(ctx context.Context, rawURL string) ([]Discussion, error) {
	u, _ := url.Parse("https://hn.algolia.com/api/v1/search")
	q := u.Query()
	q.Set("tags", "story")
	q.Set("query", rawURL)
	q.Set("restrictSearchableAttributes", "url")
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}

	resp, err := f.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("hn algolia returned %d", resp.StatusCode)
	}

	var result struct {
		Hits []struct {
			ObjectID    string `json:"objectID"`
			Title       string `json:"title"`
			URL         string `json:"url"`
			Points      int    `json:"points"`
			NumComments int    `json:"num_comments"`
		} `json:"hits"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	target := strings.TrimRight(rawURL, "/")
	discussions := make([]Discussion, 0, len(result.Hits))
	for _, h := range result.Hits {
		if h.Title == "" || h.ObjectID == "" {
			continue
		}
		if strings.TrimRight(h.URL, "/") != target {
			continue
		}
		itemURL, _ := url.Parse("https://news.ycombinator.com/item")
		iq := itemURL.Query()
		iq.Set("id", h.ObjectID)
		itemURL.RawQuery = iq.Encode()

		discussions = append(discussions, Discussion{
			Title:         h.Title,
			DiscussionURL: itemURL.String(),
			Score:         h.Points,
			CommentCount:  h.NumComments,
		})
	}
	return discussions, nil
}
