package launcher

import (
	"fmt"
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestCardSelectionHasNoOuterGridDecoration(t *testing.T) {
	for _, theme := range themes {
		for _, size := range []struct {
			width int
			scale float32
		}{{560, 1}, {560, 2}, {960, 1}, {960, 2}} {
			scale := size.scale
			t.Run(fmt.Sprintf("%s/%d/%.0fx", theme.id, size.width, scale), func(t *testing.T) {
				a := fixture()
				a.state.Settings.Theme = theme.id
				a.busy = true
				tt := ui.NewTester(a.view, size.width, 620)
				tt.SetScale(scale)
				check := func(name string, hovered bool) {
					t.Helper()
					r, ok := tt.Find(name)
					if !ok {
						t.Fatal("card missing: " + name)
					}
					y := int((r.Y + r.H/2) * scale)
					img := tt.Image()
					outside := img.RGBAAt(int((r.X-2)*scale), y)
					inside := img.RGBAAt(int((r.X+0.5)*scale), y)
					if inside != outside {
						t.Fatalf("extra outer selection at %q: %v, background %v", name, inside, outside)
					}
					want := palette(theme.id).Surface
					if hovered {
						want = palette(theme.id).SurfaceHover
					}
					body := img.RGBAAt(int((r.X+10)*scale), y)
					if body.R != want.R || body.G != want.G || body.B != want.B {
						t.Fatalf("card theme changed: %v, want %v", body, want)
					}
					if img.RGBAAt(int((r.X+2.5)*scale), y) == body {
						t.Fatal("card's own selection border must remain visible")
					}
				}
				check("Alpha App", false)
				r, _ := tt.Find("Alpha App")
				tt.Move(r.X+10, r.Y+10)
				check("Alpha App", true)
				other, _ := tt.Find("Another App")
				tt.Move(other.X+10, other.Y+10)
				check("Alpha App", false)
				check("Another App", true)
				tt.Move(float32(size.width-20), 580)
				check("Alpha App", false)
				tt.ClickAt(r.X+0.5, r.Y+r.H/2)
				check("Alpha App", false)
				tt.Key(0, ui.KeyRight)
				if a.selected != 1 {
					t.Fatal("grid arrow navigation must remain available")
				}
				check("Another App", false)
				tt.SetFocused(false)
				check("Another App", false)
			})
		}
	}
}
