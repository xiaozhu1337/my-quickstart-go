//go:build windows

package platform

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestOpenFolderUsesExplorerInWindowsRoot(t *testing.T) {
	dir := scratchDir(t)
	windows := filepath.Join(dir, "Windows")
	t.Setenv("SystemRoot", windows)
	file := filepath.Join(dir, "应用 with spaces.exe")
	if err := os.WriteFile(file, nil, 0600); err != nil {
		t.Fatal(err)
	}
	for _, item := range []struct {
		name string
		path string
	}{
		{"directory", dir},
		{"file", file},
		{"missing target", filepath.Join(dir, "missing.exe")},
	} {
		t.Run(item.name, func(t *testing.T) {
			// A missing Explorer reports its path without opening a real window.
			err := OpenFolder(item.path)
			var pathErr *os.PathError
			if !errors.As(err, &pathErr) || !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("expected missing Explorer error, got %v", err)
			}
			if want := filepath.Join(windows, "explorer.exe"); pathErr.Path != want {
				t.Fatalf("Explorer path = %q, want %q", pathErr.Path, want)
			}
		})
	}
}
