package library

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// UpdateSkill changes the directory reference without modifying skill files.
func (l *Library) UpdateSkill(id, source string) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	items, err := l.read()
	if err != nil {
		return err
	}
	for n := range items {
		item := &items[n]
		if item.ID != id {
			continue
		}
		if item.Kind != "skill" {
			return errors.New("资源不是 Skill")
		}
		source, err = skillSource(source)
		if err != nil {
			return err
		}
		document, err := os.ReadFile(filepath.Join(source, "SKILL.md"))
		if err != nil {
			return fmt.Errorf("请选择包含 SKILL.md 的目录: %w", err)
		}
		meta, _, err := skillDocument(document, item.Name, "")
		if err != nil {
			return err
		}
		item.Directory = source
		item.Description = meta.Description
		return l.write(items)
	}
	return errors.New("Skill 不存在")
}
