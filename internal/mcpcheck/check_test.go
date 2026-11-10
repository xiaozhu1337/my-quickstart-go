package mcpcheck

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"my-quickstart-go/internal/library"
)

func TestHTTPListsPagesAndClosesSession(t *testing.T) {
	for _, scenario := range []string{"success", "empty", "error", "loop"} {
		t.Run(scenario, func(t *testing.T) {
			var deleted, calls, auth atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Header.Get("Authorization") == "Bearer test" {
					auth.Add(1)
				}
				if r.Method == "DELETE" {
					deleted.Add(1)
					w.WriteHeader(204)
					return
				}
				var request struct {
					ID     any    `json:"id"`
					Method string `json:"method"`
					Params struct {
						Cursor string `json:"cursor"`
					} `json:"params"`
				}
				if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
					t.Error(err)
					w.WriteHeader(400)
					return
				}
				if request.ID == nil {
					w.WriteHeader(202)
					return
				}
				w.Header().Set("Content-Type", "application/json")
				response := map[string]any{"jsonrpc": "2.0", "id": request.ID}
				switch request.Method {
				case "initialize":
					w.Header().Set("Mcp-Session-Id", "probe-session")
					response["result"] = map[string]any{"protocolVersion": "2025-11-25", "capabilities": map[string]any{"tools": map[string]any{}}, "serverInfo": map[string]string{"name": "fixture", "version": "1"}}
				case "tools/list":
					calls.Add(1)
					result := map[string]any{"tools": []any{map[string]any{"name": "first", "description": "First tool", "inputSchema": map[string]string{"type": "object"}}}}
					if request.Params.Cursor == "" {
						result["nextCursor"] = "page2"
					} else {
						result["tools"] = []any{map[string]any{"name": "second", "inputSchema": map[string]string{"type": "object"}}}
					}
					if scenario == "empty" {
						result = map[string]any{"tools": []any{}}
					}
					if scenario == "loop" {
						result["nextCursor"] = "page2"
					}
					response["result"] = result
					if scenario == "error" {
						delete(response, "result")
						response["error"] = map[string]any{"code": -32603, "message": "listing failed"}
					}
				default:
					if request.Method == "tools/call" {
						t.Error("probe invoked a tool")
					}
					response["error"] = map[string]any{"code": -32601, "message": "unsupported"}
				}
				_ = json.NewEncoder(w).Encode(response)
			}))
			defer server.Close()
			result, err := Check(context.Background(), library.Server{URL: server.URL, Headers: map[string]string{"Authorization": "Bearer test"}})
			if (err != nil) != (scenario == "error" || scenario == "loop") {
				t.Fatalf("unexpected result: %+v %v", result, err)
			}
			if deleted.Load() != 1 {
				t.Fatalf("session not terminated: %d", deleted.Load())
			}
			if auth.Load() < 3 {
				t.Fatal("headers missing")
			}
			if scenario == "success" && (len(result.Tools) != 2 || result.Tools[1].Name != "second" || calls.Load() != 2) {
				t.Fatalf("pagination failed: %+v", result)
			}
			if scenario == "empty" && len(result.Tools) != 0 {
				t.Fatal("unexpected tools")
			}
		})
	}
}

func TestHTTPCancellation(t *testing.T) {
	finished := make(chan struct{})
	release := make(chan struct{})
	var once sync.Once
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer once.Do(func() { close(finished) })
		_, _ = io.Copy(io.Discard, r.Body)
		select {
		case <-r.Context().Done():
		case <-release:
		}
	}))
	defer server.Close()
	defer close(release)
	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()
	_, err := Check(ctx, library.Server{URL: server.URL})
	if err == nil {
		t.Fatal("expected deadline error")
	}
	select {
	case <-finished:
	case <-time.After(time.Second):
		t.Fatal("request remained connected")
	}
}
