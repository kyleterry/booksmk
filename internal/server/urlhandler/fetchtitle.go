package urlhandler

import (
	"html"
	"io"
	"net/http"
	"strings"
	"time"
)

var titleClient = &http.Client{
	Timeout: 8 * time.Second,
}

// fetchTitle fetches rawURL and returns the contents of the <title> tag, or
// an empty string if the title cannot be determined.
func fetchTitle(rawURL string) string {
	req, err := http.NewRequest(http.MethodGet, rawURL, nil)
	if err != nil {
		return ""
	}
	req.Header.Set("User-Agent", "booksmk/1.0 (title-fetch)")

	resp, err := titleClient.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return ""
	}

	// Only bother parsing HTML content types.
	ct := resp.Header.Get("Content-Type")
	if !strings.Contains(ct, "html") {
		return ""
	}

	// Read up to 64 KB — enough to find any <title> in the <head>.
	body, err := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
	if err != nil {
		return ""
	}

	return extractTitle(string(body))
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
