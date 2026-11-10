// Package library stores reusable agent resources independently of launcher settings.
package library

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"github.com/google/uuid"
	"go.yaml.in/yaml/v3"
)

type Item struct {
	ID          string  `yaml:"id"`
	Kind        string  `yaml:"kind"`
	Name        string  `yaml:"name"`
	Description string  `yaml:"description,omitempty"`
	MCP         *Server `yaml:"mcp,omitempty"`
	Directory   string  `yaml:"directory,omitempty"`
}

type Server struct {
	Command string            `yaml:"command,omitempty"`
	Args    []string          `yaml:"args,omitempty"`
	Env     map[string]string `yaml:"env,omitempty"`
	Cwd     string            `yaml:"cwd,omitempty"`
	URL     string            `yaml:"url,omitempty"`
	Headers map[string]string `yaml:"headers,omitempty"`
}

type Library struct {
	Root    string
	mu      sync.Mutex
	symlink func(string, string) error
}

var safeName = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_-]{0,63}$`)

func Open(root string) (*Library, error) {
	root, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	if err = os.MkdirAll(root, 0700); err != nil {
		return nil, err
	}
	l := &Library{Root: root, symlink: os.Symlink}
	_, err = l.Items()
	return l, err
}

func (l *Library) Items() ([]Item, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.read()
}

func (l *Library) read() ([]Item, error) {
	b, err := os.ReadFile(filepath.Join(l.Root, "library.yaml"))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var items []Item
	if err = yaml.Unmarshal(b, &items); err != nil {
		return nil, fmt.Errorf("读取资源库: %w", err)
	}
	seen := map[string]bool{}
	for _, item := range items {
		if err = validate(item); err != nil {
			return nil, err
		}
		if _, err = uuid.Parse(item.ID); err != nil {
			return nil, fmt.Errorf("无效资源 ID: %s", item.ID)
		}
		key := item.Kind + ":" + strings.ToLower(item.Name)
		if seen[item.ID] || seen[key] {
			return nil, fmt.Errorf("重复资源: %s", item.Name)
		}
		seen[item.ID], seen[key] = true, true
	}
	return items, nil
}

const libraryComment = `# Resource library. UTF-8; managed by My Quickstart Go.
# id: stable resource ID; kind: mcp or skill; name: agent-facing name.
# description: display summary; directory: imported skill directory relative to this file.
# mcp.command: executable; args: argument list; env: environment variables; cwd: absolute working directory.
# mcp.url: HTTP endpoint (exclusive with command); headers: HTTP request headers.
`

func (l *Library) write(items []Item) error {
	b, err := yaml.Marshal(items)
	if err != nil {
		return err
	}
	return atomicWrite(filepath.Join(l.Root, "library.yaml"), append([]byte(libraryComment), b...))
}

func validate(i Item) error {
	if !safeName.MatchString(i.Name) {
		return errors.New("名称须为 1–64 位字母、数字、短横线或下划线，以字母或数字开头")
	}
	if i.Kind == "skill" {
		if i.Directory == "" || filepath.IsAbs(i.Directory) || !filepath.IsLocal(i.Directory) {
			return errors.New("无效 skill 目录")
		}
		return nil
	}
	if i.Kind != "mcp" || i.MCP == nil {
		return errors.New("无效资源类型")
	}
	s := i.MCP
	if (strings.TrimSpace(s.Command) == "") == (strings.TrimSpace(s.URL) == "") {
		return errors.New("MCP 必须且只能设置 command 或 url 之一")
	}
	if s.Cwd != "" && !filepath.IsAbs(s.Cwd) {
		return errors.New("工作目录须为绝对路径")
	}
	if s.URL != "" {
		u, err := url.Parse(s.URL)
		if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
			return errors.New("MCP URL 必须为 HTTP 或 HTTPS 地址")
		}
		if len(s.Args) > 0 || len(s.Env) > 0 || s.Cwd != "" {
			return errors.New("HTTP MCP 不支持 args、env 或 cwd")
		}
	} else if len(s.Headers) > 0 {
		return errors.New("stdio MCP 不支持 headers")
	}
	return nil
}

func ParseServer(text string) (*Server, error) {
	var s Server
	d := yaml.NewDecoder(strings.NewReader(text))
	d.KnownFields(true)
	if err := d.Decode(&s); err != nil {
		return nil, fmt.Errorf("MCP YAML: %w", err)
	}
	var extra any
	if err := d.Decode(&extra); err != io.EOF {
		return nil, errors.New("MCP 配置只能包含一个 YAML 文档")
	}
	return &s, validate(Item{Kind: "mcp", Name: "server", MCP: &s})
}

func (l *Library) SaveMCP(id, name, description, config string) error {
	s, err := ParseServer(config)
	if err != nil {
		return err
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	items, err := l.read()
	if err != nil {
		return err
	}
	i := Item{ID: id, Kind: "mcp", Name: strings.TrimSpace(name), Description: description, MCP: s}
	if err = validate(i); err != nil {
		return err
	}
	index := -1
	for n, old := range items {
		if old.ID == id {
			index = n
			if old.Kind != "mcp" || old.Name != i.Name {
				return errors.New("已保存 MCP 的名称不可更改，请添加新条目")
			}
		}
		if old.ID != id && old.Kind == "mcp" && strings.EqualFold(old.Name, i.Name) {
			return errors.New("MCP 名称已存在")
		}
	}
	if id != "" && index < 0 {
		return errors.New("资源不存在")
	}
	if index < 0 {
		i.ID = uuid.NewString()
		items = append(items, i)
	} else {
		items[index] = i
	}
	return l.write(items)
}

func (l *Library) Remove(ids []string) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	items, err := l.read()
	if err != nil {
		return err
	}
	remove := map[string]bool{}
	for _, id := range ids {
		remove[id] = true
	}
	kept := make([]Item, 0, len(items))
	for _, i := range items {
		if !remove[i.ID] {
			kept = append(kept, i)
		}
	}
	// Keep imported files so existing project links remain valid.
	return l.write(kept)
}

func atomicWrite(path string, b []byte) (err error) {
	if err = os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".write-")
	if err != nil {
		return err
	}
	name := f.Name()
	defer func() {
		_ = f.Close()
		if err != nil {
			_ = os.Remove(name)
		}
	}()
	if err = f.Chmod(0600); err != nil {
		return err
	}
	if _, err = io.Copy(f, bytes.NewReader(b)); err != nil {
		return err
	}
	if err = f.Sync(); err != nil {
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	return os.Rename(name, path)
}
