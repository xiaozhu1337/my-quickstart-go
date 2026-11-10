package libraryui

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/url"
	"strings"

	"github.com/egoist/mygo/ui"
	"my-quickstart-go/internal/library"
	"my-quickstart-go/internal/mcpcheck"
)

type probeState struct {
	probeOpen, probing    bool
	probeName, probeError string
	probeResult           mcpcheck.Result
	probeCancel           context.CancelFunc
	probeList             ui.ListState
	checkMCP              func(context.Context, library.Server) (mcpcheck.Result, error)
}

func (p *Panel) startProbe(item library.Item) {
	if p.Busy || item.MCP == nil {
		return
	}
	p.probeOpen, p.probing, p.Busy = true, true, true
	p.probeName, p.probeError = item.Name, ""
	p.probeResult = mcpcheck.Result{}
	p.probeList = ui.ListState{}
	ctx, cancel := context.WithCancel(context.Background())
	p.probeCancel = cancel
	check := p.checkMCP
	if check == nil {
		check = mcpcheck.Check
	}
	config := *item.MCP
	canceled := p.tr("检测已取消，临时连接已关闭。", "Check canceled; temporary connection closed.")
	timeout := p.tr("检测超时（10 秒），临时连接已关闭。请检查服务启动速度、地址和认证配置。", "Check timed out (10 seconds); temporary connection closed. Check startup time, endpoint and authentication.")
	work := func() {
		result, err := check(ctx, config)
		cancel()
		message := ""
		if err != nil {
			message = redactProbeError(err.Error(), config)
			if errors.Is(err, context.Canceled) {
				message = canceled
			}
			if errors.Is(err, context.DeadlineExceeded) {
				message = timeout
			}
			log.Printf("MCP availability check: %s", message)
		}
		finish := func() {
			p.probing, p.Busy = false, false
			p.probeCancel = nil
			p.probeResult, p.probeError = result, message
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

func redactProbeError(message string, config library.Server) string {
	if config.URL != "" {
		if u, err := url.Parse(config.URL); err == nil {
			u.User = nil
			u.RawQuery = ""
			u.Fragment = ""
			message = strings.ReplaceAll(message, config.URL, u.String())
		}
	}
	for _, values := range []map[string]string{config.Headers, config.Env} {
		for _, value := range values {
			if len(value) >= 4 {
				message = strings.ReplaceAll(message, value, "[redacted]")
			}
		}
	}
	return message
}

func (p *Panel) probeDialog(c *ui.Context) {
	_, height := c.Size()
	ui.Modal(c, &p.probeOpen, func() {
		ui.Text(c, p.tr("MCP 可用性检测", "MCP availability check")).FontSize(20).Bold()
		ui.Text(c, p.probeName).Bold()
		if p.probing {
			ui.Text(c, p.tr("正在连接并读取工具列表（最多 10 秒）…", "Connecting and reading tools (up to 10 seconds)…"))
			ui.Box(c).Grow(1)
		} else if p.probeError != "" {
			ui.Scroll(c).Grow(1).FillWidth().Children(func() { ui.Text(c, p.probeError).TextColor(c.Theme().Danger) })
		} else {
			ui.Text(c, fmt.Sprintf(p.tr("检测成功：%d 个工具 · 连接已关闭", "Check succeeded: %d tools · connection closed"), len(p.probeResult.Tools))).TextColor(c.Theme().Accent)
			if p.probeResult.Server != "" {
				ui.Text(c, p.probeResult.Server).TextColor(c.Theme().TextMuted)
			}
			if len(p.probeResult.Tools) == 0 {
				ui.Text(c, p.tr("服务可连接，但未提供工具。", "The server is reachable but exposes no tools."))
			}
			if p.probeResult.Truncated {
				ui.Text(c, p.tr("工具数量或分页次数达到上限，仅显示已获取的部分。", "Tool or page limit reached; showing partial results."))
			}
			ui.List(c, &p.probeList, len(p.probeResult.Tools), func(n int) {
				tool := p.probeResult.Tools[n]
				ui.Column(c).Padding(10).Margin(0, 0, 8, 0).Gap(4).Background(c.Theme().Surface).Radius(6).Children(func() {
					ui.Text(c, tool.Name).Bold().SingleLine().Tooltip(tool.Name)
					if tool.Description != "" {
						ui.Text(c, tool.Description).MaxHeight(80).Tooltip(tool.Description).TextColor(c.Theme().TextMuted)
					}
				})
			}).Grow(1).Padding(0, 16, 0, 0)
		}
		ui.Text(c, p.tr("仅握手和列出工具，不调用工具；结束后释放连接及本次启动的进程。", "Only connects and lists tools; releases the connection and owned processes when finished.")).FontSize(11).TextColor(c.Theme().TextMuted)
		ui.Row(c).Justify(ui.End).Children(func() {
			if p.probing {
				if ui.Button(c, p.tr("取消检测", "Cancel check")).Clicked() {
					p.probeCancel()
				}
			} else if ui.Button(c, p.tr("关闭", "Close")).Clicked() {
				p.probeOpen = false
			}
		})
	}).Width(760).Height(min(580, height-32))
	if !p.probeOpen && p.probing && p.probeCancel != nil {
		p.probeCancel()
	}
}
