package libraryui

import (
	"strings"

	"github.com/egoist/mygo/ui"
	"my-quickstart-go/internal/library"
)

func (p *Panel) editSkill(item library.Item) {
	p.failure = ""
	p.run(func() (string, error) {
		document, err := p.store.SkillDocument(item.ID)
		if err != nil {
			return "", err
		}
		open := func() {
			p.skillID, p.skillDocument = item.ID, document
			p.resetSkill = true
			p.skillEditOpen = true
		}
		if p.Window != nil {
			p.Window.Update(open)
		} else {
			open()
		}
		return "", nil
	}, false)
}

func (p *Panel) skillEditor(c *ui.Context) {
	_, height := c.Size()
	ui.Modal(c, &p.skillEditOpen, func() {
		ui.Text(c, p.tr("编辑 Skill", "Edit Skill")).FontSize(20).Bold()
		ui.Text(c, p.tr("编辑 SKILL.md：顶部 name 为名称、description 为说明，下方是指令正文。", "Edit SKILL.md: name and description are in the frontmatter, followed by instructions.")).FontSize(12)
		document := ui.TextArea(c.Key("skill-document"), &p.skillDocument).Label("SKILL.md").Grow(1).FillWidth().Disabled(p.Busy)
		if p.resetSkill {
			document.SetTextSelection(0, 0)
			p.resetSkill = false
		}
		ui.Text(c, p.tr("保存会更新已链接项目中的内容，附带文件保持不变。改名后请重新链接，旧名称的链接会保留。", "Saving updates linked projects and preserves supporting files. After renaming, link again; old links are retained.")).FontSize(11).TextColor(c.Theme().TextMuted)
		p.dialogError(c)
		ui.Row(c).Gap(8).Justify(ui.End).Children(func() {
			if ui.Button(c, p.tr("取消", "Cancel")).Disabled(p.Busy).Clicked() {
				p.skillEditOpen = false
			}
			if ui.PrimaryButton(c, p.tr("保存", "Save")).Disabled(p.Busy || strings.TrimSpace(p.skillDocument) == "").Clicked() {
				id, document := p.skillID, p.skillDocument
				message := p.tr("Skill 已保存", "Skill saved")
				p.run(func() (string, error) { return message, p.store.SaveSkill(id, document) }, true)
			}
		})
	}).Label("Skill editor").Width(800).Height(min(650, height-32))
}
