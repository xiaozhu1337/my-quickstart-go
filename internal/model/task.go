package model

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

// ScheduledTask launches a program once per local calendar day, at Time (HH:MM).
type ScheduledTask struct {
	ID                string `json:"id" yaml:"id"`
	Name              string `json:"name" yaml:"name"`
	Time              string `json:"time" yaml:"time"`
	Path              string `json:"path" yaml:"path"`
	Args              string `json:"args,omitempty" yaml:"args,omitempty"`
	Enabled           bool   `json:"enabled" yaml:"enabled"`
	LastScheduledDate string `json:"lastScheduledDate,omitempty" yaml:"lastScheduledDate,omitempty"`
	LastRunAt         int64  `json:"lastRunAt,omitempty" yaml:"lastRunAt,omitempty"`
	LastStatus        string `json:"lastStatus,omitempty" yaml:"lastStatus,omitempty"`
	LastError         string `json:"lastError,omitempty" yaml:"lastError,omitempty"`
}

func ParseDailyTime(value string) (time.Time, error) {
	parsed, err := time.Parse("15:04", value)
	if err != nil || parsed.Format("15:04") != value {
		return time.Time{}, errors.New("time must be HH:MM (00:00-23:59)")
	}
	return parsed, nil
}

func (t ScheduledTask) Validate() error {
	if strings.TrimSpace(t.ID) == "" || strings.TrimSpace(t.Name) == "" || strings.TrimSpace(t.Path) == "" {
		return errors.New("task ID, name and program path are required")
	}
	if strings.ContainsAny(t.Path+t.Args, "\x00\r\n") {
		return errors.New("program path and arguments must be single lines without NUL")
	}
	if _, err := ParseDailyTime(t.Time); err != nil {
		return err
	}
	if t.LastScheduledDate != "" {
		if _, err := time.Parse(time.DateOnly, t.LastScheduledDate); err != nil {
			return fmt.Errorf("invalid last scheduled date: %w", err)
		}
	}
	return nil
}

// Due deliberately skips missed minutes; restarting never catches up old tasks.
func (t ScheduledTask) Due(now time.Time) bool {
	return t.Enabled && t.Time == now.Format("15:04") && t.LastScheduledDate < now.Format(time.DateOnly)
}

func (t ScheduledTask) NextRun(now time.Time) time.Time {
	clock, err := ParseDailyTime(t.Time)
	if err != nil || !t.Enabled {
		return time.Time{}
	}
	day := now
	if t.LastScheduledDate >= now.Format(time.DateOnly) {
		last, err := time.ParseInLocation(time.DateOnly, t.LastScheduledDate, now.Location())
		if err != nil {
			return time.Time{}
		}
		day = last.AddDate(0, 0, 1)
	}
	for range 3 {
		next := time.Date(day.Year(), day.Month(), day.Day(), clock.Hour(), clock.Minute(), 0, 0, now.Location())
		// A skipped local minute at a daylight-saving transition must not run early.
		if next.Format("15:04") == t.Time && !next.Before(now.Truncate(time.Minute)) {
			return next
		}
		day = day.AddDate(0, 0, 1)
	}
	return time.Time{}
}
