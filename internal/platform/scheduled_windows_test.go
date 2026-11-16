//go:build windows

package platform

import (
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"my-quickstart-go/internal/model"
)

func TestScheduledChild(t *testing.T) {
	output := os.Getenv("QUICKSTART_SCHEDULE_RESULT")
	if output == "" {
		return
	}
	window, _, _ := syscall.NewLazyDLL("kernel32.dll").NewProc("GetConsoleWindow").Call()
	result := "hidden"
	if window != 0 {
		result = "visible"
	}
	dir, _ := os.Getwd()
	result += "|" + dir + "|" + os.Getenv("QUICKSTART_SCHEDULE_ARG")
	if err := os.WriteFile(output, []byte(result), 0600); err != nil {
		os.Exit(2)
	}
	os.Exit(0)
}

func TestScheduledBatchHasNoConsole(t *testing.T) {
	dir := scratchDir(t)
	program, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	for _, ext := range []string{".bat", ".cmd"} {
		path := filepath.Join(dir, "scheduled script"+ext)
		output := filepath.Join(dir, "result"+ext)
		script := "@echo off\r\nset QUICKSTART_SCHEDULE_ARG=%~1\r\n\"" + program + "\" -test.run=^TestScheduledChild$\r\n"
		if err := os.WriteFile(path, []byte(script), 0600); err != nil {
			t.Fatal(err)
		}
		if err := LaunchScheduled(model.Entry{Path: path, Args: `"argument with spaces"`, EnvVars: []string{"QUICKSTART_SCHEDULE_RESULT=" + output}}); err != nil {
			t.Fatal(err)
		}
		if got := strings.TrimSpace(string(waitForFile(t, output))); got != "hidden|"+dir+"|argument with spaces" {
			t.Fatalf("scheduled child result: %q", got)
		}
	}
}

func TestScheduledRejectsMissingProgramsAndDirectories(t *testing.T) {
	dir := scratchDir(t)
	for _, path := range []string{dir, filepath.Join(dir, "missing.exe")} {
		if err := LaunchScheduled(model.Entry{Path: path}); err == nil {
			t.Fatal("invalid program accepted", path)
		}
	}
}
