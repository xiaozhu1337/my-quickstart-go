//go:build windows

package platform

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"

	"my-quickstart-go/internal/model"
)

func TestScheduledCommandHasNoConsole(t *testing.T) {
	python, err := exec.LookPath("python")
	if err != nil {
		t.Skip("run with mise exec go@1.27.3 python@3.15.2 -- go test to exercise Python", err)
	}
	dir := pythonTestDir(t)
	originalExecutable := executable
	executable = func() (string, error) { return filepath.Join(dir, "launcher.exe"), nil }
	t.Cleanup(func() { executable = originalExecutable })
	linkedHome := filepath.Join(dir, "Python 环境")
	customPython := python
	if err := os.Symlink(filepath.Dir(python), linkedHome); err == nil {
		removePythonTestFile(t, linkedHome)
		customPython = filepath.Join(linkedHome, filepath.Base(python))
	}
	for _, mode := range []string{"PATH", "explicit", "batch", "working-directory"} {
		t.Run(mode, func(t *testing.T) {
			script := filepath.Join(dir, "每日 report.py")
			output := filepath.Join(dir, "result.json")
			removePythonTestFile(t, script)
			removePythonTestFile(t, output)
			removePythonTestFile(t, filepath.Join(dir, "result.pending"))
			source := `import ctypes, json, os, pathlib, sys
result = {'args': sys.argv[2:], 'directory': os.getcwd(), 'console': ctypes.windll.kernel32.GetConsoleWindow(), 'pid': os.getpid(), 'parent': os.getppid()}
destination = pathlib.Path(sys.argv[1])
pending = destination.with_suffix('.pending')
pending.write_text(json.dumps(result), encoding='utf-8')
pending.replace(destination)
`
			if err := os.WriteFile(script, []byte(source), 0600); err != nil {
				t.Fatal(err)
			}
			program := "python"
			if mode == "explicit" {
				program = quoteScheduledPath(customPython)
			}
			command := program + " " + quoteScheduledPath(script) + " " + quoteScheduledPath(output) + ` "two words" 中文参数 "" "&literal"`
			workingDirectory := dir
			if mode == "working-directory" {
				workingDirectory = filepath.Join(dir, "work folder")
				if err := os.Mkdir(workingDirectory, 0700); err != nil {
					t.Fatal(err)
				}
				removePythonTestFile(t, workingDirectory)
				command = "cd /d " + quoteScheduledPath(workingDirectory) + " && " + command
			}
			if mode == "batch" {
				batch := filepath.Join(dir, "daily task.cmd")
				removePythonTestFile(t, batch)
				// Select UTF-8 before cmd reads the command containing Chinese paths and arguments.
				batchCommand := program + ` "每日 report.py" result.json "two words" 中文参数 "" "&literal"`
				if err := os.WriteFile(batch, []byte("@echo off\r\nchcp 65001 >nul\r\n"+batchCommand+"\r\n"), 0600); err != nil {
					t.Fatal(err)
				}
				command = quoteScheduledPath(batch)
			}
			if err := LaunchScheduledTask(model.ScheduledTask{Name: mode, Command: command}); err != nil {
				t.Fatal(err)
			}
			var got struct {
				Args        []string
				Directory   string
				Console     uintptr
				PID, Parent uint32
			}
			if err := json.Unmarshal(waitForFile(t, output), &got); err != nil {
				t.Fatal(err)
			}
			waitPythonExit(t, got.PID)
			waitPythonExit(t, got.Parent)
			if got.Console != 0 || got.Directory != workingDirectory || !reflect.DeepEqual(got.Args, []string{"two words", "中文参数", "", "&literal"}) {
				t.Fatalf("unexpected command result: %+v", got)
			}
		})
	}
}

func TestScheduledCommandValidation(t *testing.T) {
	for _, value := range []string{" ", "python\x00test.py", "echo a\necho b", "echo a\recho b"} {
		if err := LaunchScheduledTask(model.ScheduledTask{Command: value}); err == nil {
			t.Fatal("invalid command accepted", value)
		}
	}
}
