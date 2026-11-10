package libraryui

import (
	"fmt"
	"log"
	"strings"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"go.yaml.in/yaml/v3"
	"my-quickstart-go/internal/library"
)

type Panel struct {
	Window                                    *mygo.Window
	Language                                  string
	Busy                                      bool
	store                                     *library.Library
	items                                     []library.Item
	selected                                  map[string]bool
	query, kind, project, agent               string
	message, failure                          string
	page                                      int
	list                                      ui.ListState
	editOpen, skillOpen, removeOpen           bool
	editID, name, description, config, source string
	magpieSource                              string
	importReport                              *library.ImportReport
	importPage                                int
}

func New(root string) *Panel {
	p := &Panel{selected: map[string]bool{}, agent: "codex", magpieSource: library.DefaultMagpiePath()}
	var err error
	p.store, err = library.Open(root)
	if err == nil {
		p.items, err = p.store.Items()
	}
	if err != nil {
		p.store = nil
		p.fail(err)
	}
	return p
}

func (p *Panel) tr(zh, en string) string {
	if p.Language == "en" {
		return en
	}
	return zh
}
func (p *Panel) fail(err error) { p.failure = err.Error(); log.Print(err) }

func (p *Panel) run(job func() (string, error), closeDialog bool) {
	if p.Busy || p.store == nil {
		return
	}
	p.Busy = true
	p.failure = ""
	p.message = ""
	work := func() {
		message, err := job()
		items, readErr := p.store.Items()
		if err == nil {
			err = readErr
		}
		finish := func() {
			p.Busy = false
			if err != nil {
				p.fail(err)
				return
			}
			p.items = items
			p.message = message
			valid := map[string]bool{}
			for _, i := range items {
				if p.selected[i.ID] {
					valid[i.ID] = true
				}
			}
			p.selected = valid
			if closeDialog {
				p.editOpen, p.skillOpen, p.removeOpen = false, false, false
			}
		}
		if p.Window != nil {
			p.Window.Update(finish)
		} else {
			finish()
		}
	}
	if p.Window != nil {
		go work()
	} else {
		work()
	}
}

func (p *Panel) ids() []string {
	var ids []string
	for _, i := range p.items {
		if p.selected[i.ID] {
			ids = append(ids, i.ID)
		}
	}
	return ids
}

func (p *Panel) edit(i library.Item) {
	p.editID, p.name, p.description = i.ID, i.Name, i.Description
	p.config = "# command: executable; args: argument list\ncommand: pnpm\nargs: [dlx, '@modelcontextprotocol/server-filesystem', 'C:/workspace']\n# env: environment variables; cwd: absolute working directory\n# For HTTP, replace command/args with url and optional headers.\n"
	if i.MCP != nil {
		b, err := yaml.Marshal(i.MCP)
		if err != nil {
			p.fail(err)
			return
		}
		p.config = string(b)
	}
	p.failure = ""
	p.editOpen = true
}

func (p *Panel) pick(skill bool) {
	if p.Window == nil || p.Busy {
		return
	}
	p.Busy = true
	title := p.tr("选择项目目录", "Choose project directory")
	if skill {
		title = p.tr("选择包含 SKILL.md 的目录", "Choose directory containing SKILL.md")
	}
	go func() {
		paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{Parent: p.Window, Title: title, Directory: true})
		p.Window.Update(func() {
			p.Busy = false
			if err != nil {
				p.fail(err)
				return
			}
			if len(paths) == 0 {
				return
			}
			if skill {
				p.source = paths[0]
			} else {
				p.project = paths[0]
			}
		})
	}()
}

func (p *Panel) View(c *ui.Context) {
	ui.Column(c).Grow(1).FillWidth().Padding(16).Gap(10).Children(func() {
		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			if ui.TextInput(c.Key("resource-search"), &p.query).Label(p.tr("搜索资源", "Search resources")).Placeholder(p.tr("搜索名称或说明", "Search name or description")).Grow(1).Changed() {
				p.page = 0
			}
			if p.kind == "" || p.kind == "全部" || p.kind == "All" {
				p.kind = p.tr("全部", "All")
			}
			if ui.Select(c, &p.kind, []string{p.tr("全部", "All"), "MCP", "Skill"}).Width(100).Changed() {
				p.page = 0
			}
			if ui.Button(c, p.tr("添加 MCP", "Add MCP")).Disabled(p.Busy || p.store == nil).Clicked() {
				p.edit(library.Item{})
			}
			if ui.Button(c, p.tr("添加 Skill", "Add Skill")).Disabled(p.Busy || p.store == nil).Clicked() {
				p.source = ""
				p.failure = ""
				p.skillOpen = true
			}
		})
		p.resourceList(c)
		ui.Divider(c)
		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			ui.Text(c, "Agent")
			ui.Select(c, &p.agent, []string{"codex", "pi"}).Width(100).Disabled(p.Busy)
			ui.TextInput(c.Key("project-directory"), &p.project).Label(p.tr("项目目录", "Project directory")).Placeholder(p.tr("选择要写入的项目目录", "Choose a project directory")).Grow(1).Disabled(p.Busy)
			if ui.Button(c, p.tr("浏览", "Browse")).Disabled(p.Busy).Clicked() {
				p.pick(false)
			}
		})
		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			ui.Text(c, fmt.Sprintf(p.tr("已选择 %d 项", "%d selected"), len(p.ids()))).Grow(1)
			if ui.Button(c, p.tr("取消选择", "Clear selection")).Disabled(p.Busy).Clicked() {
				p.selected = map[string]bool{}
			}
			if ui.Button(c, p.tr("从库移除", "Remove from library")).Disabled(p.Busy || len(p.ids()) == 0).Clicked() {
				p.removeOpen = true
			}
			if ui.PrimaryButton(c, p.tr("链接到项目", "Link to project")).Disabled(p.Busy || len(p.ids()) == 0 || strings.TrimSpace(p.project) == "").Clicked() {
				ids, project, agent := p.ids(), p.project, p.agent
				format := p.tr("已链接 %d 个 MCP、%d 个 Skill 到 %s", "Linked %d MCPs and %d skills to %s")
				backupLabel := p.tr("；原配置备份：", "; backup: ")
				p.run(func() (string, error) {
					r, err := p.store.Deploy(project, agent, ids)
					message := fmt.Sprintf(format, r.MCP, r.Skills, agent)
					if r.Backup != "" {
						message += backupLabel + r.Backup
					}
					return message, err
				}, false)
			}
		})
		ui.Text(c, p.tr("按所选资源增量写入；已有项目配置会先备份，同名冲突不会覆盖。", "Adds selected resources; existing config is backed up, name conflicts are preserved.")).FontSize(11).TextColor(c.Theme().TextMuted)
		if p.Busy {
			ui.Text(c, p.tr("正在处理…", "Working…"))
		} else if p.failure != "" {
			ui.Text(c, p.failure).Tooltip(p.failure).TextColor(c.Theme().Danger).MaxHeight(60)
		} else if p.message != "" {
			ui.Text(c, p.message).Tooltip(p.message).MaxHeight(60)
		}
	})
	p.dialogs(c)
}

func (p *Panel) resourceList(c *ui.Context) {
	var matches []library.Item
	q := strings.ToLower(strings.TrimSpace(p.query))
	for _, i := range p.items {
		if (p.kind == "MCP" && i.Kind != "mcp") || (p.kind == "Skill" && i.Kind != "skill") {
			continue
		}
		if strings.Contains(strings.ToLower(i.Name+" "+i.Description), q) {
			matches = append(matches, i)
		}
	}
	const pageSize = 40
	pages := max(1, (len(matches)+pageSize-1)/pageSize)
	p.page = min(max(p.page, 0), pages-1)
	entries := matches[min(p.page*pageSize, len(matches)):min((p.page+1)*pageSize, len(matches))]
	ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
		all := len(entries) > 0
		for _, i := range entries {
			if !p.selected[i.ID] {
				all = false
			}
		}
		if ui.Checkbox(c, &all, p.tr("选择本页", "Select page")).Disabled(p.Busy).Changed() {
			for _, i := range entries {
				p.selected[i.ID] = all
			}
		}
		ui.Text(c, fmt.Sprintf(p.tr("共 %d 项", "%d resources"), len(matches))).TextColor(c.Theme().TextMuted).Grow(1)
		if ui.Button(c, p.tr("上一页", "Previous")).Disabled(p.page == 0 || p.Busy).Clicked() {
			p.page--
		}
		ui.Text(c, fmt.Sprintf("%d / %d", p.page+1, pages))
		if ui.Button(c, p.tr("下一页", "Next")).Disabled(p.page+1 == pages || p.Busy).Clicked() {
			p.page++
		}
	})
	if len(matches) == 0 {
		ui.Text(c, p.tr("暂无资源。添加 MCP 配置，或导入包含 SKILL.md 的目录。", "No resources. Add an MCP config or import a directory containing SKILL.md.")).TextColor(c.Theme().TextMuted).Padding(20, 0)
	}
	p.list.Key = func(n int) any { return entries[n].ID }
	ui.List(c, &p.list, len(entries), func(n int) {
		i := entries[n]
		ui.Row(c.Key(i.ID)).Padding(10).Gap(12).AlignItems(ui.Center).Background(c.Theme().Surface).Radius(8).Children(func() {
			checked := p.selected[i.ID]
			if ui.Checkbox(c, &checked, i.Name).Width(180).Disabled(p.Busy).Changed() {
				p.selected[i.ID] = checked
			}
			kind := "MCP"
			if i.Kind == "skill" {
				kind = "Skill"
			}
			ui.Text(c, kind).Width(40).FontSize(11).TextColor(c.Theme().Accent)
			ui.Text(c, i.Description).SingleLine().Tooltip(i.Description).Grow(1).TextColor(c.Theme().TextMuted)
			if i.Kind == "mcp" {
				if ui.Button(c, p.tr("编辑", "Edit")).Disabled(p.Busy).Clicked() {
					p.edit(i)
				}
			}
		})
	}).Grow(1)
}
