package library

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"go.yaml.in/yaml/v3"
)

const maxSkillDocument = 2 << 20

func (l *Library) skillPath(items []Item, id string) (int, string, error) {
	for n, item := range items {
		if item.ID != id {
			continue
		}
		if item.Kind != "skill" {
			return 0, "", errors.New("资源不是 Skill")
		}
		path := filepath.Join(l.Root, item.Directory, "SKILL.md")
		resolved, err := filepath.EvalSymlinks(path)
		if err != nil {
			return 0, "", err
		}
		root, err := filepath.EvalSymlinks(l.Root)
		if err != nil {
			return 0, "", err
		}
		if !samePath(resolved, filepath.Join(root, item.Directory, "SKILL.md")) {
			return 0, "", errors.New("Skill 文件路径已被软链接重定向，无法编辑")
		}
		info, err := os.Stat(path)
		if err != nil {
			return 0, "", err
		}
		if !info.Mode().IsRegular() || info.Size() > maxSkillDocument {
			return 0, "", errors.New("编辑器仅支持不超过 2 MiB 的 SKILL.md 文件")
		}
		return n, path, nil
	}
	return 0, "", errors.New("Skill 不存在")
}

// SkillDocument reads the library copy rather than the original import source.
func (l *Library) SkillDocument(id string) (string, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	items, err := l.read()
	if err != nil {
		return "", err
	}
	_, path, err := l.skillPath(items, id)
	if err != nil {
		return "", err
	}
	b, err := os.ReadFile(path)
	return string(b), err
}

// SaveSkill preserves the directory and supporting files so existing links work.
func (l *Library) SaveSkill(id, document string) error {
	if len(document) > maxSkillDocument {
		return errors.New("SKILL.md 不能超过 2 MiB")
	}
	text := strings.ReplaceAll(document, "\r\n", "\n")
	lines := strings.Split(text, "\n")
	if len(lines) < 3 || lines[0] != "---" {
		return errors.New("SKILL.md 缺少 YAML frontmatter")
	}
	end := 1
	for end < len(lines) && lines[end] != "---" {
		end++
	}
	if end == len(lines) {
		return errors.New("SKILL.md frontmatter 未闭合")
	}
	var meta struct {
		Name        string `yaml:"name"`
		Description string `yaml:"description"`
	}
	if err := yaml.Unmarshal([]byte(strings.Join(lines[1:end], "\n")), &meta); err != nil {
		return fmt.Errorf("SKILL.md YAML: %w", err)
	}
	if !safeName.MatchString(meta.Name) || strings.TrimSpace(meta.Description) == "" {
		return errors.New("SKILL.md 须包含有效的 name 和 description")
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	items, err := l.read()
	if err != nil {
		return err
	}
	index, path, err := l.skillPath(items, id)
	if err != nil {
		return err
	}
	for _, item := range items {
		if item.ID != id && item.Kind == "skill" && strings.EqualFold(item.Name, meta.Name) {
			return errors.New("Skill 名称已存在")
		}
	}
	old, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if err = atomicWrite(path, []byte(text)); err != nil {
		return err
	}
	items[index].Name, items[index].Description = meta.Name, meta.Description
	if err = l.write(items); err != nil {
		if rollbackErr := atomicWrite(path, old); rollbackErr != nil {
			return errors.Join(err, fmt.Errorf("恢复 SKILL.md 失败: %w", rollbackErr))
		}
	}
	return err
}
