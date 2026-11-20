package main

import (
	"flag"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/egoist/mygo"
	"my-quickstart-go/internal/boot"
	"my-quickstart-go/internal/launcher"
	"my-quickstart-go/internal/logging"
	"my-quickstart-go/internal/model"
	"my-quickstart-go/internal/store"
)

func main() {
	data := flag.String("data", "", "Path to launcher database")
	startup := flag.Bool(boot.Flag, false, "Run enabled tasks after Windows sign-in")
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
	path := *data
	if path == "" {
		path, err = store.DefaultPath()
		if err != nil {
			startupError(err)
			return
		}
	}
	path, err = filepath.Abs(path)
	if err != nil {
		startupError(err)
		return
	}
	// Different databases have independent schedules and login registrations.
	defaultPath, _ := store.DefaultPath()
	if boot.Profile(path) != boot.Profile(defaultPath) {
		mygo.App.SetName("My Quickstart Go " + boot.Profile(path))
	}
	if !mygo.App.RequestSingleInstanceLock() {
		return
	}
	mygo.App.SetName("My Quickstart Go")
	db, state, err := store.OpenWithState(path)
	if err != nil {
		startupError(err)
		return
	}
	defer db.Close()
	persistence := &scheduledStore{Store: db, exe: exe}
	registrationErr := persistence.syncRegistration(boot.Enabled(state.Tasks))
	app := launcher.New(state, persistence)
	app.SetLibrary(filepath.Join(filepath.Dir(db.Path()), "library"))
	defer app.Close()
	mygo.App.OnWindowAllClosed(func() {})
	runStartup := func() {
		id, err := boot.ID()
		if err != nil {
			app.ReportStartupError(err)
			return
		}
		app.RunAtBoot(id)
	}
	mygo.App.OnSecondInstance(func(args []string, _ string) {
		if len(args) > 0 && args[0] == "--"+boot.Flag {
			runStartup()
			return
		}
		app.Show()
	})
	mygo.App.WhenReady(func() {
		app.Ready()
		if registrationErr != nil {
			app.ReportStartupError(registrationErr)
		}
		if *startup {
			runStartup()
		}
	})
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
