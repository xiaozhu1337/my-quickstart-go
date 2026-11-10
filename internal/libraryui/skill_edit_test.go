package libraryui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestSkillEditorUpdatesFromPathAndKeepsSelection(t *testing.T) {
	p := New(filepath.Join(t.TempDir(), "library"))
	source := t.TempDir()
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
	if !p.skillOpen || p.editOpen || p.skillID != id || len(p.ids()) != 1 {
		t.Fatal("skill editor changed selection")
	}
	click(t, tt, "Skill 目录")
	tt.Type(filepath.Join(source, "missing"))
	click(t, tt, "更新并保存")
	if !p.skillOpen || p.failure == "" {
		t.Fatal("invalid path did not leave editor open")
	}
	if err := os.WriteFile(path, []byte("---\nname: sample\ndescription: Updated\n---\nNew content\n"), 0600); err != nil {
		t.Fatal(err)
	}
	click(t, tt, "Skill 目录")
	tt.Key(ui.Cmd, ui.KeyA)
	tt.Type(source)
	click(t, tt, "更新并保存")
	if p.skillOpen || p.failure != "" || len(p.items) != 1 || p.items[0].ID != id || p.items[0].Description != "Updated" || len(p.ids()) != 1 {
		t.Fatal("update failed", p.failure)
	}
	b, err := os.ReadFile(filepath.Join(p.store.Root, p.items[0].Directory, "SKILL.md"))
	if err != nil || !strings.Contains(string(b), "New content") {
		t.Fatal("saved files not updated", err)
	}
	click(t, tt, "编辑")
	click(t, tt, "取消")
	click(t, tt, "添加 Skill")
	if p.skillID != "" || p.source != "" || !tt.HasText("导入并保存") {
		t.Fatal("add dialog retained edit state")
	}
}
