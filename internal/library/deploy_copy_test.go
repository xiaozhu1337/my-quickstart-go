package library

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"go.yaml.in/yaml/v3"
)

func deploymentRecord(t *testing.T, l *Library) (string, Deployment) {
	t.Helper()
	profiles, err := os.ReadDir(filepath.Join(l.Root, "projects"))
	if err != nil || len(profiles) != 1 {
		t.Fatalf("deployment profiles: %v %v", profiles, err)
	}
	path := filepath.Join(l.Root, "projects", profiles[0].Name(), "deployment.yaml")
	var record Deployment
	if err := yaml.Unmarshal([]byte(read(t, path)), &record); err != nil {
		t.Fatal(err)
	}
	return path, record
}

func assertNoDeployTemps(t *testing.T, project string) {
	t.Helper()
	entries, err := os.ReadDir(filepath.Join(project, "__tmp"))
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("deployment temporary files remain: %v", entries)
	}
}

func TestSkillCopiesAreIndependentAndRefresh(t *testing.T) {
	for _, agent := range []string{"codex", "pi"} {
		t.Run(agent, func(t *testing.T) {
			l, project, ids := fixture(t)
			items, _ := l.Items()
			source := l.skillPath(items[2])
			rel := ".agents/skills/sample-skill"
			if agent == "pi" {
				rel = ".pi/skills/sample-skill"
			}
			other := t.TempDir()
			for _, dest := range []string{project, other} {
				if _, err := l.Deploy(dest, agent, ids[2:]); err != nil {
					t.Fatal(err)
				}
			}
			guide := filepath.Join(project, rel, "references", "guide.md")
			write(t, filepath.Join(source, "references", "guide.md"), "updated library")
			write(t, filepath.Join(source, "new.txt"), "new asset")
			if read(t, guide) != "Useful reference" {
				t.Fatal("library edit leaked into project")
			}
			if _, err := l.Deploy(project, agent, ids[2:]); err != nil {
				t.Fatal(err)
			}
			if read(t, guide) != "updated library" || read(t, filepath.Join(project, rel, "new.txt")) != "new asset" {
				t.Fatal("skill refresh failed")
			}
			if read(t, filepath.Join(other, rel, "references", "guide.md")) != "Useful reference" {
				t.Fatal("other project modified")
			}
			if err := os.Remove(filepath.Join(source, "new.txt")); err != nil {
				t.Fatal(err)
			}
			if _, err := l.Deploy(project, agent, ids[2:]); err != nil {
				t.Fatal(err)
			}
			if _, err := os.Lstat(filepath.Join(project, rel, "new.txt")); !errors.Is(err, os.ErrNotExist) {
				t.Fatal("obsolete asset survived")
			}
			write(t, guide, "local edit")
			if read(t, filepath.Join(source, "references", "guide.md")) != "updated library" {
				t.Fatal("project edit modified library")
			}
			obsolete := filepath.Join(project, rel, "local-only", "old.txt")
			write(t, obsolete, "local asset")
			if _, err := l.Deploy(project, agent, ids); err != nil {
				t.Fatal("modified skill was not replaced", err)
			}
			if read(t, guide) != "updated library" {
				t.Fatal("local edit survived replacement")
			}
			if _, err := os.Lstat(filepath.Dir(obsolete)); !errors.Is(err, os.ErrNotExist) {
				t.Fatal("local-only directory survived replacement", err)
			}
			assertNoDeployTemps(t, project)
		})
	}
}

func TestLegacyLinksMigrateAndRollback(t *testing.T) {
	for _, agent := range []string{"codex", "pi"} {
		t.Run(agent, func(t *testing.T) {
			l, project, ids := fixture(t)
			if _, err := l.Deploy(project, agent, ids); err != nil {
				t.Fatal(err)
			}
			manifest, record := deploymentRecord(t, l)
			configRel, skillRel, filename := ".codex/config.toml", ".agents/skills/sample-skill", "config.toml"
			if agent == "pi" {
				configRel, skillRel, filename = ".pi/mcp.json", ".pi/skills/sample-skill", "mcp.json"
			}
			config := filepath.Join(project, configRel)
			skill := filepath.Join(project, skillRel)
			generated := filepath.Join(filepath.Dir(manifest), filename)
			original := read(t, config)
			write(t, generated, original)
			if err := os.Remove(config); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(generated, config); err != nil {
				t.Skipf("symlink fixture unavailable: %v", err)
			}
			paths, _, err := skillInventory(skill)
			if err != nil {
				t.Fatal(err)
			}
			if err := removePaths(paths); err != nil {
				t.Fatal(err)
			}
			items, _ := l.Items()
			source := l.skillPath(items[2])
			if err := os.Symlink(source, skill); err != nil {
				t.Fatal(err)
			}
			record.ConfigHash, record.Skills = "", nil
			raw, err := yaml.Marshal(record)
			if err != nil {
				t.Fatal(err)
			}
			write(t, manifest, string(raw))
			if err := l.SaveMCP(ids[0], "renamed", "", "command: updated"); err != nil {
				t.Fatal(err)
			}
			l.copySkill = func(source, dest string) ([]string, error) {
				created, err := copySkillDirectory(source, dest)
				return created, errors.Join(err, errors.New("injected copy failure"))
			}
			if _, err := l.Deploy(project, agent, ids); err == nil {
				t.Fatal("expected copy failure")
			}
			if !linkedTo(config, generated) || !linkedTo(skill, source) {
				t.Fatal("legacy links not restored")
			}
			if read(t, generated) != original || read(t, manifest) != string(raw) {
				t.Fatal("legacy source or manifest modified")
			}
			assertNoDeployTemps(t, project)
			l.copySkill = copySkillDirectory
			if _, err := l.Deploy(project, agent, ids); err != nil {
				t.Fatal(err)
			}
			if info, err := os.Lstat(config); err != nil || !info.Mode().IsRegular() {
				t.Fatal("config not migrated", err)
			}
			if info, err := os.Lstat(skill); err != nil || !info.IsDir() {
				t.Fatal("skill not migrated", err)
			}
			if read(t, generated) != original {
				t.Fatal("legacy generated source changed")
			}
			key := "mcp_servers"
			if agent == "pi" {
				key = "mcpServers"
			}
			servers := configDoc(t, config, agent)[key].(map[string]any)
			if servers["local-tools"] != nil || servers["renamed"] == nil {
				t.Fatal("legacy MCP ownership lost")
			}
			write(t, filepath.Join(skill, "references", "guide.md"), "project only")
			if read(t, filepath.Join(source, "references", "guide.md")) != "Useful reference" {
				t.Fatal("migration changed source")
			}
			assertNoDeployTemps(t, project)
		})
	}
}

func TestCopyRollbackRestoresExistingSkill(t *testing.T) {
	l, project, ids := fixture(t)
	if _, err := l.Deploy(project, "codex", ids); err != nil {
		t.Fatal(err)
	}
	manifest, _ := deploymentRecord(t, l)
	originalManifest := read(t, manifest)
	guide := filepath.Join(project, ".agents", "skills", "sample-skill", "references", "guide.md")
	write(t, guide, "local edit")
	items, _ := l.Items()
	write(t, filepath.Join(l.skillPath(items[2]), "references", "guide.md"), "new source")
	copied := false
	l.copySkill = func(source, dest string) ([]string, error) {
		copied = true
		created, err := copySkillDirectory(source, dest)
		return created, errors.Join(err, errors.New("injected failure after copying"))
	}
	if _, err := l.Deploy(project, "codex", ids); err == nil {
		t.Fatal("expected failure")
	}
	if !copied {
		t.Fatal("replacement did not reach copy operation")
	}
	if read(t, guide) != "local edit" {
		t.Fatal("existing skill not restored")
	}
	if read(t, manifest) != originalManifest {
		t.Fatal("manifest changed")
	}
	assertNoDeployTemps(t, project)
}

func TestReplaceUnmanagedSkill(t *testing.T) {
	for _, agent := range []string{"codex", "pi"} {
		for _, failCopy := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/fail=%t", agent, failCopy), func(t *testing.T) {
				l, project, ids := fixture(t)
				rel := ".agents/skills"
				if agent == "pi" {
					rel = ".pi/skills"
				}
				dest := filepath.Join(project, rel, "sample-skill")
				write(t, filepath.Join(dest, "SKILL.md"), "old skill")
				obsolete := filepath.Join(dest, "old", "asset.txt")
				write(t, obsolete, "old asset")
				sibling := filepath.Join(project, rel, "other-skill", "SKILL.md")
				write(t, sibling, "unselected skill")
				copied := false
				l.copySkill = func(source, target string) ([]string, error) {
					copied = true
					if _, err := os.Lstat(target); !errors.Is(err, os.ErrNotExist) {
						t.Fatal("old destination still exists before copying", err)
					}
					created, err := copySkillDirectory(source, target)
					if failCopy {
						err = errors.Join(err, errors.New("injected copy failure"))
					}
					return created, err
				}
				result, err := l.Deploy(project, agent, ids[2:])
				if !copied || (err != nil) != failCopy {
					t.Fatalf("copied=%t, result=%+v, error=%v", copied, result, err)
				}
				if failCopy {
					if read(t, filepath.Join(dest, "SKILL.md")) != "old skill" || read(t, obsolete) != "old asset" {
						t.Fatal("original unmanaged skill was not restored")
					}
				} else {
					items, _ := l.Items()
					_, want, _ := skillInventory(l.skillPath(items[2]))
					_, got, err := skillInventory(dest)
					if err != nil || got != want || result.Skills != 1 {
						t.Fatal("replacement differs from source", err)
					}
				}
				if read(t, sibling) != "unselected skill" {
					t.Fatal("unselected skill changed")
				}
				assertNoDeployTemps(t, project)
			})
		}
	}
}

func TestCopiedSkillsRejectNestedLinks(t *testing.T) {
	l, project, ids := fixture(t)
	items, _ := l.Items()
	source := l.skillPath(items[2])
	external := filepath.Join(t.TempDir(), "external.txt")
	write(t, external, "external")
	if err := os.Symlink(external, filepath.Join(source, "linked.txt")); err != nil {
		t.Skip(err)
	}
	if _, err := l.Deploy(project, "codex", ids); err == nil {
		t.Fatal("accepted linked source asset")
	}
	if _, err := os.Lstat(filepath.Join(project, ".codex", "config.toml")); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("partial deployment")
	}
	if read(t, external) != "external" {
		t.Fatal("external file changed")
	}
}

func TestReadOnlySkillAssetsCanRefresh(t *testing.T) {
	l, project, ids := fixture(t)
	items, _ := l.Items()
	asset := filepath.Join(l.skillPath(items[2]), "references", "guide.md")
	if err := os.Chmod(asset, 0400); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(asset, 0600)
	for range 2 {
		if _, err := l.Deploy(project, "codex", ids[2:]); err != nil {
			t.Fatal(err)
		}
	}
	assertNoDeployTemps(t, project)
}
