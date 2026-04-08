package feedworker

import (
	"testing"
)

func TestResolveURL(t *testing.T) {
	tests := []struct {
		name string
		base string
		ref  string
		want string
	}{
		{
			name: "absolute ref returned unchanged",
			base: "https://example.com/feed/",
			ref:  "https://other.com/post",
			want: "https://other.com/post",
		},
		{
			name: "root-relative ref against absolute base",
			base: "https://www.pjrc.com/feed/index.xml",
			ref:  "/minitouch-2-0/",
			want: "https://www.pjrc.com/minitouch-2-0/",
		},
		{
			name: "root-relative ref against resolved site URL (pjrc pattern)",
			// pjrc feed has <link>/</link>; after resolving "/" against the feed URL
			// we get "https://www.pjrc.com/", which is the base used for items.
			base: "https://www.pjrc.com/",
			ref:  "/minitouch-2-0/",
			want: "https://www.pjrc.com/minitouch-2-0/",
		},
		{
			name: "relative ref resolved against path base",
			base: "https://example.com/blog/",
			ref:  "post/hello",
			want: "https://example.com/blog/post/hello",
		},
		{
			name: "empty ref returned as-is",
			base: "https://example.com/feed/",
			ref:  "",
			want: "",
		},
		{
			name: "empty base returns ref unchanged",
			base: "",
			ref:  "/some-post",
			want: "/some-post",
		},
		{
			name: "relative feed.Link resolved against feed URL",
			// Simulates the pjrc channel <link>/</link> resolved against its feed URL
			// before being used as the base for item links.
			base: "https://www.pjrc.com/feed/index.xml",
			ref:  "/",
			want: "https://www.pjrc.com/",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resolveURL(tt.base, tt.ref)
			if got != tt.want {
				t.Errorf("resolveURL(%q, %q) = %q; want %q", tt.base, tt.ref, got, tt.want)
			}
		})
	}
}

func TestTruncateSummary(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "strips HTML tags",
			input: "<p>Hello <b>world</b></p>",
			want:  "Hello world",
		},
		{
			name:  "trims surrounding whitespace",
			input: "  <p>  trimmed  </p>  ",
			want:  "trimmed",
		},
		{
			name:  "truncates at 500 runes",
			input: "<p>" + string(make([]rune, 600)) + "</p>",
			want:  string(make([]rune, 500)),
		},
		{
			name:  "does not truncate short input",
			input: "<p>short</p>",
			want:  "short",
		},
		{
			name:  "empty input returns empty",
			input: "",
			want:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := truncateSummary(tt.input)
			if got != tt.want {
				t.Errorf("truncateSummary(%q) = %q; want %q", tt.input, got, tt.want)
			}
		})
	}
}
