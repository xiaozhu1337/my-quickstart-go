//go:build windows

package platform

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"unsafe"

	"github.com/egoist/mygo"
	"my-quickstart-go/internal/model"
)

var shellExecute = syscall.NewLazyDLL("shell32.dll").NewProc("ShellExecuteW")

func systemProgram(name string) string {
	return filepath.Join(os.Getenv("SystemRoot"), "System32", name)
}

func commandLine(program, args string) string {
	line := syscall.EscapeArg(program)
	if args != "" {
		line += " " + args
	}
	return line
}

func start(cmd *exec.Cmd) error {
	if err := cmd.Start(); err != nil {
		return err
	}
	go func() { _ = cmd.Wait() }()
	return nil
}

func shellOpen(path string) error {
	wide, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return err
	}
	result, _, _ := shellExecute.Call(0, 0, uintptr(unsafe.Pointer(wide)), 0, 0, 1)
	if result <= 32 {
		return fmt.Errorf("ShellExecute(%q): error %d", path, result)
	}
	return nil
}

func commandProcess(command string) *exec.Cmd {
	cmd := exec.Command(systemProgram("cmd.exe"))
	cmd.SysProcAttr = &syscall.SysProcAttr{CmdLine: commandLine(cmd.Path, `/D /S /C "`+command+`"`)}
	return cmd
}

func Launch(entry model.Entry) error {
	path := cleanInput(entry.Path)
	if path == "" || strings.ContainsRune(path, 0) || strings.ContainsRune(entry.Args, 0) {
		return errors.New("invalid launch path or arguments")
	}
	lower := strings.ToLower(path)
	if strings.HasPrefix(lower, "command:") {
		command := strings.TrimSpace(path[len("command:"):])
		if command == "" {
			return errors.New("command is empty")
		}
		// Command entries inherit the launcher's environment, as in the old UI.
		return start(commandProcess(command))
	}
	if strings.HasPrefix(lower, "uwp:") {
		target, err := uwpShellPath(path)
		if err != nil {
			return err
		}
		return shellOpen(target)
	}
	path = absolutePath(path)
	ext := strings.ToLower(filepath.Ext(path))
	info, statErr := os.Stat(path)
	executable := ext == ".exe" || ext == ".com" || ext == ".bat" || ext == ".cmd"
	if entry.Args == "" && len(entry.EnvVars) == 0 && (!executable || statErr == nil && info.IsDir() || special(path)) {
		return shellOpen(path)
	}
	env, err := mergeEnv(os.Environ(), entry.EnvVars)
	if err != nil {
		return err
	}
	cmd := exec.Command(path)
	cmd.SysProcAttr = &syscall.SysProcAttr{CmdLine: commandLine(path, entry.Args)}
	if ext == ".bat" || ext == ".cmd" {
		cmd = commandProcess(commandLine(path, entry.Args))
	}
	if parent := filepath.Dir(path); parent != "." {
		if info, err := os.Stat(parent); err == nil && info.IsDir() {
			cmd.Dir = parent
		}
	}
	cmd.Env = env
	return start(cmd)
}

func OpenFolder(path string) error {
	path = cleanInput(path)
	if path == "" || special(path) || strings.ContainsRune(path, 0) {
		return errors.New("path has no folder")
	}
	path = absolutePath(path)
	info, err := os.Stat(path)
	if err == nil && info.IsDir() {
		return start(exec.Command(systemProgram("explorer.exe"), path))
	}
	if err == nil {
		return start(exec.Command(systemProgram("explorer.exe"), "/select,", path))
	}
	if !os.IsNotExist(err) {
		return err
	}
	return start(exec.Command(systemProgram("explorer.exe"), filepath.Dir(path)))
}

// InstalledApps requires a running MyGo application and mirrors the old Get-StartApps picker.
func InstalledApps() ([]model.Entry, error) {
	apps, err := mygo.Shell.InstalledApps()
	if err != nil {
		return nil, err
	}
	entries := make([]model.Entry, 0, len(apps))
	for _, app := range apps {
		if strings.TrimSpace(app.ID) != "" && strings.TrimSpace(app.Name) != "" {
			entries = append(entries, model.NewEntry(app.Name, "uwp:"+app.ID, ""))
		}
	}
	return entries, nil
}

func fileIcon(path string, size int) ([]byte, error) {
	return mygo.Shell.FileIcon(path, size)
}
