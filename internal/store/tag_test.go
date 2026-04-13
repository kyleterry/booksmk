package store

import "testing"

func TestSlug(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"Go", "go"},
		{"hello world", "hello-world"},
		{"hello--world", "hello-world"},
		{"  hello  ", "hello"},
		{"foo_bar", "foo-bar"},
		{"café", "caf"},
		{"123abc", "123abc"},
		{"---", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := Slug(tt.input); got != tt.want {
				t.Errorf("Slug(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
