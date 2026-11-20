package main

import (
	"errors"
	"fmt"
	"my-quickstart-go/internal/boot"
	"my-quickstart-go/internal/model"
	"my-quickstart-go/internal/store"
)

// The saver serializes calls. Keep registration consistent if either write fails.
type scheduledStore struct {
	*store.Store
	exe        string
	registered bool
	checked    bool
}

func (s *scheduledStore) syncRegistration(enabled bool) error {
	if s.checked && s.registered == enabled {
		return nil
	}
	if err := boot.Register(s.exe, s.Path(), enabled); err != nil {
		return fmt.Errorf("Windows startup registration: %w", err)
	}
	s.registered, s.checked = enabled, true
	return nil
}

func (s *scheduledStore) SaveChanges(before, after model.State, fields store.Fields) error {
	if fields&store.Tasks == 0 {
		return s.Store.SaveChanges(before, after, fields)
	}
	if err := s.syncRegistration(boot.Enabled(after.Tasks)); err != nil {
		return err
	}
	if err := s.Store.SaveChanges(before, after, fields); err != nil {
		return errors.Join(err, s.syncRegistration(boot.Enabled(before.Tasks)))
	}
	return nil
}
