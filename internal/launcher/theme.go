package launcher

import (
	"fmt"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

var themes = []struct{ id, zh, en string }{
	{"dark", "石墨", "Graphite"},
	{"light", "纸白", "Paper"},
	{"midnight", "午夜", "Midnight"},
	{"forest", "松林", "Forest"},
	{"rose", "玫瑰", "Rose"},
}

func palette(id string) *ui.Theme {
	t := ui.DarkTheme()
	background, surface, border := "#111720", "#1b2431", "#303e51"
	text, muted, accent, onAccent := "#edf2fa", "#a1afc3", "#a2bdff", "#142342"
	switch id {
	case "light":
		t = ui.LightTheme()
		background, surface, border = "#f2f5fa", "#ffffff", "#d5deec"
		text, muted, accent, onAccent = "#1e293b", "#5b6b82", "#315fce", "#ffffff"
	case "midnight":
		background, surface, border = "#0d1429", "#16203c", "#303f65"
		text, muted, accent, onAccent = "#e9edff", "#a5b0d2", "#b4acff", "#231b4d"
	case "forest":
		background, surface, border = "#111d1b", "#1a2c27", "#345147"
		text, muted, accent, onAccent = "#e7f3ed", "#a0bdb0", "#a5d8b5", "#153324"
	case "rose":
		t = ui.LightTheme()
		background, surface, border = "#faf2f3", "#fffafb", "#ead4dc"
		text, muted, accent, onAccent = "#392633", "#7c596a", "#a03864", "#ffffff"
	}
	t.Background, t.Surface, t.Border = ui.Hex(background), ui.Hex(surface), ui.Hex(border)
	t.Text, t.TextMuted = ui.Hex(text), ui.Hex(muted)
	t.Accent, t.AccentText = ui.Hex(accent), ui.Hex(onAccent)
	t.SurfaceHover = t.Surface.Mix(t.Accent, 0.10)
	t.SurfacePressed = t.Surface.Mix(t.Accent, 0.18)
	t.AccentHover = t.Accent.Mix(t.Text, 0.10)
	t.AccentPressed = t.Accent.Mix(t.Background, 0.15)
	t.Selection, t.Focus = t.Accent.Alpha(0.25), t.Accent.Alpha(0.7)
	t.Radius, t.Spacing = 9, 4
	return t
}

func backgroundColor(id string) string {
	c := palette(id).Background
	return fmt.Sprintf("#%02x%02x%02x", c.R, c.G, c.B)
}

func syncNativeTheme(id string) {
	source := mygo.ThemeLight
	if palette(id).Dark {
		source = mygo.ThemeDark
	}
	if mygo.Theme.Source() != source {
		mygo.Theme.SetSource(source)
	}
}
