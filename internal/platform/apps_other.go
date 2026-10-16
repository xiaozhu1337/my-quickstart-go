//go:build !windows

package platform

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

func applicationCommand(path string, args []string) (*exec.Cmd, error) {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".desktop":
		return desktopCommand(path, args)
	case ".app":
		if runtime.GOOS != "darwin" {
			return nil, errors.New("macOS application bundles require macOS")
		}
		// PlistBuddy handles both XML and binary property lists without a shell.
		output, err := exec.Command("/usr/libexec/PlistBuddy", "-c", "Print :CFBundleExecutable", filepath.Join(path, "Contents", "Info.plist")).Output()
		if err != nil {
			return nil, fmt.Errorf("read bundle executable: %w", err)
		}
		name := strings.TrimSpace(string(output))
		if name == "" || name == "." || name == ".." || filepath.Base(name) != name {
			return nil, errors.New("invalid bundle executable name")
		}
		path = filepath.Join(path, "Contents", "MacOS", name)
	}
	cmd := exec.Command(path, args...)
	cmd.Dir = filepath.Dir(path)
	return cmd, nil
}

func desktopCommand(path string, args []string) (*exec.Cmd, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	fields, err := desktopFields(string(data))
	if err != nil {
		return nil, err
	}
	if fields["Type"] != "Application" || strings.TrimSpace(fields["Exec"]) == "" {
		return nil, errors.New("desktop entry requires Type=Application and Exec")
	}
	words, err := splitArgs(fields["Exec"])
	if err != nil {
		return nil, fmt.Errorf("desktop Exec: %w", err)
	}
	words, err = desktopWords(words, fields, path)
	if err != nil {
		return nil, err
	}
	if len(words) == 0 || words[0] == "" {
		return nil, errors.New("desktop Exec has no executable")
	}
	words = append(words, args...)
	if fields["Terminal"] == "true" {
		words, err = terminalWords(words)
		if err != nil {
			return nil, err
		}
	}
	cmd := exec.Command(words[0], words[1:]...)
	cmd.Dir = fields["Path"]
	if cmd.Dir != "" && !filepath.IsAbs(cmd.Dir) {
		cmd.Dir = filepath.Join(filepath.Dir(path), cmd.Dir)
	}
	return cmd, nil
}

func desktopFields(data string) (map[string]string, error) {
	fields := map[string]string{}
	inEntry := false
	scanner := bufio.NewScanner(strings.NewReader(data))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "[") {
			inEntry = line == "[Desktop Entry]"
			continue
		}
		if !inEntry || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		switch key {
		case "Type", "Exec", "Name", "Icon", "Path", "Terminal":
			var decoded strings.Builder
			for i := 0; i < len(value); i++ {
				if value[i] != '\\' {
					decoded.WriteByte(value[i])
					continue
				}
				i++
				if i == len(value) {
					return nil, fmt.Errorf("desktop %s: trailing escape", key)
				}
				switch value[i] {
				case 's':
					decoded.WriteByte(' ')
				case 'n':
					decoded.WriteByte('\n')
				case 't':
					decoded.WriteByte('\t')
				case 'r':
					decoded.WriteByte('\r')
				case '\\':
					decoded.WriteByte('\\')
				default:
					return nil, fmt.Errorf("desktop %s: unknown escape \\%c", key, value[i])
				}
			}
			fields[key] = decoded.String()
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return fields, nil
}

func desktopWords(words []string, fields map[string]string, path string) ([]string, error) {
	var expanded []string
	for _, word := range words {
		if word == "%i" {
			if fields["Icon"] != "" {
				expanded = append(expanded, "--icon", fields["Icon"])
			}
			continue
		}
		if word == "%f" || word == "%F" || word == "%u" || word == "%U" || word == "%d" || word == "%D" || word == "%n" || word == "%N" || word == "%v" || word == "%m" {
			continue // No file/URL selection is supplied; deprecated codes are removed.
		}
		var out strings.Builder
		for i := 0; i < len(word); i++ {
			if word[i] != '%' {
				out.WriteByte(word[i])
				continue
			}
			i++
			if i == len(word) {
				return nil, errors.New("desktop Exec has trailing %")
			}
			switch word[i] {
			case '%':
				out.WriteByte('%')
			case 'c':
				out.WriteString(fields["Name"])
			case 'k':
				out.WriteString(path)
			case 'f', 'u', 'd', 'n', 'v', 'm':
			case 'F', 'U', 'i', 'D', 'N':
				return nil, fmt.Errorf("desktop Exec: %%%c must be a separate argument", word[i])
			default:
				return nil, fmt.Errorf("desktop Exec: unknown field code %%%c", word[i])
			}
		}
		expanded = append(expanded, out.String())
	}
	return expanded, nil
}

func terminalWords(words []string) ([]string, error) {
	for _, candidate := range []struct{ name, flag string }{
		{"x-terminal-emulator", "-e"}, {"xterm", "-e"}, {"gnome-terminal", "--"}, {"konsole", "-e"}, {"xfce4-terminal", "-x"},
	} {
		if path, err := exec.LookPath(candidate.name); err == nil {
			return append([]string{path, candidate.flag}, words...), nil
		}
	}
	return nil, errors.New("desktop entry requires a terminal emulator, but none was found")
}
