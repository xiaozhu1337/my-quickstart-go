package library

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type ImportReport struct {
	MCP, Skills, Skipped, Failed int
	Details                      []string
}

type magpieServer struct {
	Name           string            `json:"name"`
	Note           string            `json:"note"`
	Transport      string            `json:"transport"`
	Command        string            `json:"command"`
	Args           []string          `json:"args"`
	Env            map[string]string `json:"env"`
	Cwd            string            `json:"cwd"`
	URL            string            `json:"url"`
	Headers        map[string]string `json:"headers"`
	DisabledTools  []string          `json:"disabledTools"`
	StartupTimeout int               `json:"startupTimeout"`
}

type magpieSkill struct {
	Name   string `json:"name"`
	Note   string `json:"note"`
	Source *struct {
		Kind string `json:"kind"`
		Dir  string `json:"dir"`
	} `json:"source"`
}

// DefaultMagpiePath follows magpie's installed configuration convention on all OSes.
func DefaultMagpiePath() string {
	if x := os.Getenv("XDG_CONFIG_HOME"); filepath.IsAbs(x) {
		return filepath.Join(x, "magpie")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".config", "magpie")
}

func magpieFile(source string) (string, error) {
	if strings.TrimSpace(source) == "" {
		source = DefaultMagpiePath()
	}
	if source == "" {
		return "", errors.New("无法定位 magpie，请选择其数据目录")
	}
	source, err := filepath.Abs(source)
	if err != nil {
		return "", err
	}
	st, err := os.Stat(source)
	if err != nil {
		return "", fmt.Errorf("找不到 magpie 数据，请选择含 library.json 的目录: %w", err)
	}
	if !st.IsDir() {
		return source, nil
	}
	for _, path := range []string{filepath.Join(source, "library.json"), filepath.Join(source, "data", "library.json")} {
		if info, e := os.Stat(path); e == nil && info.Mode().IsRegular() {
			return path, nil
		}
	}
	return "", errors.New("所选目录及其 data 子目录中没有 library.json")
}

// ImportMagpie copies local resources only; it never changes magpie or deploys to projects.
// Entries are committed individually so a missing skill does not block other resources.
func (l *Library) ImportMagpie(source string) (ImportReport, error) {
	var report ImportReport
	path, err := magpieFile(source)
	if err != nil {
		return report, err
	}
	f, err := os.Open(path)
	if err != nil {
		return report, err
	}
	defer f.Close()
	const limit = 16 << 20
	b, err := io.ReadAll(io.LimitReader(f, limit+1))
	if err != nil {
		return report, err
	}
	if len(b) > limit {
		return report, errors.New("magpie library.json 超过 16 MiB")
	}
	var doc struct {
		MCP    []magpieServer `json:"mcp"`
		Skills []magpieSkill  `json:"skills"`
	}
	b = bytes.TrimPrefix(b, []byte{0xef, 0xbb, 0xbf})
	if err = json.Unmarshal(b, &doc); err != nil {
		return report, fmt.Errorf("解析 magpie library.json: %w", err)
	}
	var shape map[string]json.RawMessage
	if err = json.Unmarshal(b, &shape); err != nil || shape == nil || (shape["mcp"] == nil && shape["skills"] == nil) {
		return report, errors.New("文件不是 magpie 资源库，缺少 mcp / skills 字段")
	}
	if len(doc.MCP)+len(doc.Skills) > 10000 {
		return report, errors.New("magpie 资源超过 10000 项")
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	items, err := l.read()
	if err != nil {
		return report, err
	}
	seen := map[string]bool{}
	for _, item := range items {
		seen[item.Kind+":"+strings.ToLower(item.Name)] = true
	}
	duplicate := func(kind, name string) bool {
		if !seen[kind+":"+strings.ToLower(name)] {
			return false
		}
		report.Skipped++
		report.Details = append(report.Details, kind+" "+name+": 已有同名资源，跳过")
		return true
	}
	failed := func(kind, name string, e error) {
		report.Failed++
		report.Details = append(report.Details, kind+" "+name+": "+e.Error())
	}
	for _, s := range doc.MCP {
		if duplicate("mcp", s.Name) {
			continue
		}
		if s.Transport != "" && s.Transport != "stdio" && s.Transport != "http" {
			failed("mcp", s.Name, fmt.Errorf("暂不支持传输类型 %s", s.Transport))
			continue
		}
		if len(s.DisabledTools) > 0 || s.StartupTimeout != 0 {
			failed("mcp", s.Name, errors.New("含禁用工具或启动超时设置，当前不能完整导入"))
			continue
		}
		server := &Server{Command: s.Command, Args: s.Args, Env: s.Env, Cwd: s.Cwd, URL: s.URL, Headers: s.Headers}
		if e := l.saveMCP("", s.Name, s.Note, server); e != nil {
			failed("mcp", s.Name, e)
			continue
		}
		seen["mcp:"+strings.ToLower(s.Name)] = true
		report.MCP++
	}
	for _, s := range doc.Skills {
		if duplicate("skill", s.Name) {
			continue
		}
		if !safeName.MatchString(s.Name) {
			failed("skill", s.Name, errors.New("无效 Skill 名称"))
			continue
		}
		dir := filepath.Join(filepath.Dir(path), "library", "skills", s.Name)
		if _, e := os.Stat(filepath.Join(dir, "SKILL.md")); errors.Is(e, os.ErrNotExist) && s.Source != nil && s.Source.Kind == "folder" && filepath.IsAbs(s.Source.Dir) {
			dir = s.Source.Dir
		}
		if e := l.importSkill(dir, s.Name, s.Note); e != nil {
			failed("skill", s.Name, e)
			continue
		}
		seen["skill:"+strings.ToLower(s.Name)] = true
		report.Skills++
	}
	return report, nil
}
