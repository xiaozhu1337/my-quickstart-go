package launcher

import (
	"reflect"
	"runtime"
	"slices"
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestShortcutSearchUpdatesGroupCounts(t *testing.T) {
	a := fixture()
	tt := ui.NewTester(a.view, 960, 620)
	ids := []string{a.state.Groups[0].ID, a.state.Groups[1].ID, a.state.Groups[2].ID}
	counts := func(want []int) {
		t.Helper()
		got := []int{a.matches[ids[0]], a.matches[ids[1]], a.matches[ids[2]]}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("group counts = %v, want %v", got, want)
		}
	}
	counts([]int{2, 1, 0})
	tt.Key(ui.Cmd, ui.KeyF)
	tt.Type(" BETA ")
	counts([]int{0, 1, 0})
	if !slices.Contains(tt.Texts(), "0") || !slices.Contains(tt.Texts(), "1") || slices.Contains(tt.Texts(), "2") {
		t.Fatalf("sidebar must display match counts: %q", tt.Texts())
	}
	tt.Key(ui.Cmd, ui.KeyF)
	tt.Type("app")
	if a.query != "app" {
		t.Fatal("repeating the shortcut must select the existing query")
	}
	counts([]int{2, 0, 0})
	tt.Key(ui.Cmd, ui.KeyF)
	tt.Type("missing")
	counts([]int{0, 0, 0})
	tt.Key(0, ui.KeyEscape)
	counts([]int{2, 1, 0})
	if len(a.results) != 2 || a.searchOpen {
		t.Fatal("closing search must restore the current group")
	}
	tt.Key(ui.Cmd, ui.KeyF)
	tt.Type("beta")
	if a.query != "beta" || !tt.Focused("Search") {
		t.Fatal("reopening search must focus the newly created field")
	}
	a.openEditor(a.results[0])
	tt.Frame()
	tt.Key(ui.Cmd, ui.KeyF)
	if tt.Focused("Search") {
		t.Fatal("the launcher shortcut must not steal focus from an editor")
	}
	tt.Key(0, ui.KeyEscape)
	if a.editorOpen || a.query != "beta" || !a.searchOpen {
		t.Fatal("closing an editor must preserve the search behind it")
	}
}

func TestEditorWidthAndSmallWindowSave(t *testing.T) {
	for _, size := range []struct{ width, height int }{{960, 620}, {560, 380}} {
		a := fixture()
		a.openEditor(a.state.Groups[0].Apps[0])
		tt := ui.NewTester(a.view, size.width, size.height)
		panel, ok := tt.Find("Edit application")
		wantWidth := float32(min(720, size.width-40))
		if !ok || panel.W < wantWidth-1 || panel.W > float32(size.width) || panel.Y < 0 || panel.Y+panel.H > float32(size.height) {
			t.Fatalf("editor must be wide and fit the window: %+v", panel)
		}
		save, ok := tt.Find("Save")
		if !ok || save.Y+save.H > float32(size.height) {
			t.Fatalf("save button must stay visible: %+v", save)
		}
		tt.Key(ui.Cmd, ui.KeyA)
		tt.Type("Edited in wide dialog")
		if size.height < 540 {
			tt.Scroll(panel.X+panel.W/2, panel.Y+panel.H/2, 0, 260)
		}
		if err := tt.Click("Environment"); err != nil {
			t.Fatal(err)
		}
		tt.Type("KEY=value")
		if err := tt.Click("Save"); err != nil {
			t.Fatal(err)
		}
		if a.editorOpen || a.state.Groups[0].Apps[0].Name != "Edited in wide dialog" || !reflect.DeepEqual(a.state.Groups[0].Apps[0].EnvVars, []string{"KEY=value"}) {
			t.Fatal("the resized editor must preserve editing, scrolling, and save")
		}
	}
}

func TestInstalledAppMenuTitle(t *testing.T) {
	a := fixture()
	want := "Add installed application"
	if runtime.GOOS == "windows" {
		want = "Add UWP application"
	}
	if a.installedAppsTitle() != want {
		t.Fatalf("application menu title = %q, want %q", a.installedAppsTitle(), want)
	}
	tt := ui.NewTester(a.view, 960, 620)
	// The empty grid owns the add menu after removing the top button.
	tt.RightClickAt(900, 500)
	if !slices.Contains(tt.Menu(), a.installedAppsTitle()) || !slices.Contains(tt.Menu(), "Add command") {
		t.Fatalf("right-click add actions are missing: %q", tt.Menu())
	}
	if err := tt.ChooseMenuItem(a.installedAppsTitle()); err != nil {
		t.Fatal(err)
	}
	if !a.addOpen || !tt.HasText(a.installedAppsTitle()) {
		t.Fatal("the application picker must use the same title as its menu")
	}
}
