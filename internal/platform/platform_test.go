package platform

import (
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"
)

func controlledBase(t *testing.T) string {
	t.Helper()
	base, err := filepath.Abs(filepath.Join("..", "..", "__tmp", "platform-base"))
	if err != nil {
		t.Fatal(err)
	}
	previous := executable
	executable = func() (string, error) { return filepath.Join(base, "launcher.exe"), nil }
	t.Cleanup(func() { executable = previous })
	return base
}

func TestResolvePaths(t *testing.T) {
	base := controlledBase(t)
	for _, tt := range []struct {
		path     string
		relative bool
		want     string
	}{
		{"tools/editor.exe", false, filepath.Join(base, "tools", "editor.exe")},
		{"tools/editor.exe", true, filepath.Join("tools", "editor.exe")},
		{"%QUICKSTART_TEST_ROOT%/editor.exe", true, filepath.Join("%QUICKSTART_TEST_ROOT%", "editor.exe")},
		{`"tools/editor.exe"`, true, filepath.Join("tools", "editor.exe")},
		{"https://example.com/a?q=1", true, "https://example.com/a?q=1"},
		{"uwp:Package_hash!App", true, "uwp:Package_hash!App"},
		{"command:echo hello", true, "command:echo hello"},
	} {
		entry, err := Resolve(tt.path, tt.relative)
		if err != nil || entry.Path != tt.want {
			t.Errorf("Resolve(%q, %v) = %q, %v; want %q", tt.path, tt.relative, entry.Path, err, tt.want)
		}
		if entry.ID == "" || entry.AddedAt <= 0 || entry.Name == "" {
			t.Errorf("missing import metadata: %+v", entry)
		}
	}
	if BaseDir() != base {
		t.Fatal("base did not use controlled executable")
	}
	for _, path := range []string{"", " ", "bad\x00path"} {
		if _, err := Resolve(path, false); err == nil {
			t.Errorf("Resolve(%q) should fail", path)
		}
	}
}

func TestResolveWindowsVolumes(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows volumes")
	}
	previous := executable
	executable = func() (string, error) { return `C:\Launcher\launcher.exe`, nil }
	t.Cleanup(func() { executable = previous })
	for _, tt := range []struct{ path, want string }{
		{`C:\Launcher\tools\app.exe`, `tools\app.exe`},
		{`C:\Elsewhere\app.exe`, `..\Elsewhere\app.exe`},
		{`D:\Tools\app.exe`, `D:\Tools\app.exe`},
		{`\\server\share\app.exe`, `\\server\share\app.exe`},
	} {
		entry, err := Resolve(tt.path, true)
		if err != nil || entry.Path != tt.want {
			t.Errorf("Resolve(%q) = %q, %v", tt.path, entry.Path, err)
		}
	}
}

func TestMergeEnvironment(t *testing.T) {
	base := []string{"Path=old", "KEEP=value", "EMPTY=before", "PATH=duplicate"}
	got, err := mergeEnv(base, []string{" path =first", "PATH=last=part", "EMPTY=", " NEW =  spaced  ", " ", "IGNORED", "=ignored"})
	want := []string{"KEEP=value", "PATH=last=part", "EMPTY=", "NEW=  spaced  "}
	if runtime.GOOS != "windows" {
		want = []string{"Path=old", "KEEP=value", "path=first", "PATH=last=part", "EMPTY=", "NEW=  spaced  "}
	}
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("mergeEnv = %q, %v; want %q", got, err, want)
	}
	if base[0] != "Path=old" {
		t.Fatal("inherited environment was modified")
	}
	for _, line := range []string{"BAD\x00=value", "KEY=value\x00", "INVALID\x00"} {
		if _, err := mergeEnv(base, []string{line}); err == nil {
			t.Errorf("invalid override accepted: %q", line)
		}
	}
}

func TestUWPPaths(t *testing.T) {
	got, err := uwpShellPath("uwp: Package_hash!App ")
	if err != nil || got != `shell:AppsFolder\Package_hash!App` {
		t.Fatalf("UWP shell path = %q, %v", got, err)
	}
	if _, err := uwpShellPath("uwp: "); err == nil {
		t.Fatal("empty UWP ID accepted")
	}
}

func TestCustomIcons(t *testing.T) {
	for _, path := range []string{"data:image/png;base64,AQID", "data:image/svg+xml,%01%02%03"} {
		got, err := Icon(path, 32)
		if err != nil || !reflect.DeepEqual(got, []byte{1, 2, 3}) {
			t.Errorf("Icon(%q) = %v, %v", path, got, err)
		}
	}
	if _, err := Icon("data:image/png;base64,%%%", 32); err == nil {
		t.Fatal("invalid base64 accepted")
	}
	if _, err := Icon("data:image/png;base64,AQID", 0); err == nil {
		t.Fatal("invalid size accepted")
	}
	dir := scratchDir(t)
	path := filepath.Join(dir, "custom.png")
	if err := os.WriteFile(path, []byte{1, 2, 3}, 0600); err != nil {
		t.Fatal(err)
	}
	got, err := Icon(path, 32)
	if err != nil || !reflect.DeepEqual(got, []byte{1, 2, 3}) {
		t.Fatalf("custom icon file = %v, %v", got, err)
	}
}

func scratchDir(t *testing.T) string {
	t.Helper()
	root := filepath.Join("..", "..", "__tmp")
	if err := os.MkdirAll(root, 0700); err != nil {
		t.Fatal(err)
	}
	dir, err := os.MkdirTemp(root, "platform-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	absolute, err := filepath.Abs(dir)
	if err != nil {
		t.Fatal(err)
	}
	return absolute
}
