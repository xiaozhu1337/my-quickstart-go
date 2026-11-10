//go:build windows

package mcpcheck

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"syscall"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"golang.org/x/sys/windows"
	"my-quickstart-go/internal/library"
)

func TestMCPHelper(t *testing.T) {
	if os.Getenv("QUICKSTART_MCP_HELPER") != "1" {
		return
	}
	if os.Getenv("QUICKSTART_MCP_LEAF") == "1" {
		_ = os.WriteFile(os.Getenv("QUICKSTART_MCP_PID"), []byte(strconv.Itoa(os.Getpid())), 0600)
		_, _ = os.Stdout.Write([]byte{1})
		<-time.After(10 * time.Second)
		os.Exit(0)
	}
	child := exec.Command(os.Args[0], "-test.run=^TestMCPHelper$")
	child.Env = append(os.Environ(), "QUICKSTART_MCP_LEAF=1")
	child.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: windows.CREATE_NO_WINDOW}
	ready, err := child.StdoutPipe()
	if err != nil {
		os.Exit(2)
	}
	if err := child.Start(); err != nil {
		os.Exit(2)
	}
	if _, err := io.ReadFull(ready, make([]byte, 1)); err != nil {
		os.Exit(2)
	}
	ready.Close()
	_ = os.WriteFile(os.Getenv("QUICKSTART_MCP_PARENT_PID"), []byte(strconv.Itoa(os.Getpid())), 0600)
	if os.Getenv("QUICKSTART_MCP_HANG") == "1" {
		_, _ = io.Copy(io.Discard, os.Stdin)
		os.Exit(0)
	}
	server := mcp.NewServer(&mcp.Implementation{Name: "stdio-fixture", Version: "1"}, nil)
	server.AddTool(&mcp.Tool{Name: "environment", Description: os.Getenv("PROBE_VALUE"), InputSchema: json.RawMessage(`{"type":"object"}`)}, func(context.Context, *mcp.CallToolRequest) (*mcp.CallToolResult, error) { os.Exit(3); return nil, nil })
	_ = server.Run(context.Background(), &mcp.StdioTransport{})
	os.Exit(0)
}

func TestCommandProbeReapsProcessTree(t *testing.T) {
	for _, hang := range []bool{false, true} {
		t.Run(strconv.FormatBool(hang), func(t *testing.T) {
			dir := t.TempDir()
			childPID := filepath.Join(dir, "child.pid")
			parentPID := filepath.Join(dir, "parent.pid")
			config := library.Server{Command: os.Args[0], Args: []string{"-test.run=^TestMCPHelper$"}, Cwd: dir, Env: map[string]string{"QUICKSTART_MCP_HELPER": "1", "QUICKSTART_MCP_PID": childPID, "QUICKSTART_MCP_PARENT_PID": parentPID, "PROBE_VALUE": "configured value"}}
			if hang {
				config.Env["QUICKSTART_MCP_HANG"] = "1"
			}
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			result, err := Check(ctx, config)
			if hang && err == nil {
				t.Fatal("expected timeout")
			}
			if !hang && (err != nil || len(result.Tools) != 1 || result.Tools[0].Description != "configured value") {
				t.Fatalf("%+v %v", result, err)
			}
			for _, path := range []string{parentPID, childPID} {
				b, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				pid, _ := strconv.Atoi(string(b))
				h, err := windows.OpenProcess(windows.SYNCHRONIZE, false, uint32(pid))
				if err != nil {
					continue
				}
				state, err := windows.WaitForSingleObject(h, 1000)
				windows.CloseHandle(h)
				if err != nil || state != windows.WAIT_OBJECT_0 {
					t.Fatalf("process %d still alive: %d %v", pid, state, err)
				}
			}
		})
	}
}

func TestBatchCommandProbe(t *testing.T) {
	dir := t.TempDir()
	wrapper := filepath.Join(dir, "mcp launcher.cmd")
	if err := os.WriteFile(wrapper, []byte("@\""+os.Args[0]+"\" -test.run=^TestMCPHelper$\r\n"), 0600); err != nil {
		t.Fatal(err)
	}
	result, err := Check(context.Background(), library.Server{Command: wrapper, Cwd: dir, Env: map[string]string{
		"QUICKSTART_MCP_HELPER": "1", "QUICKSTART_MCP_PID": filepath.Join(dir, "child.pid"),
		"QUICKSTART_MCP_PARENT_PID": filepath.Join(dir, "parent.pid"), "PROBE_VALUE": "batch wrapper",
	}})
	if err != nil || len(result.Tools) != 1 || result.Tools[0].Description != "batch wrapper" {
		t.Fatalf("batch command failed: %+v %v", result, err)
	}
}
