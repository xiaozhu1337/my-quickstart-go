//go:build windows

package platform

import (
	"path/filepath"
	"runtime"
	"syscall"
	"testing"
	"unsafe"
)

func quoteTestArg(arg string) string { return syscall.EscapeArg(arg) }

func TestResolveShortcut(t *testing.T) {
	dir := scratchDir(t)
	path := filepath.Join(dir, "Friendly Name.LNK")
	target := filepath.Join(dir, "Target App.exe")
	args := `"C:\path with spaces\file" "" trailing\`
	writeShortcut(t, path, target, args)
	previous := executable
	executable = func() (string, error) { return filepath.Join(dir, "launcher.exe"), nil }
	t.Cleanup(func() { executable = previous })
	entry, err := Resolve(path, true)
	if err != nil || entry.Name != "Friendly Name" || entry.Path != "Target App.exe" || entry.Args != args || entry.Icon != "Friendly Name.LNK" {
		t.Fatalf("shortcut entry = %+v, %v", entry, err)
	}
	fallback, err := Resolve(filepath.Join(dir, "Missing.lnk"), true)
	if err != nil || fallback.Path != "Missing.lnk" || fallback.Name != "Missing" || fallback.Args != "" {
		t.Fatalf("unresolved shortcut fallback = %+v, %v", fallback, err)
	}
}

func writeShortcut(t *testing.T, path, target, args string) {
	t.Helper()
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	hr, _, _ := coInitialize.Call(0, 2)
	if err := comError("CoInitializeEx", hr); err != nil {
		t.Fatal(err)
	}
	defer coUninitialize.Call()
	clsid := syscall.GUID{Data1: 0x00021401, Data4: [8]byte{0xc0, 0, 0, 0, 0, 0, 0, 0x46}}
	iid := syscall.GUID{Data1: 0x000214f9, Data4: clsid.Data4}
	persistID := syscall.GUID{Data1: 0x0000010b, Data4: clsid.Data4}
	var link, persist *comInterface
	hr, _, _ = coCreate.Call(uintptr(unsafe.Pointer(&clsid)), 0, 1, uintptr(unsafe.Pointer(&iid)), uintptr(unsafe.Pointer(&link)))
	if err := comError("CoCreateInstance", hr); err != nil {
		t.Fatal(err)
	}
	defer comCall(link, 2)
	if err := comError("QueryInterface", comCall(link, 0, uintptr(unsafe.Pointer(&persistID)), uintptr(unsafe.Pointer(&persist)))); err != nil {
		t.Fatal(err)
	}
	defer comCall(persist, 2)
	for _, item := range []struct {
		slot  int
		value string
	}{{20, target}, {11, args}} {
		wide, _ := syscall.UTF16PtrFromString(item.value)
		if err := comError("IShellLink setter", comCall(link, item.slot, uintptr(unsafe.Pointer(wide)))); err != nil {
			t.Fatal(err)
		}
	}
	wide, _ := syscall.UTF16PtrFromString(path)
	if err := comError("IPersistFile.Save", comCall(persist, 6, uintptr(unsafe.Pointer(wide)), 1)); err != nil {
		t.Fatal(err)
	}
}
