package library

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pelletier/go-toml/v2"
)

func fixture(t *testing.T) (*Library, string, []string) {
	t.Helper()
	base := t.TempDir()
	l, err := Open(filepath.Join(base, "library"))
	if err != nil {
		t.Fatal(err)
	}
	if err = l.SaveMCP("", "local-tools", "Local tools", "command: tool\nargs: ['two words', '--flag']\nenv:\n  TEST: value\n"); err != nil {
		t.Fatal(err)
	}
	if err = l.SaveMCP("", "remote-tools", "HTTP tools", "url: https://example.com/mcp\nheaders:\n  X-Test: value\n"); err != nil {
		t.Fatal(err)
	}
	skill := filepath.Join(base, "source")
	write(t, filepath.Join(skill, "SKILL.md"), "---\nname: sample-skill\ndescription: A sample skill\n---\nRead the reference.\n")
	write(t, filepath.Join(skill, "references", "guide.md"), "Useful reference")
	if err = l.ImportSkill(skill); err != nil {
		t.Fatal(err)
	}
	items, err := l.Items()
	if err != nil {
		t.Fatal(err)
	}
	var ids []string
	for _, i := range items {
		ids = append(ids, i.ID)
	}
	project := filepath.Join(base, "project with spaces")
	if err = os.MkdirAll(project, 0700); err != nil {
		t.Fatal(err)
	}
	return l, project, ids
}

func write(t *testing.T, path, text string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(text), 0600); err != nil {
		t.Fatal(err)
	}
}

func read(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func configDoc(t *testing.T, path, agent string) map[string]any {
	t.Helper()
	b := []byte(read(t, path))
	var doc map[string]any
	var err error
	if agent == "codex" {
		err = toml.Unmarshal(b, &doc)
	} else {
		err = json.Unmarshal(b, &doc)
	}
	if err != nil {
		t.Fatal(err)
	}
	return doc
}

func TestDeployBothAgentsAndUpdate(t *testing.T) {
	for _, agent := range []string{"codex", "pi"} {
		t.Run(agent, func(t *testing.T) {
			l, project, ids := fixture(t)
			rel, key, skills := ".codex/config.toml", "mcp_servers", ".agents/skills"
			original := "# user comment\nmodel = 'custom-model'\n[mcp_servers.personal]\ncommand = 'personal'\n[features]\nexample = true\n"
			if agent == "pi" {
				rel, key, skills = ".pi/mcp.json", "mcpServers", ".pi/skills"
				original = `{"custom":true,"mcpServers":{"personal":{"command":"personal"}}}`
			}
			path := filepath.Join(project, rel)
			write(t, path, original)
			r, err := l.Deploy(project, agent, ids)
			if err != nil {
				t.Fatal(err)
			}
			if r.MCP != 2 || r.Skills != 1 || r.Backup == "" {
				t.Fatalf("bad result: %+v", r)
			}
			if read(t, r.Backup) != original {
				t.Fatal("backup must preserve exact original bytes")
			}
			if info, err := os.Lstat(path); err != nil || !info.Mode().IsRegular() {
				t.Fatal("config is not a regular file", err)
			}
			doc := configDoc(t, path, agent)
			servers := doc[key].(map[string]any)
			if len(servers) != 3 || servers["personal"].(map[string]any)["command"] != "personal" {
				t.Fatal("existing server lost", doc)
			}
			headerKey := "http_headers"
			if agent == "pi" {
				headerKey = "headers"
			}
			if servers["remote-tools"].(map[string]any)[headerKey].(map[string]any)["X-Test"] != "value" {
				t.Fatal("HTTP headers were not converted")
			}
			if agent == "codex" && doc["model"] != "custom-model" {
				t.Fatal("unrelated config lost")
			}
			if agent == "pi" && doc["custom"] != true {
				t.Fatal("unrelated config lost")
			}
			skillLink := filepath.Join(project, skills, "sample-skill")
			if info, err := os.Lstat(skillLink); err != nil || !info.IsDir() {
				t.Fatal("skill is not a regular directory", err)
			}
			if read(t, filepath.Join(skillLink, "references", "guide.md")) != "Useful reference" {
				t.Fatal("skill assets lost")
			}
			if err = l.SaveMCP(ids[0], "local-tools", "Changed", "command: updated\n"); err != nil {
				t.Fatal(err)
			}
			r, err = l.Deploy(project, agent, ids[:1])
			if err != nil {
				t.Fatal(err)
			}
			if r.Backup != "" {
				t.Fatal("managed config should not create another backup")
			}
			servers = configDoc(t, path, agent)[key].(map[string]any)
			if len(servers) != 3 || servers["local-tools"].(map[string]any)["command"] != "updated" {
				t.Fatal("incremental update failed")
			}
			if err = l.Remove(ids); err != nil {
				t.Fatal(err)
			}
			if read(t, filepath.Join(skillLink, "references", "guide.md")) != "Useful reference" {
				t.Fatal("removal broke deployed skill")
			}
			if _, err = os.ReadFile(path); err != nil {
				t.Fatal("removal broke deployed MCP", err)
			}
		})
	}
}

func TestConflictsAndInvalidConfigDoNotModifyProject(t *testing.T) {
	for _, original := range []string{"[invalid", "[mcp_servers.local-tools]\ncommand = 'other'\n"} {
		l, project, ids := fixture(t)
		path := filepath.Join(project, ".codex", "config.toml")
		write(t, path, original)
		if _, err := l.Deploy(project, "codex", ids); err == nil {
			t.Fatal("expected conflict/parse failure")
		}
		if read(t, path) != original {
			t.Fatal("original config modified")
		}
		if _, err := os.Lstat(filepath.Join(project, ".agents", "skills", "sample-skill")); !errors.Is(err, os.ErrNotExist) {
			t.Fatal("partial skill deployment")
		}
	}
	l, project, ids := fixture(t)
	write(t, filepath.Join(project, ".pi", "skills", "sample-skill", "SKILL.md"), "user skill")
	if _, err := l.Deploy(project, "pi", ids); err == nil {
		t.Fatal("expected skill conflict")
	}
	if _, err := os.Lstat(filepath.Join(project, ".pi", "mcp.json")); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("partial MCP deployment")
	}
}

func TestCopyFailureRollsBackConfigAndManifest(t *testing.T) {
	for _, existing := range []bool{false, true} {
		t.Run(map[bool]string{false: "new", true: "existing"}[existing], func(t *testing.T) {
			l, project, ids := fixture(t)
			path := filepath.Join(project, ".codex", "config.toml")
			original := "model = 'original'\n"
			if existing {
				write(t, path, original)
			}
			l.copySkill = func(source, path string) ([]string, error) {
				created, err := copySkillDirectory(source, path)
				return created, errors.Join(err, errors.New("injected copy failure"))
			}
			if _, err := l.Deploy(project, "codex", ids); err == nil || !strings.Contains(err.Error(), "injected") {
				t.Fatal("expected injected failure", err)
			}
			if existing {
				if read(t, path) != original {
					t.Fatal("original not restored")
				}
				if _, err := os.Readlink(path); err == nil {
					t.Fatal("original replaced by link")
				}
			} else if _, err := os.Lstat(path); !errors.Is(err, os.ErrNotExist) {
				t.Fatal("partial configuration survived rollback")
			}
			_ = filepath.WalkDir(filepath.Join(l.Root, "projects"), func(path string, entry os.DirEntry, err error) error {
				if err == nil && !entry.IsDir() {
					t.Errorf("partial generated file: %s", path)
				}
				return err
			})
		})
	}
}

func TestValidationPersistenceAndImport(t *testing.T) {
	l, _, ids := fixture(t)
	for _, config := range []string{"command: tool\nurl: https://example.com", "url: file:///tmp", "command: tool\nunknown: true", "command: tool\nargs: wrong", "command: tool\n---\nurl: https://example.com", "command: tool\nheaders: {X: value}"} {
		if err := l.SaveMCP("", "bad", "", config); err == nil {
			t.Fatal("accepted invalid config", config)
		}
	}
	if err := l.SaveMCP("", "LOCAL-tools", "", "command: tool"); err == nil {
		t.Fatal("accepted case-insensitive duplicate")
	}
	if err := l.SaveMCP("", "../escape", "", "command: tool"); err == nil {
		t.Fatal("accepted unsafe name")
	}
	l2, err := Open(l.Root)
	if err != nil {
		t.Fatal(err)
	}
	items, err := l2.Items()
	if err != nil || len(items) != 3 || items[0].ID != ids[0] {
		t.Fatal("persistence failed", items, err)
	}
	write(t, filepath.Join(filepath.Dir(l.Root), "source", "references", "guide.md"), "changed source")
	if read(t, filepath.Join(l.Root, items[2].Directory, "references", "guide.md")) != "Useful reference" {
		t.Fatal("import must be independent of source")
	}
	if err := l.ImportSkill(filepath.Join(filepath.Dir(l.Root), "source")); err == nil {
		t.Fatal("accepted duplicate skill")
	}
}

func TestExternalSymlinksArePreserved(t *testing.T) {
	l, project, ids := fixture(t)
	external := filepath.Join(t.TempDir(), "config.toml")
	write(t, external, "model = 'external'")
	path := filepath.Join(project, ".codex", "config.toml")
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(external, path); err != nil {
		t.Fatal(err)
	}
	if _, err := l.Deploy(project, "codex", ids); err == nil {
		t.Fatal("accepted foreign config symlink")
	}
	if read(t, external) != "model = 'external'" || !linkedTo(path, external) {
		t.Fatal("foreign config changed")
	}
}

func TestProjectsStayIndependentAndDetachedConfigIsProtected(t *testing.T) {
	l, project, ids := fixture(t)
	second := t.TempDir()
	if _, err := l.Deploy(project, "codex", ids[:1]); err != nil {
		t.Fatal(err)
	}
	if _, err := l.Deploy(second, "codex", ids[:1]); err != nil {
		t.Fatal(err)
	}
	if err := l.SaveMCP(ids[0], "local-tools", "", "command: updated"); err != nil {
		t.Fatal(err)
	}
	if _, err := l.Deploy(project, "codex", ids[:1]); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(second, ".codex", "config.toml")
	if configDoc(t, path, "codex")["mcp_servers"].(map[string]any)["local-tools"].(map[string]any)["command"] != "tool" {
		t.Fatal("updating one project modified another")
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	write(t, path, "[mcp_servers.local-tools]\ncommand = 'user-replacement'\n")
	if _, err := l.Deploy(second, "codex", ids[:1]); err == nil {
		t.Fatal("a detached config must not inherit ownership from old manifest")
	}
}

func TestRollbackRestoresPreviouslyDeployedConfig(t *testing.T) {
	l, project, ids := fixture(t)
	if _, err := l.Deploy(project, "pi", ids[:1]); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(project, ".pi", "mcp.json")
	original := read(t, path)
	l.copySkill = func(string, string) ([]string, error) { return nil, errors.New("injected failure") }
	if _, err := l.Deploy(project, "pi", ids); err == nil {
		t.Fatal("expected failure")
	}
	if read(t, path) != original {
		t.Fatal("existing generated config not restored")
	}
}
