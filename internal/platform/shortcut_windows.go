//go:build windows

package platform

import (
	"errors"
	"fmt"
	"runtime"
	"syscall"
	"unsafe"
)

var (
	ole32          = syscall.NewLazyDLL("ole32.dll")
	coInitialize   = ole32.NewProc("CoInitializeEx")
	coUninitialize = ole32.NewProc("CoUninitialize")
	coCreate       = ole32.NewProc("CoCreateInstance")
)

type comInterface struct{ vtable *[32]uintptr }

func comCall(object *comInterface, slot int, args ...uintptr) uintptr {
	result, _, _ := syscall.SyscallN(object.vtable[slot], append([]uintptr{uintptr(unsafe.Pointer(object))}, args...)...)
	runtime.KeepAlive(object)
	return result
}

func comError(operation string, hr uintptr) error {
	if int32(hr) < 0 {
		return fmt.Errorf("%s: HRESULT 0x%08x", operation, uint32(hr))
	}
	return nil
}

func resolveShortcut(path string) (string, string, error) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	hr, _, _ := coInitialize.Call(0, 2) // COINIT_APARTMENTTHREADED
	if int32(hr) >= 0 {
		defer coUninitialize.Call()
	} else if uint32(hr) != 0x80010106 { // RPC_E_CHANGED_MODE: use the existing apartment.
		return "", "", comError("CoInitializeEx", hr)
	}
	clsid := syscall.GUID{Data1: 0x00021401, Data4: [8]byte{0xc0, 0, 0, 0, 0, 0, 0, 0x46}}
	iid := syscall.GUID{Data1: 0x000214f9, Data4: clsid.Data4}
	persistID := syscall.GUID{Data1: 0x0000010b, Data4: clsid.Data4}
	var link, persist *comInterface
	hr, _, _ = coCreate.Call(uintptr(unsafe.Pointer(&clsid)), 0, 1, uintptr(unsafe.Pointer(&iid)), uintptr(unsafe.Pointer(&link)))
	if err := comError("CoCreateInstance(ShellLink)", hr); err != nil {
		return "", "", err
	}
	defer comCall(link, 2)
	if err := comError("QueryInterface(IPersistFile)", comCall(link, 0, uintptr(unsafe.Pointer(&persistID)), uintptr(unsafe.Pointer(&persist)))); err != nil {
		return "", "", err
	}
	defer comCall(persist, 2)
	wide, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return "", "", err
	}
	if err := comError("IPersistFile.Load", comCall(persist, 5, uintptr(unsafe.Pointer(wide)), 0)); err != nil {
		return "", "", err
	}
	// Offline targets can still be imported using their stored raw path.
	comCall(link, 19, 0, 1) // Resolve(SLR_NO_UI)
	target := make([]uint16, 32768)
	if err := comError("IShellLink.GetPath", comCall(link, 3, uintptr(unsafe.Pointer(&target[0])), uintptr(len(target)), 0, 4)); err != nil {
		return "", "", err
	}
	args := make([]uint16, 32768)
	if err := comError("IShellLink.GetArguments", comCall(link, 10, uintptr(unsafe.Pointer(&args[0])), uintptr(len(args)))); err != nil {
		return "", "", err
	}
	value := syscall.UTF16ToString(target)
	if value == "" {
		return "", "", errors.New("shortcut target is empty")
	}
	return value, syscall.UTF16ToString(args), nil
}
