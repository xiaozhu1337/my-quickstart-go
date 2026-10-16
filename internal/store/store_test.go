package store

import (
	"bytes"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"my-quickstart-go/internal/model"
)

func sampleState() model.State {
	s := model.Default()
	s.Groups = []model.Group{
		{ID: "group-one", Name: "工具", Apps: []model.Entry{{ID: "app-one", Name: "编辑器", Path: `C:\工具\edit.exe`, Args: `--name "你好"`, EnvVars: []string{"MODE=测试", "EMPTY="}, Icon: "data:image/png;base64,aGVsbG8=", AddedAt: 1720000000123}}},
		{ID: "group-two", Name: "命令", Apps: []model.Entry{{ID: "app-two", Name: "终端", Path: "command:echo hello", AddedAt: 1720000000124}}},
	}
	s.ActiveGroupID = "group-two"
	s.Settings.AlwaysOnTop = false
	s.Settings.DblClickBlankToHide = false
	s.Settings.WindowBounds = &model.Bounds{X: -30, Y: 20, Width: 800, Height: 600}
	return s
}

func assertState(t *testing.T, want, got model.State) {
	t.Helper()
	a, _ := json.Marshal(want)
	b, _ := json.Marshal(got)
	if !bytes.Equal(a, b) {
		t.Fatalf("state mismatch\nwant %s\ngot  %s", a, b)
	}
}

func openTestStore(t *testing.T) *Store {
	t.Helper()
	s, err := Open(filepath.Join(t.TempDir(), "launcher.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestStoreSaveLoadAndReopen(t *testing.T) {
	s := openTestStore(t)
	initial, err := s.Load()
	if err != nil || initial.Validate() != nil {
		t.Fatalf("initial state: %v", err)
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
	if err = s.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(s.Path())
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	got, err = reopened.Load()
	if err != nil {
		t.Fatal(err)
	}
	assertState(t, want, got)
}

func TestBackupRoundTrips(t *testing.T) {
	for _, ext := range []string{".json", ".yaml", ".yml", ".db", ".YAML"} {
		t.Run(ext, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "backup"+ext)
			want := sampleState()
			if err := WriteBackup(path, want); err != nil {
				t.Fatal(err)
			}
			got, err := ReadBackup(path)
			if err != nil {
				t.Fatal(err)
			}
			assertState(t, want, got)
			want.Groups[0].Name = "替换备份"
			if err = WriteBackup(path, want); err != nil {
				t.Fatal(err)
			}
			got, err = ReadBackup(path)
			if err != nil {
				t.Fatal(err)
			}
			assertState(t, want, got)
			if ext == ".yaml" {
				raw, readErr := os.ReadFile(path)
				if readErr != nil {
					t.Fatal(readErr)
				}
				if bytes.HasPrefix(raw, []byte{0xef, 0xbb, 0xbf}) || !strings.Contains(string(raw), "# 配置格式版本") || !strings.Contains(string(raw), "# 启动环境变量") {
					t.Fatal("missing YAML comments or unexpected BOM")
				}
			}
		})
	}
}

func TestSaveRollback(t *testing.T) {
	s := openTestStore(t)
	original := sampleState()
	if err := s.Save(original); err != nil {
		t.Fatal(err)
	}
	// Fail after DELETE and metadata updates, exercising transaction rollback rather than only prevalidation.
	if _, err := s.db.Exec(`CREATE TRIGGER reject_group BEFORE INSERT ON groups WHEN NEW.name = 'reject' BEGIN SELECT RAISE(ABORT, 'injected failure'); END;`); err != nil {
		t.Fatal(err)
	}
	failed := sampleState()
	failed.Groups[0].Name = "reject"
	failed.Settings.Theme = "light"
	if err := s.Save(failed); err == nil || !strings.Contains(err.Error(), "injected failure") {
		t.Fatalf("expected injected failure, got %v", err)
	}
	got, err := s.Load()
	if err != nil {
		t.Fatal(err)
	}
	assertState(t, original, got)
	for _, mutate := range []func(*model.State){
		func(s *model.State) { s.Version = 2 },
		func(s *model.State) { s.Groups[1].Apps[0].ID = s.Groups[0].Apps[0].ID },
		func(s *model.State) { s.ActiveGroupID = "missing" },
		func(s *model.State) { s.Settings.CardWidth = math.NaN() },
	} {
		failed = sampleState()
		mutate(&failed)
		if err = s.Save(failed); err == nil {
			t.Fatal("invalid save succeeded")
		}
		got, err = s.Load()
		if err != nil {
			t.Fatal(err)
		}
		assertState(t, original, got)
	}
}

func writeLegacyDB(t *testing.T, path string, optionalColumns bool) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	db, err := connect(path, "rwc")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	legacySchema := schema
	if !optionalColumns {
		legacySchema = strings.ReplaceAll(legacySchema, "env_vars TEXT NOT NULL DEFAULT '', icon TEXT NOT NULL DEFAULT '',", "")
	}
	if _, err = db.Exec(legacySchema); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`INSERT INTO groups VALUES('legacy-group','旧分组',0);
 INSERT INTO apps(id,group_id,name,path,args,position,added_at) VALUES('legacy-app','legacy-group','旧应用','C:\old.exe','',0,123);
 INSERT INTO meta VALUES('active_group_id','legacy-group');
 INSERT INTO meta VALUES('ui_settings','{"cardSize":132,"alwaysOnTop":false}');`); err != nil {
		t.Fatal(err)
	}
	if optionalColumns {
		if _, err = db.Exec(`UPDATE apps SET env_vars='["OLD=1"]',icon='saved-icon'`); err != nil {
			t.Fatal(err)
		}
	}
}

func TestLegacySQLiteAndMigration(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows legacy migration")
	}
	t.Setenv("APPDATA", t.TempDir())
	for _, optional := range []bool{false, true} {
		t.Run(map[bool]string{false: "old-columns", true: "current-columns"}[optional], func(t *testing.T) {
			base := t.TempDir()
			t.Setenv("LOCALAPPDATA", base)
			legacy := filepath.Join(base, "my-quickstart", "launcher.db")
			writeLegacyDB(t, legacy, optional)
			before, err := os.ReadFile(legacy)
			if err != nil {
				t.Fatal(err)
			}
			oldDir, _ := os.ReadDir(filepath.Dir(legacy))
			want, err := ReadBackup(legacy)
			if err != nil {
				t.Fatal(err)
			}
			if want.Settings.CardWidth != 132 || want.Settings.AlwaysOnTop || !want.Settings.DblClickBlankToHide || want.Settings.FontFamily != model.Default().Settings.FontFamily {
				t.Fatalf("defaults/alias lost: %+v", want.Settings)
			}
			if optional && !reflect.DeepEqual(want.Groups[0].Apps[0].EnvVars, []string{"OLD=1"}) {
				t.Fatal("environment lost")
			}
			path, err := DefaultPath()
			if err != nil {
				t.Fatal(err)
			}
			s, err := Open(path)
			if err != nil {
				t.Fatal(err)
			}
			got, err := s.Load()
			if err != nil {
				t.Fatal(err)
			}
			assertState(t, want, got)
			if err = s.Close(); err != nil {
				t.Fatal(err)
			}
			after, _ := os.ReadFile(legacy)
			newDir, _ := os.ReadDir(filepath.Dir(legacy))
			if !bytes.Equal(before, after) || len(oldDir) != len(newDir) {
				t.Fatal("legacy database or sidecars changed")
			}
			// Once the new database exists, an invalid old database must have no effect.
			if err = os.WriteFile(legacy, []byte("broken old database"), 0600); err != nil {
				t.Fatal(err)
			}
			s, err = Open(path)
			if err != nil {
				t.Fatal(err)
			}
			defer s.Close()
			got, err = s.Load()
			if err != nil {
				t.Fatal(err)
			}
			assertState(t, want, got)
		})
	}
}

func TestInvalidLegacyMigrationDoesNotCreateDatabase(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows legacy migration")
	}
	t.Setenv("APPDATA", t.TempDir())
	base := t.TempDir()
	t.Setenv("LOCALAPPDATA", base)
	legacy := filepath.Join(base, "my-quickstart", "launcher.db")
	if err := os.MkdirAll(filepath.Dir(legacy), 0700); err != nil {
		t.Fatal(err)
	}
	raw := []byte("not a SQLite database")
	if err := os.WriteFile(legacy, raw, 0600); err != nil {
		t.Fatal(err)
	}
	path, _ := DefaultPath()
	if _, err := Open(path); err == nil {
		t.Fatal("invalid legacy database accepted")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("failed migration created destination")
	}
	after, _ := os.ReadFile(legacy)
	if !bytes.Equal(raw, after) {
		t.Fatal("invalid legacy database changed")
	}
}

func TestLegacyJSONAndYAMLSettingsDefaults(t *testing.T) {
	for _, ext := range []string{".json", ".yaml"} {
		t.Run(ext, func(t *testing.T) {
			raw := `{"version":1,"activeGroupId":"g","groups":[{"id":"g","name":"旧配置","apps":[{"id":"a","name":"程序","path":"a.exe","addedAt":123,"args":null,"envVars":null,"icon":null}]}],"settings":{"cardSize":150,"hideOnStartup":true,"alwaysOnTop":false}}`
			if ext == ".yaml" {
				raw = "version: 1\nactiveGroupId: g\ngroups:\n  - id: g\n    name: 旧配置\n    apps: []\nsettings:\n  hideOnStartup: true\n  alwaysOnTop: false\n"
			}
			path := filepath.Join(t.TempDir(), "old"+ext)
			if err := os.WriteFile(path, []byte(raw), 0600); err != nil {
				t.Fatal(err)
			}
			got, err := ReadBackup(path)
			if err != nil {
				t.Fatal(err)
			}
			defaults := model.Default().Settings
			if !got.Settings.HideOnStartup || got.Settings.AlwaysOnTop || !got.Settings.DblClickBlankToHide || got.Settings.FontSize != defaults.FontSize || got.Settings.FontFamily != defaults.FontFamily {
				t.Fatalf("settings defaults changed: %+v", got.Settings)
			}
			if ext == ".json" && got.Settings.CardWidth != 150 {
				t.Fatal("cardSize alias not read")
			}
		})
	}
}

func TestReadBackupRejectsInvalidInput(t *testing.T) {
	cases := map[string]string{
		"empty.json":           "",
		"invalid.json":         "{",
		"unknown-version.json": `{"version":99,"groups":[{"id":"g","name":"g","apps":[]}]}`,
		"duplicate-id.json":    `{"version":1,"groups":[{"id":"g","name":"g","apps":[{"id":"g","name":"a","path":"x"}]}]}`,
		"unknown-field.json":   `{"version":1,"groups":[],"garbage":true}`,
		"wrong-settings.json":  `{"version":1,"groups":[{"id":"g","name":"g","apps":[]}],"settings":{"alwaysOnTop":"false"}}`,
		"trailing.json":        `{"version":1} {}`,
		"invalid.yaml":         "version: [",
		"unknown-version.yaml": "version: 9\ngroups: []\n",
		"duplicate-id.yaml":    "version: 1\ngroups:\n- id: g\n  name: g\n  apps: []\n- id: g\n  name: g\n  apps: []\n",
		"extra-document.yaml":  "version: 1\n---\nversion: 1\n",
		"duplicate-key.yaml":   "version: 1\nversion: 1\n",
		"invalid.db":           "not SQLite",
		"wrong-extension.txt":  "{}",
	}
	for name, raw := range cases {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), name)
			if err := os.WriteFile(path, []byte(raw), 0600); err != nil {
				t.Fatal(err)
			}
			if _, err := ReadBackup(path); err == nil {
				t.Fatal("invalid backup accepted")
			}
		})
	}
	t.Run("size-limit", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "large.json")
		file, err := os.Create(path)
		if err != nil {
			t.Fatal(err)
		}
		if err = file.Truncate(MaxBackupSize + 1); err != nil {
			file.Close()
			t.Fatal(err)
		}
		file.Close()
		if _, err = ReadBackup(path); err == nil || !strings.Contains(err.Error(), "exceeds") {
			t.Fatalf("size limit missing: %v", err)
		}
	})
}

func TestFailedBackupPreservesDestination(t *testing.T) {
	for _, ext := range []string{".json", ".yaml", ".db"} {
		t.Run(ext, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "saved"+ext)
			original := sampleState()
			if err := WriteBackup(path, original); err != nil {
				t.Fatal(err)
			}
			before, _ := os.ReadFile(path)
			invalid := sampleState()
			invalid.Version = 2
			if err := WriteBackup(path, invalid); err == nil {
				t.Fatal("invalid export succeeded")
			}
			after, _ := os.ReadFile(path)
			if !bytes.Equal(before, after) {
				t.Fatal("failed export changed destination")
			}
		})
	}
}

func TestOpenInvalidExistingDatabasePreservesBytes(t *testing.T) {
	path := filepath.Join(t.TempDir(), "launcher.db")
	raw := []byte("broken database")
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(path); err == nil {
		t.Fatal("invalid existing database accepted")
	}
	after, _ := os.ReadFile(path)
	if !bytes.Equal(raw, after) {
		t.Fatal("existing database was overwritten")
	}
}

func TestSQLiteInvalidData(t *testing.T) {
	for name, query := range map[string]string{
		"bad-settings":         `UPDATE meta SET value='{"alwaysOnTop":42}' WHERE key='ui_settings'`,
		"bad-env":              `UPDATE apps SET env_vars='garbage'`,
		"unknown-version":      `INSERT INTO meta VALUES('version','2')`,
		"unknown-user-version": `PRAGMA user_version=9`,
		"orphan":               `PRAGMA foreign_keys=OFF; UPDATE apps SET group_id='missing'`,
		"duplicate-id":         `UPDATE apps SET id='legacy-group'`,
	} {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "old.db")
			writeLegacyDB(t, path, true)
			db, err := connect(path, "rw")
			if err != nil {
				t.Fatal(err)
			}
			if _, err = db.Exec(query); err != nil {
				db.Close()
				t.Fatal(err)
			}
			db.Close()
			if _, err = ReadBackup(path); err == nil {
				t.Fatal("invalid database accepted")
			}
		})
	}
}
