package library

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAbsoluteUserPath(t *testing.T) {
	home := t.TempDir()
	t.Setenv("USERPROFILE", home)
	t.Setenv("HOME", home)
	for _, input := range []string{"~", "~/", `~\`, "~/code/kernel-debug", `~\code\kernel-debug`, `  ~\code\kernel-debug  `} {
		want := home
		if len(input) > 2 {
			want = filepath.Join(home, "code", "kernel-debug")
		}
		got, err := absoluteUserPath(input)
		if err != nil || got != want {
			t.Errorf("absoluteUserPath(%q) = %q, %v; want %q", input, got, err, want)
		}
	}
	for _, input := range []string{home, "relative", "~someone", "folder/~"} {
		want, _ := filepath.Abs(input)
		got, err := absoluteUserPath(input)
		if err != nil || got != want {
			t.Errorf("ordinary path %q = %q, %v; want %q", input, got, err, want)
		}
	}
}

func TestDeployHomePath(t *testing.T) {
	for _, agent := range []string{"codex", "pi"} {
		t.Run(agent, func(t *testing.T) {
			l, _, ids := fixture(t)
			home := t.TempDir()
			t.Setenv("USERPROFILE", home)
			t.Setenv("HOME", home)
			project := filepath.Join(home, "code", "kernel-debug")
			if err := os.MkdirAll(project, 0700); err != nil {
				t.Fatal(err)
			}
			for _, input := range []string{`~\code\kernel-debug`, "~/code/kernel-debug", project} {
				if _, err := l.Deploy(input, agent, ids[2:]); err != nil {
					t.Fatalf("deploy %q: %v", input, err)
				}
				_, record := deploymentRecord(t, l)
				canonical, err := filepath.EvalSymlinks(project)
				if err != nil || record.Project != canonical {
					t.Fatalf("project = %q, want %q: %v", record.Project, canonical, err)
				}
			}
			rel := ".codex"
			if agent == "pi" {
				rel = ".pi"
			}
			read(t, filepath.Join(project, rel, "skills", "sample-skill", "SKILL.md"))
		})
	}
}
