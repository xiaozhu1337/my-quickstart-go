//go:build windows

package platform

import (
	"errors"
	"syscall"
	"unsafe"

	"github.com/egoist/mygo"
)

var (
	frameUser32       = syscall.NewLazyDLL("user32.dll")
	findSurface       = frameUser32.NewProc("FindWindowExW")
	frameMessage      = frameUser32.NewProc("SendMessageW")
	invalidateSurface = frameUser32.NewProc("InvalidateRect")
)

// PrepareWindow draws the native surface before Show, including while hidden.
func PrepareWindow(w *mygo.Window) error {
	var err error
	mygo.RunOnMain(func() {
		class, _ := syscall.UTF16PtrFromString("MyGoSurface")
		surface, _, _ := findSurface.Call(w.NativeHandle(), 0, uintptr(unsafe.Pointer(class)), 0)
		if surface == 0 {
			err = errors.New("cannot prepare the native window surface")
			return
		}
		// Hidden windows do not receive scheduled paints; send one without showing or activating them.
		invalidateSurface.Call(surface, 0, 0)
		frameMessage.Call(surface, 0x000F, 0, 0) // WM_PAINT
	})
	return err
}
