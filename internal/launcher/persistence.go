package launcher

import (
	"log"
	"sync"
	"time"

	"my-quickstart-go/internal/model"
)

type persistence interface {
	Save(model.State) error
	Path() string
}

type saver struct {
	store    persistence
	mu       sync.Mutex
	writeMu  sync.Mutex
	latest   model.State
	revision uint64
	saved    uint64
	wake     chan struct{}
	errors   chan error
	stop     chan struct{}
	done     chan struct{}
}

func newSaver(p persistence) *saver {
	s := &saver{store: p, wake: make(chan struct{}, 1), errors: make(chan error, 1), stop: make(chan struct{}), done: make(chan struct{})}
	go s.run()
	return s
}

func clone(state model.State) model.State {
	state.Groups = append([]model.Group(nil), state.Groups...)
	for i := range state.Groups {
		state.Groups[i].Apps = append([]model.Entry(nil), state.Groups[i].Apps...)
		for j := range state.Groups[i].Apps {
			e := &state.Groups[i].Apps[j]
			e.EnvVars = append([]string(nil), e.EnvVars...)
		}
	}
	if state.Settings.WindowBounds != nil {
		b := *state.Settings.WindowBounds
		state.Settings.WindowBounds = &b
	}
	return state
}

func (s *saver) set(state model.State) {
	s.mu.Lock()
	s.latest = clone(state)
	s.revision++
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
	state, revision, saved := s.latest, s.revision, s.saved
	s.mu.Unlock()
	if revision == saved {
		return nil
	}
	if err := s.store.Save(state); err != nil {
		return err
	}
	s.mu.Lock()
	s.saved = revision
	s.mu.Unlock()
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
