package logger

import "testing"

func TestRedactTokens(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "deep link with trailing token",
			in:   "plex://play/?metadataKey=/library/metadata/1179&server=abc&X-Plex-Token=TEA4zXjCZvog-5P3qrZw",
			want: "plex://play/?metadataKey=/library/metadata/1179&server=abc&X-Plex-Token=REDACTED",
		},
		{
			name: "token followed by another parameter",
			in:   "plex://play/?X-Plex-Token=secret123&shuffle=1",
			want: "plex://play/?X-Plex-Token=REDACTED&shuffle=1",
		},
		{
			name: "case insensitive",
			in:   "http://host/path?x-plex-token=secret123",
			want: "http://host/path?x-plex-token=REDACTED",
		},
		{
			name: "url without a token is unchanged",
			in:   "plex://play/?metadataKey=/playlists/42&server=abc&shuffle=1",
			want: "plex://play/?metadataKey=/playlists/42&server=abc&shuffle=1",
		},
		{
			name: "token embedded in a json body",
			in:   `{"media_content_id":"plex://play/?server=abc&X-Plex-Token=secret123"}`,
			want: `{"media_content_id":"plex://play/?server=abc&X-Plex-Token=REDACTED"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := RedactTokens(tt.in); got != tt.want {
				t.Errorf("RedactTokens() = %q, want %q", got, tt.want)
			}
		})
	}
}
