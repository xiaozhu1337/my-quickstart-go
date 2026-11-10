package libraryui

import (
	"fmt"
	"log"
	"strings"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

func (p *Panel) SettingsView(c *ui.Context) {
	ui.Column(c).Gap(12).FillWidth().Children(func() {
		ui.Text(c, p.tr("从 magpie 导入", "Import from magpie")).Bold()
		ui.Text(c, p.tr("导入现有 MCP 配置和完整 Skill 目录，同名资源会跳过。", "Import MCP configs and complete skill folders; existing names are skipped."))
		ui.Text(c, p.tr("默认读取本机 magpie。便携版可选择安装目录或 data 目录。", "Uses the local magpie by default. For portable installs, choose the install or data directory.")).FontSize(12).TextColor(c.Theme().TextMuted)
		ui.Row(c).Gap(8).Children(func() {
			ui.TextInput(c.Key("magpie-source"), &p.magpieSource).Label(p.tr("magpie 数据目录", "magpie data directory")).Grow(1).Disabled(p.Busy)
			if ui.Button(c, p.tr("浏览", "Browse")).Disabled(p.Busy).Clicked() {
				p.pickMagpie()
			}
		})
		if ui.PrimaryButton(c, p.tr("从 magpie 导入 MCP 和 Skill", "Import MCPs and skills from magpie")).Disabled(p.Busy || p.store == nil).Clicked() {
			p.importMagpie()
		}
		if p.Busy {
			ui.Text(c, p.tr("正在导入…", "Importing…"))
		}
		if p.failure != "" {
			ui.Text(c, p.failure).Tooltip(p.failure).TextColor(c.Theme().Danger)
		}
		if r := p.importReport; r != nil {
			ui.Text(c, fmt.Sprintf(p.tr("已导入 %d 个 MCP、%d 个 Skill；跳过 %d 项，失败 %d 项。", "Imported %d MCPs and %d skills; skipped %d, failed %d."), r.MCP, r.Skills, r.Skipped, r.Failed)).Bold()
			if r.MCP+r.Skills > 0 {
				ui.Text(c, p.tr("可在 MCP / Skills 中查看并选择要链接到项目的资源。", "Open MCP / Skills to select resources and link them to a project."))
			}
			const size = 20
			pages := max(1, (len(r.Details)+size-1)/size)
			p.importPage = min(p.importPage, pages-1)
			if len(r.Details) > size {
				ui.Row(c).Gap(8).Children(func() {
					if ui.Button(c, p.tr("上一页", "Previous")).Disabled(p.importPage == 0).Clicked() {
						p.importPage--
					}
					ui.Text(c, fmt.Sprintf("%d / %d", p.importPage+1, pages))
					if ui.Button(c, p.tr("下一页", "Next")).Disabled(p.importPage == pages-1).Clicked() {
						p.importPage++
					}
				})
			}
			for _, detail := range r.Details[min(p.importPage*size, len(r.Details)):min((p.importPage+1)*size, len(r.Details))] {
				ui.Text(c, detail).FontSize(12)
			}
		}
	})
}

func (p *Panel) importMagpie() {
	source := strings.TrimSpace(p.magpieSource)
	p.importReport = nil
	p.importPage = 0
	p.run(func() (string, error) {
		report, err := p.store.ImportMagpie(source)
		if report.Failed > 0 {
			log.Printf("magpie import: %s", strings.Join(report.Details, "; "))
		}
		finish := func() { p.importReport = &report }
		if p.Window != nil {
			p.Window.Update(finish)
		} else {
			finish()
		}
		return "", err
	}, false)
}

func (p *Panel) pickMagpie() {
	if p.Window == nil || p.Busy {
		return
	}
	p.Busy = true
	title := p.tr("选择 magpie 数据目录", "Choose magpie data directory")
	go func() {
		paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{Parent: p.Window, Title: title, Directory: true})
		p.Window.Update(func() {
			p.Busy = false
			if err != nil {
				p.fail(err)
				return
			}
			if len(paths) > 0 {
				p.magpieSource = paths[0]
			}
		})
	}()
}
