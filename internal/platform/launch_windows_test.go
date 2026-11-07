//go:build windows

package platform

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"my-quickstart-go/internal/model"
)

func TestBatchConsoleChild(t *testing.T) {
	output := os.Getenv("QUICKSTART_CONSOLE_RESULT")
	if output == "" {
		return
	}
	result := "ok"
	for _, stream := range []int{syscall.STD_INPUT_HANDLE, syscall.STD_OUTPUT_HANDLE, syscall.STD_ERROR_HANDLE} {
		handle, err := syscall.GetStdHandle(stream)
		var mode uint32
		if err != nil || syscall.GetConsoleMode(handle, &mode) != nil {
			result = "invalid console handle"
		}
	}
	dir, _ := os.Getwd()
	result += "|" + dir + "|" + os.Getenv("QUICKSTART_BATCH_VALUE")
	if err := os.WriteFile(output, []byte(result), 0600); err != nil {
		os.Exit(2)
	}
	os.Exit(0)
}

func TestBatchLaunchHasConsoleAndEnvironment(t *testing.T) {
	dir := scratchDir(t)
	program, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	for _, ext := range []string{".bat", ".cmd"} {
		path := filepath.Join(dir, "script with spaces"+ext)
		output := filepath.Join(dir, "result"+ext)
		script := "@echo off\r\nset QUICKSTART_BATCH_VALUE=%~1\r\n\"" + program + "\" -test.run=^TestBatchConsoleChild$\r\n"
		if err := os.WriteFile(path, []byte(script), 0600); err != nil {
			t.Fatal(err)
		}
		if err := Launch(model.Entry{Path: path, Args: `"argument with spaces"`, EnvVars: []string{"QUICKSTART_CONSOLE_RESULT=" + output}}); err != nil {
			t.Fatal(err)
		}
		if got := strings.TrimSpace(string(waitForFile(t, output))); got != "ok|"+dir+"|argument with spaces" {
			t.Fatalf("batch result: %q", got)
		}
	}
}

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
