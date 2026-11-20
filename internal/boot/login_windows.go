package boot

import (
	"errors"
	"fmt"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

const runKey = `Software\Microsoft\Windows\CurrentVersion\Run`

func Register(exe, data string, enabled bool) error {
	return registerAt(runKey, exe, data, enabled)
}

func registerAt(keyPath, exe, data string, enabled bool) error {
	name := "My Quickstart Go " + Profile(data)
	const rights = registry.QUERY_VALUE | registry.SET_VALUE
	if !enabled {
		key, err := registry.OpenKey(registry.CURRENT_USER, keyPath, registry.QUERY_VALUE)
		if errors.Is(err, registry.ErrNotExist) {
			return nil
		}
		if err != nil {
			return err
		}
		_, _, err = key.GetValue(name, nil)
		key.Close()
		if errors.Is(err, registry.ErrNotExist) {
			return nil
		}
		if err != nil {
			return err
		}
		key, err = registry.OpenKey(registry.CURRENT_USER, keyPath, registry.SET_VALUE)
		if err != nil {
			return err
		}
		defer key.Close()
		err = key.DeleteValue(name)
		if errors.Is(err, registry.ErrNotExist) {
			return nil
		}
		return err
	}
	value := windows.ComposeCommandLine([]string{exe, "--" + Flag, "--data", data})
	wide, err := syscall.UTF16FromString(value)
	if err != nil {
		return err
	}
	if len(wide) > 261 {
		return fmt.Errorf("Windows startup command exceeds 260 characters: %s", data)
	}
	key, _, err := registry.CreateKey(registry.CURRENT_USER, keyPath, rights)
	if err != nil {
		return err
	}
	defer key.Close()
	current, _, err := key.GetStringValue(name)
	if err == nil && current == value {
		return nil
	}
	return key.SetStringValue(name, value)
}

func ID() (string, error) {
	var info struct {
		BootIdentifier windows.GUID
		FirmwareType   uint32
		Padding        uint32
		BootFlags      uint64
	}
	if err := windows.NtQuerySystemInformation(windows.SystemBootEnvironmentInformation, unsafe.Pointer(&info), uint32(unsafe.Sizeof(info)), nil); err != nil {
		return "", err
	}
	return info.BootIdentifier.String(), nil
}
