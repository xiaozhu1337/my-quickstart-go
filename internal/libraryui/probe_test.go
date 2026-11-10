package libraryui

import (
	"context"
	"errors"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
	"my-quickstart-go/internal/library"
	"my-quickstart-go/internal/mcpcheck"
)

func TestProbeButtonResultsAndSelection(t *testing.T) {
	p := New(filepath.Join(t.TempDir(), "library"))
	if err := p.store.SaveMCP("", "sample", "", "command: probe"); err != nil {
		t.Fatal(err)
	}
	p.items, _ = p.store.Items()
	p.selected[p.items[0].ID] = true
	p.checkMCP = func(ctx context.Context, config library.Server) (mcpcheck.Result, error) {
		if config.Command != "probe" {
			t.Fatal("wrong config")
		}
		return mcpcheck.Result{Server: "test-server 1.0", Tools: []mcpcheck.Tool{{Name: "search", Description: "Search project files"}, {Name: "read_file", Description: "Read a selected file"}}}, nil
	}
	tt := ui.NewTester(p.View, 960, 620)
	click(t, tt, "检测可用性")
	if !tt.HasText("2 个工具") || !tt.HasText("search") || !tt.HasText("连接已关闭") || p.Busy || len(p.ids()) != 1 {
		t.Fatal("unexpected check state", tt.Texts())
	}
	if path := os.Getenv("QUICKSTART_PROBE_PREVIEW"); path != "" {
		f, err := os.Create(path)
		if err != nil {
			t.Fatal(err)
		}
		if err := png.Encode(f, tt.Image()); err != nil {
			t.Fatal(err)
		}
		f.Close()
	}
	click(t, tt, "关闭")
	p.checkMCP = func(context.Context, library.Server) (mcpcheck.Result, error) {
		return mcpcheck.Result{}, errors.New("server unavailable")
	}
	click(t, tt, "检测可用性")
	if !tt.HasText("server unavailable") || p.Busy {
		t.Fatal("missing error")
	}
	click(t, tt, "关闭")
	p.items[0].Kind = "skill"
	tt.Frame()
	if _, ok := tt.Find("检测可用性"); ok {
		t.Fatal("skill has probe button")
	}
}

func TestClosingProbeCancels(t *testing.T) {
	p := New(filepath.Join(t.TempDir(), "library"))
	ctx, cancel := context.WithCancel(context.Background())
	p.probing, p.Busy, p.probeOpen = true, true, false
	p.probeCancel = cancel
	ui.NewTester(p.View, 960, 620)
	if ctx.Err() == nil {
		t.Fatal("closing dialog did not cancel")
	}
	if !p.Busy {
		t.Fatal("must remain busy until cleanup finishes")
	}
}

func TestProbeErrorsHideCredentials(t *testing.T) {
	config := library.Server{URL: "http://name:password@example.com/mcp?token=private", Headers: map[string]string{"Authorization": "Bearer secret"}, Env: map[string]string{"API_KEY": "sensitive"}}
	got := redactProbeError(config.URL+" Bearer secret sensitive", config)
	for _, secret := range []string{"password", "private", "secret", "sensitive"} {
		if strings.Contains(got, secret) {
			t.Fatal("credential exposed", got)
		}
	}
}
