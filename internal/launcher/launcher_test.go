package launcher

import (
	"reflect"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
	"my-quickstart-go/internal/model"
)

func fixture() *App {
	s := model.Default()
	s.Settings.Language = "en"
	s.Groups[0].Apps = []model.Entry{model.NewEntry("Alpha App", "alpha.exe", "--one"), model.NewEntry("Another App", "another.exe", "")}
	s.Groups[1].Apps = []model.Entry{model.NewEntry("Beta Tool", "beta.exe", "")}
	return New(s, nil)
}

func TestEntryOperations(t *testing.T) {
	a := fixture()
	first := a.state.Groups[0].ID
	second := a.state.Groups[1].ID
	a.query = " BETA "
	a.filter()
	if len(a.results) != 1 || a.results[0].Name != "Beta Tool" {
		t.Fatal("search must include other groups and trim query")
	}
	e := a.results[0]
	a.openEditor(e)
	a.draft.Name = " Renamed "
	a.envText = "NO_EQUALS\nKEY=a=b\n   \nKEY="
	a.saveEditor()
	g, edited := a.state.Entry(e.ID)
	if g.ID != second || edited.Name != "Renamed" || !reflect.DeepEqual(edited.EnvVars, []string{"NO_EQUALS", "KEY=a=b", "KEY="}) {
		t.Fatalf("editing search result failed: %+v", edited)
	}
	a.query = ""
	a.moveEntry(e.ID, first, a.state.Groups[0].Apps[0].ID)
	if a.state.Groups[0].Apps[0].ID != e.ID || len(a.state.Groups[1].Apps) != 0 {
		t.Fatal("cross-group move failed")
	}
	a.moveEntry(e.ID, first, "")
	if a.state.Groups[0].Apps[2].ID != e.ID {
		t.Fatal("same-group move to end failed")
	}
	if n := a.addEntries(first, "", []model.Entry{a.state.Groups[0].Apps[0], a.state.Groups[0].Apps[0]}); n != 0 {
		t.Fatal("file duplicates must be rejected")
	}
	a.chooseGroup(second)
	a.removeGroup(second)
	if a.state.ActiveGroupID != first {
		t.Fatal("removing active group must choose the first group")
	}
	for len(a.state.Groups) > 1 {
		a.removeGroup(a.state.Groups[1].ID)
	}
	a.removeGroup(first)
	if len(a.state.Groups) != 1 {
		t.Fatal("last group must remain")
	}
}

func TestCommandEditor(t *testing.T) {
	a := fixture()
	a.newCommand()
	a.draft.Path = "  " + strings.Repeat("测", 41) + "  "
	a.saveEditor()
	e := a.state.Groups[0].Apps[0]
	if e.Name != strings.Repeat("测", 40)+"..." || e.Path != "command:"+strings.Repeat("测", 41) {
		t.Fatalf("command default name or trim failed: %+v", e)
	}
	original := e
	a.openEditor(e)
	a.draft.Path = "echo changed"
	a.draft.Args = "ignored"
	a.envText = "KEY=ignored"
	a.saveEditor()
	e = a.state.Groups[0].Apps[0]
	if e.ID != original.ID || e.AddedAt != original.AddedAt || e.Args != "" || len(e.EnvVars) != 0 {
		t.Fatal("command editing must preserve identity and clear app arguments")
	}
}

func TestNativeViewSearchGroupsAndEditor(t *testing.T) {
	a := fixture()
	tt := ui.NewTester(a.view, 960, 620)
	if !tt.HasText("Alpha App") {
		t.Fatalf("initial grid is missing: %q", tt.Texts())
	}
	if _, visible := tt.Find("Search"); visible || tt.HasText("Add") {
		t.Fatal("idle view must hide the search and top add button")
	}
	tt.Key(ui.Cmd, ui.KeyF)
	if !tt.Focused("Search") {
		t.Fatal("the shortcut must show and focus search")
	}
	tt.Type("beta")
	if len(a.results) != 1 || a.results[0].Name != "Beta Tool" {
		t.Fatalf("search input did not update results: %q", tt.Texts())
	}
	tt.Key(0, ui.KeyEscape)
	if a.query != "" || a.searchOpen {
		t.Fatal("Escape must clear and hide search")
	}
	if _, visible := tt.Find("Search"); visible {
		t.Fatal("the search field must be removed after Escape")
	}
	if err := tt.Click("New group"); err != nil {
		t.Fatal(err)
	}
	if len(a.state.Groups) != 4 {
		t.Fatal("new group button did not create a group")
	}
	if err := tt.RightClick("Group-1"); err != nil {
		t.Fatal(err)
	}
	if err := tt.ChooseMenuItem("Rename"); err != nil {
		t.Fatal(err)
	}
	if !a.renameOpen {
		t.Fatal("rename menu did not open the dialog")
	}
	tt.Key(ui.Cmd, ui.KeyA)
	tt.Type("Renamed group")
	if err := tt.Click("Save"); err != nil {
		t.Fatal(err)
	}
	if a.state.Groups[0].Name != "Renamed group" {
		t.Fatal("rename dialog did not save")
	}
	a.openEditor(a.state.Groups[0].Apps[0])
	tt.Frame()
	if err := tt.Click("Name"); err != nil {
		t.Fatal(err)
	}
	tt.Key(ui.Cmd, ui.KeyA)
	tt.Type("Edited app")
	if err := tt.Click("Save"); err != nil {
		t.Fatal(err)
	}
	if a.state.Groups[0].Apps[0].Name != "Edited app" {
		t.Fatal("entry dialog did not save")
	}
	if err := tt.Click("Settings"); err != nil {
		t.Fatal(err)
	}
	if err := tt.Click("Behavior"); err != nil {
		t.Fatal(err)
	}
	if err := tt.Click("Hide window when the application starts"); err != nil {
		t.Fatal(err)
	}
	if !a.state.Settings.HideOnStartup {
		t.Fatal("behavior checkbox did not save")
	}
}

func TestContextualSearchAndMessages(t *testing.T) {
	for _, width := range []int{560, 960} {
		a := fixture()
		tt := ui.NewTester(a.view, width, 380)
		if tt.HasText("Dismiss") {
			t.Fatal("idle view must not reserve a message bar")
		}
		tt.Key(ui.Cmd, ui.KeyF)
		tt.Type("beta")
		if !tt.HasText("Search results (1)") || !tt.HasText("Beta Tool") {
			t.Fatalf("global search context is missing: %q", tt.Texts())
		}
		a.message = "Hotkey unavailable"
		tt.Frame()
		if !tt.HasText(a.message) {
			t.Fatal("errors must remain visible")
		}
		if err := tt.Click("Settings"); err != nil {
			t.Fatal(err)
		}
		if !tt.HasText(a.message) {
			t.Fatal("settings must preserve the error message")
		}
		if err := tt.Click("Dismiss"); err != nil {
			t.Fatal(err)
		}
		if a.message != "" || tt.HasText("Dismiss") {
			t.Fatal("dismissing a message must remove the bar")
		}
		if err := tt.Click("Close"); err != nil {
			t.Fatal(err)
		}
		if !tt.HasText("Beta Tool") || a.query != "beta" {
			t.Fatal("returning to the launcher must preserve search")
		}
	}
}
