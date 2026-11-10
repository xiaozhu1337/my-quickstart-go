package libraryui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
	"my-quickstart-go/internal/library"
)

func TestRowsHighlightToggleAndKeepEditSeparate(t *testing.T) {
	p := New(filepath.Join(t.TempDir(), "library"))
	for _, name := range []string{"first", "second"} {
		if err := p.store.SaveMCP("", name, "Description "+name, "command: tool"); err != nil {
			t.Fatal(err)
		}
	}
	p.items, _ = p.store.Items()
	tt := ui.NewTester(p.View, 960, 580)
	r, ok := tt.Find("first")
	if !ok {
		t.Fatal("row missing")
	}
	second, _ := tt.Find("second")
	if second.Y-(r.Y+r.H) < 6 {
		t.Fatal("rows need visible separation", r, second)
	}
	x, y := r.X+r.W/2, r.Y+4
	before := tt.Image().RGBAAt(int(x), int(y))
	tt.Move(x, y)
	after := tt.Image().RGBAAt(int(x), int(y))
	if before == after {
		t.Fatal("hover did not highlight the row")
	}
	tt.ClickAt(x, y)
	if len(p.ids()) != 1 {
		t.Fatal("blank row area did not select")
	}
	click(t, tt, "Description second")
	if len(p.ids()) != 2 {
		t.Fatal("description did not select independently")
	}
	click(t, tt, "编辑")
	if !p.editOpen || len(p.ids()) != 2 {
		t.Fatal("edit changed row selection")
	}
	click(t, tt, "取消")
	click(t, tt, "first")
	if len(p.ids()) != 1 || !p.selected[p.items[1].ID] {
		t.Fatal("row toggle changed other selections")
	}
	p.Busy = true
	tt.Frame()
	click(t, tt, "first")
	if len(p.ids()) != 1 {
		t.Fatal("busy row changed selection")
	}
}

func TestSkillEditorSaveAndCancel(t *testing.T) {
	p := New(filepath.Join(t.TempDir(), "library"))
	source := t.TempDir()
	document := "---\nname: sample\ndescription: Before editing\n---\nOriginal instructions.\n"
	if err := os.WriteFile(filepath.Join(source, "SKILL.md"), []byte(document), 0600); err != nil {
		t.Fatal(err)
	}
	if err := p.store.ImportSkill(source); err != nil {
		t.Fatal(err)
	}
	p.items, _ = p.store.Items()
	tt := ui.NewTester(p.View, 960, 580)
	click(t, tt, "编辑")
	if !p.skillEditOpen || p.skillDocument != document || len(p.ids()) != 0 {
		t.Fatal("skill edit did not open independently")
	}
	click(t, tt, "SKILL.md")
	tt.Key(ui.Cmd, ui.KeyA)
	tt.Type("invalid")
	click(t, tt, "保存")
	if !p.skillEditOpen || p.failure == "" {
		t.Fatal("invalid document should keep editor open")
	}
	click(t, tt, "取消")
	click(t, tt, "编辑")
	if p.skillDocument != document {
		t.Fatal("cancel changed saved document")
	}
	click(t, tt, "SKILL.md")
	tt.Key(ui.Cmd, ui.KeyA)
	updated := strings.ReplaceAll(document, "Before editing", "After editing")
	tt.Type(updated)
	click(t, tt, "保存")
	if p.skillEditOpen || p.failure != "" || p.items[0].Description != "After editing" {
		t.Fatal("skill save failed", p.failure)
	}
}

func TestMCPExamplesAreValidAndRemainVisibleOnEdit(t *testing.T) {
	if _, err := library.ParseServer(mcpExample); err != nil {
		t.Fatal(err)
	}
	p := New(filepath.Join(t.TempDir(), "library"))
	p.edit(library.Item{MCP: &library.Server{URL: "https://example.com/mcp"}})
	if _, err := library.ParseServer(p.config); err != nil {
		t.Fatal(err)
	}
	for _, example := range []string{"# env:", "#   PORT: '3000'", "# cwd: 'C:/workspace/my-project'", "# headers:", "#   Authorization:"} {
		if !strings.Contains(p.config, example) || !strings.Contains(mcpExample, example) {
			t.Fatal("missing commented example", example)
		}
	}
}
