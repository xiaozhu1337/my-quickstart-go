//go:build !windows

package platform

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"my-quickstart-go/internal/model"
)

func TestDesktopFieldsAndCodes(t *testing.T) {
	fields, err := desktopFields(`[Desktop Entry]
Type=Application
Name=Some\sApp
Icon=app-icon
Exec=/usr/bin/tool %c %k %i %% %f %F %u %U "C:\\\\files\\\\example"
Path=/some\sspace
Terminal=false
[Desktop Action Other]
Exec=ignored
`)
	if err != nil || fields["Name"] != "Some App" || fields["Path"] != "/some space" {
		t.Fatalf("desktop fields = %v, %v", fields, err)
	}
	words, err := splitArgs(fields["Exec"])
	if err != nil {
		t.Fatal(err)
	}
	got, err := desktopWords(words, fields, "/apps/tool.desktop")
	want := []string{"/usr/bin/tool", "Some App", "/apps/tool.desktop", "--icon", "app-icon", "%", `C:\files\example`}
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("desktop words = %q, %v; want %q", got, err, want)
	}
	for _, word := range []string{"%x", "%", "prefix%F", "prefix%i"} {
		if _, err := desktopWords([]string{"tool", word}, fields, "entry.desktop"); err == nil {
			t.Errorf("invalid field code accepted: %s", word)
		}
	}
	if _, err := desktopFields("[Desktop Entry]\nExec=bad\\x"); err == nil {
		t.Fatal("unknown desktop escape accepted")
	}
}

func TestLaunchDesktopArgumentsAndEnvironment(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux desktop entries")
	}
	dir := scratchDir(t)
	output := filepath.Join(dir, "desktop.json")
	program, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "test.desktop")
	text := "[Desktop Entry]\nType=Application\nName=Desktop Name\nExec=" + quoteTestArg(program) + " -test.run=^TestLaunchChild$ -- %c %f\nPath=" + dir + "\nTerminal=false\n"
	if err := os.WriteFile(path, []byte(text), 0600); err != nil {
		t.Fatal(err)
	}
	if err := Launch(model.Entry{Path: path, Args: quoteTestArg("custom arg"), EnvVars: []string{
		"QUICKSTART_CHILD_OUTPUT=" + output, "QUICKSTART_VALUE=desktop override",
	}}); err != nil {
		t.Fatal(err)
	}
	var got childResult
	if err := json.Unmarshal(waitForFile(t, output), &got); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got.Args, []string{"Desktop Name", "custom arg"}) || got.Value != "desktop override" || got.Directory != dir {
		t.Fatalf("desktop child = %+v", got)
	}
	if err := os.WriteFile(path, []byte("[Desktop Entry]\nType=Application\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := desktopCommand(path, nil); err == nil || !strings.Contains(err.Error(), "Exec") {
		t.Fatalf("missing Exec error = %v", err)
	}
}

func TestDesktopTerminalMissing(t *testing.T) {
	t.Setenv("PATH", scratchDir(t))
	if _, err := terminalWords([]string{"tool"}); err == nil {
		t.Fatal("missing terminal should return an error")
	}
}

func TestLaunchBundleArgumentsAndEnvironment(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("macOS bundles")
	}
	dir := scratchDir(t)
	bundle := filepath.Join(dir, "Test.app")
	contents := filepath.Join(bundle, "Contents")
	macos := filepath.Join(contents, "MacOS")
	if err := os.MkdirAll(macos, 0700); err != nil {
		t.Fatal(err)
	}
	program, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	binary, err := os.ReadFile(program)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(macos, "Helper"), binary, 0700); err != nil {
		t.Fatal(err)
	}
	plist := `<?xml version="1.0"?><plist version="1.0"><dict><key>CFBundleExecutable</key><string>Helper</string></dict></plist>`
	if err := os.WriteFile(filepath.Join(contents, "Info.plist"), []byte(plist), 0600); err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(dir, "bundle.json")
	if err := Launch(model.Entry{Path: bundle, Args: "-test.run=^TestLaunchChild$ -- 'bundle arg'", EnvVars: []string{
		"QUICKSTART_CHILD_OUTPUT=" + output, "QUICKSTART_VALUE=bundle override",
	}}); err != nil {
		t.Fatal(err)
	}
	var got childResult
	if err := json.Unmarshal(waitForFile(t, output), &got); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got.Args, []string{"bundle arg"}) || got.Value != "bundle override" || got.Directory != macos {
		t.Fatalf("bundle child = %+v", got)
	}
}
