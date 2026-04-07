package discuss

import (
	"context"
	"encoding/xml"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// LobstersFetcher searches Lobste.rs via its search RSS feed.
type LobstersFetcher struct {
	client *http.Client
	// slot is a size-1 channel used as a rate limit token.
	// A goroutine returns the token after 1 second, capping throughput to 1 req/s.
	slot chan struct{}
}

// NewLobstersFetcher returns a new LobstersFetcher.
func NewLobstersFetcher() *LobstersFetcher {
	slot := make(chan struct{}, 1)
	slot <- struct{}{}

	return &LobstersFetcher{
		client: &http.Client{Timeout: 10 * time.Second},
		slot:   slot,
	}
}

func (f *LobstersFetcher) Name() string { return "lobsters" }

func (f *LobstersFetcher) Fetch(ctx context.Context, rawURL string) ([]Discussion, error) {
	select {
	case <-f.slot:
		go func() {
			time.Sleep(time.Second)
			f.slot <- struct{}{}
		}()
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	feedURL := "https://lobste.rs/search.rss?order=relevance&what=stories&q=" + url.QueryEscape(rawURL)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, feedURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "booksmk/1.0 discussion-finder (personal bookmark manager)")
	req.Header.Set("Accept", "application/rss+xml, application/xml;q=0.9, text/xml;q=0.8, */*;q=0.1")

	resp, err := f.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusTooManyRequests {
		return nil, fmt.Errorf("lobsters rate limited")
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("lobsters returned %d", resp.StatusCode)
	}

	var feed struct {
		Items []struct {
			Title string `xml:"title"`
			Link  string `xml:"link"`
		} `xml:"channel>item"`
	}
	if err := xml.NewDecoder(resp.Body).Decode(&feed); err != nil {
		return nil, err
	}

	discussions := make([]Discussion, 0, len(feed.Items))
	for _, item := range feed.Items {
		if item.Title == "" || item.Link == "" {
			continue
		}
		// Only include links to lobste.rs story pages, not the external URL itself.
		if !strings.HasPrefix(item.Link, "https://lobste.rs/s/") {
			continue
		}
		discussions = append(discussions, Discussion{
			Title:         item.Title,
			DiscussionURL: item.Link,
		})
	}
	return discussions, nil
}
