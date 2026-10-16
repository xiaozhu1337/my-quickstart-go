package model

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"
)

type Entry struct {
	ID      string   `json:"id" yaml:"id"`
	Name    string   `json:"name" yaml:"name"`
	Path    string   `json:"path" yaml:"path"`
	Args    string   `json:"args,omitempty" yaml:"args,omitempty"`
	EnvVars []string `json:"envVars,omitempty" yaml:"envVars,omitempty"`
	Icon    string   `json:"icon,omitempty" yaml:"icon,omitempty"`
	AddedAt int64    `json:"addedAt" yaml:"addedAt"`
}

type Group struct {
	ID   string  `json:"id" yaml:"id"`
	Name string  `json:"name" yaml:"name"`
	Apps []Entry `json:"apps" yaml:"apps"`
}

type Bounds struct {
	X      int `json:"x" yaml:"x"`
	Y      int `json:"y" yaml:"y"`
	Width  int `json:"width" yaml:"width"`
	Height int `json:"height" yaml:"height"`
}

type Settings struct {
	Language             string  `json:"language" yaml:"language"`
	CardWidth            float64 `json:"cardWidth" yaml:"cardWidth"`
	CardHeight           float64 `json:"cardHeight" yaml:"cardHeight"`
	ToggleHotkey         string  `json:"toggleHotkey" yaml:"toggleHotkey"`
	Theme                string  `json:"theme" yaml:"theme"`
	SidebarWidth         float64 `json:"sidebarWidth" yaml:"sidebarWidth"`
	FontFamily           string  `json:"fontFamily" yaml:"fontFamily"`
	FontSize             float64 `json:"fontSize" yaml:"fontSize"`
	CardFontSize         float64 `json:"cardFontSize" yaml:"cardFontSize"`
	CardIconScale        float64 `json:"cardIconScale" yaml:"cardIconScale"`
	DblClickBlankToHide  bool    `json:"dblClickBlankToHide" yaml:"dblClickBlankToHide"`
	AlwaysOnTop          bool    `json:"alwaysOnTop" yaml:"alwaysOnTop"`
	HideOnStartup        bool    `json:"hideOnStartup" yaml:"hideOnStartup"`
	UseRelativePath      bool    `json:"useRelativePath" yaml:"useRelativePath"`
	RememberWindowBounds bool    `json:"rememberWindowBounds" yaml:"rememberWindowBounds"`
	WindowBounds         *Bounds `json:"windowBounds" yaml:"windowBounds"`
}

type State struct {
	Version       int      `json:"version" yaml:"version"`
	ActiveGroupID string   `json:"activeGroupId" yaml:"activeGroupId"`
	Groups        []Group  `json:"groups" yaml:"groups"`
	Settings      Settings `json:"settings" yaml:"settings"`
}

func ID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b[:])
}

func Default() State {
	g := Group{ID: ID(), Name: "Group-1", Apps: []Entry{}}
	return State{Version: 1, ActiveGroupID: g.ID, Groups: []Group{g}, Settings: Settings{
		Language: "zh-CN", CardWidth: 120, CardHeight: 96, Theme: "dark", SidebarWidth: 140,
		FontFamily: "Segoe UI", FontSize: 13, CardFontSize: 11, CardIconScale: 56,
		DblClickBlankToHide: true, AlwaysOnTop: true,
	}}
}

func (s *State) Normalize() {
	s.Version = 1
	v := &s.Settings
	if v.Language != "en" {
		v.Language = "zh-CN"
	}
	if v.Theme != "light" {
		v.Theme = "dark"
	}
	v.CardWidth = clamp(v.CardWidth, 72, 240, 120)
	v.CardHeight = clamp(v.CardHeight, 64, 200, 96)
	v.SidebarWidth = clamp(v.SidebarWidth, 80, 320, 140)
	v.FontSize = clamp(v.FontSize, 10, 24, 13)
	v.CardFontSize = clamp(v.CardFontSize, 8, 24, 11)
	v.CardIconScale = clamp(v.CardIconScale, 16, min(v.CardWidth, v.CardHeight)*0.82, 56)
	if strings.TrimSpace(v.FontFamily) == "" {
		v.FontFamily = "Segoe UI"
	}
	if len(s.Groups) == 0 {
		g := Default().Groups[0]
		s.Groups = []Group{g}
	}
	if s.Group(s.ActiveGroupID) == nil {
		s.ActiveGroupID = s.Groups[0].ID
	}
}

func clamp(v, lo, hi, fallback float64) float64 {
	if v == 0 {
		v = fallback
	}
	return max(lo, min(hi, v))
}

func (s *State) Group(id string) *Group {
	for i := range s.Groups {
		if s.Groups[i].ID == id {
			return &s.Groups[i]
		}
	}
	return nil
}

func (s *State) Entry(id string) (*Group, *Entry) {
	for i := range s.Groups {
		for j := range s.Groups[i].Apps {
			if s.Groups[i].Apps[j].ID == id {
				return &s.Groups[i], &s.Groups[i].Apps[j]
			}
		}
	}
	return nil, nil
}

func (s State) Validate() error {
	if s.Version != 1 {
		return fmt.Errorf("unsupported version: %d", s.Version)
	}
	if len(s.Groups) == 0 {
		return errors.New("at least one group is required")
	}
	seen := map[string]bool{}
	for _, g := range s.Groups {
		if strings.TrimSpace(g.ID) == "" || seen[g.ID] || strings.TrimSpace(g.Name) == "" {
			return errors.New("invalid or duplicate group")
		}
		seen[g.ID] = true
		for _, e := range g.Apps {
			if e.ID == "" || seen[e.ID] || strings.TrimSpace(e.Name) == "" || strings.TrimSpace(e.Path) == "" {
				return errors.New("invalid or duplicate entry")
			}
			seen[e.ID] = true
			if err := ValidateEnv(e.EnvVars); err != nil {
				return err
			}
		}
	}
	return nil
}

func ValidateEnv(lines []string) error {
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		key, _, ok := strings.Cut(line, "=")
		if !ok || strings.TrimSpace(key) == "" || strings.ContainsAny(line, "\x00\r\n") {
			return fmt.Errorf("invalid environment variable: %q", line)
		}
	}
	return nil
}

func NewEntry(name, path, args string) Entry {
	return Entry{ID: ID(), Name: name, Path: path, Args: args, AddedAt: time.Now().UnixMilli()}
}
