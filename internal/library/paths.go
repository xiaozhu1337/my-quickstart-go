package library

import (
	"os"
	"path/filepath"
	"strings"
)

// absoluteUserPath expands a leading home-directory shorthand before resolving the path.
func absoluteUserPath(path string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "~" || strings.HasPrefix(path, "~/") || strings.HasPrefix(path, `~\`) {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		suffix := strings.ReplaceAll(path[1:], `\`, "/")
		path = filepath.Join(home, filepath.FromSlash(suffix))
	}
	return filepath.Abs(path)
}
