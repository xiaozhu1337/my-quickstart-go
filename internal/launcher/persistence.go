package launcher

import (
	"log"
	"sync"
	"time"

	"my-quickstart-go/internal/model"
	"my-quickstart-go/internal/store"
)

type persistence interface {
	SaveChanges(model.State, model.State, store.Fields) error
	Path() string
}

type saver struct {
	store     persistence
	mu        sync.Mutex
	writeMu   sync.Mutex
	latest    model.State
	committed model.State
	dirty     store.Fields
	wake      chan struct{}
	errors    chan error
	stop      chan struct{}
	done      chan struct{}
}

func newSaver(p persistence, initial model.State) *saver {
	s := &saver{store: p, wake: make(chan struct{}, 1), errors: make(chan error, 1), stop: make(chan struct{}), done: make(chan struct{})}
	s.latest = clone(initial)
	s.committed = s.latest
	go s.run()
	return s
}

func clone(state model.State) model.State {
	var snapshot model.State
	updateSnapshot(&snapshot, state, store.All)
	return snapshot
}

func updateSnapshot(dst *model.State, src model.State, fields store.Fields) {
	dst.Version = src.Version
	if fields&store.Groups != 0 {
		dst.Groups = append([]model.Group(nil), src.Groups...)
		for i := range dst.Groups {
			g := &dst.Groups[i]
			g.Apps = append([]model.Entry(nil), g.Apps...)
			for j := range g.Apps {
				e := &g.Apps[j]
				e.EnvVars = append([]string(nil), e.EnvVars...)
			}
		}
	}
	if fields&store.ActiveGroup != 0 {
		dst.ActiveGroupID = src.ActiveGroupID
	}
	if fields&store.Tasks != 0 {
		dst.Tasks = append([]model.ScheduledTask(nil), src.Tasks...)
	}
	if fields&store.Settings != 0 {
		dst.Settings = src.Settings
		if b := src.Settings.WindowBounds; b != nil {
			bounds := *b
			dst.Settings.WindowBounds = &bounds
		}
	}
}

func (s *saver) set(state model.State) { s.update(state, store.All) }

func (s *saver) update(state model.State, fields store.Fields) {
	s.mu.Lock()
	updateSnapshot(&s.latest, state, fields)
	s.dirty |= fields
	s.mu.Unlock()
	select {
	case s.wake <- struct{}{}:
	default:
	}
}

func (s *saver) flush() error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	s.mu.Lock()
	state, fields := s.latest, s.dirty
	s.dirty = 0
	s.mu.Unlock()
	if fields == 0 {
		return nil
	}
	if err := s.store.SaveChanges(s.committed, state, fields); err != nil {
		s.mu.Lock()
		s.dirty |= fields
		s.mu.Unlock()
		return err
	}
	s.committed = state
	return nil
}

func (s *saver) run() {
	defer close(s.done)
	for {
		select {
		case <-s.stop:
			return
		case <-s.wake:
			timer := time.NewTimer(250 * time.Millisecond)
			select {
			case <-s.stop:
				timer.Stop()
				return
			case <-timer.C:
			}
			if err := s.flush(); err != nil {
				log.Print(err)
				select {
				case s.errors <- err:
				default:
				}
			}
		}
	}
}
