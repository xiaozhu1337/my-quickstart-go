//go:build windows

package platform

import (
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"syscall"
	"testing"
	"time"

	"my-quickstart-go/internal/model"
)

func pythonTestDir(t *testing.T) string {
	t.Helper()
	if err := os.MkdirAll(filepath.Join("..", "..", "__tmp"), 0700); err != nil {
		t.Fatal(err)
	}
	dir, err := os.MkdirTemp(filepath.Join("..", "..", "__tmp"), "python-")
	if err != nil {
		t.Fatal(err)
	}
	dir, err = filepath.Abs(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		removePythonTestPath(t, dir)
	})
	return dir
}

func removePythonTestFile(t *testing.T, path string) {
	t.Helper()
	t.Cleanup(func() {
		removePythonTestPath(t, path)
	})
}

func removePythonTestPath(t *testing.T, path string) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for {
		err := os.Remove(path)
		if err == nil || os.IsNotExist(err) {
			return
		}
		retryable := errors.Is(err, syscall.Errno(5)) || // ERROR_ACCESS_DENIED
			errors.Is(err, syscall.Errno(32)) || // ERROR_SHARING_VIOLATION
			errors.Is(err, syscall.Errno(145)) // ERROR_DIR_NOT_EMPTY
		if !retryable || time.Now().After(deadline) {
			t.Error(err)
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestScheduledPythonHasNoConsole(t *testing.T) {
	python, err := exec.LookPath("python")
	if err != nil {
		t.Skip("run via mise exec go@1.27.3 python@3.15.2 -- go test to exercise Python")
	}
	dir := pythonTestDir(t)
	// A directory link exercises an explicit interpreter path containing spaces
	// without copying or changing the managed Python installation.
	linkedHome := filepath.Join(dir, "Python 环境")
	customPython := python
	if err := os.Symlink(filepath.Dir(python), linkedHome); err != nil {
		t.Logf("interpreter directory link unavailable; using the original path: %v", err)
	} else {
		removePythonTestFile(t, linkedHome)
		customPython = filepath.Join(linkedHome, filepath.Base(python))
	}
	for _, ext := range []string{".py", ".PYW"} {
		for _, explicit := range []bool{false, true} {
			t.Run(ext+map[bool]string{false: "-PATH", true: "-explicit"}[explicit], func(t *testing.T) {
				script := filepath.Join(dir, "每日 report"+ext)
				output := filepath.Join(dir, "result.json")
				removePythonTestFile(t, script)
				removePythonTestFile(t, output)
				removePythonTestFile(t, filepath.Join(dir, "result.pending"))
				source := `import ctypes, json, os, pathlib, sys
result = {'args': sys.argv[2:], 'directory': os.getcwd(), 'console': ctypes.windll.kernel32.GetConsoleWindow(), 'pid': os.getpid()}
destination = pathlib.Path(sys.argv[1])
pending = destination.with_suffix('.pending')
pending.write_text(json.dumps(result), encoding='utf-8')
pending.replace(destination)
`
				if err := os.WriteFile(script, []byte(source), 0600); err != nil {
					t.Fatal(err)
				}
				want := []string{"two words", "中文参数", "", `quote"inside`, `trailing\`, "&literal"}
				args := []string{syscall.EscapeArg(output)}
				for _, arg := range want {
					args = append(args, syscall.EscapeArg(arg))
				}
				task := model.ScheduledTask{Path: script, Args: strings.Join(args, " ")}
				if explicit {
					task.PythonPath = `"` + customPython + `"`
				}
				if err := LaunchScheduledTask(task); err != nil {
					t.Fatal(err)
				}
				var got struct {
					Args      []string
					Directory string
					Console   uintptr
					PID       uint32
				}
				if err := json.Unmarshal(waitForFile(t, output), &got); err != nil {
					t.Fatal(err)
				}
				waitPythonExit(t, got.PID)
				if got.Console != 0 || got.Directory != dir || !reflect.DeepEqual(got.Args, want) {
					t.Fatalf("Python result = %+v; want hidden console, cwd %q, args %q", got, dir, want)
				}
			})
		}
	}
}

func waitPythonExit(t *testing.T, pid uint32) {
	t.Helper()
	if pid == 0 {
		t.Fatal("Python did not report its process ID")
	}
	handle, err := syscall.OpenProcess(0x00100000, false, pid) // SYNCHRONIZE
	if err == syscall.Errno(87) {                              // ERROR_INVALID_PARAMETER: the process has exited.
		return
	}
	if err != nil {
		t.Fatal(err)
	}
	defer syscall.CloseHandle(handle)
	status, err := syscall.WaitForSingleObject(handle, 5000)
	if err != nil || status != syscall.WAIT_OBJECT_0 {
		t.Fatal("Python did not exit before fixture cleanup", status, err)
	}
}

func TestScheduledPythonMissingInterpreter(t *testing.T) {
	dir := pythonTestDir(t)
	script := filepath.Join(dir, "task.py")
	removePythonTestFile(t, script)
	if err := os.WriteFile(script, []byte("pass\n"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, interpreter := range []string{filepath.Join(dir, "missing.exe"), dir, "invalid\x00path"} {
		if err := LaunchScheduledTask(model.ScheduledTask{Path: script, PythonPath: interpreter}); err == nil {
			t.Fatal("invalid interpreter accepted", interpreter)
		}
	}
	t.Setenv("PATH", dir)
	if err := LaunchScheduledTask(model.ScheduledTask{Path: script}); err == nil || !strings.Contains(err.Error(), "specify the Python interpreter") {
		t.Fatal("missing PATH interpreter must explain how to fix the task", err)
	}
}

func TestPythonErrorCaptureIsBounded(t *testing.T) {
	w := &scheduledStderr{}
	for _, value := range []string{strings.Repeat("x", 12000), "\nTraceback: example", strings.Repeat("y", 6000), "\nValueError: last line"} {
		if n, err := w.Write([]byte(value)); err != nil || n != len(value) || len(w.data) > 8192 {
			t.Fatal("stderr must be consumed completely and bounded", n, err, len(w.data))
		}
	}
	if !strings.HasSuffix(string(w.data), "ValueError: last line") {
		t.Fatal("stderr must retain the exception at the end")
	}
}
