package logger

import "regexp"

// plexTokenPattern matches the token value in a Plex URL or deep link.
var plexTokenPattern = regexp.MustCompile(`(?i)(X-Plex-Token=)[^&\s"]+`)

// RedactTokens strips secret values out of a string so it is safe to log.
// Deep links carry a live Plex auth token, and container logs are neither
// short-lived nor access-controlled.
func RedactTokens(s string) string {
	return plexTokenPattern.ReplaceAllString(s, "${1}REDACTED")
}
