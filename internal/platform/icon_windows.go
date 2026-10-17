//go:build windows

package platform

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/png"
	"runtime"
	"strings"
	"syscall"
	"unsafe"
)

var (
	createShellItem  = syscall.NewLazyDLL("shell32.dll").NewProc("SHCreateItemFromParsingName")
	iconGDI          = syscall.NewLazyDLL("gdi32.dll")
	getIconObject    = iconGDI.NewProc("GetObjectW")
	deleteIconObject = iconGDI.NewProc("DeleteObject")
)

const maxShellIconSize = 4096

// iconDIBSection matches DIBSECTION, including BITMAP and BITMAPINFOHEADER.
type iconDIBSection struct {
	kind, width, height, stride      int32
	planes, bitsPixel                uint16
	bits                             unsafe.Pointer
	headerSize                       uint32
	headerWidth, headerHeight        int32
	headerPlanes, headerBitsPixel    uint16
	compression, imageSize           uint32
	xPixelsPerMeter, yPixelsPerMeter int32
	colorsUsed, colorsImportant      uint32
	bitfields                        [3]uint32
	section                          uintptr
	offset                           uint32
}

// fileIcon can run on a worker without a MyGo application or UI message loop.
func fileIcon(path string, size int) ([]byte, error) {
	if size <= 0 || size > maxShellIconSize {
		return nil, fmt.Errorf("icon size must be between 1 and %d", maxShellIconSize)
	}
	if strings.TrimSpace(path) == "" {
		return nil, errors.New("icon path is empty")
	}
	wide, err := syscall.UTF16FromString(path)
	if err != nil {
		return nil, err
	}
	if len(wide) > 32768 {
		return nil, errors.New("icon path is too long")
	}

	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	hr, _, _ := coInitialize.Call(0, 2) // COINIT_APARTMENTTHREADED
	if int32(hr) >= 0 {
		defer coUninitialize.Call()
	} else if uint32(hr) != 0x80010106 { // RPC_E_CHANGED_MODE: use the caller's apartment.
		return nil, comError("CoInitializeEx", hr)
	}

	iid := syscall.GUID{Data1: 0xbcc18b79, Data2: 0xba16, Data3: 0x442f, Data4: [8]byte{0x80, 0xc4, 0x8a, 0x59, 0xc3, 0x0c, 0x46, 0x3b}}
	var factory *comInterface
	hr, _, _ = createShellItem.Call(uintptr(unsafe.Pointer(&wide[0])), 0, uintptr(unsafe.Pointer(&iid)), uintptr(unsafe.Pointer(&factory)))
	if err := comError("SHCreateItemFromParsingName", hr); err != nil {
		return nil, err
	}
	if factory == nil {
		return nil, errors.New("shell returned no icon factory")
	}
	defer comCall(factory, 2)

	// SIZE is two signed 32-bit values passed together by value on Windows amd64/arm64.
	dimensions := uintptr(uint64(uint32(size)) | uint64(uint32(size))<<32)
	var bitmap uintptr
	hr = comCall(factory, 3, dimensions, 0x4, uintptr(unsafe.Pointer(&bitmap))) // SIIGBF_ICONONLY
	if bitmap != 0 {
		defer deleteIconObject.Call(bitmap)
	}
	if err := comError("IShellItemImageFactory.GetImage", hr); err != nil {
		return nil, err
	}
	if bitmap == 0 {
		return nil, errors.New("shell returned no icon bitmap")
	}
	var dib iconDIBSection
	if n, _, _ := getIconObject.Call(bitmap, unsafe.Sizeof(dib), uintptr(unsafe.Pointer(&dib))); n != unsafe.Sizeof(dib) {
		return nil, errors.New("shell icon is not a DIB section")
	}
	img, err := iconDIBImage(&dib)
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func iconDIBImage(dib *iconDIBSection) (*image.NRGBA, error) {
	w, h, stride := int(dib.width), int64(dib.height), int(dib.stride)
	if h < 0 {
		h = -h
	}
	if dib.bits == nil || dib.planes != 1 || dib.bitsPixel != 32 || dib.compression != 0 ||
		w <= 0 || w > maxShellIconSize || h <= 0 || h > maxShellIconSize ||
		stride < w*4 || stride > maxShellIconSize*4 ||
		dib.headerWidth != dib.width || int64(dib.headerHeight) != h && int64(dib.headerHeight) != -h {
		return nil, errors.New("invalid shell icon bitmap")
	}
	src := unsafe.Slice((*byte)(dib.bits), stride*int(h))
	img := image.NewNRGBA(image.Rect(0, 0, w, int(h)))
	for y := 0; y < int(h); y++ {
		row := y
		if dib.headerHeight > 0 { // Positive DIB height stores the bottom row first.
			row = int(h) - 1 - y
		}
		s, d := src[row*stride:], img.Pix[y*img.Stride:]
		for x := 0; x < w; x++ {
			// Shell DIBs use straight BGRA alpha; NRGBA preserves translucent colors.
			d[4*x], d[4*x+1], d[4*x+2], d[4*x+3] = s[4*x+2], s[4*x+1], s[4*x], s[4*x+3]
		}
	}
	return img, nil
}
