package library

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"

	"github.com/pelletier/go-toml/v2"
	"go.yaml.in/yaml/v3"
)

type Deployment struct {
	Project    string                   `yaml:"project"`
	Agent      string                   `yaml:"agent"`
	Managed    map[string]string        `yaml:"managed"`
	ConfigHash string                   `yaml:"config_hash,omitempty"`
	Skills     map[string]deployedSkill `yaml:"skills,omitempty"`
}

type Result struct {
	MCP, Skills int
	Backup      string
}

type transaction struct {
	undo    []func() error
	project string
	temp    string
	cleanup []string
}

func (t *transaction) rollback(err error) error {
	for n := len(t.undo) - 1; n >= 0; n-- {
		err = errors.Join(err, t.undo[n]())
	}
	if t.temp != "" {
		err = errors.Join(err, os.Remove(t.temp))
	}
	return err
}

func (t *transaction) write(path string, data []byte) error {
	old, err := os.ReadFile(path)
	existed := err == nil
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err = atomicWrite(path, data); err != nil {
		return err
	}
	t.undo = append(t.undo, func() error {
		if existed {
			return atomicWrite(path, old)
		}
		return os.Remove(path)
	})
	return nil
}

func samePath(a, b string) bool {
	a, b = filepath.Clean(a), filepath.Clean(b)
	if runtime.GOOS == "windows" {
		return strings.EqualFold(a, b)
	}
	return a == b
}

func linkedTo(path, target string) bool {
	link, err := os.Readlink(path)
	if err != nil {
		return false
	}
	if !filepath.IsAbs(link) {
		link = filepath.Join(filepath.Dir(path), link)
	}
	return samePath(link, target)
}

// safeParent refuses redirected project configuration directories.
func safeParent(project, relative string) error {
	path := project
	for _, part := range strings.Split(filepath.Clean(relative), string(filepath.Separator)) {
		path = filepath.Join(path, part)
		info, err := os.Lstat(path)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return err
		}
		if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("项目配置目录不是普通目录: %s", path)
		}
		resolved, err := filepath.EvalSymlinks(path)
		if err != nil {
			return err
		}
		if !samePath(path, resolved) {
			return fmt.Errorf("项目配置目录被重定向: %s", path)
		}
	}
	return nil
}

func (l *Library) Deploy(project, agent string, ids []string) (Result, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	var result Result
	if agent != "codex" && agent != "pi" {
		return result, errors.New("仅支持 codex 和 pi")
	}
	if strings.TrimSpace(project) == "" {
		return result, errors.New("请选择项目目录")
	}
	project, err := filepath.Abs(project)
	if err != nil {
		return result, err
	}
	project, err = filepath.EvalSymlinks(project)
	if err != nil {
		return result, err
	}
	info, err := os.Stat(project)
	if err != nil {
		return result, err
	}
	if !info.IsDir() {
		return result, errors.New("项目路径不是目录")
	}
	items, err := l.read()
	if err != nil {
		return result, err
	}
	wanted := map[string]bool{}
	for _, id := range ids {
		wanted[id] = true
	}
	var selected []Item
	for _, i := range items {
		if wanted[i.ID] {
			selected = append(selected, i)
			delete(wanted, i.ID)
		}
	}
	if len(wanted) > 0 || len(selected) == 0 {
		return result, errors.New("请选择有效的资源")
	}
	key := project
	if runtime.GOOS == "windows" {
		key = strings.ToLower(key)
	}
	hash := sha256.Sum256([]byte(key + "\x00" + agent))
	profile := filepath.Join(l.Root, "projects", fmt.Sprintf("%x", hash[:16]))
	configRel, skillRel, filename := filepath.Join(".codex", "config.toml"), filepath.Join(".agents", "skills"), "config.toml"
	if agent == "pi" {
		configRel, skillRel, filename = filepath.Join(".pi", "mcp.json"), filepath.Join(".pi", "skills"), "mcp.json"
	}
	config, generated := filepath.Join(project, configRel), filepath.Join(profile, filename)
	manifestPath := filepath.Join(profile, "deployment.yaml")
	manifest := Deployment{Project: project, Agent: agent, Managed: map[string]string{}}
	if b, e := os.ReadFile(manifestPath); e == nil {
		if e = yaml.Unmarshal(b, &manifest); e != nil {
			return result, e
		}
		if !samePath(manifest.Project, project) || manifest.Agent != agent {
			return result, errors.New("项目资源记录不匹配")
		}
		if manifest.Managed == nil {
			manifest.Managed = map[string]string{}
		}
	} else if !errors.Is(e, os.ErrNotExist) {
		return result, e
	}
	if manifest.Skills == nil {
		manifest.Skills = map[string]deployedSkill{}
	}
	var skills []Item
	for _, i := range selected {
		if i.Kind == "skill" {
			skills = append(skills, i)
			result.Skills++
		} else {
			result.MCP++
		}
	}
	skillHashes := map[string]string{}
	skillSources := map[string]string{}
	if len(skills) > 0 {
		if err = safeParent(project, skillRel); err != nil {
			return result, err
		}
		for _, i := range skills {
			target := l.skillPath(i)
			if !filepath.IsAbs(i.Directory) {
				if err = safeParent(l.Root, i.Directory); err != nil {
					return result, err
				}
			}
			target, err = filepath.EvalSymlinks(target)
			if err != nil {
				return result, err
			}
			skillSources[i.ID] = target
			for _, other := range skills {
				dest := filepath.Join(project, skillRel, other.Name)
				if directoriesOverlap(target, dest) {
					return result, fmt.Errorf("Skill 源目录与目标目录重叠，未复制: %s", dest)
				}
			}
			if _, err = os.Stat(filepath.Join(target, "SKILL.md")); err != nil {
				return result, err
			}
			_, digest, e := skillInventory(target)
			if e != nil {
				return result, e
			}
			skillHashes[i.ID] = digest
			path := filepath.Join(project, skillRel, i.Name)
			if info, e := os.Lstat(path); e == nil && !linkedTo(path, target) {
				owned, ok := manifest.Skills[i.Name]
				if !info.IsDir() || !ok || owned.ID != i.ID {
					return result, fmt.Errorf("Skill 同名冲突，未覆盖: %s", path)
				}
				_, digest, e := skillInventory(path)
				if e != nil {
					return result, e
				}
				if digest != owned.Hash {
					return result, fmt.Errorf("项目 Skill 已修改，未覆盖: %s", path)
				}
			} else if e != nil && !errors.Is(e, os.ErrNotExist) {
				return result, e
			}
		}
	}
	var rendered []byte
	var regularConfig bool
	if result.MCP > 0 {
		if err = safeParent(project, filepath.Dir(configRel)); err != nil {
			return result, err
		}
		if st, e := os.Lstat(config); e == nil {
			if st.Mode()&os.ModeSymlink != 0 {
				if !linkedTo(config, generated) {
					return result, errors.New("项目 MCP 配置已链接到其他位置，未覆盖")
				}
			} else if !st.Mode().IsRegular() {
				return result, errors.New("项目 MCP 配置不是普通文件")
			} else {
				regularConfig = true
			}
		} else if !errors.Is(e, os.ErrNotExist) {
			return result, e
		}
		var data []byte
		data, err = os.ReadFile(config)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return result, err
		}
		owned := linkedTo(config, generated) || (manifest.ConfigHash != "" && manifest.ConfigHash == contentHash(data))
		if !owned {
			manifest.Managed = map[string]string{}
		}
		regularConfig = regularConfig && !owned
		rendered, err = mergeConfig(data, agent, selected, &manifest)
		if err != nil {
			return result, err
		}
	}
	tx := &transaction{project: project}
	fail := func(e error) (Result, error) { return Result{}, tx.rollback(e) }
	if result.MCP > 0 {
		result.Backup, err = tx.replaceConfig(config, rendered, regularConfig)
		if err != nil {
			return fail(err)
		}
		manifest.ConfigHash = contentHash(rendered)
	}
	for _, i := range skills {
		path := filepath.Join(project, skillRel, i.Name)
		_, err = tx.replace(path, false, func(dest string) ([]string, error) {
			return l.copySkill(skillSources[i.ID], dest)
		})
		if err != nil {
			return fail(err)
		}
		_, digest, e := skillInventory(path)
		if e != nil {
			return fail(e)
		}
		if digest != skillHashes[i.ID] {
			return fail(errors.New("Skill 在复制期间发生变化"))
		}
		changed, err := normalizeDeployedSkill(path, i.Name)
		if err != nil {
			return fail(err)
		}
		if changed {
			_, digest, e = skillInventory(path)
			if e != nil {
				return fail(e)
			}
		}
		manifest.Skills[i.Name] = deployedSkill{ID: i.ID, Hash: digest}
	}
	b, err := yaml.Marshal(manifest)
	if err != nil {
		return fail(err)
	}
	comment := "# project: canonical project path; agent: codex or pi.\n# managed: MCP names mapped to owned resource IDs.\n# config_hash: SHA-256 of the last deployed MCP file.\n# skills: deployed names mapped to resource id and content hash.\n"
	if err = tx.write(manifestPath, append([]byte(comment), b...)); err != nil {
		return fail(err)
	}
	if err = tx.finish(); err != nil {
		return result, fmt.Errorf("部署已完成，但临时备份清理失败: %w", err)
	}
	return result, nil
}

func mergeConfig(data []byte, agent string, items []Item, manifest *Deployment) ([]byte, error) {
	doc := map[string]any{}
	var err error
	if len(data) > 0 {
		if agent == "codex" {
			err = toml.Unmarshal(data, &doc)
		} else {
			err = json.Unmarshal(data, &doc)
		}
	}
	if err != nil {
		return nil, fmt.Errorf("现有项目配置无法解析，未修改: %w", err)
	}
	if doc == nil {
		return nil, errors.New("现有项目配置须为对象")
	}
	key := "mcp_servers"
	if agent == "pi" {
		key = "mcpServers"
	}
	servers := map[string]any{}
	if existing, ok := doc[key]; ok {
		var valid bool
		servers, valid = existing.(map[string]any)
		if !valid {
			return nil, errors.New("现有 MCP 配置须为对象")
		}
	}
	for _, i := range items {
		if i.Kind != "mcp" {
			continue
		}
		s := i.MCP
		entry := map[string]any{}
		if s.Command != "" {
			entry["command"] = s.Command
			if len(s.Args) > 0 {
				entry["args"] = s.Args
			}
			if len(s.Env) > 0 {
				entry["env"] = s.Env
			}
			if s.Cwd != "" {
				entry["cwd"] = s.Cwd
			}
		} else {
			entry["url"] = s.URL
			if len(s.Headers) > 0 {
				h := "headers"
				if agent == "codex" {
					h = "http_headers"
				}
				entry[h] = s.Headers
			}
		}
		if existing, ok := servers[i.Name]; ok && manifest.Managed[i.Name] != i.ID {
			// JSON normalization makes string slices/maps comparable across decoders.
			a, _ := json.Marshal(existing)
			b, _ := json.Marshal(entry)
			if !reflect.DeepEqual(a, b) {
				return nil, fmt.Errorf("MCP 同名冲突，未覆盖: %s", i.Name)
			}
		}
		// A resource keeps its ID through renames. Retire only names owned by it.
		for oldName, id := range manifest.Managed {
			if id == i.ID && oldName != i.Name {
				delete(servers, oldName)
				delete(manifest.Managed, oldName)
			}
		}
		servers[i.Name] = entry
		manifest.Managed[i.Name] = i.ID
	}
	doc[key] = servers
	if agent == "codex" {
		return toml.Marshal(doc)
	}
	b, err := json.MarshalIndent(doc, "", "  ")
	return append(b, '\n'), err
}
