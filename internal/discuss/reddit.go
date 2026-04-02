package discuss

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

var redditClient = &http.Client{Timeout: 10 * time.Second}

// RedditFetcher searches Reddit for link submissions matching a URL.
type RedditFetcher struct{}

func (f *RedditFetcher) Name() string { return "reddit" }

func (f *RedditFetcher) Fetch(ctx context.Context, rawURL string) ([]Discussion, error) {
	apiURL := "https://www.reddit.com/api/info.json?url=" + url.QueryEscape(rawURL)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "booksmk/1.0 discussion-finder (personal bookmark manager)")

	resp, err := redditClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusTooManyRequests {
		return nil, fmt.Errorf("reddit rate limited")
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("reddit returned %d", resp.StatusCode)
	}

	var result struct {
		Data struct {
			Children []struct {
				Data struct {
					Title       string `json:"title"`
					Permalink   string `json:"permalink"`
					Score       int    `json:"score"`
					NumComments int    `json:"num_comments"`
				} `json:"data"`
			} `json:"children"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	discussions := make([]Discussion, 0, len(result.Data.Children))
	for _, c := range result.Data.Children {
		d := c.Data
		if d.Title == "" || d.Permalink == "" {
			continue
		}
		discussions = append(discussions, Discussion{
			Title:         d.Title,
			DiscussionURL: "https://www.reddit.com" + d.Permalink,
			Score:         d.Score,
			CommentCount:  d.NumComments,
		})
	}
	return discussions, nil
}
