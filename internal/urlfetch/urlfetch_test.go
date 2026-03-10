package urlfetch

import "testing"

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
