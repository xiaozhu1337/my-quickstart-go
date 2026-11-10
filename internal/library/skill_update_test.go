package library

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestUpdateSkillReplacesSavedFilesAndPreservesIdentity(t *testing.T) {
	l, project, ids := fixture(t)
	if err := l.SetPinned(ids[2], true); err != nil {
		t.Fatal(err)
	}
	before, _ := l.Items()
	old := before[2]
	dest := filepath.Join(l.Root, old.Directory)
	if _, err := l.Deploy(project, "codex", ids[2:]); err != nil {
		t.Fatal(err)
	}
	source := t.TempDir()
	document := "---\nname: upstream-name\ndescription: Updated description\n---\nNew instructions.\n"
	write(t, filepath.Join(source, "SKILL.md"), document)
	write(t, filepath.Join(source, "scripts", "tool.py"), "print('new')\n")
	if err := l.UpdateSkill(old.ID, source); err != nil {
		t.Fatal(err)
	}
	after, err := l.Items()
	if err != nil {
		t.Fatal(err)
	}
	got := after[2]
	if len(after) != len(before) || got.ID != old.ID || got.Name != old.Name || got.Directory != old.Directory || !got.Pinned || got.Description != "Updated description" {
		t.Fatalf("update changed identity or lost metadata: %+v", got)
	}
	if _, err := os.Stat(filepath.Join(dest, "references", "guide.md")); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("obsolete asset retained", err)
	}
	if read(t, filepath.Join(dest, "scripts", "tool.py")) != "print('new')\n" {
		t.Fatal("new asset missing")
	}
	if text := read(t, filepath.Join(dest, "SKILL.md")); !strings.Contains(text, "name: sample-skill") || !strings.Contains(text, "New instructions.") {
		t.Fatal(text)
	}
	if read(t, filepath.Join(source, "SKILL.md")) != document {
		t.Fatal("source modified")
	}
	projectDoc := filepath.Join(project, ".agents", "skills", old.Name, "SKILL.md")
	if strings.Contains(read(t, projectDoc), "New instructions.") {
		t.Fatal("project copy changed before redeployment")
	}
	if _, err := l.Deploy(project, "codex", ids[2:]); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(read(t, projectDoc), "New instructions.") {
		t.Fatal("redeployment failed")
	}
	if err := l.UpdateSkill(old.ID, filepath.Join(dest, "SKILL.md")); err != nil {
		t.Fatal("updating from saved copy failed", err)
	}
	assertNoDeployTemps(t, l.Root)
}

func TestUpdateSkillFailuresKeepSavedFiles(t *testing.T) {
	for _, failure := range []string{"missing", "invalid", "link", "copy", "metadata"} {
		t.Run(failure, func(t *testing.T) {
			l, _, ids := fixture(t)
			items, _ := l.Items()
			dest := filepath.Join(l.Root, items[2].Directory)
			_, digest, err := skillInventory(dest)
			if err != nil {
				t.Fatal(err)
			}
			manifest := filepath.Join(l.Root, "library.yaml")
			before := read(t, manifest)
			source := t.TempDir()
			write(t, filepath.Join(source, "SKILL.md"), "---\nname: sample-skill\ndescription: New description\n---\nNew content\n")
			switch failure {
			case "missing":
				source = filepath.Join(source, "missing")
			case "invalid":
				write(t, filepath.Join(source, "SKILL.md"), "Invalid document")
			case "link":
				if err := os.Symlink(filepath.Join(dest, "SKILL.md"), filepath.Join(source, "linked.md")); err != nil {
					t.Skip(err)
				}
			case "copy":
				l.copySkill = func(source, target string) ([]string, error) {
					if err := os.Mkdir(target, 0700); err != nil {
						return nil, err
					}
					return []string{target}, errors.New("injected copy failure")
				}
			case "metadata":
				l.copySkill = func(source, target string) ([]string, error) {
					paths, err := copySkillDirectory(source, target)
					if err != nil {
						return paths, err
					}
					if err := os.Rename(manifest, manifest+".saved"); err != nil {
						return paths, err
					}
					return paths, os.Mkdir(manifest, 0700)
				}
			}
			if err := l.UpdateSkill(ids[2], source); err == nil {
				t.Fatal("expected update failure")
			}
			if failure == "metadata" {
				if err := os.Remove(manifest); err != nil {
					t.Fatal(err)
				}
				if err := os.Rename(manifest+".saved", manifest); err != nil {
					t.Fatal(err)
				}
			}
			_, after, err := skillInventory(dest)
			if err != nil || digest != after {
				t.Fatal("saved files changed after failure", err)
			}
			if read(t, manifest) != before {
				t.Fatal("metadata changed after failure")
			}
			assertNoDeployTemps(t, l.Root)
		})
	}
}
