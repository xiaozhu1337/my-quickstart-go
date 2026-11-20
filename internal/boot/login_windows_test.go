package boot

import (
	"errors"
	"fmt"
	"os"
	"reflect"
	"testing"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

func TestLoginCommandAndBootIdentity(t *testing.T) {
	id, err := ID()
	if err != nil || len(id) != 38 {
		t.Fatal("Windows boot identifier", id, err)
	}
	next, err := ID()
	if err != nil || next != id {
		t.Fatal("unstable boot identifier", next, err)
	}
	keyPath := fmt.Sprintf("Software\\QuickstartGoTest-%d", os.Getpid())
	key, already, err := registry.CreateKey(registry.CURRENT_USER, keyPath, registry.ALL_ACCESS)
	if err != nil {
		t.Fatal(err)
	}
	defer key.Close()
	if already {
		t.Fatal("test registry key already exists")
	}
	t.Cleanup(func() {
		if err := registry.DeleteKey(registry.CURRENT_USER, keyPath); err != nil {
			t.Error(err)
		}
	})
	exe, data := "C:\\Program Files\\Quickstart\\launcher.exe", "C:\\Test Profiles\\我的数据库.db"
	if err := registerAt(keyPath, exe, data, true); err != nil {
		t.Fatal(err)
	}
	name := "My Quickstart Go " + Profile(data)
	command, _, err := key.GetStringValue(name)
	if err != nil {
		t.Fatal(err)
	}
	args, err := windows.DecomposeCommandLine(command)
	if err != nil || !reflect.DeepEqual(args, []string{exe, "--" + Flag, "--data", data}) {
		t.Fatal(args, err)
	}
	if err := registerAt(keyPath, exe, data, false); err != nil {
		t.Fatal(err)
	}
	if _, _, err = key.GetStringValue(name); !errors.Is(err, registry.ErrNotExist) {
		t.Fatal("startup value still present", err)
	}
	if err := registerAt(keyPath, exe, data, false); err != nil {
		t.Fatal(err)
	}
}
