package library

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPinRoundTripAndMCPEdit(t *testing.T) {
	l, _, ids := fixture(t)
	for _, id := range []string{ids[0], ids[2]} {
		if err := l.SetPinned(id, true); err != nil {
			t.Fatal(err)
		}
	}
	if err := l.SaveMCP(ids[0], "renamed", "Updated", "command: updated"); err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(l.Root)
	if err != nil {
		t.Fatal(err)
	}
	items, err := reopened.Items()
	if err != nil || !items[0].Pinned || items[1].Pinned || !items[2].Pinned || items[0].Name != "renamed" {
		t.Fatal("pin state lost or changed other resources", err)
	}
	if err := reopened.SetPinned(ids[0], false); err != nil {
		t.Fatal(err)
	}
	items, _ = reopened.Items()
	if items[0].Pinned || !items[2].Pinned {
		t.Fatal("unpin changed other resources")
	}
	if err := reopened.SetPinned("missing", true); err == nil {
		t.Fatal("missing resource accepted")
	}
}

func TestSkillDirectoryUsesImportedCopy(t *testing.T) {
	l, _, ids := fixture(t)
	items, _ := l.Items()
	path, err := l.SkillDirectory(ids[2])
	if err != nil || path != filepath.Join(l.Root, items[2].Directory) {
		t.Fatal("wrong directory", path, err)
	}
	for _, id := range []string{ids[0], "missing"} {
		if _, err := l.SkillDirectory(id); err == nil {
			t.Fatal("invalid skill accepted")
		}
	}
	if err := os.Rename(path, path+"-moved"); err != nil {
		t.Fatal(err)
	}
	if _, err := l.SkillDirectory(ids[2]); err == nil {
		t.Fatal("missing directory accepted")
	}
}
