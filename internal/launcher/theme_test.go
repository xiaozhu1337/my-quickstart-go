package launcher

import (
	"math"
	"testing"

	"github.com/egoist/mygo/ui"
)

func luminance(c ui.Color) float64 {
	channel := func(v uint8) float64 {
		s := float64(v) / 255
		if s <= 0.04045 {
			return s / 12.92
		}
		return math.Pow((s+0.055)/1.055, 2.4)
	}
	return 0.2126*channel(c.R) + 0.7152*channel(c.G) + 0.0722*channel(c.B)
}

func TestThemeTextContrast(t *testing.T) {
	for _, theme := range themes {
		p := palette(theme.id)
		for _, pair := range []struct {
			name             string
			text, background ui.Color
		}{
			{"body", p.Text, p.Background},
			{"card", p.Text, p.Surface},
			{"hint", p.TextMuted, p.Background},
			{"section hint", p.TextMuted, p.Surface},
			{"button", p.AccentText, p.Accent},
		} {
			a, b := luminance(pair.text), luminance(pair.background)
			ratio := (max(a, b) + 0.05) / (min(a, b) + 0.05)
			if ratio < 4.5 {
				t.Errorf("%s %s text contrast %.2f is below 4.5", theme.id, pair.name, ratio)
			}
		}
	}
}

func TestNativeThemeSettings(t *testing.T) {
	for _, size := range []struct {
		name          string
		width, height int
	}{{"normal", 960, 620}, {"small", 560, 380}} {
		t.Run(size.name, func(t *testing.T) {
			a := fixture()
			tt := ui.NewTester(a.view, size.width, size.height)
			if err := tt.Click("Settings"); err != nil {
				t.Fatal(err)
			}
			for _, theme := range themes {
				if r, ok := tt.Find(theme.en); ok && r.Y+r.H > float32(size.height-32) {
					tt.Scroll(float32(size.width-40), float32(size.height-80), 0, 180)
				}
				if err := tt.Click(theme.en); err != nil {
					t.Fatal(err)
				}
				if a.state.Settings.Theme != theme.id {
					t.Fatalf("theme %s was not applied", theme.id)
				}
			}
			if err := tt.Click("Hotkey"); err != nil {
				t.Fatal(err)
			}
			if !tt.HasText("Apply shortcut") {
				t.Fatal("hotkey controls are missing")
			}
			if err := tt.Click("Behavior"); err != nil {
				t.Fatal(err)
			}
			if err := tt.Click("Always on top"); err != nil {
				t.Fatal(err)
			}
			if a.state.Settings.AlwaysOnTop {
				t.Fatal("switch label must toggle the setting")
			}
			tt.Key(0, ui.KeySpace)
			if !a.state.Settings.AlwaysOnTop {
				t.Fatal("focused switch must support the keyboard")
			}
			if err := tt.Click("Close"); err != nil {
				t.Fatal(err)
			}
			if a.settingsOpen || !tt.HasText("Alpha App") {
				t.Fatal("return must restore the launcher")
			}
			if a.state.Settings.Theme != "rose" {
				t.Fatal("theme must survive leaving settings")
			}
		})
	}
}
