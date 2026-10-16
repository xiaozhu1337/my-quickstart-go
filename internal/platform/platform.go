package platform

import (
	"encoding/base64"
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"my-quickstart-go/internal/model"
)

var executable = os.Executable

// BaseDir is the executable's directory, independent of the working directory.
func BaseDir() string {
	path, err := executable()
	if err != nil {
		return "."
	}
	return filepath.Dir(path)
}

func cleanInput(path string) string {
	path = strings.TrimSpace(path)
	if len(path) >= 2 && (path[0] == '"' && path[len(path)-1] == '"' || path[0] == '\'' && path[len(path)-1] == '\'') {
		path = path[1 : len(path)-1]
	}
	return path
}

func special(path string) bool {
	lower := strings.ToLower(path)
	if strings.HasPrefix(lower, "command:") || strings.HasPrefix(lower, "uwp:") || strings.HasPrefix(lower, "shell:") {
		return true
	}
	u, err := url.Parse(path)
	return err == nil && len(u.Scheme) > 1
}

func absolutePath(path string) string {
	path = cleanInput(path)
	if special(path) {
		return path
	}
	if !filepath.IsAbs(path) {
		path = filepath.Join(BaseDir(), path)
	}
	return filepath.Clean(path)
}

// Resolve imports a path; shortcuts retain their name and icon while launching their target.
func Resolve(path string, relative bool) (model.Entry, error) {
	path = cleanInput(path)
	if path == "" || strings.ContainsRune(path, 0) {
		return model.Entry{}, errors.New("path is empty or contains NUL")
	}
	original := absolutePath(path)
	name := filepath.Base(original)
	if !special(original) {
		name = strings.TrimSuffix(name, filepath.Ext(name))
		if name == "" {
			name = filepath.Base(original)
		}
	}
	target, args := original, ""
	shortcut := !special(original) && strings.EqualFold(filepath.Ext(original), ".lnk")
	if shortcut {
		var err error
		target, args, err = resolveShortcut(original)
		if err != nil {
			target, args = original, ""
		}
		target = absolutePath(target)
	}
	stored := func(p string) string {
		if relative && !special(p) {
			if rel, err := filepath.Rel(BaseDir(), p); err == nil {
				return rel
			}
		}
		return p
	}
	entry := model.NewEntry(name, stored(target), args)
	if shortcut {
		entry.Icon = stored(original)
	}
	return entry, nil
}

func mergeEnv(base, overrides []string) ([]string, error) {
	result := append([]string(nil), base...)
	for _, line := range overrides {
		if strings.ContainsRune(line, 0) {
			return nil, errors.New("environment variable contains NUL")
		}
		key, value, ok := strings.Cut(line, "=")
		key = strings.TrimSpace(key)
		if !ok || key == "" {
			continue
		}
		// The last override wins, with case-insensitive keys on Windows.
		result = slicesWithoutKey(result, key)
		result = append(result, key+"="+value)
	}
	return result, nil
}

func slicesWithoutKey(env []string, key string) []string {
	out := env[:0]
	for _, item := range env {
		name, _, _ := strings.Cut(item, "=")
		matches := name == key || runtime.GOOS == "windows" && strings.EqualFold(name, key)
		if !matches {
			out = append(out, item)
		}
	}
	return out
}

func uwpShellPath(path string) (string, error) {
	id := strings.TrimSpace(path[len("uwp:"):])
	if id == "" || strings.ContainsRune(id, 0) {
		return "", errors.New("empty or invalid UWP app ID")
	}
	return `shell:AppsFolder\` + id, nil
}

// Icon accepts a custom image data URL, image file, file icon or UWP path.
// Shell icon lookup requires MyGo's application to be running.
func Icon(path string, size int) ([]byte, error) {
	if size <= 0 {
		return nil, errors.New("icon size must be positive")
	}
	if strings.HasPrefix(path, "data:image/") {
		header, payload, ok := strings.Cut(path, ",")
		if !ok {
			return nil, errors.New("invalid image data URL")
		}
		if strings.HasSuffix(header, ";base64") {
			return base64.StdEncoding.DecodeString(payload)
		}
		decoded, err := url.PathUnescape(payload)
		return []byte(decoded), err
	}
	path = absolutePath(path)
	if strings.HasPrefix(strings.ToLower(path), "uwp:") {
		var err error
		path, err = uwpShellPath(path)
		if err != nil {
			return nil, err
		}
	}
	switch strings.ToLower(filepath.Ext(path)) {
	case ".png", ".jpg", ".jpeg", ".gif", ".bmp", ".webp", ".ico", ".svg":
		return os.ReadFile(path)
	}
	return fileIcon(path, size)
}
