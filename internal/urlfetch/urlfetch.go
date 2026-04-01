package urlfetch

import (
	"encoding/json"
	"html"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

var defaultTagsByHost = map[string][]string{
	"youtube.com":          {"youtube", "video"},
	"youtu.be":             {"youtube", "video"},
	"vimeo.com":            {"vimeo", "video"},
	"twitch.tv":            {"twitch", "video", "streaming"},
	"github.com":           {"github", "code"},
	"gitlab.com":           {"gitlab", "code"},
	"stackoverflow.com":    {"stackoverflow", "programming"},
	"news.ycombinator.com": {"hackernews"},
	"reddit.com":           {"reddit", "social"},
	"twitter.com":          {"twitter", "social"},
	"x.com":                {"twitter", "social"},
	"medium.com":           {"medium", "article"},
	"substack.com":         {"substack", "article"},
	"wikipedia.org":        {"wikipedia"},
	"arxiv.org":            {"arxiv", "research"},
	"lobste.rs":            {"lobsters"},
}

// DefaultTags returns tags inferred from rawURL's hostname, or nil if the host
// is not recognised. Only applied when the user has not provided any tags.
func DefaultTags(rawURL string) []string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil
	}
	host := strings.TrimPrefix(u.Hostname(), "www.")
	return defaultTagsByHost[host]
}

var titleClient = &http.Client{
	Timeout: 8 * time.Second,
}

// Meta holds metadata extracted from a URL.
type Meta struct {
	Title string
	Tags  []string
}

// Fetch fetches metadata for rawURL in a single HTTP request where possible.
// For YouTube URLs the oEmbed API is used for the title; for all other URLs
// the page HTML is fetched once and both title and meta tags are extracted.
// Domain-based default tags are always merged into the result.
func Fetch(rawURL string) Meta {
	defaults := DefaultTags(rawURL)

	if title := youtubeOEmbedTitle(rawURL); title != "" {
		return Meta{Title: title, Tags: defaults}
	}

	if meta, ok := githubRepoMeta(rawURL); ok {
		meta.Tags = mergeTags(defaults, meta.Tags)
		return meta
	}

	body, title := fetchPageHTML(rawURL)
	tags := mergeTags(defaults, extractMetaTags(body))
	// For GitHub URLs, strip the trailing " · GitHub" suffix that the HTML
	// title includes so the result is cleaner.
	if title != "" && defaults != nil && defaults[0] == "github" {
		title = strings.TrimSuffix(title, " · GitHub")
	}
	return Meta{Title: title, Tags: tags}
}

// FetchTitle returns the title for rawURL. For YouTube URLs it uses the oEmbed
// API; for everything else it fetches the page and extracts the <title> tag.
func FetchTitle(rawURL string) string {
	return Fetch(rawURL).Title
}

// youtubeOEmbedTitle fetches the title via YouTube's oEmbed API, returning
// empty string for non-YouTube URLs or on any error.
func youtubeOEmbedTitle(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	switch u.Hostname() {
	case "www.youtube.com", "youtube.com", "youtu.be":
	default:
		return ""
	}

	oembedURL := "https://www.youtube.com/oembed?format=json&url=" + url.QueryEscape(rawURL)
	resp, err := titleClient.Get(oembedURL)
	if err != nil || resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return ""
	}
	defer func() { _ = resp.Body.Close() }()

	var data struct {
		Title string `json:"title"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return ""
	}
	return data.Title
}

// githubRepoMeta fetches the title and topics for a GitHub repository URL
// using the GitHub API (no auth required for public repos). Returns false for
// non-repository GitHub URLs or on any error.
func githubRepoMeta(rawURL string) (Meta, bool) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return Meta{}, false
	}
	host := strings.ToLower(u.Hostname())
	if host != "github.com" && host != "www.github.com" {
		return Meta{}, false
	}
	// Path must be /owner/repo (exactly two non-empty segments).
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return Meta{}, false
	}
	owner, repo := parts[0], parts[1]

	apiURL := "https://api.github.com/repos/" + owner + "/" + repo
	req, err := http.NewRequest(http.MethodGet, apiURL, nil)
	if err != nil {
		return Meta{}, false
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	resp, err := titleClient.Do(req)
	if err != nil || resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return Meta{}, false
	}
	defer func() { _ = resp.Body.Close() }()

	var data struct {
		FullName    string   `json:"full_name"`
		Description string   `json:"description"`
		Topics      []string `json:"topics"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return Meta{}, false
	}

	title := data.FullName
	if data.Description != "" {
		title += ": " + data.Description
	}
	return Meta{Title: title, Tags: data.Topics}, true
}

// fetchPageHTML fetches rawURL and returns the raw HTML body (up to 64 KB) and
// extracted page title. Returns empty strings on any error or non-HTML response.
func fetchPageHTML(rawURL string) (body, title string) {
	req, err := http.NewRequest(http.MethodGet, rawURL, nil)
	if err != nil {
		return "", ""
	}
	req.Header.Set("User-Agent", "booksmk/1.0 (title-fetch)")

	resp, err := titleClient.Do(req)
	if err != nil {
		return "", ""
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", ""
	}

	// Only bother parsing HTML content types.
	ct := resp.Header.Get("Content-Type")
	if !strings.Contains(ct, "html") {
		return "", ""
	}

	// Read up to 64 KB — enough to find the <title> and <head> meta for most sites.
	b, err := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
	if err != nil {
		return "", ""
	}

	src := string(b)
	return src, extractTitle(src)
}

// reMetaProperty matches <meta property="..." content="..."> (order-insensitive).
var reMetaProp = regexp.MustCompile(`(?i)<meta[^>]+property=["']([^"']+)["'][^>]+content=["']([^"']*)["'][^>]*>|<meta[^>]+content=["']([^"']*)["'][^>]+property=["']([^"']+)["'][^>]*>`)

// extractMetaTags returns tags inferred from Open Graph / article meta tags
// found in src. Recognised properties:
//   - og:type mapped to a category tag (video, article, music, book)
//   - article:tag used verbatim (one tag per occurrence)
func extractMetaTags(src string) []string {
	matches := reMetaProp.FindAllStringSubmatch(src, -1)
	var tags []string
	for _, m := range matches {
		// m[1],m[2] = property-first form; m[4],m[3] = content-first form
		prop, content := m[1], m[2]
		if prop == "" {
			prop, content = m[4], m[3]
		}
		prop = strings.ToLower(strings.TrimSpace(prop))
		content = strings.TrimSpace(content)
		switch {
		case prop == "og:type":
			switch {
			case strings.HasPrefix(content, "video"):
				tags = append(tags, "video")
			case strings.HasPrefix(content, "music"):
				tags = append(tags, "music")
			case content == "article":
				tags = append(tags, "article")
			case content == "book":
				tags = append(tags, "book")
			}
		case prop == "article:tag" && content != "":
			tags = append(tags, content)
		}
	}
	return tags
}

// mergeTags combines base and extra, deduplicating while preserving order
// (base tags come first).
func mergeTags(base, extra []string) []string {
	seen := make(map[string]bool, len(base))
	out := make([]string, 0, len(base)+len(extra))
	for _, t := range base {
		if !seen[t] {
			seen[t] = true
			out = append(out, t)
		}
	}
	for _, t := range extra {
		if !seen[t] {
			seen[t] = true
			out = append(out, t)
		}
	}
	return out
}

// extractTitle finds the first <title>...</title> in src (case-insensitive).
func extractTitle(src string) string {
	lower := strings.ToLower(src)

	start := strings.Index(lower, "<title")
	if start < 0 {
		return ""
	}
	// Skip to the closing '>' of the opening tag.
	gt := strings.Index(lower[start:], ">")
	if gt < 0 {
		return ""
	}
	contentStart := start + gt + 1

	end := strings.Index(lower[contentStart:], "</title")
	if end < 0 {
		return ""
	}

	title := strings.TrimSpace(src[contentStart : contentStart+end])
	return html.UnescapeString(title)
}
