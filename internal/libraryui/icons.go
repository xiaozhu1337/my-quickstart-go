package libraryui

import (
	_ "embed"

	"github.com/egoist/mygo/ui"
)

//go:embed icons/pin.svg
var pinSVG []byte

//go:embed icons/edit.svg
var editSVG []byte

//go:embed icons/folder.svg
var folderSVG []byte

//go:embed icons/check.svg
var checkSVG []byte

var (
	pinIcon    = ui.MustParseSVG(pinSVG)
	editIcon   = ui.MustParseSVG(editSVG)
	folderIcon = ui.MustParseSVG(folderSVG)
	checkIcon  = ui.MustParseSVG(checkSVG)
)

func actionButton(c *ui.Context, label string, icon *ui.SVG, active, disabled bool) ui.Element {
	button := ui.ButtonBase(c).Label(label).Tooltip(label).Size(32, 32).Radius(6).
		AlignItems(ui.Center).Justify(ui.Center).Disabled(disabled)
	color := c.Theme().TextMuted
	if active {
		color = c.Theme().Accent
		button.Background(color.Alpha(0.12))
	}
	if button.Hovered() {
		color = c.Theme().Accent
		button.Background(color.Alpha(0.18))
	}
	button.Children(func() {
		ui.Icon(c, icon).Size(18, 18).TextColor(color)
	})
	return button
}
