package model

import (
	"math"
	"testing"
)

func TestNormalizeSettings(t *testing.T) {
	s := Default()
	s.Settings.CardWidth = 50
	s.Settings.CardHeight = 40
	s.Settings.CardIconScale = 128
	s.Settings.FontSize = math.Inf(1)
	s.Settings.FontFamily = " "
	s.ActiveGroupID = "missing"
	s.Normalize()
	if s.Settings.CardIconScale != 33 || s.Settings.FontSize != 13 || s.Settings.FontFamily != "system" || s.ActiveGroupID != s.Groups[0].ID {
		t.Fatalf("unexpected normalized state: %+v", s)
	}
	if err := s.Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestNormalizeTheme(t *testing.T) {
	for _, tc := range []struct {
		theme string
		want  string
	}{
		{theme: "dark", want: "dark"},
		{theme: "light", want: "light"},
		{theme: "midnight", want: "midnight"},
		{theme: "forest", want: "forest"},
		{theme: "rose", want: "rose"},
		{theme: "", want: "dark"},
		{theme: "unknown", want: "dark"},
		{theme: "system", want: "dark"},
	} {
		t.Run(tc.theme, func(t *testing.T) {
			s := Default()
			s.Settings.Theme = tc.theme
			s.Normalize()
			if s.Settings.Theme != tc.want {
				t.Fatalf("normalized theme: want %q, got %q", tc.want, s.Settings.Theme)
			}
		})
	}
}

func TestValidatePreservesLegacyEnvironment(t *testing.T) {
	s := Default()
	e := NewEntry("App", "app.exe", "")
	e.EnvVars = []string{"malformed", "=empty-key", "KEY=value=tail", "KEY="}
	s.Groups[0].Apps = append(s.Groups[0].Apps, e)
	if err := s.Validate(); err != nil {
		t.Fatalf("legacy environment strings must remain storable: %v", err)
	}
	s.Groups[1].Apps = append(s.Groups[1].Apps, e)
	if err := s.Validate(); err == nil {
		t.Fatal("duplicate entry IDs must be rejected")
	}
}
