package mcpcheck

import (
	"context"
	"os"
	"os/exec"
	"sync"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"my-quickstart-go/internal/library"
)

func commandTransport(ctx context.Context, config library.Server) (mcp.Transport, func() error, error) {
	cmd := exec.Command(config.Command, config.Args...)
	cmd.Dir = config.Cwd
	cmd.Env = os.Environ()
	for k, v := range config.Env {
		cmd.Env = append(cmd.Env, k+"="+v)
	}
	// A nil Stderr goes to the null device without a copying goroutine.
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, nil, err
	}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		stdout.Close()
		return nil, nil, err
	}
	terminate, err := startProcess(cmd)
	if err != nil {
		stdout.Close()
		stdin.Close()
		return nil, nil, err
	}
	var once sync.Once
	var cleanupErr error
	kill := func() { once.Do(func() { cleanupErr = terminate() }) }
	stop := context.AfterFunc(ctx, kill)
	cleanup := func() error {
		stop()
		stdin.Close()
		kill()
		stdout.Close()
		// The process was explicitly terminated; its exit code is not a probe error.
		_ = cmd.Wait()
		return cleanupErr
	}
	return &mcp.IOTransport{Reader: stdout, Writer: stdin, MaxLineLength: 4 << 20}, cleanup, nil
}
