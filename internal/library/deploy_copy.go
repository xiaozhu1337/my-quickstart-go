package library

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/google/uuid"
)

type deployedSkill struct {
	ID   string `yaml:"id"`
	Hash string `yaml:"hash"`
}

func directoriesOverlap(a, b string) bool {
	for _, pair := range [][2]string{{a, b}, {b, a}} {
		rel, err := filepath.Rel(pair[0], pair[1])
		if err == nil && filepath.IsLocal(rel) {
			return true
		}
	}
	return false
}

// normalizeDeployedSkill keeps library aliases in the copy only.
func normalizeDeployedSkill(root, name string) (bool, error) {
	path := filepath.Join(root, "SKILL.md")
	document, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}
	_, normalized, err := skillDocument(document, name, "")
	if err != nil || bytes.Equal(document, normalized) {
		return false, err
	}
	info, err := os.Stat(path)
	if err != nil {
		return false, err
	}
	if err = removePaths([]string{path}); err != nil {
		return false, err
	}
	return true, os.WriteFile(path, normalized, info.Mode().Perm())
}

func contentHash(data []byte) string {
	return fmt.Sprintf("%x", sha256.Sum256(data))
}

// skillInventory validates ordinary files and directories, without following links.
// The digest includes paths, permissions and contents so local edits are protected.
func skillInventory(root string) ([]string, string, error) {
	var paths []string
	var total int64
	h := sha256.New()
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if !info.IsDir() && !info.Mode().IsRegular() {
			return fmt.Errorf("Skill 包含软链接或特殊文件: %s", path)
		}
		paths = append(paths, path)
		if !info.IsDir() {
			total += info.Size()
		}
		if len(paths) > 10000 || total > 100<<20 {
			return errors.New("Skill 超过 10000 个路径或 100 MiB")
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if info.IsDir() {
			fmt.Fprintf(h, "%q:directory\n", filepath.ToSlash(rel))
			return nil
		}
		fmt.Fprintf(h, "%q:%d:%d\n", filepath.ToSlash(rel), info.Mode(), info.Size())
		f, err := os.Open(path)
		if err != nil {
			return err
		}
		n, copyErr := io.Copy(h, io.LimitReader(f, info.Size()+1))
		closeErr := f.Close()
		if copyErr != nil || closeErr != nil {
			return errors.Join(copyErr, closeErr)
		}
		if n != info.Size() {
			return errors.New("Skill 在读取期间发生变化")
		}
		return nil
	})
	return paths, fmt.Sprintf("%x", h.Sum(nil)), err
}

func copySkillDirectory(source, dest string) ([]string, error) {
	paths, _, err := skillInventory(source)
	if err != nil {
		return nil, err
	}
	var created []string
	for _, path := range paths {
		rel, err := filepath.Rel(source, path)
		if err != nil {
			return created, err
		}
		target := filepath.Join(dest, rel)
		info, err := os.Lstat(path)
		if err != nil {
			return created, err
		}
		if info.IsDir() {
			if err = os.Mkdir(target, info.Mode().Perm()|0700); err != nil {
				return created, err
			}
			created = append(created, target)
			continue
		}
		if !info.Mode().IsRegular() {
			return created, errors.New("Skill 在复制期间发生变化")
		}
		input, err := os.Open(path)
		if err != nil {
			return created, err
		}
		output, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_EXCL, info.Mode().Perm())
		if err != nil {
			return created, errors.Join(err, input.Close())
		}
		created = append(created, target)
		if err = output.Chmod(info.Mode().Perm()); err != nil {
			return created, errors.Join(err, input.Close(), output.Close())
		}
		n, copyErr := io.Copy(output, io.LimitReader(input, info.Size()+1))
		err = errors.Join(copyErr, input.Close(), output.Close())
		if err != nil {
			return created, err
		}
		if n != info.Size() {
			return created, errors.New("Skill 在复制期间发生变化")
		}
	}
	return created, nil
}

// removePaths removes only inventoried entries, children first, never recursively.
func removePaths(paths []string) error {
	var err error
	for n := len(paths) - 1; n >= 0; n-- {
		e := os.Remove(paths[n])
		// Windows refuses to unlink read-only files, including copied assets.
		if errors.Is(e, os.ErrPermission) {
			if info, statErr := os.Lstat(paths[n]); statErr == nil && info.Mode().IsRegular() && info.Mode().Perm()&0200 == 0 {
				if chmodErr := os.Chmod(paths[n], info.Mode().Perm()|0200); chmodErr == nil {
					e = os.Remove(paths[n])
				}
			}
		}
		if !errors.Is(e, os.ErrNotExist) {
			err = errors.Join(err, e)
		}
	}
	return err
}

// replace stashes the original before writing, including legacy symlinks themselves.
func (t *transaction) replace(path string, permanent bool, create func(string) ([]string, error)) (string, error) {
	var backup string
	if info, err := os.Lstat(path); err == nil {
		paths := []string{path}
		if info.IsDir() {
			paths, _, err = skillInventory(path)
			if err != nil {
				return "", err
			}
		}
		backup = path + ".quickstart-backup-" + uuid.NewString()
		if !permanent {
			if t.temp == "" {
				if err = safeParent(t.project, "__tmp"); err != nil {
					return "", err
				}
				if err = os.MkdirAll(filepath.Join(t.project, "__tmp"), 0700); err != nil {
					return "", err
				}
				t.temp, err = os.MkdirTemp(filepath.Join(t.project, "__tmp"), "quickstart-deploy-")
				if err != nil {
					return "", err
				}
			}
			backup = filepath.Join(t.temp, uuid.NewString())
		}
		if err = os.Rename(path, backup); err != nil {
			return "", err
		}
		t.undo = append(t.undo, func() error { return os.Rename(backup, path) })
		if !permanent {
			for _, old := range paths {
				rel, _ := filepath.Rel(path, old)
				t.cleanup = append(t.cleanup, filepath.Join(backup, rel))
			}
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return "", err
	}
	created, err := create(path)
	t.undo = append(t.undo, func() error { return removePaths(created) })
	if permanent {
		return backup, err
	}
	return "", err
}

func (t *transaction) replaceConfig(path string, data []byte, backup bool) (string, error) {
	return t.replace(path, backup, func(path string) ([]string, error) {
		if err := atomicWrite(path, data); err != nil {
			return nil, err
		}
		return []string{path}, nil
	})
}

func (t *transaction) finish() error {
	err := removePaths(t.cleanup)
	if t.temp != "" {
		err = errors.Join(err, os.Remove(t.temp))
	}
	return err
}
