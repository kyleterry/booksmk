package urls

import "testing"

func TestYoutubeEmbedURL(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "standard watch URL",
			input: "https://www.youtube.com/watch?v=dQw4w9WgXcQ",
			want:  "https://www.youtube.com/embed/dQw4w9WgXcQ",
		},
		{
			name:  "short youtu.be URL",
			input: "https://youtu.be/dQw4w9WgXcQ",
			want:  "https://www.youtube.com/embed/dQw4w9WgXcQ",
		},
		{
			name:  "shorts URL",
			input: "https://www.youtube.com/shorts/dQw4w9WgXcQ",
			want:  "https://www.youtube.com/embed/dQw4w9WgXcQ",
		},
		{
			name:  "youtube.com without www",
			input: "https://youtube.com/watch?v=dQw4w9WgXcQ",
			want:  "https://www.youtube.com/embed/dQw4w9WgXcQ",
		},
		{
			name:  "non-youtube URL returns empty",
			input: "https://example.com/watch?v=dQw4w9WgXcQ",
			want:  "",
		},
		{
			name:  "youtube channel URL returns empty",
			input: "https://www.youtube.com/@someuser",
			want:  "",
		},
		{
			name:  "youtube playlist URL returns empty",
			input: "https://www.youtube.com/playlist?list=PLxxx",
			want:  "",
		},
		{
			name:  "empty string returns empty",
			input: "",
			want:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := youtubeEmbedURL(tt.input)
			if got != tt.want {
				t.Errorf("youtubeEmbedURL(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
