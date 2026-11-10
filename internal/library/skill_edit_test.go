package library

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSkillEditUpdatesLinkedProjects(t *testing.T) {
	l, project, ids := fixture(t)
	for _, agent := range []string{"codex", "pi"} {
		if _, err := l.Deploy(project, agent, ids[2:]); err != nil {
			t.Fatal(err)
		}
	}
	document := "---\nname: renamed-skill\ndescription: Updated description\nallowed-tools: Read\n---\n# Instructions\nRead references/guide.md first.\n"
	if err := l.SaveSkill(ids[2], document); err != nil {
		t.Fatal(err)
	}
	items, err := l.Items()
	if err != nil || items[2].Name != "renamed-skill" || items[2].Description != "Updated description" {
		t.Fatal("metadata not updated", err)
	}
	for _, parent := range []string{".agents", ".pi"} {
		linked := filepath.Join(project, parent, "skills", "sample-skill")
		if read(t, filepath.Join(linked, "SKILL.md")) != document || read(t, filepath.Join(linked, "references", "guide.md")) != "Useful reference" {
			t.Fatal("existing link or supporting file damaged")
		}
	}
	got, err := l.SkillDocument(ids[2])
	if err != nil || got != document {
		t.Fatal("document did not round trip", err)
	}
	if !strings.Contains(read(t, filepath.Join(filepath.Dir(l.Root), "source", "SKILL.md")), "name: sample-skill") {
		t.Fatal("original import source was modified")
	}
}

func TestSkillEditRejectsInvalidDocumentsWithoutChanges(t *testing.T) {
	l, _, ids := fixture(t)
	original, _ := l.SkillDocument(ids[2])
	other := filepath.Join(t.TempDir(), "other")
	write(t, filepath.Join(other, "SKILL.md"), "---\nname: other\ndescription: Other skill\n---\n")
	if err := l.ImportSkill(other); err != nil {
		t.Fatal(err)
	}
	for _, document := range []string{
		"No frontmatter", "---\nname: sample-skill", "---\nname: sample-skill\n---\n",
		"---\nname: ../bad\ndescription: Bad\n---\n", "---\nname: OTHER\ndescription: Duplicate\n---\n",
		"---\nname: sample-skill\ndescription: Valid\n---invalid\n", strings.Repeat("x", maxSkillDocument+1),
	} {
		if err := l.SaveSkill(ids[2], document); err == nil {
			t.Fatal("invalid document accepted")
		}
		got, err := l.SkillDocument(ids[2])
		if err != nil || got != original {
			t.Fatal("invalid save changed document", err)
		}
	}
	if err := l.SaveSkill(ids[0], original); err == nil {
		t.Fatal("MCP edited as skill")
	}
	if _, err := l.SkillDocument("missing"); err == nil {
		t.Fatal("missing skill accepted")
	}
}

func TestSkillEditorRefusesRedirectedFile(t *testing.T) {
	l, _, ids := fixture(t)
	items, _ := l.Items()
	path := filepath.Join(l.Root, items[2].Directory, "SKILL.md")
	original := read(t, path)
	external := filepath.Join(t.TempDir(), "SKILL.md")
	write(t, external, original)
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(external, path); err != nil {
		t.Fatal(err)
	}
	if err := l.SaveSkill(ids[2], strings.Replace(original, "Read the reference.", "Changed.", 1)); err == nil {
		t.Fatal("redirected file accepted")
	}
	if read(t, external) != original {
		t.Fatal("external file was modified")
	}
}
