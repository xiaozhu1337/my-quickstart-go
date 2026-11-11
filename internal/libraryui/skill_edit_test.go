package libraryui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestSkillEditorChangesDirectoryAndKeepsSelection(t *testing.T) {
	p := New(filepath.Join(t.TempDir(), "library"))
	source := t.TempDir()
	source, _ = filepath.EvalSymlinks(source)
	path := filepath.Join(source, "SKILL.md")
	if err := os.WriteFile(path, []byte("---\nname: sample\ndescription: Original\n---\nOld content\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := p.store.ImportSkill(source); err != nil {
		t.Fatal(err)
	}
	p.items, _ = p.store.Items()
	id := p.items[0].ID
	tt := ui.NewTester(p.View, 960, 580)
	click(t, tt, "sample")
	click(t, tt, "编辑")
	if !p.skillOpen || p.editOpen || p.skillID != id || p.source != source || len(p.ids()) != 1 {
		t.Fatal("skill editor changed selection")
	}
	click(t, tt, "Skill 目录")
	tt.Key(ui.Cmd, ui.KeyA)
	tt.Type(filepath.Join(source, "missing"))
	click(t, tt, "更新并保存")
	if !p.skillOpen || p.failure == "" {
		t.Fatal("invalid path did not leave editor open")
	}
	replacement := t.TempDir()
	replacement, _ = filepath.EvalSymlinks(replacement)
	newPath := filepath.Join(replacement, "SKILL.md")
	if err := os.WriteFile(newPath, []byte("---\nname: sample\ndescription: Updated\n---\nNew content\n"), 0600); err != nil {
		t.Fatal(err)
	}
	click(t, tt, "Skill 目录")
	tt.Key(ui.Cmd, ui.KeyA)
	tt.Type(replacement)
	click(t, tt, "更新并保存")
	if p.skillOpen || p.failure != "" || len(p.items) != 1 || p.items[0].ID != id || p.items[0].Description != "Updated" || len(p.ids()) != 1 {
		t.Fatal("update failed", p.failure)
	}
	b, err := os.ReadFile(filepath.Join(p.items[0].Directory, "SKILL.md"))
	if err != nil || !strings.Contains(string(b), "New content") {
		t.Fatal("saved files not updated", err)
	}
	old, err := os.ReadFile(path)
	if err != nil || !strings.Contains(string(old), "Old content") {
		t.Fatal("old directory changed", err)
	}
	if p.items[0].Directory != replacement {
		t.Fatal("directory reference not updated")
	}
	click(t, tt, "编辑")
	if p.source != replacement {
		t.Fatal("editor did not prefill current directory")
	}
	click(t, tt, "取消")
	click(t, tt, "添加 Skill")
	if p.skillID != "" || p.source != "" || !tt.HasText("导入并保存") {
		t.Fatal("add dialog retained edit state")
	}
}
