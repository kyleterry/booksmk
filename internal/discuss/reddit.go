package discuss

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

var redditClient = &http.Client{Timeout: 10 * time.Second}

// RedditFetcher searches Reddit for link submissions using app-only OAuth.
type RedditFetcher struct {
	clientID     string
	clientSecret string

	mu          sync.Mutex
	token       string
	tokenExpiry time.Time
}

func (f *RedditFetcher) Name() string { return "reddit" }

func (f *RedditFetcher) token_(ctx context.Context) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.token != "" && time.Now().Before(f.tokenExpiry) {
		return f.token, nil
	}

	body := strings.NewReader("grant_type=client_credentials")
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://www.reddit.com/api/v1/access_token", body)
	if err != nil {
		return "", err
	}
	req.SetBasicAuth(f.clientID, f.clientSecret)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", "booksmk:discussion-finder:v1.0 (personal use)")

	resp, err := redditClient.Do(req)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("reddit token request returned %d", resp.StatusCode)
	}

	var tok struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tok); err != nil {
		return "", err
	}
	if tok.AccessToken == "" {
		return "", fmt.Errorf("reddit returned empty access token")
	}

	f.token = tok.AccessToken
	f.tokenExpiry = time.Now().Add(time.Duration(tok.ExpiresIn)*time.Second - 30*time.Second)
	return f.token, nil
}

func (f *RedditFetcher) Fetch(ctx context.Context, rawURL string) ([]Discussion, error) {
	tok, err := f.token_(ctx)
	if err != nil {
		return nil, fmt.Errorf("reddit auth: %w", err)
	}

	apiURL := "https://oauth.reddit.com/api/info?url=" + url.QueryEscape(rawURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+tok)
	req.Header.Set("User-Agent", "booksmk:discussion-finder:v1.0 (personal use)")

	resp, err := redditClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusTooManyRequests {
		return nil, fmt.Errorf("reddit rate limited")
	}
	if resp.StatusCode == http.StatusUnauthorized {
		f.mu.Lock()
		f.token = ""
		f.mu.Unlock()
		return nil, fmt.Errorf("reddit unauthorized (token invalidated)")
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
