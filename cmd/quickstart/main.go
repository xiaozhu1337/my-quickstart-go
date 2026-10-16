package main

import (
	"flag"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/egoist/mygo"
	"my-quickstart-go/internal/launcher"
	"my-quickstart-go/internal/logging"
	"my-quickstart-go/internal/model"
	"my-quickstart-go/internal/store"
)

func main() {
	data := flag.String("data", "", "Path to launcher database")
	flag.Parse()
	exe, err := os.Executable()
	if err != nil {
		startupError(err)
		return
	}
	logs, err := logging.Open(filepath.Dir(exe))
	if err != nil {
		startupError(err)
		return
	}
	defer logs.Close()
	log.SetOutput(logs)
	log.SetFlags(0)
	mygo.App.SetName("My Quickstart Go")
	if languages := mygo.App.PreferredLanguages(); len(languages) > 0 && strings.HasPrefix(strings.ToLower(languages[0]), "zh") {
		model.DefaultLanguage = "zh-CN"
	}
	if !mygo.App.RequestSingleInstanceLock() {
		return
	}
	path := *data
	if path == "" {
		path, err = store.DefaultPath()
		if err != nil {
			startupError(err)
			return
		}
	}
	db, err := store.Open(path)
	if err != nil {
		startupError(err)
		return
	}
	defer db.Close()
	state, err := db.Load()
	if err != nil {
		startupError(err)
		return
	}
	app := launcher.New(state, db)
	defer app.Close()
	mygo.App.OnWindowAllClosed(func() {})
	mygo.App.OnSecondInstance(func([]string, string) { app.Show() })
	mygo.App.WhenReady(app.Ready)
	mygo.App.OnBeforeQuit(app.BeforeQuit)
	if err := mygo.App.Run(); err != nil {
		log.Print(err)
	}
}

func startupError(err error) {
	log.Print(err)
	mygo.App.WhenReady(func() {
		_, dialogErr := mygo.Dialog.Message(mygo.MessageOptions{Type: mygo.MessageError, Title: "My Quickstart Go", Message: "Unable to start launcher", Detail: err.Error()})
		if dialogErr != nil {
			log.Print(dialogErr)
		}
		mygo.App.Quit()
	})
	if runErr := mygo.App.Run(); runErr != nil {
		log.Print(runErr)
	}
}
