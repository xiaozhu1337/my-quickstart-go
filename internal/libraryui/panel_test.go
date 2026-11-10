package libraryui

import (
	"fmt"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestPaginationKeepsSelectionAcrossSearch(t *testing.T) {
	p := New(filepath.Join(t.TempDir(), "library"))
	for n := 0; n < 45; n++ {
		if err := p.store.SaveMCP("", fmt.Sprintf("server-%02d", n), "", "command: tool"); err != nil {
			t.Fatal(err)
		}
	}
	p.items, _ = p.store.Items()
	tt := ui.NewTester(p.View, 960, 580)
	click(t, tt, "选择本页")
	if len(p.ids()) != 40 {
		t.Fatal("page selection must select exactly 40 resources")
	}
	click(t, tt, "下一页")
	click(t, tt, "server-44")
	if len(p.ids()) != 41 {
		t.Fatal("selection was lost across pages")
	}
	click(t, tt, "搜索资源")
	tt.Type("server-44")
	if p.page != 0 || len(p.ids()) != 41 || !tt.HasText("共 1 项") {
		t.Fatal("search lost selection or did not reset pagination")
	}
}

func click(t *testing.T, tt *ui.Tester, label string) {
	t.Helper()
	if err := tt.Click(label); err != nil {
		t.Fatal(err)
	}
}

func TestAddSelectDeployAndReload(t *testing.T) {
	root := filepath.Join(t.TempDir(), "library")
	p := New(root)
	tt := ui.NewTester(p.View, 960, 580)
	click(t, tt, "添加 MCP")
	click(t, tt, "MCP 名称")
	tt.Type("my-tools")
	click(t, tt, "MCP 配置")
	tt.Key(ui.Cmd, ui.KeyA)
	tt.Type("command: test-server\nargs: [--stdio]\n")
	click(t, tt, "保存")
	if p.editOpen || len(p.items) != 1 || p.failure != "" {
		t.Fatal("save failed", p.failure, tt.Texts())
	}
	click(t, tt, "my-tools")
	project := t.TempDir()
	click(t, tt, "项目目录")
	tt.Type(project)
	click(t, tt, "链接到项目")
	if p.failure != "" {
		t.Fatal(p.failure)
	}
	if _, err := os.Readlink(filepath.Join(project, ".codex", "config.toml")); err != nil {
		t.Fatal(err)
	}
	if len(New(root).items) != 1 {
		t.Fatal("saved resource was not reloaded")
	}
	click(t, tt, "编辑")
	click(t, tt, "MCP 名称")
	tt.Key(ui.Cmd, ui.KeyA)
	tt.Type("renamed-tools")
	click(t, tt, "保存")
	if p.failure != "" || p.items[0].Name != "renamed-tools" || len(p.ids()) != 1 {
		t.Fatal("rename failed or lost selection", p.failure)
	}
	click(t, tt, "从库移除")
	click(t, tt, "取消")
	if len(p.items) != 1 {
		t.Fatal("cancel removed item")
	}
	click(t, tt, "从库移除")
	click(t, tt, "确认移除")
	if len(p.items) != 0 {
		t.Fatal("remove failed")
	}
}

func TestSmallWindowAndDialogs(t *testing.T) {
	for _, width := range []int{560, 960} {
		p := New(filepath.Join(t.TempDir(), "library"))
		tt := ui.NewTester(p.View, width, 345)
		click(t, tt, "添加 MCP")
		click(t, tt, "MCP 名称")
		tt.Type("sample")
		click(t, tt, "保存")
		if p.editOpen || p.failure != "" {
			t.Fatal("small-window save failed", p.failure)
		}
		click(t, tt, "sample")
		p.project = t.TempDir()
		tt.Frame()
		click(t, tt, "链接到项目")
		if p.failure != "" {
			t.Fatal(p.failure)
		}
		click(t, tt, "添加 Skill")
		click(t, tt, "取消")
	}
}

func TestLibraryPreview(t *testing.T) {
	destination := os.Getenv("QUICKSTART_LIBRARY_PREVIEW")
	if destination == "" {
		t.Skip("preview output not requested")
	}
	p := New(filepath.Join(t.TempDir(), "library"))
	for _, item := range []struct{ name, description, config string }{
		{"filesystem", "访问项目文件与参考资料", "command: pnpm\nargs: [dlx, '@modelcontextprotocol/server-filesystem', 'C:/workspace']"},
		{"browser-tools", "浏览器自动化与页面调试", "url: http://localhost:3000/mcp"},
		{"internal-api", "团队内部服务接口", "url: https://example.com/mcp"},
	} {
		if err := p.store.SaveMCP("", item.name, item.description, item.config); err != nil {
			t.Fatal(err)
		}
	}
	source := filepath.Join(t.TempDir(), "review")
	if err := os.MkdirAll(source, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "SKILL.md"), []byte("---\nname: code-review\ndescription: 检查代码质量、回归风险与测试覆盖\n---\nReview changes.\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := p.store.ImportSkill(source); err != nil {
		t.Fatal(err)
	}
	p.items, _ = p.store.Items()
	p.selected[p.items[0].ID] = true
	p.selected[p.items[3].ID] = true
	p.project = "C:/Users/Administrator/code/my-project"
	tt := ui.NewTester(func(c *ui.Context) {
		ui.Column(c).Fill().Children(func() {
			ui.Row(c).FillWidth().AlignItems(ui.Center).Children(func() {
				tab := 1
				ui.Tabs(c, &tab, "应用启动", "MCP / Skills")
				ui.Box(c).Grow(1)
				ui.Button(c, "设置").Margin(0, 12)
			})
			ui.Divider(c)
			p.View(c)
		})
	}, 1100, 680)
	f, err := os.Create(destination)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, tt.Image()); err != nil {
		t.Fatal(err)
	}
}
