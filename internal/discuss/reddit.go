package discuss

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type redditToken struct {
	token  string
	expiry time.Time
}

// RedditFetcher searches Reddit for link submissions using app-only OAuth.
type RedditFetcher struct {
	client       *http.Client
	clientID     string
	clientSecret string
	baseURL      string

	mu    sync.Mutex
	token atomic.Pointer[redditToken]
}

// NewRedditFetcher returns a new RedditFetcher using the given OAuth credentials.
func NewRedditFetcher(clientID, clientSecret string) *RedditFetcher {
	return &RedditFetcher{
		client:       &http.Client{Timeout: 10 * time.Second},
		clientID:     clientID,
		clientSecret: clientSecret,
		baseURL:      "https://www.reddit.com",
	}
}

func (f *RedditFetcher) Name() string { return "reddit" }

func (f *RedditFetcher) fetchToken(ctx context.Context) (string, error) {
	if t := f.token.Load(); t != nil && time.Now().Before(t.expiry) {
		return t.token, nil
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	// Double-check after acquiring lock
	if t := f.token.Load(); t != nil && time.Now().Before(t.expiry) {
		return t.token, nil
	}

	u, err := url.Parse(f.baseURL + "/api/v1/access_token")
	if err != nil {
		return "", fmt.Errorf("parse reddit token url: %w", err)
	}

	body := strings.NewReader("grant_type=client_credentials")
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u.String(), body)
	if err != nil {
		return "", err
	}
	req.SetBasicAuth(f.clientID, f.clientSecret)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", "booksmk:discussion-finder:v1.0 (personal use)")

	resp, err := f.client.Do(req)
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

	newToken := &redditToken{
		token:  tok.AccessToken,
		expiry: time.Now().Add(time.Duration(tok.ExpiresIn)*time.Second - 30*time.Second),
	}
	f.token.Store(newToken)
	return newToken.token, nil
}

func (f *RedditFetcher) Fetch(ctx context.Context, rawURL string) ([]Discussion, error) {
	tok, err := f.fetchToken(ctx)
	if err != nil {
		return nil, fmt.Errorf("reddit auth: %w", err)
	}

	apiDomain := "https://oauth.reddit.com"
	if strings.Contains(f.baseURL, "127.0.0.1") || strings.Contains(f.baseURL, "localhost") {
		apiDomain = f.baseURL
	}

	u, err := url.Parse(apiDomain + "/api/info")
	if err != nil {
		return nil, fmt.Errorf("parse reddit api url: %w", err)
	}
	q := u.Query()
	q.Set("url", rawURL)
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+tok)
	req.Header.Set("User-Agent", "booksmk:discussion-finder:v1.0 (personal use)")

	resp, err := f.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusTooManyRequests {
		return nil, fmt.Errorf("reddit rate limited")
	}
	if resp.StatusCode == http.StatusUnauthorized {
		f.token.Store(nil)
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
		u, err := url.Parse("https://www.reddit.com" + d.Permalink)
		if err != nil {
			continue
		}
		discussions = append(discussions, Discussion{
			Title:         d.Title,
			DiscussionURL: u.String(),
			Score:         d.Score,
			CommentCount:  d.NumComments,
		})
	}
	return discussions, nil
}
