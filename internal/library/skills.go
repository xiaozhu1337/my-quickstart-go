package library

import (
	"errors"
	"fmt"
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

func skillSource(source string) (string, error) {
	if strings.TrimSpace(source) == "" {
		return "", errors.New("请选择包含 SKILL.md 的目录")
	}
	source, err := filepath.Abs(strings.TrimSpace(source))
	if err != nil {
		return "", err
	}
	info, err := os.Stat(source)
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		source = filepath.Dir(source)
	}
	return filepath.EvalSymlinks(source)
}

type skillMetadata struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
}

func skillDocument(b []byte, name, note string) (skillMetadata, []byte, error) {
	var meta skillMetadata
	text := strings.ReplaceAll(string(b), "\r\n", "\n")
	if !strings.HasPrefix(text, "---\n") {
		return meta, nil, errors.New("SKILL.md 缺少 YAML frontmatter")
	}
	end := strings.Index(text[4:], "\n---")
	if end < 0 {
		return meta, nil, errors.New("SKILL.md frontmatter 未闭合")
	}
	if err := yaml.Unmarshal([]byte(text[4:4+end]), &meta); err != nil {
		return meta, nil, err
	}
	if strings.TrimSpace(meta.Description) == "" {
		return meta, nil, errors.New("SKILL.md 须包含有效的 description")
	}
	if name != "" && name != meta.Name {
		// Updates and Magpie imports retain the resource's library name.
		var front map[string]any
		if err := yaml.Unmarshal([]byte(text[4:4+end]), &front); err != nil {
			return meta, nil, err
		}
		front["name"] = name
		raw, e := yaml.Marshal(front)
		if e != nil {
			return meta, nil, e
		}
		b = []byte("---\n" + string(raw) + text[4+end+1:])
		meta.Name = name
	}
	if note != "" {
		meta.Description = strings.TrimSpace(meta.Description + "\n" + note)
	}
	if !safeName.MatchString(meta.Name) || strings.TrimSpace(meta.Description) == "" {
		return meta, nil, errors.New("SKILL.md 须包含有效的 name 和 description")
	}
	return meta, b, nil
}

func (l *Library) importSkill(source, name, note string) error {
	source, err := skillSource(source)
	if err != nil {
		return err
	}
	b, err := os.ReadFile(filepath.Join(source, "SKILL.md"))
	if err != nil {
		return fmt.Errorf("请选择包含 SKILL.md 的目录: %w", err)
	}
	meta, _, err := skillDocument(b, name, note)
	if err != nil {
		return err
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
	items = append(items, Item{ID: uuid.NewString(), Kind: "skill", Name: meta.Name, Description: meta.Description, Directory: source})
	return l.write(items)
}
