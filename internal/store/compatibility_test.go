package store

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestSaveUpgradesOptionalLegacyColumns(t *testing.T) {
	path := filepath.Join(t.TempDir(), "old.db")
	writeLegacyDB(t, path, false)
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if _, err = s.db.Exec("INSERT INTO meta VALUES('unrelated','preserve me')"); err != nil {
		t.Fatal(err)
	}
	want, err := s.Load()
	if err != nil {
		t.Fatal(err)
	}
	want.Groups[0].Apps[0].EnvVars = []string{"NEW=1"}
	want.Groups[0].Apps[0].Icon = "icon"
	if err = s.Save(want); err != nil {
		t.Fatal(err)
	}
	got, err := s.Load()
	if err != nil {
		t.Fatal(err)
	}
	assertState(t, want, got)
	var unrelated string
	if err = s.db.QueryRow("SELECT value FROM meta WHERE key='unrelated'").Scan(&unrelated); err != nil || unrelated != "preserve me" {
		t.Fatalf("unrelated metadata lost: %q %v", unrelated, err)
	}
}

func TestLegacyEnvironmentRowsPreserved(t *testing.T) {
	rows := []string{"NO_EQUALS", "=empty-key", "   ", "GOOD=value", "GOOD=last"}
	path := filepath.Join(t.TempDir(), "legacy.db")
	writeLegacyDB(t, path, true)
	db, err := connect(path, "rw")
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(rows)
	if err != nil {
		db.Close()
		t.Fatal(err)
	}
	_, err = db.Exec("UPDATE apps SET env_vars=?", string(raw))
	db.Close()
	if err != nil {
		t.Fatal(err)
	}
	state, err := readLegacyState(path)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(rows, state.Groups[0].Apps[0].EnvVars) {
		t.Fatal("legacy environment rows changed")
	}
	s := openTestStore(t)
	if err = s.Save(state); err != nil {
		t.Fatal(err)
	}
	got, err := s.Load()
	if err != nil {
		t.Fatal(err)
	}
	assertState(t, state, got)
}

func TestDatabasePathEscaping(t *testing.T) {
	path := filepath.Join(t.TempDir(), "中文 空格 # % &.db")
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if _, err = os.Stat(path); err != nil {
		t.Fatal(err)
	}
	want := sampleState()
	if err = s.Save(want); err != nil {
		t.Fatal(err)
	}
	got, err := s.Load()
	if err != nil {
		t.Fatal(err)
	}
	assertState(t, want, got)
}
