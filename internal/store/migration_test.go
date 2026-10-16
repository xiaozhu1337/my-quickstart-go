package store

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"my-quickstart-go/internal/model"
)

// Every path Open may inspect belongs to this test, including platform fallback paths.
func isolateMigrationPaths(t *testing.T, useXDG bool) []string {
	t.Helper()
	base := t.TempDir()
	home := filepath.Join(base, "home")
	local := filepath.Join(base, "local")
	roaming := filepath.Join(base, "roaming")
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("LOCALAPPDATA", local)
	t.Setenv("APPDATA", roaming)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(base, "config"))
	t.Setenv("XDG_DATA_HOME", "")
	data := filepath.Join(home, ".local", "share")
	if useXDG {
		data = filepath.Join(base, "data")
		t.Setenv("XDG_DATA_HOME", data)
	}
	switch runtime.GOOS {
	case "windows":
		data = local
	case "darwin":
		data = filepath.Join(home, "Library", "Application Support")
	}
	paths := []string{
		filepath.Join(data, "my-quickstart", "launcher.db"),
		filepath.Join(data, "com.cat.tauri-app", "launcher.db"),
	}
	if runtime.GOOS == "windows" {
		paths = append(paths, filepath.Join(roaming, "com.cat.tauri-app", "launcher.db"))
	}
	return paths
}

func TestDefaultPath(t *testing.T) {
	isolateMigrationPaths(t, true)
	expectedBase := os.Getenv("LOCALAPPDATA")
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
	for _, useXDG := range []bool{false, true} {
		t.Run(fmt.Sprintf("xdg-%t", useXDG), func(t *testing.T) {
			paths := isolateMigrationPaths(t, useXDG)
			for i := range paths {
				t.Run(fmt.Sprint(i), func(t *testing.T) {
					legacy := isolateMigrationPaths(t, useXDG)[i]
					writeLegacyDB(t, legacy, false)
					want, err := readLegacyState(legacy)
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
		})
	}
}

func TestMigrationDoesNotSkipInvalidPrimary(t *testing.T) {
	paths := isolateMigrationPaths(t, true)
	writeLegacyDB(t, paths[1], true)
	primary := paths[0]
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

func TestOpenDefaultWithoutLegacyDatabase(t *testing.T) {
	isolateMigrationPaths(t, true)
	path, _ := DefaultPath()
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	state, err := s.Load()
	if err != nil || state.Validate() != nil {
		t.Fatalf("default state: %v", err)
	}
	if len(state.Groups) != len(model.Default().Groups) || state.Settings != model.Default().Settings {
		t.Fatal("unexpected default state")
	}
}

func TestMigrationOnlyAtDefaultPath(t *testing.T) {
	legacy := isolateMigrationPaths(t, true)[0]
	writeLegacyDB(t, legacy, true)
	path, _ := DefaultPath()
	if runtime.GOOS == "windows" {
		path = strings.ToUpper(path)
	} else {
		path = filepath.Join(filepath.Dir(filepath.Dir(path)), "MY-QUICKSTART-GO", "launcher.db")
	}
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	state, err := s.Load()
	if err != nil {
		t.Fatal(err)
	}
	migrated := state.Groups[0].ID == "legacy-group"
	if migrated != (runtime.GOOS == "windows") {
		t.Fatal("default path case comparison does not match OS")
	}
	custom, err := Open(filepath.Join(t.TempDir(), "custom.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer custom.Close()
	state, err = custom.Load()
	if err != nil || state.Groups[0].ID == "legacy-group" {
		t.Fatalf("custom path imported legacy database: %v", err)
	}
}

func TestLegacySQLiteReadCompatibility(t *testing.T) {
	for _, optional := range []bool{false, true} {
		path := filepath.Join(t.TempDir(), "legacy.db")
		writeLegacyDB(t, path, optional)
		got, err := readLegacyState(path)
		if err != nil {
			t.Fatal(err)
		}
		if got.Groups[0].ID != "legacy-group" || got.Groups[0].Apps[0].ID != "legacy-app" || got.Settings.CardWidth != 132 {
			t.Fatal("legacy fields lost")
		}
	}
}

func TestLegacyWALReadLeavesDataUntouched(t *testing.T) {
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
	for _, suffix := range []string{"", "-wal"} {
		before[suffix], err = os.ReadFile(path + suffix)
		if err != nil {
			t.Fatal(err)
		}
	}
	got, err := readLegacyState(path)
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

func TestMigrationDuringWALCheckpoint(t *testing.T) {
	legacy := isolateMigrationPaths(t, true)[0]
	writeLegacyDB(t, legacy, true)
	db, err := connect(legacy, "rw")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err = db.Exec("CREATE TABLE padding (data BLOB); INSERT INTO padding VALUES(zeroblob(8388608)); PRAGMA journal_mode=WAL; PRAGMA wal_autocheckpoint=0"); err != nil {
		t.Fatal(err)
	}
	path, _ := DefaultPath()
	for i := range 20 {
		// A is in the main file; B commits to WAL before either migration or checkpoint starts.
		if _, err = db.Exec("UPDATE groups SET name='A'; PRAGMA wal_checkpoint(TRUNCATE); UPDATE groups SET name='B'"); err != nil {
			t.Fatal(err)
		}
		start := make(chan struct{})
		checkpointed := make(chan error, 1)
		go func() {
			<-start
			for range 10 {
				if _, checkpointErr := db.Exec("PRAGMA wal_checkpoint(TRUNCATE)"); checkpointErr != nil {
					checkpointed <- checkpointErr
					return
				}
				runtime.Gosched()
			}
			checkpointed <- nil
		}()
		close(start)
		s, openErr := Open(path)
		checkpointErr := <-checkpointed
		if checkpointErr != nil {
			t.Fatal(checkpointErr)
		}
		if openErr != nil {
			if _, statErr := os.Stat(path); !os.IsNotExist(statErr) {
				t.Fatalf("iteration %d: failed migration published destination: %v", i, openErr)
			}
			continue
		}
		got, loadErr := s.Load()
		closeErr := s.Close()
		if loadErr != nil || closeErr != nil {
			t.Fatalf("iteration %d: load %v, close %v", i, loadErr, closeErr)
		}
		if got.Groups[0].Name != "B" {
			t.Fatalf("iteration %d: committed B lost: %q", i, got.Groups[0].Name)
		}
		if err = os.Remove(path); err != nil {
			t.Fatal(err)
		}
	}
}

func TestMigrationDatabaseLargerThan64MiB(t *testing.T) {
	legacy := isolateMigrationPaths(t, true)[0]
	writeLegacyDB(t, legacy, true)
	db, err := connect(legacy, "rw")
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec("CREATE TABLE padding (data BLOB); INSERT INTO padding VALUES(zeroblob(68157440))")
	closeErr := db.Close()
	if err != nil || closeErr != nil {
		t.Fatalf("create large database: %v, close: %v", err, closeErr)
	}
	info, err := os.Stat(legacy)
	if err != nil || info.Size() <= 64<<20 {
		t.Fatalf("large fixture must exceed 64MiB: %v", err)
	}
	want, err := readLegacyState(legacy)
	if err != nil {
		t.Fatal(err)
	}
	path, _ := DefaultPath()
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
}

func TestCreateDatabaseExclusiveAndCleanup(t *testing.T) {
	path := filepath.Join(t.TempDir(), "launcher.db")
	original := []byte("existing target")
	if err := os.WriteFile(path, original, 0600); err != nil {
		t.Fatal(err)
	}
	if err := createDatabase(path, sampleState()); !os.IsExist(err) {
		t.Fatalf("expected exclusive create error, got %v", err)
	}
	after, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(original, after) {
		t.Fatalf("existing target changed: %v", err)
	}
	created := filepath.Join(filepath.Dir(path), "failed.db")
	invalid := sampleState()
	invalid.Version = 2
	if err = createDatabase(created, invalid); err == nil {
		t.Fatal("invalid state published")
	}
	if _, err = os.Stat(created); !os.IsNotExist(err) {
		t.Fatal("failed initialization left destination")
	}
	after, err = os.ReadFile(path)
	if err != nil || !bytes.Equal(original, after) {
		t.Fatalf("cleanup changed neighboring file: %v", err)
	}
}
