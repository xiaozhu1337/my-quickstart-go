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

// LaunchScheduledTask preserves the task's interpreter selection for Python scripts.
func LaunchScheduledTask(task model.ScheduledTask) error {
	entry := model.Entry{Name: task.Name, Path: task.Path, Args: task.Args}
	if task.IsPython() {
		return launchPython(entry, task.PythonPath)
	}
	return LaunchScheduled(entry)
}

func pythonInterpreter(value string) (string, error) {
	value = cleanInput(value)
	if value == "" {
		for _, name := range []string{"python", "python3"} {
			if path, err := exec.LookPath(name); err == nil {
				return filepath.Abs(path)
			}
		}
		return "", errors.New("Python was not found on PATH; select a Python interpreter in the task")
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
	// Keep only a bounded stderr tail; the application's logger rotates its files.
	stderr := &pythonStderr{}
	cmd.Stderr = stderr
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start Python script: %w", err)
	}
	go func() {
		if err := cmd.Wait(); err != nil {
			log.Printf("scheduled Python script %q: %v\n%s", script, err, strings.TrimSpace(string(stderr.data)))
		}
	}()
	return nil
}

type pythonStderr struct{ data []byte }

func (w *pythonStderr) Write(data []byte) (int, error) {
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
