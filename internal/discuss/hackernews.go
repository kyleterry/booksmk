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

var hnClient = &http.Client{Timeout: 10 * time.Second}

// HackerNewsFetcher searches Hacker News via the Algolia API.
type HackerNewsFetcher struct{}

func (f *HackerNewsFetcher) Name() string { return "hackernews" }

func (f *HackerNewsFetcher) Fetch(ctx context.Context, rawURL string) ([]Discussion, error) {
	params := url.Values{}
	params.Set("tags", "story")
	params.Set("query", rawURL)
	params.Set("restrictSearchableAttributes", "url")
	apiURL := "https://hn.algolia.com/api/v1/search?" + params.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, err
	}

	resp, err := hnClient.Do(req)
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
		discussions = append(discussions, Discussion{
			Title:         h.Title,
			DiscussionURL: "https://news.ycombinator.com/item?id=" + h.ObjectID,
			Score:         h.Points,
			CommentCount:  h.NumComments,
		})
	}
	return discussions, nil
}
