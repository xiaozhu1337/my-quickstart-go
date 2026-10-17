package launcher

import (
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"my-quickstart-go/internal/model"
)

func dragFixture() *App {
	a := fixture()
	a.state.Groups[0].Apps = nil
	for i := 0; i < 8; i++ {
		a.state.Groups[0].Apps = append(a.state.Groups[0].Apps, model.NewEntry(fmt.Sprintf("Item %d", i), fmt.Sprintf("item%d.exe", i), ""))
	}
	a.filter()
	return a
}

func appNames(a *App) []string {
	var names []string
	for _, e := range a.state.Groups[0].Apps {
		names = append(names, e.Name)
	}
	return names
}

func TestDragPositionsPreviewAndCancel(t *testing.T) {
	for _, size := range []struct {
		width int
		scale float32
	}{{560, 1}, {560, 2}, {960, 1}, {960, 2}} {
		width := size.width
		for _, tc := range []struct {
			name     string
			from, to int
			fraction float32
		}{
			{"before", 7, 1, 0.1}, {"after", 0, 2, 0.8}, {"cell padding", 7, 1, 0.98}, {"across rows", 1, 6, 0.1},
		} {
			t.Run(fmt.Sprintf("%d/%.0fx/%s", width, size.scale, tc.name), func(t *testing.T) {
				a := dragFixture()
				tt := ui.NewTester(a.view, width, 620)
				tt.SetScale(size.scale)
				src, ok := tt.Find(fmt.Sprintf("Item %d", tc.from))
				if !ok {
					t.Fatal("source missing")
				}
				dst, ok := tt.Find(fmt.Sprintf("Item %d", tc.to))
				if !ok {
					t.Fatal("target missing")
				}
				original := appNames(a)
				x, y := dst.X+dst.W*tc.fraction, dst.Y+dst.H/2
				before := append([]byte(nil), tt.Image().Pix...)
				tt.Press(src.X+10, src.Y+src.H/2)
				tt.Move(src.X+20, src.Y+src.H/2)
				tt.Move(x, y)
				if !reflect.DeepEqual(appNames(a), original) {
					t.Fatal("preview must not modify saved order")
				}
				if reflect.DeepEqual(before, tt.Image().Pix) {
					t.Fatal("drag must render a preview")
				}
				tt.Key(0, ui.KeyEscape)
				tt.Release(x, y)
				if !reflect.DeepEqual(appNames(a), original) {
					t.Fatal("Escape must discard the preview")
				}
				tt.Press(src.X+10, src.Y+src.H/2)
				tt.Move(src.X+20, src.Y+src.H/2)
				tt.Move(x, y)
				tt.Release(x, y)
				at := tc.to
				if tc.fraction >= 0.5 {
					at++
				}
				want := append([]string(nil), original[:tc.from]...)
				want = append(want, original[tc.from+1:]...)
				if at > tc.from {
					at--
				}
				want = append(want[:at], append([]string{original[tc.from]}, want[at:]...)...)
				if !reflect.DeepEqual(appNames(a), want) {
					t.Fatalf("got %v want %v", appNames(a), want)
				}
			})
		}
	}
}

func TestDragAcrossGroupsAndSearchDisablesReordering(t *testing.T) {
	a := dragFixture()
	tt := ui.NewTester(a.view, 960, 620)
	source := a.state.Groups[0].Apps[3]
	src, _ := tt.Find(source.Name)
	dst, _ := tt.Find(a.state.Groups[1].Name)
	tt.Press(src.X+10, src.Y+20)
	tt.Move(src.X+20, src.Y+20)
	tt.Move(dst.X+20, dst.Y+20)
	tt.Release(dst.X+20, dst.Y+20)
	if len(a.state.Groups[0].Apps) != 7 || a.state.Groups[1].Apps[0].ID != source.ID {
		t.Fatal("dragging a card to another group must move it to the front")
	}
	tt.Key(ui.Cmd, ui.KeyF)
	tt.Type("item")
	src, _ = tt.Find("Item 0")
	dst, _ = tt.Find("Item 7")
	before := appNames(a)
	tt.Press(src.X+10, src.Y+20)
	tt.Move(src.X+20, src.Y+20)
	tt.Move(dst.X+10, dst.Y+20)
	tt.Release(dst.X+10, dst.Y+20)
	if !reflect.DeepEqual(appNames(a), before) {
		t.Fatal("filtered results must not reorder the underlying list")
	}
}

func TestDropIndexIncludesGapsAndWhitespace(t *testing.T) {
	slots := []dragSlot{
		{index: 0, bounds: ui.Rect{X: 20, Y: 20, W: 150, H: 96}},
		{index: 1, bounds: ui.Rect{X: 180, Y: 20, W: 150, H: 96}},
		{index: 2, bounds: ui.Rect{X: 20, Y: 124, W: 150, H: 96}},
		{index: 3, bounds: ui.Rect{X: 180, Y: 124, W: 150, H: 96}},
	}
	for _, tc := range []struct {
		x, y float32
		want int
	}{
		{21, 40, 0}, {160, 40, 1}, {175, 40, 1}, {320, 40, 2},
		{21, 119, 0}, {21, 121, 2}, {21, 140, 2}, {900, 140, 4}, {20, 400, 4},
	} {
		if got := dropIndex(slots, tc.x, tc.y, 4); got != tc.want {
			t.Errorf("drop at %v,%v = %d want %d", tc.x, tc.y, got, tc.want)
		}
	}
}

func TestDropCommitsDisplayedSlot(t *testing.T) {
	a := dragFixture()
	tt := ui.NewTester(a.view, 960, 620)
	src, _ := tt.Find("Item 7")
	dst, _ := tt.Find("Item 1")
	tt.Press(src.X+10, src.Y+20)
	tt.Move(src.X+20, src.Y+20)
	tt.Move(dst.X+10, dst.Y+20)
	if a.dragBeforeID != a.results[1].ID {
		t.Fatal("preview must identify the target entry")
	}
	// Simulate a different current pointer when the completed drop is consumed.
	tt.Release(src.X+10, src.Y+20)
	if a.state.Groups[0].Apps[1].Name != "Item 7" {
		t.Fatal("a completed drop must retain the displayed insertion slot")
	}
}

func TestDragPreviewRefreshesAfterVirtualGridScroll(t *testing.T) {
	a := dragFixture()
	for i := 8; i < 80; i++ {
		a.state.Groups[0].Apps = append(a.state.Groups[0].Apps, model.NewEntry(fmt.Sprintf("Item %d", i), fmt.Sprintf("item%d.exe", i), ""))
	}
	a.filter()
	tt := ui.NewTester(a.view, 560, 620)
	src, _ := tt.Find("Item 0")
	tt.Press(src.X+10, src.Y+20)
	tt.Move(src.X+20, src.Y+20)
	tt.Move(320, 300)
	tt.Scroll(320, 300, 0, 330)
	tt.Advance(time.Second / 15)
	before := a.dragBeforeID
	if before == "" {
		t.Fatal("scroll preview must find an actual destination")
	}
	tt.Frame()
	if a.dragBeforeID != before {
		t.Fatal("laid-out virtual rows must not require another input to correct the preview")
	}
	tt.Release(320, 300)
	_, entry := a.state.Entry(before)
	if entry == nil {
		t.Fatal("preview target was lost")
	}
	for i, e := range a.state.Groups[0].Apps {
		if e.ID == before && (i == 0 || a.state.Groups[0].Apps[i-1].Name != "Item 0") {
			t.Fatal("the scrolled drop must match the preview")
		}
	}
}

func TestDragInsertsBeforeTargetInsteadOfAppending(t *testing.T) {
	a := dragFixture()
	tt := ui.NewTester(a.view, 960, 620)
	src, _ := tt.Find("Item 7")
	dst, _ := tt.Find("Item 1")
	tt.Press(src.X+src.W/2, src.Y+src.H/2)
	tt.Move(src.X+src.W/2+8, src.Y+src.H/2)
	tt.Move(dst.X+10, dst.Y+dst.H/2)
	tt.Release(dst.X+10, dst.Y+dst.H/2)
	want := []string{"Item 0", "Item 7", "Item 1", "Item 2", "Item 3", "Item 4", "Item 5", "Item 6"}
	if !reflect.DeepEqual(appNames(a), want) {
		t.Fatalf("drop before target: got %v want %v", appNames(a), want)
	}
}
