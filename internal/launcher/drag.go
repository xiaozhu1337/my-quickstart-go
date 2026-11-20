package launcher

import (
	"math"
	"time"

	"github.com/egoist/mygo/ui"
)

type dragSlot struct {
	index  int
	bounds ui.Rect
	card   ui.Element
}

// Drop targets stay in their original slots while the cards move visually.
func (a *App) previewReorder(c *ui.Context, grid ui.Element, slots []dragSlot) {
	over, hovering := ui.DragOver[entryDrag](grid)
	dropped, dropping := ui.Drop[entryDrag](grid)
	if dropping {
		over = dropped
	} else if !hovering {
		a.dragPreviewID, a.dragBeforeID = "", ""
		for _, slot := range slots {
			if slot.card.Dragging() {
				slot.card.Opacity(0.4)
			}
		}
		return
	}
	from := -1
	for i, e := range a.results {
		if e.ID == over.ID {
			from = i
			break
		}
	}
	if from < 0 {
		return
	}
	x, y, _ := grid.PointerPosition()
	bounds := grid.Bounds()
	to := dropIndex(slots, x+bounds.X, y+bounds.Y, len(a.results))
	before := ""
	if to < len(a.results) {
		before = a.results[to].ID
	}
	if dropping {
		// shortcut: typed Drop hides release coordinates; commit the last preview until MyGo exposes them.
		if a.dragPreviewID == over.ID {
			before = a.dragBeforeID
		}
		a.dragPreviewID, a.dragBeforeID = "", ""
		group := a.state.ActiveGroupID
		a.deferAction(func() { a.moveEntry(over.ID, group, before) })
		return
	}
	a.dragPreviewID, a.dragBeforeID = over.ID, before
	// Refresh after scrolling has laid out newly visible virtual rows.
	c.After(time.Second / 60)
	dest := to
	if to > from {
		dest--
	}
	positions := make(map[int]ui.Rect, len(slots))
	for _, slot := range slots {
		positions[slot.index] = slot.bounds
	}
	for _, slot := range slots {
		if slot.index == from {
			slot.card.Opacity(0)
			continue
		}
		target := slot.index
		if from < dest && target > from && target <= dest {
			target--
		}
		if dest < from && target >= dest && target < from {
			target++
		}
		if target != slot.index {
			if rect, ok := positions[target]; ok {
				slot.card.Translate(rect.X-slot.bounds.X, rect.Y-slot.bounds.Y)
			} else {
				slot.card.Opacity(0)
			}
		}
	}
	if rect, ok := positions[dest]; ok {
		rect.W = float32(a.state.Settings.CardWidth)
		rect.H = float32(a.state.Settings.CardHeight)
		accent, background := c.Theme().Accent, c.Theme().Background
		grid.DrawOver(func(p *ui.Painter, _ ui.Rect) {
			p.Fill(rect, accent.Alpha(0.5), 10)
			p.Fill(ui.Rect{X: rect.X + 2, Y: rect.Y + 2, W: rect.W - 4, H: rect.H - 4}, background, 8)
		})
	}
}

// Use the nearest cell in the nearest row, including cell padding and row gaps.
func dropIndex(slots []dragSlot, x, y float32, n int) int {
	best := -1
	vertical, horizontal := float32(math.Inf(1)), float32(math.Inf(1))
	after := false
	for _, slot := range slots {
		r := slot.bounds
		if r.W <= 0 || r.H <= 0 {
			continue
		}
		if slot.index == n-1 && y >= r.Y+r.H {
			return n
		}
		dy := max(r.Y-y, y-r.Y-r.H, 0)
		dx := float32(math.Abs(float64(x - r.X - r.W/2)))
		if dy < vertical || (dy == vertical && dx < horizontal) {
			best, vertical, horizontal = slot.index, dy, dx
			after = x >= r.X+r.W/2
		}
	}
	if best < 0 {
		return n
	}
	if after {
		best++
	}
	return best
}
