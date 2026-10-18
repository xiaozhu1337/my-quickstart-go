package launcher

import (
	"fmt"
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestCardHighlightFollowsHoverAndKeyboardFocus(t *testing.T) {
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
				other, ok := tt.Find("Another App")
				if !ok {
					t.Fatal("reference card missing")
				}
				idleBorder := tt.Image().RGBAAt(int((other.X+2.5)*scale), int((other.Y+other.H/2)*scale))
				check := func(name string, hovered, focused bool) {
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
					border := img.RGBAAt(int((r.X+2.5)*scale), y)
					if (border != idleBorder) != (hovered || focused) {
						t.Fatalf("card %q highlight must follow hover or keyboard focus: border %v, idle %v, hovered %v, focused %v", name, border, idleBorder, hovered, focused)
					}
				}
				check("Alpha App", false, false)
				r, _ := tt.Find("Alpha App")
				tt.Move(r.X+10, r.Y+10)
				check("Alpha App", true, false)
				tt.Move(other.X+10, other.Y+10)
				check("Alpha App", false, false)
				check("Another App", true, false)
				tt.Move(float32(size.width-20), 580)
				check("Alpha App", false, false)
				tt.ClickAt(r.X+0.5, r.Y+r.H/2)
				check("Alpha App", false, false)
				tt.Key(0, ui.KeyRight)
				if a.selected != 1 {
					t.Fatal("grid arrow navigation must remain available")
				}
				check("Alpha App", false, false)
				check("Another App", false, false)
				tt.Key(0, ui.KeyTab)
				tt.Key(ui.Shift, ui.KeyTab)
				check("Another App", false, true)
				tt.Key(0, ui.KeyLeft)
				if a.selected != 0 {
					t.Fatal("keyboard focus must keep grid arrow navigation available")
				}
				check("Alpha App", false, true)
				check("Another App", false, false)
				tt.Key(0, ui.KeyRight)
				check("Another App", false, true)
				tt.SetFocused(false)
				check("Another App", false, false)
				tt.SetFocused(true)
				check("Another App", false, true)
				if err := tt.Click("Another App"); err != nil {
					t.Fatal(err)
				}
				tt.Move(float32(size.width-20), 580)
				check("Another App", false, false)
			})
		}
	}
}
