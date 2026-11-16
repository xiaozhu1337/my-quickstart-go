package platform

import (
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"my-quickstart-go/internal/model"
)

// LaunchScheduledTask accepts complete commands and preserves legacy launch behavior.
func LaunchScheduledTask(task model.ScheduledTask) error {
	if task.Command != "" {
		command := strings.TrimSpace(task.Command)
		if command == "" || strings.ContainsAny(command, "\x00\r\n") {
			return errors.New("command must be a nonempty single line without NUL")
		}
		cmd := scheduledCommand(command)
		cmd.Dir = BaseDir()
		return startScheduled(cmd, "scheduled command "+task.Name)
	}
	entry := model.Entry{Name: task.Name, Path: task.Path, Args: task.Args}
	if task.IsPython() {
		return launchPython(entry, task.PythonPath)
	}
	return LaunchScheduled(entry)
}

// ScheduledCommand combines old task fields for display without changing saved tasks.
func ScheduledCommand(task model.ScheduledTask) string {
	if task.Command != "" {
		return task.Command
	}
	if strings.TrimSpace(task.Path) == "" {
		return ""
	}
	command := quoteScheduledPath(absolutePath(task.Path))
	if task.IsPython() {
		python := cleanInput(task.PythonPath)
		if python == "" {
			python = "python"
		} else if strings.ContainsAny(python, `/\:`) {
			python = absolutePath(python)
		}
		command = quoteScheduledPath(python) + " " + command
	}
	if task.Args != "" {
		command += " " + task.Args
	}
	return command
}

func pythonInterpreter(value string) (string, error) {
	value = cleanInput(value)
	if value == "" {
		for _, name := range []string{"python", "python3"} {
			if path, err := exec.LookPath(name); err == nil {
				return filepath.Abs(path)
			}
		}
		return "", errors.New("Python was not found on PATH; specify the Python interpreter in the task command")
	}
	if !strings.ContainsAny(value, `/\:`) {
		path, err := exec.LookPath(value)
		if err != nil {
			return "", fmt.Errorf("find Python interpreter: %w", err)
		}
		return filepath.Abs(path)
	}
	path := absolutePath(value)
	info, err := os.Stat(path)
	if err != nil {
		return "", fmt.Errorf("Python interpreter: %w", err)
	}
	if info.IsDir() {
		return "", errors.New("Python interpreter must be an executable file")
	}
	return path, nil
}

func launchPython(entry model.Entry, interpreter string) error {
	if strings.ContainsAny(entry.Path+entry.Args+interpreter, "\x00\r\n") {
		return errors.New("script path, interpreter and arguments must be single lines without NUL")
	}
	script := absolutePath(entry.Path)
	info, err := os.Stat(script)
	if err != nil {
		return err
	}
	if info.IsDir() {
		return errors.New("Python script must be a file")
	}
	python, err := pythonInterpreter(interpreter)
	if err != nil {
		return err
	}
	cmd, err := pythonCommand(python, script, entry.Args)
	if err != nil {
		return err
	}
	cmd.Dir = filepath.Dir(script)
	cmd.Env, err = mergeEnv(os.Environ(), entry.EnvVars)
	if err != nil {
		return err
	}
	return startScheduled(cmd, "scheduled Python script "+script)
}

func startScheduled(cmd *exec.Cmd, label string) error {
	// Keep only a bounded stderr tail; the application's logger rotates its files.
	stderr := &scheduledStderr{}
	cmd.Stderr = stderr
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("%s: %w", label, err)
	}
	go func() {
		if err := cmd.Wait(); err != nil {
			log.Printf("%s: %v\n%s", label, err, strings.TrimSpace(string(stderr.data)))
		}
	}()
	return nil
}

type scheduledStderr struct{ data []byte }

func (w *scheduledStderr) Write(data []byte) (int, error) {
	const limit = 8 * 1024
	n := len(data)
	if n >= limit {
		w.data = append(w.data[:0], data[n-limit:]...)
	} else {
		if overflow := len(w.data) + n - limit; overflow > 0 {
			w.data = w.data[overflow:]
		}
		w.data = append(w.data, data...)
	}
	return n, nil
}
