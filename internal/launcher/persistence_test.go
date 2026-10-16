package launcher

import (
	"errors"
	"testing"

	"my-quickstart-go/internal/model"
)

type memoryStore struct {
	state  model.State
	writes int
	fail   bool
}

func (m *memoryStore) Path() string { return "memory" }
func (m *memoryStore) Save(s model.State) error {
	if m.fail {
		return errors.New("write failed")
	}
	m.state = s
	m.writes++
	return nil
}

func TestSaverSnapshotsAndRetries(t *testing.T) {
	p := &memoryStore{}
	s := &saver{store: p, wake: make(chan struct{}, 1)}
	state := model.Default()
	state.Groups[0].Apps = []model.Entry{model.NewEntry("App", "app.exe", "")}
	state.Groups[0].Apps[0].EnvVars = []string{"KEY=old"}
	state.Settings.WindowBounds = &model.Bounds{Width: 800, Height: 600}
	s.set(state)
	state.Groups[0].Apps[0].Name = "Modified"
	state.Groups[0].Apps[0].EnvVars[0] = "KEY=new"
	state.Settings.WindowBounds.Width = 900
	p.fail = true
	if err := s.flush(); err == nil {
		t.Fatal("failed save must be reported")
	}
	p.fail = false
	if err := s.flush(); err != nil {
		t.Fatal(err)
	}
	if p.state.Groups[0].Apps[0].Name != "App" || p.state.Groups[0].Apps[0].EnvVars[0] != "KEY=old" || p.state.Settings.WindowBounds.Width != 800 {
		t.Fatal("save snapshot aliases mutable UI state")
	}
	if err := s.flush(); err != nil {
		t.Fatal(err)
	}
	if p.writes != 1 {
		t.Fatal("an unchanged revision must not write again")
	}
	s.set(state)
	if err := s.flush(); err != nil {
		t.Fatal(err)
	}
	if p.state.Groups[0].Apps[0].Name != "Modified" || p.writes != 2 {
		t.Fatal("latest state was not saved")
	}
}
