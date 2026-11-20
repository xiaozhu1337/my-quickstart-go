//go:build windows

package platform

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"unicode/utf16"
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

// Plain executable commands can receive the window preference directly. Shell
// builtins, expansions, pipelines and batch files keep cmd.exe semantics.
func scheduledCommand(command, mode string) *exec.Cmd {
	if mode != "" {
		if path, args, ok := directScheduledProgram(command); ok {
			cmd := exec.Command(path)
			cmd.SysProcAttr = &syscall.SysProcAttr{CmdLine: commandLine(path, args)}
			return cmd
		}
	}
	return commandProcess(command)
}

func directScheduledProgram(command string) (string, string, bool) {
	if strings.ContainsAny(command, "%!^") {
		return "", "", false
	}
	quoted := false
	split := len(command)
	for i, c := range command {
		if c == '"' {
			quoted = !quoted
			continue
		}
		if quoted {
			continue
		}
		if strings.ContainsRune("&|<>()", c) {
			return "", "", false
		}
		if (c == ' ' || c == '\t') && split == len(command) {
			split = i
		}
	}
	if quoted {
		return "", "", false
	}
	name := strings.Trim(command[:split], "\"")
	if name == "" || strings.ContainsRune(name, '"') {
		return "", "", false
	}
	if filepath.Ext(name) == "" {
		builtins := " assoc break call cd chdir cls color copy date del dir echo endlocal erase exit for ftype goto if md mkdir mklink move path pause popd prompt pushd rd rem ren rename rmdir set setlocal shift start time title type ver verify vol "
		if strings.Contains(builtins, " "+strings.ToLower(name)+" ") {
			return "", "", false
		}
	}
	path, err := exec.LookPath(absolutePath(name))
	if err != nil && filepath.Base(name) == name {
		path, err = exec.LookPath(name)
	}
	ext := strings.ToLower(filepath.Ext(path))
	if err != nil || (ext != ".exe" && ext != ".com") {
		return "", "", false
	}
	return path, strings.TrimSpace(command[split:]), true
}

func startTaskWindow(cmd *exec.Cmd, label, mode string) error {
	if mode == model.WindowShown {
		return startConsole(cmd)
	}
	cmd.SysProcAttr.CreationFlags = 0x08000000 // CREATE_NO_WINDOW
	cmd.SysProcAttr.HideWindow = mode == model.WindowHidden
	return startScheduled(cmd, label)
}

func quoteScheduledPath(path string) string { return `"` + path + `"` }

// startConsole lets Windows initialize real console handles. exec.Cmd instead
// supplies NUL handles when its standard streams are unset, leaving a blank window.
func startConsole(cmd *exec.Cmd) error {
	program, err := syscall.UTF16PtrFromString(cmd.Path)
	if err != nil {
		return err
	}
	line, err := syscall.UTF16PtrFromString(cmd.SysProcAttr.CmdLine)
	if err != nil {
		return err
	}
	var directory *uint16
	if cmd.Dir != "" {
		directory, err = syscall.UTF16PtrFromString(cmd.Dir)
		if err != nil {
			return err
		}
	}
	var environment *uint16
	if cmd.Env != nil {
		env := append([]string(nil), cmd.Env...)
		sort.Slice(env, func(i, j int) bool { return strings.ToUpper(env[i]) < strings.ToUpper(env[j]) })
		block := utf16.Encode([]rune(strings.Join(env, "\x00") + "\x00\x00"))
		environment = &block[0]
	}
	si := syscall.StartupInfo{Cb: uint32(unsafe.Sizeof(syscall.StartupInfo{}))}
	var pi syscall.ProcessInformation
	const createNewConsole = 0x00000010
	const createUnicodeEnvironment = 0x00000400
	if err := syscall.CreateProcess(program, line, nil, nil, false, createNewConsole|createUnicodeEnvironment, environment, directory, &si, &pi); err != nil {
		return fmt.Errorf("start console: %w", err)
	}
	syscall.CloseHandle(pi.Thread)
	syscall.CloseHandle(pi.Process)
	return nil
}

func Launch(entry model.Entry) error {
	return launch(entry, false, "")
}

func launchScheduledEntry(entry model.Entry, mode string) error {
	path := absolutePath(entry.Path)
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if info.IsDir() {
		return errors.New("scheduled program must be a file")
	}
	switch strings.ToLower(filepath.Ext(path)) {
	case ".exe", ".com", ".bat", ".cmd":
		return launch(entry, true, mode)
	case ".py", ".pyw":
		return launchPython(entry, "", mode)
	default:
		return errors.New("scheduled program must be an .exe, .com, .bat, .cmd, .py or .pyw file")
	}
}

func pythonCommand(python, script, args string) (*exec.Cmd, error) {
	cmd := exec.Command(python)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CmdLine:       commandLine(python, commandLine(script, args)),
		CreationFlags: 0x08000000, // CREATE_NO_WINDOW
	}
	return cmd, nil
}

func launch(entry model.Entry, scheduled bool, mode string) error {
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
	if scheduled {
		return startTaskWindow(cmd, "scheduled program "+path, mode)
	} else if ext == ".bat" || ext == ".cmd" {
		return startConsole(cmd)
	}
	return start(cmd)
}

func OpenFolder(path string) error {
	path = cleanInput(path)
	if path == "" || special(path) || strings.ContainsRune(path, 0) {
		return errors.New("path has no folder")
	}
	path = absolutePath(path)
	explorer := filepath.Join(os.Getenv("SystemRoot"), "explorer.exe")
	info, err := os.Stat(path)
	if err == nil && info.IsDir() {
		return start(exec.Command(explorer, path))
	}
	if err == nil {
		return start(exec.Command(explorer, "/select,", path))
	}
	if !os.IsNotExist(err) {
		return err
	}
	return start(exec.Command(explorer, filepath.Dir(path)))
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
