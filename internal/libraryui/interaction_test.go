package libraryui

import (
	"errors"
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
	hoveredRow, _ := tt.Find("first")
	nextRow, _ := tt.Find("second")
	if hoveredRow != r || nextRow != second {
		t.Fatalf("hover changed row geometry: first %v -> %v, second %v -> %v", r, hoveredRow, second, nextRow)
	}
	after := tt.Image().RGBAAt(int(x), int(y))
	if before == after {
		t.Fatal("hover did not highlight the row")
	}
	tt.ClickAt(x, y)
	if len(p.ids()) != 1 {
		t.Fatal("blank row area did not select")
	}
	for _, point := range [][2]float32{{1, 1}, {x, y}, {second.X + 10, second.Y + 10}, {1, 1}} {
		tt.Move(point[0], point[1])
		firstNow, _ := tt.Find("first")
		secondNow, _ := tt.Find("second")
		if firstNow != r || secondNow != second {
			t.Fatal("selected or hovered rows changed geometry", firstNow, secondNow)
		}
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

func TestSkillOpensLibraryDirectoryWithoutChangingSelection(t *testing.T) {
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
	opened := ""
	p.openFolder = func(path string) error { opened = path; return nil }
	click(t, tt, "sample")
	click(t, tt, "打开目录")
	want := filepath.Join(p.store.Root, p.items[0].Directory)
	if opened != want || opened == source || len(p.ids()) != 1 || p.editOpen || p.failure != "" {
		t.Fatal("opening folder changed selection or opened the wrong folder", opened, p.failure)
	}
	if !tt.HasText("编辑") {
		t.Fatal("skill editor button missing")
	}
	p.openFolder = func(string) error { return errors.New("Explorer failed") }
	click(t, tt, "打开目录")
	if p.failure != "Explorer failed" || p.Busy {
		t.Fatal("opening failure was not reported", p.failure)
	}
}

func TestPinPersistsAndRestoresStableOrder(t *testing.T) {
	root := filepath.Join(t.TempDir(), "library")
	p := New(root)
	for _, name := range []string{"first", "second", "third"} {
		if err := p.store.SaveMCP("", name, "", "command: tool"); err != nil {
			t.Fatal(err)
		}
	}
	p.items, _ = p.store.Items()
	tt := ui.NewTester(p.View, 960, 580)
	click(t, tt, "second")
	p.query = "second"
	tt.Frame()
	click(t, tt, "置顶")
	p.query = ""
	tt.Frame()
	first, _ := tt.Find("first")
	second, _ := tt.Find("second")
	third, _ := tt.Find("third")
	if second.Y >= first.Y || first.Y >= third.Y || len(p.ids()) != 1 {
		t.Fatal("pin did not move row or changed selection")
	}
	p = New(root)
	tt = ui.NewTester(p.View, 960, 580)
	first, _ = tt.Find("first")
	second, _ = tt.Find("second")
	if second.Y >= first.Y {
		t.Fatal("pin was lost after restart")
	}
	click(t, tt, "取消置顶")
	first, _ = tt.Find("first")
	second, _ = tt.Find("second")
	if first.Y >= second.Y {
		t.Fatal("unpin did not restore the original order")
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
