package library

import (
	"errors"
	"os"
	"path/filepath"
)

func (l *Library) SetPinned(id string, pinned bool) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	items, err := l.read()
	if err != nil {
		return err
	}
	for n := range items {
		if items[n].ID == id {
			items[n].Pinned = pinned
			return l.write(items)
		}
	}
	return errors.New("资源不存在")
}

// SkillDirectory returns the imported library copy, not the original source.
func (l *Library) SkillDirectory(id string) (string, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	items, err := l.read()
	if err != nil {
		return "", err
	}
	for _, item := range items {
		if item.ID != id {
			continue
		}
		if item.Kind != "skill" {
			return "", errors.New("资源不是 Skill")
		}
		path := filepath.Join(l.Root, item.Directory)
		info, err := os.Stat(path)
		if err != nil {
			return "", err
		}
		if !info.IsDir() {
			return "", errors.New("Skill 目录不存在")
		}
		return path, nil
	}
	return "", errors.New("Skill 不存在")
}
