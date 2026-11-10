package library

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// UpdateSkill replaces the saved files while preserving identity and pin state.
func (l *Library) UpdateSkill(id, source string) (err error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	items, err := l.read()
	if err != nil {
		return err
	}
	index := -1
	for n, item := range items {
		if item.ID == id {
			if item.Kind != "skill" {
				return errors.New("资源不是 Skill")
			}
			index = n
			break
		}
	}
	if index < 0 {
		return errors.New("Skill 不存在")
	}
	source, err = skillSource(source)
	if err != nil {
		return err
	}
	item := &items[index]
	if err = safeParent(l.Root, item.Directory); err != nil {
		return err
	}
	if err = safeParent(l.Root, "__tmp"); err != nil {
		return err
	}
	tempRoot := filepath.Join(l.Root, "__tmp")
	if err = os.MkdirAll(tempRoot, 0700); err != nil {
		return err
	}
	stage, err := os.MkdirTemp(tempRoot, "skill-update-")
	if err != nil {
		return err
	}
	tx := &transaction{project: l.Root, temp: stage}
	staged := filepath.Join(stage, "content")
	created, err := l.copySkill(source, staged)
	// Staging first also supports updating from the saved directory itself.
	tx.undo = append(tx.undo, func() error { return removePaths(created) })
	if err != nil {
		return tx.rollback(err)
	}
	document, err := os.ReadFile(filepath.Join(staged, "SKILL.md"))
	if err != nil {
		return tx.rollback(fmt.Errorf("请选择包含 SKILL.md 的目录: %w", err))
	}
	meta, normalized, err := skillDocument(document, item.Name, "")
	if err != nil {
		return tx.rollback(err)
	}
	if !bytes.Equal(document, normalized) {
		path := filepath.Join(staged, "SKILL.md")
		info, e := os.Stat(path)
		if e != nil {
			return tx.rollback(e)
		}
		if err = removePaths([]string{path}); err != nil {
			return tx.rollback(err)
		}
		if err = os.WriteFile(path, normalized, info.Mode().Perm()); err != nil {
			return tx.rollback(err)
		}
	}
	dest := filepath.Join(l.Root, item.Directory)
	_, err = tx.replace(dest, false, func(path string) ([]string, error) {
		if e := os.Rename(staged, path); e != nil {
			return nil, e
		}
		moved := make([]string, 0, len(created))
		for _, old := range created {
			rel, _ := filepath.Rel(staged, old)
			moved = append(moved, filepath.Join(path, rel))
		}
		return moved, nil
	})
	if err != nil {
		return tx.rollback(err)
	}
	item.Description = meta.Description
	if err = l.write(items); err != nil {
		return tx.rollback(err)
	}
	if err = tx.finish(); err != nil {
		return fmt.Errorf("Skill 已更新，但临时备份清理失败: %w", err)
	}
	return nil
}
