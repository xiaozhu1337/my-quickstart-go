package libraryui

import (
	"strings"

	"github.com/egoist/mygo/ui"
)

func (p *Panel) dialogs(c *ui.Context) {
	_, height := c.Size()
	ui.Modal(c, &p.editOpen, func() {
		ui.Text(c, p.tr("保存 MCP 到资源库", "Save MCP to library")).FontSize(20).Bold()
		ui.Scroll(c).Grow(1).FillWidth().Children(func() {
			ui.Column(c).FillWidth().Gap(8).Children(func() {
				ui.Text(c, p.tr("名称", "Name"))
				ui.TextInput(c.Key("mcp-name"), &p.name).Label(p.tr("MCP 名称", "MCP name")).Disabled(p.Busy).AutoFocus()
				ui.Text(c, p.tr("说明", "Description"))
				ui.TextInput(c.Key("mcp-description"), &p.description).Label(p.tr("MCP 说明", "MCP description")).Disabled(p.Busy)
				ui.Text(c, p.tr("配置（YAML）：注释中有参数、环境变量、工作目录及 HTTP 示例，取消 # 即可启用。", "YAML config: comments show arguments, environment, working directory and HTTP examples; remove # to enable.")).FontSize(12)
				config := ui.TextArea(c.Key("mcp-config"), &p.config).Label(p.tr("MCP 配置", "MCP configuration")).Height(280).Disabled(p.Busy)
				if p.resetMCP {
					config.SetTextSelection(0, 0)
					p.resetMCP = false
				}
				ui.Text(c, p.tr("编辑后需重新链接到项目，才会更新该项目的 MCP 配置。", "Link again after editing to update the project's MCP config.")).FontSize(11).TextColor(c.Theme().TextMuted)
			})
		})
		p.dialogError(c)
		ui.Row(c).Gap(8).Justify(ui.End).Children(func() {
			if ui.Button(c, p.tr("取消", "Cancel")).Disabled(p.Busy).Clicked() {
				p.editOpen = false
			}
			if ui.PrimaryButton(c, p.tr("保存", "Save")).Disabled(p.Busy || strings.TrimSpace(p.name) == "").Clicked() {
				id, name, description, config := p.editID, p.name, p.description, p.config
				message := p.tr("MCP 已保存", "MCP saved")
				p.run(func() (string, error) {
					return message, p.store.SaveMCP(id, name, description, config)
				}, true)
			}
		})
	}).Label("MCP").Width(760).Height(min(650, height-32))
	ui.Modal(c, &p.skillOpen, func() {
		ui.Text(c, p.tr("导入 Skill", "Import skill")).FontSize(20).Bold()
		ui.Text(c, p.tr("选择包含 SKILL.md 的目录；整个目录会复制并统一保存到资源库。", "Choose a directory containing SKILL.md; its contents are copied into the library."))
		ui.Row(c).Gap(8).Children(func() {
			ui.TextInput(c.Key("skill-source"), &p.source).Label(p.tr("Skill 目录", "Skill directory")).Grow(1).Disabled(p.Busy)
			if ui.Button(c, p.tr("选择目录", "Choose directory")).Disabled(p.Busy).Clicked() {
				p.pick(true)
			}
		})
		p.dialogError(c)
		ui.Row(c).Gap(8).Justify(ui.End).Children(func() {
			if ui.Button(c, p.tr("取消", "Cancel")).Disabled(p.Busy).Clicked() {
				p.skillOpen = false
			}
			if ui.PrimaryButton(c, p.tr("导入并保存", "Import and save")).Disabled(p.Busy || strings.TrimSpace(p.source) == "").Clicked() {
				source := p.source
				message := p.tr("Skill 已保存", "Skill saved")
				p.run(func() (string, error) { return message, p.store.ImportSkill(source) }, true)
			}
		})
	}).Label("Skill").Width(680)
	ui.Modal(c, &p.removeOpen, func() {
		ui.Text(c, p.tr("从资源库移除所选条目？", "Remove selected library entries?")).Bold()
		ui.Text(c, p.tr("已链接到项目的配置和 Skill 文件会保留，可继续使用。", "Existing project configs and skill files are kept and remain usable."))
		p.dialogError(c)
		ui.Row(c).Gap(8).Justify(ui.End).Children(func() {
			if ui.Button(c, p.tr("取消", "Cancel")).Disabled(p.Busy).Clicked() {
				p.removeOpen = false
			}
			if ui.Button(c, p.tr("确认移除", "Confirm removal")).Disabled(p.Busy).Clicked() {
				ids := p.ids()
				message := p.tr("所选条目已移除", "Selected entries removed")
				p.run(func() (string, error) {
					return message, p.store.Remove(ids)
				}, true)
			}
		})
	}).Width(540)
}

func (p *Panel) dialogError(c *ui.Context) {
	if p.Busy {
		ui.Text(c, p.tr("正在处理…", "Working…"))
	}
	if p.failure != "" {
		ui.Text(c, p.failure).Tooltip(p.failure).TextColor(c.Theme().Danger).MaxHeight(70)
	}
}
