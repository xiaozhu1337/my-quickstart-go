//go:build !windows

package platform

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/egoist/mygo"
	"my-quickstart-go/internal/model"
)

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
		return start(exec.Command("/bin/sh", "-c", command))
	}
	if strings.HasPrefix(lower, "uwp:") || strings.HasPrefix(lower, "shell:") {
		return errors.New("Windows shell applications are only supported on Windows")
	}
	if special(path) {
		return mygo.Shell.OpenExternal(path)
	}
	path = absolutePath(path)
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if entry.Args == "" && len(entry.EnvVars) == 0 {
		ext := strings.ToLower(filepath.Ext(path))
		if ext == ".app" || ext == ".desktop" {
			return mygo.Shell.LaunchApp(mygo.InstalledApp{Path: path})
		}
		if info.IsDir() || info.Mode()&0111 == 0 {
			return mygo.Shell.OpenPath(path)
		}
	}
	args, err := splitArgs(entry.Args)
	if err != nil {
		return err
	}
	env, err := mergeEnv(os.Environ(), entry.EnvVars)
	if err != nil {
		return err
	}
	cmd, err := applicationCommand(path, args)
	if err != nil {
		return err
	}
	cmd.Env = env
	return start(cmd)
}

func start(cmd *exec.Cmd) error {
	if err := cmd.Start(); err != nil {
		return err
	}
	go func() { _ = cmd.Wait() }()
	return nil
}

func OpenFolder(path string) error {
	path = cleanInput(path)
	if path == "" || special(path) || strings.ContainsRune(path, 0) {
		return errors.New("path has no folder")
	}
	path = absolutePath(path)
	info, err := os.Stat(path)
	if err == nil && info.IsDir() {
		return mygo.Shell.OpenPath(path)
	}
	if err == nil {
		mygo.Shell.ShowItemInFolder(path)
		return nil
	}
	if !os.IsNotExist(err) {
		return err
	}
	return mygo.Shell.OpenPath(filepath.Dir(path))
}

// InstalledApps requires a running MyGo application.
func InstalledApps() ([]model.Entry, error) {
	apps, err := mygo.Shell.InstalledApps()
	if err != nil {
		return nil, err
	}
	entries := make([]model.Entry, 0, len(apps))
	for _, app := range apps {
		if strings.TrimSpace(app.Name) != "" && app.Path != "" {
			entries = append(entries, model.NewEntry(app.Name, app.Path, ""))
		}
	}
	return entries, nil
}

func resolveShortcut(path string) (string, string, error) { return path, "", nil }
func fileIcon(path string, size int) ([]byte, error)      { return mygo.Shell.FileIcon(path, size) }

// splitArgs uses shell quoting without expanding variables or executing shell operators.
func splitArgs(raw string) ([]string, error) {
	var args []string
	var word strings.Builder
	var quote byte
	started := false
	for i := 0; i < len(raw); i++ {
		ch := raw[i]
		if ch == '\\' && quote != '\'' {
			if i+1 == len(raw) {
				return nil, errors.New("trailing escape in arguments")
			}
			next := raw[i+1]
			if quote != '"' || strings.ContainsRune("\"\\$`\n", rune(next)) {
				i++
				if next != '\n' {
					word.WriteByte(next)
					started = true
				}
				continue
			}
		}
		if quote != 0 {
			if ch == quote {
				quote = 0
			} else {
				word.WriteByte(ch)
			}
		} else if ch == '\'' || ch == '"' {
			quote = ch
			started = true
		} else if strings.ContainsRune(" \t\r\n", rune(ch)) {
			if started {
				args = append(args, word.String())
				word.Reset()
				started = false
			}
		} else {
			word.WriteByte(ch)
			started = true
		}
	}
	if quote != 0 {
		return nil, errors.New("unclosed quote in arguments")
	}
	if started {
		args = append(args, word.String())
	}
	return args, nil
}
