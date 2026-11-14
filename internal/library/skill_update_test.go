package library

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestUpdateSkillChangesReferenceWithoutChangingFiles(t *testing.T) {
	l, project, ids := fixture(t)
	if err := l.SetPinned(ids[2], true); err != nil {
		t.Fatal(err)
	}
	before, _ := l.Items()
	old := before[2]
	oldPath := l.skillPath(old)
	_, oldHash, err := skillInventory(oldPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := l.Deploy(project, "codex", ids[2:]); err != nil {
		t.Fatal(err)
	}
	source := t.TempDir()
	source, _ = filepath.EvalSymlinks(source)
	document := "---\nname: upstream-name\ndescription: Updated description\n---\nNew instructions.\n"
	write(t, filepath.Join(source, "SKILL.md"), document)
	write(t, filepath.Join(source, "scripts", "tool.py"), "print('new')\n")
	_, sourceHash, err := skillInventory(source)
	if err != nil {
		t.Fatal(err)
	}
	l.copySkill = func(string, string) ([]string, error) { t.Fatal("editing copied files"); return nil, nil }
	if err := l.UpdateSkill(old.ID, source); err != nil {
		t.Fatal(err)
	}
	l.copySkill = copySkillDirectory
	reopened, err := Open(l.Root)
	if err != nil {
		t.Fatal(err)
	}
	after, err := reopened.Items()
	if err != nil {
		t.Fatal(err)
	}
	got := after[2]
	if len(after) != len(before) || got.ID != old.ID || got.Name != old.Name || !samePath(got.Directory, source) || !got.Pinned || got.Description != "Updated description" {
		t.Fatalf("reference or metadata incorrect: %+v", got)
	}
	for path, want := range map[string]string{oldPath: oldHash, source: sourceHash} {
		_, hash, err := skillInventory(path)
		if err != nil || hash != want {
			t.Fatal("editing changed source files", path, err)
		}
	}
	projectDoc := filepath.Join(project, ".codex", "skills", old.Name, "SKILL.md")
	if strings.Contains(read(t, projectDoc), "New instructions.") {
		t.Fatal("project changed before deployment")
	}
	write(t, filepath.Join(source, "scripts", "tool.py"), "print('latest')\n")
	if _, err := l.Deploy(project, "codex", ids[2:]); err != nil {
		t.Fatal(err)
	}
	if text := read(t, projectDoc); !strings.Contains(text, "New instructions.") || !strings.Contains(text, "name: sample-skill") {
		t.Fatal(text)
	}
	if read(t, filepath.Join(filepath.Dir(projectDoc), "scripts", "tool.py")) != "print('latest')\n" {
		t.Fatal("latest source not copied")
	}
	if read(t, filepath.Join(source, "SKILL.md")) != document {
		t.Fatal("deployment changed source")
	}
	if _, err := l.Deploy(project, "codex", ids[2:]); err != nil {
		t.Fatal("alias redeployment failed", err)
	}
	if err := l.UpdateSkill(old.ID, filepath.Join(source, "SKILL.md")); err != nil {
		t.Fatal(err)
	}
	assertNoDeployTemps(t, l.Root)
}

func TestUpdateSkillFailuresKeepReferencesAndFiles(t *testing.T) {
	for _, failure := range []string{"missing", "invalid", "no-document", "wrong-kind", "unknown"} {
		t.Run(failure, func(t *testing.T) {
			l, _, ids := fixture(t)
			items, _ := l.Items()
			oldPath := l.skillPath(items[2])
			_, digest, err := skillInventory(oldPath)
			if err != nil {
				t.Fatal(err)
			}
			manifest := filepath.Join(l.Root, "library.yaml")
			before := read(t, manifest)
			source := t.TempDir()
			write(t, filepath.Join(source, "SKILL.md"), "---\nname: sample-skill\ndescription: New description\n---\nNew content\n")
			id := ids[2]
			switch failure {
			case "missing":
				source = filepath.Join(source, "missing")
			case "invalid":
				write(t, filepath.Join(source, "SKILL.md"), "Invalid document")
			case "no-document":
				source = t.TempDir()
			case "wrong-kind":
				id = ids[0]
			case "unknown":
				id = "missing"
			}
			if err := l.UpdateSkill(id, source); err == nil {
				t.Fatal("expected failure")
			}
			_, after, err := skillInventory(oldPath)
			if err != nil || digest != after {
				t.Fatal("old files changed", err)
			}
			if read(t, manifest) != before {
				t.Fatal("reference changed after failure")
			}
			assertNoDeployTemps(t, l.Root)
		})
	}
}

func TestImportOnlyReferencesSourceAndRemoveKeepsFiles(t *testing.T) {
	l, _, ids := fixture(t)
	items, _ := l.Items()
	source := filepath.Join(filepath.Dir(l.Root), "source")
	if !samePath(items[2].Directory, source) {
		t.Fatal(items[2].Directory)
	}
	if _, err := os.Stat(filepath.Join(l.Root, "skills")); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("import created a copy", err)
	}
	before := read(t, filepath.Join(source, "SKILL.md"))
	if err := l.Remove(ids[2:]); err != nil {
		t.Fatal(err)
	}
	if read(t, filepath.Join(source, "SKILL.md")) != before {
		t.Fatal("removal modified source")
	}
}

func TestLegacyRelativeSkillDirectoryAndMissingSourceCanBeChanged(t *testing.T) {
	l, project, ids := fixture(t)
	items, _ := l.Items()
	source := l.skillPath(items[2])
	legacy := filepath.Join(l.Root, "skills", "legacy")
	if err := os.MkdirAll(filepath.Dir(legacy), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(source, legacy); err != nil {
		t.Fatal(err)
	}
	items[2].Directory = filepath.Join("skills", "legacy")
	if err := l.write(items); err != nil {
		t.Fatal(err)
	}
	l, err := Open(l.Root)
	if err != nil {
		t.Fatal(err)
	}
	if path, err := l.SkillDirectory(ids[2]); err != nil || !samePath(path, legacy) {
		t.Fatal(path, err)
	}
	if _, err := l.Deploy(project, "codex", ids[2:]); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(legacy, source); err != nil {
		t.Fatal(err)
	}
	if err := l.UpdateSkill(ids[2], source); err != nil {
		t.Fatal("cannot repair missing reference", err)
	}
	if path, err := l.SkillDirectory(ids[2]); err != nil || !samePath(path, source) {
		t.Fatal(path, err)
	}
}

func TestDeployRejectsOverlappingSourceAndDestination(t *testing.T) {
	for _, relation := range []string{"same", "source-parent", "destination-parent"} {
		t.Run(relation, func(t *testing.T) {
			l, project, ids := fixture(t)
			dest := filepath.Join(project, ".codex", "skills", "sample-skill")
			source := dest
			if relation == "source-parent" {
				source = project
			}
			if relation == "destination-parent" {
				source = filepath.Join(dest, "nested")
			}
			write(t, filepath.Join(source, "SKILL.md"), "---\nname: sample-skill\ndescription: Source\n---\nKeep me\n")
			if err := l.UpdateSkill(ids[2], source); err != nil {
				t.Fatal(err)
			}
			_, before, err := skillInventory(source)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := l.Deploy(project, "codex", ids); err == nil || !strings.Contains(err.Error(), "重叠") {
				t.Fatal("overlapping copy accepted", err)
			}
			_, after, err := skillInventory(source)
			if err != nil || before != after {
				t.Fatal("source changed", err)
			}
		})
	}
}
