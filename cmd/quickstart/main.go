package main

import (
	"flag"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/nsgo-dev/nsgo"
	"github.com/nsgo-dev/nsgo/core"
	"my-quickstart-go/internal/launcher"
	"my-quickstart-go/internal/logging"
	"my-quickstart-go/internal/model"
	"my-quickstart-go/internal/store"
)

func main() {
	data := flag.String("data", "", "Path to launcher database")
	flag.Parse()
	var closeApp func()
	nsgo.Run(func(cx *core.Scope) {
		var err error
		closeApp, err = start(cx, *data)
		if err != nil {
			log.Print(err)
			cx.OpenWindow(core.WindowOptions{Title: "My Quickstart Go", Width: 560, Height: 200}, func(*core.Scope) core.Node {
				return core.Text{Content: err.Error(), Style: core.Style{Padding: core.All(20)}}
			})
		}
	}, nsgo.InstanceKey("my-quickstart-go"))
	if closeApp != nil {
		closeApp()
	}
}

func start(cx *core.Scope, path string) (func(), error) {
	exe, err := os.Executable()
	if err != nil {
		return nil, err
	}
	logs, err := logging.Open(filepath.Dir(exe))
	if err != nil {
		return nil, err
	}
	log.SetOutput(logs)
	log.SetFlags(0)
	if strings.HasPrefix(strings.ToLower(cx.Env().Locale()), "zh") {
		model.DefaultLanguage = "zh-CN"
	}
	closeLogs := func() { logs.Close() }
	if path == "" {
		path, err = store.DefaultPath()
		if err != nil {
			return closeLogs, err
		}
	}
	db, err := store.Open(path)
	if err != nil {
		return closeLogs, err
	}
	state, err := db.Load()
	if err != nil {
		db.Close()
		return closeLogs, err
	}
	app := launcher.New(state, db)
	app.SetLibrary(filepath.Join(filepath.Dir(db.Path()), "library"))
	app.Mount(cx)
	return func() { app.Close(); db.Close(); logs.Close() }, nil
}
