package store

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestDefaultPath(t *testing.T) {
	base := t.TempDir()
	t.Setenv("LOCALAPPDATA", base)
	t.Setenv("HOME", base)
	t.Setenv("XDG_CONFIG_HOME", base)
	expectedBase := base
	if runtime.GOOS != "windows" {
		var err error
		expectedBase, err = os.UserConfigDir()
		if err != nil {
			t.Fatal(err)
		}
	}
	got, err := DefaultPath()
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(expectedBase, "my-quickstart-go", "launcher.db"); got != want {
		t.Fatalf("path = %q, want %q", got, want)
	}
	if runtime.GOOS == "windows" {
		t.Setenv("LOCALAPPDATA", "")
		if _, err = DefaultPath(); err == nil {
			t.Fatal("missing LOCALAPPDATA accepted")
		}
	}
}

func TestHistoricalMigrationPaths(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows legacy migration")
	}
	for _, source := range []string{"primary", "local-tauri", "roaming-tauri"} {
		t.Run(source, func(t *testing.T) {
			base := t.TempDir()
			local := filepath.Join(base, "local")
			roaming := filepath.Join(base, "roaming")
			t.Setenv("LOCALAPPDATA", local)
			t.Setenv("APPDATA", roaming)
			legacy := filepath.Join(local, "my-quickstart", "launcher.db")
			if source == "local-tauri" {
				legacy = filepath.Join(local, "com.cat.tauri-app", "launcher.db")
			}
			if source == "roaming-tauri" {
				legacy = filepath.Join(roaming, "com.cat.tauri-app", "launcher.db")
			}
			writeLegacyDB(t, legacy, false)
			want, err := ReadBackup(legacy)
			if err != nil {
				t.Fatal(err)
			}
			path, err := DefaultPath()
			if err != nil {
				t.Fatal(err)
			}
			s, err := Open(path)
			if err != nil {
				t.Fatal(err)
			}
			defer s.Close()
			got, err := s.Load()
			if err != nil {
				t.Fatal(err)
			}
			assertState(t, want, got)
		})
	}
}

func TestMigrationDoesNotSkipInvalidPrimary(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows legacy migration")
	}
	base := t.TempDir()
	t.Setenv("LOCALAPPDATA", base)
	t.Setenv("APPDATA", t.TempDir())
	writeLegacyDB(t, filepath.Join(base, "com.cat.tauri-app", "launcher.db"), true)
	primary := filepath.Join(base, "my-quickstart", "launcher.db")
	if err := os.MkdirAll(filepath.Dir(primary), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(primary, []byte("broken primary"), 0600); err != nil {
		t.Fatal(err)
	}
	path, _ := DefaultPath()
	if _, err := Open(path); err == nil {
		t.Fatal("invalid primary migration silently skipped")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("invalid migration created destination")
	}
}

func TestLegacySQLiteReadCompatibility(t *testing.T) {
	for _, optional := range []bool{false, true} {
		path := filepath.Join(t.TempDir(), "legacy.db")
		writeLegacyDB(t, path, optional)
		got, err := ReadBackup(path)
		if err != nil {
			t.Fatal(err)
		}
		if got.Groups[0].ID != "legacy-group" || got.Groups[0].Apps[0].ID != "legacy-app" || got.Settings.CardWidth != 132 {
			t.Fatal("legacy fields lost")
		}
	}
}

func TestSQLiteWALBackupLeavesSourceUntouched(t *testing.T) {
	path := filepath.Join(t.TempDir(), "legacy.db")
	writeLegacyDB(t, path, true)
	db, err := connect(path, "rw")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err = db.Exec("PRAGMA journal_mode=WAL; PRAGMA wal_autocheckpoint=0; UPDATE groups SET name='committed in WAL'"); err != nil {
		t.Fatal(err)
	}
	before := map[string][]byte{}
	for _, suffix := range []string{"", "-wal", "-shm"} {
		before[suffix], err = os.ReadFile(path + suffix)
		if err != nil {
			t.Fatal(err)
		}
	}
	got, err := ReadBackup(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.Groups[0].Name != "committed in WAL" {
		t.Fatal("committed WAL data lost")
	}
	for suffix, raw := range before {
		after, readErr := os.ReadFile(path + suffix)
		if readErr != nil {
			t.Fatal(readErr)
		}
		if !bytes.Equal(raw, after) {
			t.Fatalf("source %q changed", suffix)
		}
	}
}
