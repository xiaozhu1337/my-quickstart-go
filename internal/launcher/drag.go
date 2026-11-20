package launcher

import (
	"math"
	"strings"

	"github.com/egoist/mygo/ui"
	"github.com/nsgo-dev/nsgo/core"

	"my-quickstart-go/internal/model"
)

// cardDestination uses surface coordinates so scrolling and display scaling do
// not change which insertion point the preview and release select.
func cardDestination(at core.Point, grid *core.Ref, targets map[string]dropTarget, entries []model.Entry, active string) (dropTarget, string, bool) {
	b, mounted := grid.Bounds()
	if !mounted || !b.Contains(at) {
		for key, dst := range targets {
			if dst.group == "" || !strings.HasPrefix(key, "group-") {
				continue
			}
			if bounds, ok := dst.ref.Bounds(); ok && bounds.Contains(at) {
				return dst, key, true
			}
		}
		return dropTarget{}, "", false
	}
	slots := make([]dragSlot, 0, len(targets))
	for _, dst := range targets {
		if dst.index < 0 {
			continue
		}
		if cell, ok := dst.ref.Bounds(); ok {
			slots = append(slots, dragSlot{index: dst.index, bounds: ui.Rect{
				X: float32(cell.Origin.X), Y: float32(cell.Origin.Y), W: float32(cell.Size.Width), H: float32(cell.Size.Height),
			}})
		}
	}
	if i := dropIndex(slots, float32(at.X), float32(at.Y), len(entries)); i < len(entries) {
		next := entries[i].ID
		return dropTarget{group: active, before: next}, next, true
	}
	return dropTarget{group: active}, "$end", true
}

type dragSlot struct {
	index  int
	bounds ui.Rect
	card   ui.Element
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
