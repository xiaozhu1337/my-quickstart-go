// Package mcpcheck performs short-lived MCP discovery without invoking tools.
package mcpcheck

import (
	"context"
	"errors"
	"fmt"
	"log"
	"log/slog"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"my-quickstart-go/internal/library"
)

const Timeout = 10 * time.Second
const maxTools = 500

type Tool struct{ Name, Description string }
type Result struct {
	Server    string
	Tools     []Tool
	Truncated bool
}

// Check returns only after the temporary session and owned processes are closed.
func Check(parent context.Context, config library.Server) (result Result, err error) {
	ctx, cancel := context.WithTimeout(parent, Timeout)
	defer cancel()
	var transport mcp.Transport
	var cleanup func() error
	if config.Command != "" && config.URL == "" {
		transport, cleanup, err = commandTransport(ctx, config)
	} else if config.URL != "" && config.Command == "" {
		transport, cleanup, err = httpTransport(config)
	} else {
		return result, errors.New("必须且只能设置 command 或 url")
	}
	if err != nil {
		return result, fmt.Errorf("启动检测失败: %w", err)
	}
	defer func() { err = errors.Join(err, cleanup()) }()
	client := mcp.NewClient(&mcp.Implementation{Name: "my-quickstart-go-check", Version: "1.0"}, &mcp.ClientOptions{
		Capabilities: &mcp.ClientCapabilities{},
		Logger:       slog.New(slog.NewTextHandler(log.Writer(), &slog.HandlerOptions{Level: slog.LevelError})),
	})
	session, err := client.Connect(ctx, transport, nil)
	if err != nil {
		return result, fmt.Errorf("MCP 连接或握手失败: %w", err)
	}
	defer func() {
		if closeErr := session.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("关闭 MCP 连接失败: %w", closeErr))
		}
	}()
	if info := session.InitializeResult(); info != nil && info.ServerInfo != nil {
		result.Server = info.ServerInfo.Name + " " + info.ServerInfo.Version
	}
	cursor := ""
	seen := map[string]bool{}
	for page := 0; page < 50; page++ {
		list, listErr := session.ListTools(ctx, &mcp.ListToolsParams{Cursor: cursor})
		if listErr != nil {
			return result, fmt.Errorf("列出工具失败: %w", listErr)
		}
		if list == nil {
			return result, errors.New("服务返回了空的工具列表响应")
		}
		for _, tool := range list.Tools {
			if tool == nil || tool.Name == "" {
				return result, errors.New("服务返回了无效的工具条目")
			}
			if len(result.Tools) == maxTools {
				result.Truncated = true
				return result, nil
			}
			result.Tools = append(result.Tools, Tool{Name: limitText(tool.Name, 256), Description: limitText(tool.Description, 4000)})
		}
		if list.NextCursor == "" {
			return result, nil
		}
		if seen[list.NextCursor] {
			return result, errors.New("服务返回了重复的分页游标")
		}
		seen[list.NextCursor] = true
		cursor = list.NextCursor
	}
	result.Truncated = true
	return result, nil
}

func limitText(s string, max int) string {
	r := []rune(s)
	if len(r) > max {
		return string(r[:max]) + "…"
	}
	return s
}
