package launcher

import (
	"testing"

	"github.com/egoist/mygo/ui"
	"my-quickstart-go/internal/model"
)

func TestSearchSelectsFirstMatchingTabAndFiltersThatGroup(t *testing.T) {
	a := fixture()
	a.state.Groups[1].Apps = append(a.state.Groups[1].Apps, model.NewEntry("Beta App", "beta-app.exe", ""))
	a.filter()
	original := a.state.ActiveGroupID
	first, second, empty := a.state.Groups[0].ID, a.state.Groups[1].ID, a.state.Groups[2].ID
	a.chooseGroup(second)
	tt := ui.NewTester(a.view, 960, 620)
	tt.Key(ui.Cmd, ui.KeyF)
	tt.Type("app")
	if a.state.ActiveGroupID != first || len(a.results) != 2 || tt.HasText("Beta App") {
		t.Fatalf("new queries must select the first matching tab and show its entries: %q", tt.Texts())
	}
	if err := tt.Click(a.state.Groups[1].Name); err != nil {
		t.Fatal(err)
	}
	if a.state.ActiveGroupID != second || len(a.results) != 1 || a.results[0].Name != "Beta App" || tt.HasText("Alpha App") {
		t.Fatal("choosing a matching tab must display only its matches")
	}
	a.chooseGroup(empty)
	tt.Frame()
	if a.state.ActiveGroupID != second {
		t.Fatal("empty result tabs must reject selection")
	}
	tt.Key(ui.Cmd, ui.KeyF)
	tt.Type("alpha")
	if a.state.ActiveGroupID != first || a.matches[second] != 0 {
		t.Fatal("query changes must select the first matching group")
	}
	if err := tt.Click(a.state.Groups[1].Name); err != nil {
		t.Fatal(err)
	}
	if a.state.ActiveGroupID != first {
		t.Fatal("disabled zero-match tabs must not respond to clicks")
	}
	tt.Key(ui.Cmd, ui.KeyF)
	tt.Type("no match")
	if len(a.results) != 0 {
		t.Fatal("an unmatched query must display an empty grid")
	}
	for _, g := range a.state.Groups {
		if err := tt.Click(g.Name); err != nil {
			t.Fatal(err)
		}
		if a.state.ActiveGroupID != first {
			t.Fatal("all zero-match tabs must stay disabled")
		}
	}
	tt.Key(0, ui.KeyEscape)
	if a.state.ActiveGroupID != second || len(a.results) != 2 {
		t.Fatal("closing search must restore the group selected before searching")
	}
	a.chooseGroup(empty)
	if a.state.ActiveGroupID != empty || len(a.results) != 0 {
		t.Fatal("empty groups become selectable after search closes")
	}
	a.chooseGroup(original)
	if a.state.ActiveGroupID != original {
		t.Fatal("normal group selection must still work")
	}
}
