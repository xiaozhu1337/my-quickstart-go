package library

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
	"go.yaml.in/yaml/v3"
)

func (l *Library) ImportSkill(source string) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.importSkill(source, "", "")
}

func (l *Library) importSkill(source, name, note string) error {
	source, err := filepath.Abs(strings.TrimSpace(source))
	if err != nil {
		return err
	}
	info, err := os.Stat(source)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		source = filepath.Dir(source)
	}
	source, err = filepath.EvalSymlinks(source)
	if err != nil {
		return err
	}
	b, err := os.ReadFile(filepath.Join(source, "SKILL.md"))
	if err != nil {
		return fmt.Errorf("请选择包含 SKILL.md 的目录: %w", err)
	}
	text := strings.ReplaceAll(string(b), "\r\n", "\n")
	if !strings.HasPrefix(text, "---\n") {
		return errors.New("SKILL.md 缺少 YAML frontmatter")
	}
	end := strings.Index(text[4:], "\n---")
	if end < 0 {
		return errors.New("SKILL.md frontmatter 未闭合")
	}
	var meta struct {
		Name        string `yaml:"name"`
		Description string `yaml:"description"`
	}
	if err = yaml.Unmarshal([]byte(text[4:4+end]), &meta); err != nil {
		return err
	}
	if strings.TrimSpace(meta.Description) == "" {
		return errors.New("SKILL.md 须包含有效的 description")
	}
	if name != "" {
		// Magpie can keep a skill under a different library name.
		var front map[string]any
		if err = yaml.Unmarshal([]byte(text[4:4+end]), &front); err != nil {
			return err
		}
		front["name"] = name
		raw, e := yaml.Marshal(front)
		if e != nil {
			return e
		}
		b = []byte("---\n" + string(raw) + text[4+end+1:])
		meta.Name = name
	}
	if note != "" {
		meta.Description = strings.TrimSpace(meta.Description + "\n" + note)
	}
	if !safeName.MatchString(meta.Name) || strings.TrimSpace(meta.Description) == "" {
		return errors.New("SKILL.md 须包含有效的 name 和 description")
	}
	items, err := l.read()
	if err != nil {
		return err
	}
	for _, i := range items {
		if i.Kind == "skill" && strings.EqualFold(i.Name, meta.Name) {
			return errors.New("Skill 名称已存在")
		}
	}
	id := uuid.NewString()
	rel := filepath.Join("skills", id, meta.Name)
	dest := filepath.Join(l.Root, rel)
	var paths []string
	var total int64
	err = filepath.WalkDir(source, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("导入目录包含软链接，请先展开: %s", path)
		}
		if !entry.IsDir() {
			info, e := entry.Info()
			if e != nil {
				return e
			}
			if !info.Mode().IsRegular() {
				return fmt.Errorf("不支持的 skill 文件: %s", path)
			}
			total += info.Size()
		}
		paths = append(paths, path)
		if len(paths) > 10000 || total > 100<<20 {
			return errors.New("Skill 超过 10000 个文件或 100 MiB")
		}
		return nil
	})
	if err != nil {
		return err
	}
	var created []string
	rollback := func() {
		for n := len(created) - 1; n >= 0; n-- {
			_ = os.Remove(created[n])
		}
	}
	for _, path := range paths {
		r, e := filepath.Rel(source, path)
		if e != nil {
			rollback()
			return e
		}
		target := filepath.Join(dest, r)
		info, e := os.Lstat(path)
		if e != nil {
			rollback()
			return e
		}
		if info.Mode()&os.ModeSymlink != 0 {
			rollback()
			return errors.New("Skill 在导入期间发生变化")
		}
		if info.IsDir() {
			e = os.MkdirAll(target, 0700)
		} else {
			var raw []byte
			raw, e = os.ReadFile(path)
			if r == "SKILL.md" && name != "" {
				raw = b
			}
			if e == nil {
				e = os.WriteFile(target, raw, info.Mode().Perm())
			}
		}
		if e != nil {
			rollback()
			return e
		}
		created = append(created, target)
	}
	items = append(items, Item{ID: id, Kind: "skill", Name: meta.Name, Description: meta.Description, Directory: rel})
	if err = l.write(items); err != nil {
		rollback()
	}
	return err
}
