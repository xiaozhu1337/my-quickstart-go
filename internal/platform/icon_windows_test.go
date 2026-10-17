//go:build windows

package platform

import (
	"bytes"
	"fmt"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"unsafe"
)

func checkShellIcon(data []byte, size int) error {
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		return err
	}
	bounds := img.Bounds()
	if bounds.Dx() <= 0 || bounds.Dy() <= 0 || bounds.Dx() > size || bounds.Dy() > size {
		return fmt.Errorf("unexpected icon dimensions: %v", bounds)
	}
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			if _, _, _, a := img.At(x, y).RGBA(); a != 0 {
				return nil
			}
		}
	}
	return fmt.Errorf("icon is fully transparent")
}

func TestFileIcon(t *testing.T) {
	unicodeFolder := filepath.Join(scratchDir(t), "图标 café")
	if err := os.Mkdir(unicodeFolder, 0700); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{systemProgram("cmd.exe"), filepath.Dir(systemProgram("cmd.exe")), unicodeFolder} {
		t.Run(filepath.Base(path), func(t *testing.T) {
			data, err := fileIcon(path, 48)
			if err != nil {
				t.Fatal(err)
			}
			if err := checkShellIcon(data, 48); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestFileIconAppsFolder(t *testing.T) {
	// Windows Server and stripped installations may have no Calculator package.
	path := `shell:AppsFolder\Microsoft.WindowsCalculator_8wekyb3d8bbwe!App`
	data, err := fileIcon(path, 48)
	if err != nil {
		t.Skipf("Calculator shell item unavailable: %v", err)
	}
	if err := checkShellIcon(data, 48); err != nil {
		t.Fatal(err)
	}
}

func TestFileIconErrors(t *testing.T) {
	for _, tc := range []struct {
		path string
		size int
	}{
		{"", 32}, {" \t", 32}, {"bad\x00path", 32}, {strings.Repeat("a", 32768), 32},
		{systemProgram("cmd.exe"), 0}, {systemProgram("cmd.exe"), -1},
		{systemProgram("cmd.exe"), maxShellIconSize + 1},
		{filepath.Join(scratchDir(t), "missing.exe"), 32},
		{`shell:AppsFolder\Missing.Package_0000000000000!Missing`, 32},
	} {
		if data, err := fileIcon(tc.path, tc.size); err == nil || len(data) != 0 {
			t.Errorf("fileIcon(path length %d, size %d) returned %d bytes, %v", len(tc.path), tc.size, len(data), err)
		}
	}
}

func TestFileIconConcurrent(t *testing.T) {
	paths := []string{systemProgram("cmd.exe"), filepath.Dir(systemProgram("cmd.exe"))}
	var workers sync.WaitGroup
	for i := 0; i < 8; i++ {
		workers.Go(func() {
			for _, path := range paths {
				data, err := fileIcon(path, 32)
				if err == nil {
					err = checkShellIcon(data, 32)
				}
				if err != nil {
					t.Errorf("concurrent fileIcon(%q): %v", path, err)
				}
			}
		})
	}
	workers.Wait()
}

func TestFileIconCOMBalance(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	getApartment := ole32.NewProc("CoGetApartmentType")
	apartment := func() (uint32, uint32, uint32) {
		var kind, qualifier uint32
		hr, _, _ := getApartment.Call(uintptr(unsafe.Pointer(&kind)), uintptr(unsafe.Pointer(&qualifier)))
		return uint32(hr), kind, qualifier
	}
	assertUninitialized := func() {
		t.Helper()
		hr, kind, qualifier := apartment()
		// Shell workers may leave an implicit MTA; this thread must hold no explicit COM reference.
		if hr != 0x800401f0 && !(hr == 0 && kind == 1 && qualifier == 1) {
			t.Fatalf("thread holds a COM initialization: hr=0x%08x kind=%d qualifier=%d", hr, kind, qualifier)
		}
	}
	assertUninitialized()
	paths := []string{systemProgram("cmd.exe"), filepath.Join(scratchDir(t), "missing.exe")}
	for _, mode := range []int{-1, 2, 0} { // Uninitialized, STA (S_FALSE), MTA (RPC_E_CHANGED_MODE).
		func() {
			if mode >= 0 {
				hr, _, _ := coInitialize.Call(0, uintptr(mode))
				if err := comError("test CoInitializeEx", hr); err != nil {
					t.Fatal(err)
				}
				if hr != 0 { // A fresh explicit initialization must return S_OK, not S_FALSE.
					t.Fatalf("test CoInitializeEx found a leaked reference: 0x%08x", uint32(hr))
				}
				defer coUninitialize.Call()
			}
			for i, path := range paths {
				_, err := fileIcon(path, 32)
				if (err != nil) != (i == 1) {
					t.Fatalf("mode %d, path %q: %v", mode, path, err)
				}
				if mode == -1 {
					assertUninitialized()
				} else if hr, kind, qualifier := apartment(); hr != 0 || qualifier == 1 ||
					mode == 0 && kind != 1 || mode == 2 && kind != 0 && kind != 3 {
					t.Fatalf("fileIcon changed COM apartment in mode %d: hr=0x%08x kind=%d qualifier=%d", mode, hr, kind, qualifier)
				}
			}
		}()
		assertUninitialized()
	}
}

func TestIconDIBImage(t *testing.T) {
	for _, height := range []int32{2, -2} {
		for _, headerHeight := range []int32{2, -2} {
			// Two padded BGRA rows, including a straight-alpha color greater than alpha.
			pixels := []byte{10, 20, 240, 80, 30, 40, 50, 255, 0, 0, 0, 0, 60, 70, 80, 255, 90, 100, 110, 0, 0, 0, 0, 0}
			dib := iconDIBSection{width: 2, height: height, stride: 12, planes: 1, bitsPixel: 32,
				bits: unsafe.Pointer(&pixels[0]), headerWidth: 2, headerHeight: headerHeight}
			img, err := iconDIBImage(&dib)
			if err != nil {
				t.Fatal(err)
			}
			y := 0
			if headerHeight > 0 {
				y = 1
			}
			if got := img.NRGBAAt(0, y); got != (color.NRGBA{R: 240, G: 20, B: 10, A: 80}) {
				t.Fatalf("height %d, header %d: BGRA/alpha conversion = %v", height, headerHeight, got)
			}
			if got := img.NRGBAAt(0, 1-y); got != (color.NRGBA{R: 80, G: 70, B: 60, A: 255}) {
				t.Fatalf("height %d, header %d: row orientation = %v", height, headerHeight, got)
			}
			var encoded bytes.Buffer
			if err := png.Encode(&encoded, img); err != nil {
				t.Fatal(err)
			}
			decoded, err := png.Decode(&encoded)
			if err != nil {
				t.Fatal(err)
			}
			if got := color.NRGBAModel.Convert(decoded.At(0, y)); got != img.NRGBAAt(0, y) {
				t.Fatalf("PNG altered straight alpha: %v", got)
			}
		}
	}
}

func TestIconDIBImageRejectsInvalidBitmap(t *testing.T) {
	pixels := make([]byte, 16)
	valid := iconDIBSection{width: 2, height: 2, stride: 8, planes: 1, bitsPixel: 32,
		bits: unsafe.Pointer(&pixels[0]), headerWidth: 2, headerHeight: 2}
	for _, mutate := range []func(*iconDIBSection){
		func(d *iconDIBSection) { d.bits = nil },
		func(d *iconDIBSection) { d.planes = 0 },
		func(d *iconDIBSection) { d.bitsPixel = 24 },
		func(d *iconDIBSection) { d.compression = 1 },
		func(d *iconDIBSection) { d.width = 0 },
		func(d *iconDIBSection) { d.width = maxShellIconSize + 1 },
		func(d *iconDIBSection) { d.height = -1 << 31 },
		func(d *iconDIBSection) { d.stride = 4 },
		func(d *iconDIBSection) { d.stride = maxShellIconSize*4 + 1 },
		func(d *iconDIBSection) { d.headerWidth = 1 },
		func(d *iconDIBSection) { d.headerHeight = 0 },
	} {
		dib := valid
		mutate(&dib)
		if _, err := iconDIBImage(&dib); err == nil {
			t.Errorf("accepted invalid DIB: %+v", dib)
		}
	}
}
