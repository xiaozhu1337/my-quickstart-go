package library

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRenameMCPDeployBothAgents(t *testing.T) {
	for _, agent := range []string{"codex", "pi"} {
		t.Run(agent, func(t *testing.T) {
			l, project, ids := fixture(t)
			if _, err := l.Deploy(project, agent, ids); err != nil {
				t.Fatal(err)
			}
			if err := l.SaveMCP(ids[0], "renamed", "updated", "command: new-tool"); err != nil {
				t.Fatal(err)
			}
			if err := l.SaveMCP(ids[0], "remote-tools", "", "command: new-tool"); err == nil {
				t.Fatal("rename allowed duplicate")
			}
			if _, err := l.Deploy(project, agent, []string{ids[0]}); err != nil {
				t.Fatal(err)
			}
			rel, key := ".codex/config.toml", "mcp_servers"
			if agent == "pi" {
				rel, key = ".pi/mcp.json", "mcpServers"
			}
			servers := configDoc(t, filepath.Join(project, rel), agent)[key].(map[string]any)
			if _, ok := servers["local-tools"]; ok {
				t.Fatal("old name survived rename")
			}
			if servers["renamed"] == nil || servers["remote-tools"] == nil || len(servers) != 2 {
				t.Fatal(servers)
			}
			if err := l.SaveMCP(ids[0], "again", "", "command: new-tool"); err != nil {
				t.Fatal(err)
			}
			if _, err := l.Deploy(project, agent, []string{ids[0]}); err != nil {
				t.Fatal(err)
			}
			servers = configDoc(t, filepath.Join(project, rel), agent)[key].(map[string]any)
			if servers["again"] == nil || servers["renamed"] != nil {
				t.Fatal(servers)
			}
		})
	}
}

func TestImportMagpiePortableAndRepeat(t *testing.T) {
	l, _, _ := fixture(t)
	base := t.TempDir()
	root := filepath.Join(base, "data")
	skill := filepath.Join(t.TempDir(), "external-skill")
	write(t, filepath.Join(skill, "SKILL.md"), "---\nname: original\ndescription: A complete skill\ncustom: keep\n---\nInstructions\n")
	write(t, filepath.Join(skill, "scripts", "example.py"), "print('sample')")
	if err := os.MkdirAll(filepath.Join(root, "library", "skills"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(skill, filepath.Join(root, "library", "skills", "imported")); err != nil {
		t.Fatal(err)
	}
	doc := map[string]any{
		"mcp": []map[string]any{
			{"name": "local-tools", "command": "must-not-replace"},
			{"name": "stdio", "transport": "stdio", "command": "tool", "args": []string{"--x", "two words"}, "env": map[string]string{"TOKEN": "test"}, "note": "A note"},
			{"name": "http", "transport": "http", "url": "https://example.com/mcp", "headers": map[string]string{"Authorization": "test"}},
			{"name": "sse", "transport": "sse", "url": "https://example.com/sse"},
			{"name": "invalid", "command": ""},
		},
		"skills": []map[string]any{{"name": "imported", "note": "Extra note"}, {"name": "missing"}},
	}
	b, _ := json.Marshal(doc)
	write(t, filepath.Join(root, "library.json"), string(b))
	report, err := l.ImportMagpie(base)
	if err != nil {
		t.Fatal(err)
	}
	if report.MCP != 2 || report.Skills != 1 || report.Skipped != 1 || report.Failed != 3 {
		t.Fatalf("%+v", report)
	}
	items, _ := l.Items()
	if len(items) != 6 || items[0].MCP.Command != "tool" {
		t.Fatal(items)
	}
	for _, item := range items {
		if item.Name == "stdio" && (item.Description != "A note" || item.MCP.Env["TOKEN"] != "test" || item.MCP.Args[1] != "two words") {
			t.Fatal(item)
		}
		if item.Name == "imported" {
			copied := filepath.Join(l.Root, item.Directory)
			if read(t, filepath.Join(copied, "scripts", "example.py")) != "print('sample')" {
				t.Fatal("support file lost")
			}
			body := read(t, filepath.Join(copied, "SKILL.md"))
			if !strings.Contains(body, "name: imported") || !strings.Contains(body, "custom: keep") || !strings.HasSuffix(body, "---\nInstructions\n") {
				t.Fatal(body)
			}
			if !strings.Contains(item.Description, "Extra note") {
				t.Fatal(item)
			}
		}
	}
	if !strings.Contains(read(t, filepath.Join(skill, "SKILL.md")), "name: original") {
		t.Fatal("source changed")
	}
	if read(t, filepath.Join(root, "library.json")) != string(b) {
		t.Fatal("magpie config changed")
	}
	report, err = l.ImportMagpie(filepath.Join(root, "library.json"))
	if err != nil || report.MCP != 0 || report.Skills != 0 || report.Skipped != 4 || report.Failed != 3 {
		t.Fatalf("%+v %v", report, err)
	}
}

func TestMagpieValidationAndDefault(t *testing.T) {
	root := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", root)
	if DefaultMagpiePath() != filepath.Join(root, "magpie") {
		t.Fatal(DefaultMagpiePath())
	}
	l, _, _ := fixture(t)
	path := filepath.Join(root, "magpie", "library.json")
	for _, content := range []string{"{", "null", `{}`, `{"mcp":{}}`} {
		write(t, path, content)
		if _, err := l.ImportMagpie(""); err == nil {
			t.Fatal("accepted", content)
		}
	}
	write(t, path, `{"mcp":[],"skills":[]}`)
	if report, err := l.ImportMagpie(""); err != nil || report.Failed != 0 {
		t.Fatal(report, err)
	}
	items, _ := l.Items()
	if len(items) != 3 {
		t.Fatal("invalid imports changed existing resources")
	}
}

func TestMagpieFolderFallbackAndInvalidSkill(t *testing.T) {
	l, _, _ := fixture(t)
	root, source := t.TempDir(), t.TempDir()
	write(t, filepath.Join(source, "SKILL.md"), "---\nname: original\ndescription: Local skill\n---\nInstructions\n")
	doc := map[string]any{"skills": []map[string]any{
		{"name": "fallback", "source": map[string]string{"kind": "folder", "dir": source}},
		{"name": "broken", "note": "A note cannot replace missing skill metadata"},
	}}
	b, _ := json.Marshal(doc)
	write(t, filepath.Join(root, "library.json"), string(b))
	write(t, filepath.Join(root, "library", "skills", "broken", "SKILL.md"), "---\nnull\n---\nBroken\n")
	report, err := l.ImportMagpie(root)
	if err != nil || report.Skills != 1 || report.Failed != 1 {
		t.Fatal(report, err)
	}
}
