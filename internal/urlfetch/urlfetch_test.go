package urlfetch

import (
	"net/url"
	"reflect"
	"strings"
	"testing"
)

func TestGitHubRepoMetaURLParsing(t *testing.T) {
	tests := []struct {
		url     string
		wantHit bool
	}{
		{"https://github.com/golang/go", true},
		{"https://www.github.com/golang/go", true},
		{"https://github.com/golang", false},
		{"https://github.com", false},
		{"https://github.com/golang/go/issues", false},
		{"https://example.com/golang/go", false},
	}
	for _, tt := range tests {
		t.Run(tt.url, func(t *testing.T) {
			u, _ := url.Parse(tt.url)
			host := strings.ToLower(u.Hostname())
			isGitHub := host == "github.com" || host == "www.github.com"
			parts := strings.Split(strings.Trim(u.Path, "/"), "/")
			isRepo := isGitHub && len(parts) == 2 && parts[0] != "" && parts[1] != ""
			if isRepo != tt.wantHit {
				t.Errorf("url %q: isRepo=%v, want %v", tt.url, isRepo, tt.wantHit)
			}
		})
	}
}

func TestExtractMetaTags(t *testing.T) {
	tests := []struct {
		name string
		src  string
		want []string
	}{
		{
			name: "og:type video",
			src:  `<meta property="og:type" content="video.other">`,
			want: []string{"video"},
		},
		{
			name: "og:type article",
			src:  `<meta property="og:type" content="article">`,
			want: []string{"article"},
		},
		{
			name: "og:type music",
			src:  `<meta property="og:type" content="music.song">`,
			want: []string{"music"},
		},
		{
			name: "og:type book",
			src:  `<meta property="og:type" content="book">`,
			want: []string{"book"},
		},
		{
			name: "og:type website ignored",
			src:  `<meta property="og:type" content="website">`,
			want: nil,
		},
		{
			name: "article:tag",
			src:  `<meta property="article:tag" content="golang"><meta property="article:tag" content="programming">`,
			want: []string{"golang", "programming"},
		},
		{
			name: "content-first attribute order",
			src:  `<meta content="article" property="og:type">`,
			want: []string{"article"},
		},
		{
			name: "no meta tags",
			src:  `<html><head><title>Hello</title></head></html>`,
			want: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractMetaTags(tt.src)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("extractMetaTags() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMergeTags(t *testing.T) {
	tests := []struct {
		name  string
		base  []string
		extra []string
		want  []string
	}{
		{
			name:  "no duplicates",
			base:  []string{"youtube", "video"},
			extra: []string{"music"},
			want:  []string{"youtube", "video", "music"},
		},
		{
			name:  "deduplicates",
			base:  []string{"youtube", "video"},
			extra: []string{"video", "music"},
			want:  []string{"youtube", "video", "music"},
		},
		{
			name:  "empty extra",
			base:  []string{"github", "code"},
			extra: nil,
			want:  []string{"github", "code"},
		},
		{
			name:  "empty base",
			base:  nil,
			extra: []string{"article"},
			want:  []string{"article"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mergeTags(tt.base, tt.extra)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("mergeTags() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDefaultTags(t *testing.T) {
	tests := []struct {
		input string
		want  []string
	}{
		{"https://www.youtube.com/watch?v=abc", []string{"youtube", "video"}},
		{"https://youtu.be/abc", []string{"youtube", "video"}},
		{"https://github.com/user/repo", []string{"github", "code"}},
		{"https://news.ycombinator.com/item?id=1", []string{"hackernews"}},
		{"https://www.reddit.com/r/golang", []string{"reddit", "social"}},
		{"https://example.com", nil},
		{"not a url", nil},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := DefaultTags(tt.input)
			if len(got) != len(tt.want) {
				t.Fatalf("DefaultTags(%q) = %v, want %v", tt.input, got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("DefaultTags(%q)[%d] = %q, want %q", tt.input, i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestYoutubeOEmbedTitle(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantHit  bool
	}{
		{"standard watch URL", "https://www.youtube.com/watch?v=dQw4w9WgXcQ", true},
		{"youtu.be URL", "https://youtu.be/dQw4w9WgXcQ", true},
		{"non-youtube URL", "https://example.com/video", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := youtubeOEmbedTitle(tt.input)
			if tt.wantHit && got == "" {
				t.Errorf("expected a title, got empty string")
			}
			if !tt.wantHit && got != "" {
				t.Errorf("expected empty string, got %q", got)
			}
			if got != "" && !strings.Contains(got, "Rick Astley") {
				t.Errorf("title %q doesn't look right", got)
			}
		})
	}
}

func TestExtractTitle(t *testing.T) {
	tests := []struct {
		name string
		src  string
		want string
	}{
		{
			name: "basic",
			src:  `<html><head><title>Hello World</title></head></html>`,
			want: "Hello World",
		},
		{
			name: "case insensitive tag",
			src:  `<TITLE>Hello</TITLE>`,
			want: "Hello",
		},
		{
			name: "title tag with attributes",
			src:  `<title lang="en">My Page</title>`,
			want: "My Page",
		},
		{
			name: "html entities unescaped",
			src:  `<title>Hello &amp; World &lt;3&gt;</title>`,
			want: "Hello & World <3>",
		},
		{
			name: "whitespace trimmed",
			src:  "<title>\n  Spaced Out\n</title>",
			want: "Spaced Out",
		},
		{
			name: "no title tag",
			src:  `<html><body>No title here</body></html>`,
			want: "",
		},
		{
			name: "empty title",
			src:  `<title></title>`,
			want: "",
		},
		{
			name: "empty input",
			src:  ``,
			want: "",
		},
		{
			name: "first title wins",
			src:  `<title>First</title><title>Second</title>`,
			want: "First",
		},
		{
			name: "title in body ignored when head title present",
			src:  `<head><title>Head Title</title></head><body><title>Body Title</title></body>`,
			want: "Head Title",
		},
		{
			name: "unclosed title tag",
			src:  `<title>No closing tag`,
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractTitle(tt.src)
			if got != tt.want {
				t.Errorf("extractTitle() = %q, want %q", got, tt.want)
			}
		})
	}
}
