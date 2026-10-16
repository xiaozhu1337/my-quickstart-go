package platform

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"

	"my-quickstart-go/internal/model"
)

type childResult struct {
	Args      []string
	Directory string
	Value     string
	Empty     string
}

func TestLaunchChild(t *testing.T) {
	output := os.Getenv("QUICKSTART_CHILD_OUTPUT")
	if output == "" {
		return
	}
	dir, _ := os.Getwd()
	result := childResult{os.Args[3:], dir, os.Getenv("QUICKSTART_VALUE"), os.Getenv("QUICKSTART_EMPTY")}
	data, err := json.Marshal(result)
	if err != nil {
		os.Exit(2)
	}
	if os.WriteFile(output+".pending", data, 0600) != nil || os.Rename(output+".pending", output) != nil {
		os.Exit(3)
	}
	os.Exit(0)
}

func TestLaunchPreservesArgumentsAndEnvironment(t *testing.T) {
	dir := scratchDir(t)
	output := filepath.Join(dir, "child.json")
	program, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("QUICKSTART_VALUE", "inherited")
	t.Setenv("QUICKSTART_EMPTY", "before")
	wantArgs := []string{`C:\work\file`, "", `ends\`, `a"b`, " spaced ", "中文"}
	args := []string{"-test.run=^TestLaunchChild$", "--"}
	args = append(args, wantArgs...)
	quoted := make([]string, len(args))
	for i, arg := range args {
		quoted[i] = quoteTestArg(arg)
	}
	entry := model.Entry{Path: program, Args: strings.Join(quoted, " "), EnvVars: []string{
		"QUICKSTART_CHILD_OUTPUT=" + output, "QUICKSTART_VALUE=overridden=exact", "QUICKSTART_EMPTY=", "IGNORED", "=ignored",
	}}
	if err := Launch(entry); err != nil {
		t.Fatal(err)
	}
	data := waitForFile(t, output)
	var got childResult
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got.Args, wantArgs) || got.Directory != filepath.Dir(program) || got.Value != "overridden=exact" || got.Empty != "" {
		t.Fatalf("child result: %+v; want args %q, cwd %q", got, wantArgs, filepath.Dir(program))
	}
}

func TestLaunchCommand(t *testing.T) {
	dir := scratchDir(t)
	output := filepath.Join(dir, "command.txt")
	t.Setenv("QUICKSTART_COMMAND_VALUE", "inherited")
	command := `printf '%s' "$QUICKSTART_COMMAND_VALUE" > ` + quoteTestArg(output)
	if runtime.GOOS == "windows" {
		command = `echo %QUICKSTART_COMMAND_VALUE%>` + quoteTestArg(output)
	}
	if err := Launch(model.Entry{Path: "command:" + command, Args: "must be ignored", EnvVars: []string{"QUICKSTART_COMMAND_VALUE=wrong"}}); err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(string(waitForFile(t, output))); got != "inherited" {
		t.Fatalf("command output = %q", got)
	}
}

func TestLaunchErrors(t *testing.T) {
	for _, entry := range []model.Entry{
		{Path: "command: "},
		{Path: ""},
		{Path: "uwp: "},
		{Path: filepath.Join(scratchDir(t), "missing.exe"), Args: "--force-direct"},
		{Path: "invalid\x00path"},
	} {
		if err := Launch(entry); err == nil {
			t.Errorf("Launch(%+v) should fail", entry)
		}
	}
	for _, path := range []string{"", "command:echo x", "uwp:App", "https://example.com"} {
		if err := OpenFolder(path); err == nil {
			t.Errorf("OpenFolder(%q) should fail", path)
		}
	}
}

func waitForFile(t *testing.T, path string) []byte {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		data, err := os.ReadFile(path)
		if err == nil && len(data) != 0 {
			return data
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("child did not produce %s", path)
	return nil
}
