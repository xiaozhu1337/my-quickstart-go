package logging

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestRotationBoundsFilesAndPreservesLatest(t *testing.T) {
	root := t.TempDir()
	w, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	for n := 0; n < 8; n++ {
		if _, err = w.Write(bytes.Repeat([]byte("x"), maxLogSize/2)); err != nil {
			t.Fatal(err)
		}
	}
	if _, err = w.Write([]byte("latest message\n")); err != nil {
		t.Fatal(err)
	}
	if err = w.Close(); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(filepath.Join(root, "logs"))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 3 {
		t.Fatalf("expected 3 bounded log files, got %d", len(entries))
	}
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			t.Fatal(err)
		}
		if info.Size() > maxLogSize {
			t.Fatal("log exceeded limit")
		}
	}
	latest, err := os.ReadFile(filepath.Join(root, "logs", "launcher.log"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(latest, []byte("latest message")) {
		t.Fatal("latest message lost")
	}
}
