package libraryui

import (
	"fmt"
	"slices"
	"strings"

	"github.com/nsgo-dev/nsgo/core"
	"my-quickstart-go/internal/library"
	w "my-quickstart-go/internal/nativeui"
)

func (p *Panel) status() string {
	if p.Busy {
		return p.tr("正在处理…", "Working…")
	}
	if p.failure != "" {
		return p.failure
	}
	return p.message
}

func (p *Panel) ModalOpen() bool { return p.editOpen || p.skillOpen || p.removeOpen || p.probeOpen }

func (p *Panel) Native(v w.Page) core.Node {
	if p.ModalOpen() {
		return p.nativeDialog(v)
	}
	disabled := p.Busy || p.store == nil
	all := p.tr("全部", "All")
	if p.kind != "MCP" && p.kind != "Skill" {
		p.kind = all
	}
	filter := w.Select(p.tr("资源类型", "Resource type"), p.kind, []string{all, "MCP", "Skill"}, func(s string) { p.kind = s; v.Refresh() })
	actions := w.Row(v.Primary(p.tr("添加 MCP", "Add MCP"), func() { p.edit(library.Item{}) }, disabled),
		v.Button(p.tr("添加 Skill", "Add skill"), func() { p.skillID, p.skillName, p.source, p.failure = "", "", "", ""; p.skillOpen = true }, disabled))
	browser := v.Panel(w.Spread(filter, actions),
		w.Search(v.Scope, p.tr("搜索资源", "Search resources"), &p.query, func(q string) core.Node {
			var items []library.Item
			for _, item := range p.items {
				if (p.kind == all || strings.EqualFold(item.Kind, p.kind)) && strings.Contains(strings.ToLower(item.Name+" "+item.Description), strings.ToLower(strings.TrimSpace(q))) {
					items = append(items, item)
				}
			}
			slices.SortStableFunc(items, func(a, b library.Item) int {
				if a.Pinned == b.Pinned {
					return 0
				}
				if a.Pinned {
					return -1
				}
				return 1
			})
			if len(items) == 0 {
				message := p.tr("还没有资源，可以添加 MCP 或 Skill。", "Add an MCP or skill to start your library.")
				if len(p.items) > 0 {
					message = p.tr("没有匹配的资源", "No matching resources")
				}
				empty := w.Column(v.Muted(message))
				empty.Style.JustifyContent = core.JustifyCenter
				empty.Style.AlignItems = core.AlignCenter
				return empty
			}
			return w.Column(v.Muted(fmt.Sprintf(p.tr("共 %d 项", "%d resources"), len(items))), w.List(len(items), 1, 64, func(i int) any { return items[i].ID }, func(i int) core.Node {
				item := items[i]
				pin := p.tr("置顶", "Pin")
				if item.Pinned {
					pin = p.tr("取消置顶", "Unpin")
				}
				kind := "MCP"
				if item.Kind == "skill" {
					kind = "Skill"
				}
				actions := []core.Node{v.Button(pin, func() { p.run(func() (string, error) { return "", p.store.SetPinned(item.ID, !item.Pinned) }, false) }, disabled), v.Button(p.tr("编辑", "Edit"), func() { p.edit(item) }, disabled)}
				if item.Kind == "mcp" {
					actions = append(actions, v.Button(p.tr("检测可用性", "Check availability"), func() { p.startProbe(item) }, disabled))
				} else {
					actions = append(actions, v.Button(p.tr("打开目录", "Open folder"), func() {
						p.run(func() (string, error) {
							path, err := p.store.SkillDirectory(item.ID)
							if err != nil {
								return "", err
							}
							return "", p.openFolder(path)
						}, false)
					}, disabled))
				}
				identity := w.Row(core.Text{Content: item.Name, Style: core.Style{FontWeight: core.FontWeightSemibold}}, v.Muted(kind))
				row := w.Column(w.Spread(identity, w.Row(actions...)))
				if item.Description != "" {
					row.Children = append(row.Children, v.Muted(item.Description))
				}
				row.Style.Borders = core.Edges{Bottom: 1}
				row.Style.BorderColor = v.Scope.Palette().Border
				row.Role, row.Label = core.RoleCheckbox, item.Name
				row.Disabled = func() bool { return disabled }
				row.Checked = func() bool { return p.selected[item.ID] }
				row.OnPress = func(core.PressEvent) { p.selected[item.ID] = !p.selected[item.ID]; v.Refresh() }
				row.Style.Padding = core.All(6)
				row.Style.CornerRadius = 6
				hover := core.NewSignal(v.Scope, false)
				row.OnMouseEnter = func(core.MouseEvent) { hover.Set(true) }
				row.OnMouseLeave = func(core.MouseEvent) { hover.Set(false) }
				row.StyleFunc = func() core.Style {
					palette := v.Scope.Palette()
					color := palette.Surface
					if p.selected[item.ID] {
						color = palette.SurfacePressed
					} else if hover.Get() {
						color = palette.SurfaceHover
					}
					return core.Style{Background: color}
				}
				return row
			}))
		}))
	browser.Style.FlexGrow = 1
	browser.Style.FlexShrink = 1
	browser.Style.FlexBasis = core.Pt(0)
	browse := v.Directory(p.tr("浏览", "Browse"), &p.project, p.fail)
	agent := w.Select("Agent", p.agent, []string{"codex", "pi"}, func(s string) { p.agent = s; v.Refresh() })
	count := len(p.ids())
	deploy := v.Panel(w.Spread(w.Heading(p.tr("项目部署", "Project deployment")), v.Muted(fmt.Sprintf(p.tr("已选 %d 项", "%d selected"), count))),
		w.Spread(w.Text(p.tr("项目目录", "Project directory")), browse),
		w.Input(p.tr("项目目录", "Project directory"), &p.project, false),
		w.Text(p.tr("目标工具", "Target agent")), agent, v.Primary(p.tr("复制到项目", "Copy to project"), p.nativeDeploy, disabled || count == 0))
	if count > 0 {
		deploy.Children = append(deploy.Children, w.Row(v.Button(p.tr("取消选择", "Clear selection"), p.ClearSelection, disabled),
			v.Button(p.tr("从库移除", "Remove from library"), func() { p.removeOpen = true }, disabled)))
	}
	if status := p.status(); status != "" {
		deploy.Children = append(deploy.Children, w.Text(status))
	}
	if v.Scope.Env().Window().Width >= 770 {
		deploy.Style.Width = core.Pt(244)
		right := core.Scroll{Style: core.Style{Width: core.Pt(244)}, Children: []core.Node{deploy}}
		return core.View{Style: core.Style{Direction: core.Row, FlexGrow: 1, Gap: 12}, Children: []core.Node{browser, right}}
	}
	if p.compactDeployment {
		return w.Column(w.Row(v.Button(p.tr("返回资源列表", "Back to resources"), func() { p.compactDeployment = false })), w.Form(deploy))
	}
	open := v.Button(p.tr("项目部署", "Project deployment"), func() { p.compactDeployment = true })
	return w.Column(browser, w.Spread(v.Muted(fmt.Sprintf(p.tr("已选 %d 项", "%d selected"), count)), open))
}

func (p *Panel) nativeDeploy() {
	ids, project, agent := p.ids(), strings.TrimSpace(p.project), p.agent
	if project == "" {
		p.failure = p.tr("请选择项目目录", "Choose a project directory")
		return
	}
	format := p.tr("已复制 %d 个 MCP、%d 个 Skill", "Copied %d MCPs and %d skills")
	p.run(func() (string, error) {
		r, err := p.store.Deploy(project, agent, ids)
		msg := fmt.Sprintf(format, r.MCP, r.Skills)
		if r.Backup != "" {
			msg += "  " + r.Backup
		}
		return msg, err
	}, false)
}

func (p *Panel) nativeDialog(v w.Page) core.Node {
	cancel := v.Button(p.tr("取消", "Cancel"), func() { p.editOpen, p.skillOpen, p.removeOpen = false, false, false }, p.Busy)
	switch {
	case p.editOpen:
		return v.Editor("MCP", w.Field(p.tr("名称", "Name"), &p.name, false), w.Field(p.tr("说明", "Description"), &p.description, false),
			w.Field(p.tr("配置（YAML）", "Configuration (YAML)"), &p.config, true), w.Text(p.status()), w.Row(cancel, v.Primary(p.tr("保存", "Save"), func() {
				id, name, desc, cfg := p.editID, p.name, p.description, p.config
				msg := p.tr("MCP 已保存", "MCP saved")
				p.run(func() (string, error) { return msg, p.store.SaveMCP(id, name, desc, cfg) }, true)
			}, p.Busy)))
	case p.skillOpen:
		return v.Editor(strings.TrimSpace("Skill  "+p.skillName), w.Field(p.tr("包含 SKILL.md 的目录", "Directory containing SKILL.md"), &p.source, false), v.Directory(p.tr("选择目录", "Choose directory"), &p.source, p.fail),
			w.Text(p.status()), w.Row(cancel, v.Button(p.tr("保存", "Save"), func() {
				id, source := p.skillID, p.source
				msg := p.tr("Skill 已保存", "Skill saved")
				p.run(func() (string, error) {
					if id != "" {
						return msg, p.store.UpdateSkill(id, source)
					}
					return msg, p.store.ImportSkill(source)
				}, true)
			}, p.Busy)))
	case p.removeOpen:
		return v.Editor(p.tr("移除资源", "Remove resources"), w.Text(p.tr("从资源库移除所选条目？已复制到项目的内容会保留。", "Remove selected resources? Existing project copies are kept.")), w.Text(p.status()), w.Row(cancel,
			v.Button(p.tr("确认移除", "Confirm removal"), func() { ids := p.ids(); p.run(func() (string, error) { return "", p.store.Remove(ids) }, true) }, p.Busy)))
	default:
		nodes := []core.Node{w.Text(p.tr("MCP 可用性检测", "MCP availability check") + "  " + p.probeName)}
		if p.probing {
			return w.Column(append(nodes, w.Text(p.status()), v.Button(p.tr("取消检测", "Cancel check"), p.probeCancel))...)
		}
		nodes = append(nodes, w.Text(p.probeError), v.Button(p.tr("关闭", "Close"), func() { p.probeOpen = false }))
		if p.probeError == "" {
			r := p.probeResult
			nodes = append(nodes, w.Text(fmt.Sprintf(p.tr("%s · %d 个工具", "%s · %d tools"), r.Server, len(r.Tools))))
			if r.Truncated {
				nodes = append(nodes, w.Text(p.tr("已达到工具列表上限", "Tool list limit reached")))
			}
			nodes = append(nodes, w.List(len(r.Tools), 1, 68, func(i int) any { return i }, func(i int) core.Node { return w.Column(w.Text(r.Tools[i].Name), w.Text(r.Tools[i].Description)) }))
		}
		return w.Column(nodes...)
	}
}

func (p *Panel) NativeImport(v w.Page) core.Node {
	nodes := []core.Node{w.Field(p.tr("magpie 数据目录", "magpie data directory"), &p.magpieSource, false),
		w.Row(v.Directory(p.tr("浏览", "Browse"), &p.magpieSource, p.fail), v.Button(p.tr("从 magpie 导入 MCP 和 Skill", "Import MCPs and skills from magpie"), p.importMagpie, p.Busy || p.store == nil)), w.Text(p.status())}
	if r := p.importReport; r != nil {
		nodes = append(nodes, w.Text(fmt.Sprintf(p.tr("导入 %d 个 MCP、%d 个 Skill，跳过 %d 项，失败 %d 项", "Imported %d MCPs and %d skills, skipped %d, failed %d"), r.MCP, r.Skills, r.Skipped, r.Failed)))
		const size = 20
		pages := max(1, (len(r.Details)+size-1)/size)
		p.importPage = min(p.importPage, pages-1)
		if pages > 1 {
			nodes = append(nodes, w.Row(v.Button(p.tr("上一页", "Previous"), func() { p.importPage-- }, p.importPage == 0), w.Text(fmt.Sprintf("%d / %d", p.importPage+1, pages)), v.Button(p.tr("下一页", "Next"), func() { p.importPage++ }, p.importPage+1 == pages)))
		}
		for _, line := range r.Details[p.importPage*size : min((p.importPage+1)*size, len(r.Details))] {
			nodes = append(nodes, w.Text(line))
		}
	}
	return w.Form(nodes...)
}
