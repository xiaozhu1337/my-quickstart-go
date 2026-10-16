package store

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	_ "modernc.org/sqlite"
	"my-quickstart-go/internal/model"
)

const schema = `
CREATE TABLE IF NOT EXISTS meta (key TEXT PRIMARY KEY, value TEXT NOT NULL);
CREATE TABLE IF NOT EXISTS groups (id TEXT PRIMARY KEY, name TEXT NOT NULL, position INTEGER NOT NULL);
CREATE TABLE IF NOT EXISTS apps (
 id TEXT PRIMARY KEY, group_id TEXT NOT NULL, name TEXT NOT NULL, path TEXT NOT NULL,
 args TEXT NOT NULL, env_vars TEXT NOT NULL DEFAULT '', icon TEXT NOT NULL DEFAULT '',
 position INTEGER NOT NULL, added_at INTEGER NOT NULL,
 FOREIGN KEY(group_id) REFERENCES groups(id) ON DELETE CASCADE
);`

type Store struct {
	db   *sql.DB
	path string
}

func DefaultPath() (string, error) {
	if runtime.GOOS != "windows" {
		base, err := os.UserConfigDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(base, "my-quickstart-go", "launcher.db"), nil
	}
	base := os.Getenv("LOCALAPPDATA")
	if base == "" {
		return "", errors.New("LOCALAPPDATA is not set")
	}
	return filepath.Join(base, "my-quickstart-go", "launcher.db"), nil
}

// Open imports the old database only when the default new database does not exist.
func Open(path string) (*Store, error) {
	if path == "" {
		return nil, errors.New("database path is empty")
	}
	path, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	_, err = os.Stat(path)
	if errors.Is(err, os.ErrNotExist) {
		state := model.Default()
		defaultPath, pathErr := DefaultPath()
		if pathErr == nil {
			defaultPath, pathErr = filepath.Abs(defaultPath)
			if pathErr != nil {
				return nil, pathErr
			}
			if runtime.GOOS == "windows" && strings.EqualFold(path, defaultPath) {
				legacyPaths := []string{
					filepath.Join(os.Getenv("LOCALAPPDATA"), "my-quickstart", "launcher.db"),
					filepath.Join(os.Getenv("LOCALAPPDATA"), "com.cat.tauri-app", "launcher.db"),
				}
				if base := os.Getenv("APPDATA"); base != "" {
					legacyPaths = append(legacyPaths, filepath.Join(base, "com.cat.tauri-app", "launcher.db"))
				}
				for _, legacy := range legacyPaths {
					if _, statErr := os.Stat(legacy); statErr == nil {
						state, err = readSQLiteBackup(legacy)
						if err != nil {
							return nil, fmt.Errorf("import legacy database %s: %w", legacy, err)
						}
						break
					} else if !errors.Is(statErr, os.ErrNotExist) {
						return nil, statErr
					}
				}
			}
		}
		if err = createDatabase(path, state); err != nil {
			return nil, err
		}
	} else if err != nil {
		return nil, err
	}
	db, err := connect(path, "rw")
	if err != nil {
		return nil, err
	}
	s := &Store{db: db, path: path}
	if _, err = s.Load(); err != nil {
		db.Close()
		return nil, fmt.Errorf("load database: %w", err)
	}
	return s, nil
}

func connect(path, mode string) (*sql.DB, error) {
	path, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	uriPath := filepath.ToSlash(path)
	if !strings.HasPrefix(uriPath, "/") {
		uriPath = "/" + uriPath
	}
	u := url.URL{Scheme: "file", Path: uriPath}
	q := url.Values{"mode": {mode}, "_pragma": {"foreign_keys(1)", "busy_timeout(5000)"}}
	u.RawQuery = q.Encode()
	db, err := sql.Open("sqlite", u.String())
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	if err = db.Ping(); err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}

func (s *Store) Path() string { return s.path }
func (s *Store) Close() error { return s.db.Close() }

func (s *Store) Load() (model.State, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return model.State{}, err
	}
	defer tx.Rollback()
	state, err := load(tx)
	if err != nil {
		return model.State{}, err
	}
	if err = tx.Commit(); err != nil {
		return model.State{}, err
	}
	return state, nil
}

func load(tx *sql.Tx) (model.State, error) {
	state := model.State{Version: 1, Settings: model.Default().Settings}
	var version int
	if err := tx.QueryRow("PRAGMA user_version").Scan(&version); err != nil {
		return state, err
	}
	if version != 0 && version != 1 {
		return state, fmt.Errorf("unsupported database version: %d", version)
	}
	rows, err := tx.Query("SELECT key, value FROM meta")
	if err != nil {
		return state, err
	}
	for rows.Next() {
		var key, value string
		if err = rows.Scan(&key, &value); err != nil {
			rows.Close()
			return state, err
		}
		switch key {
		case "version":
			state.Version, err = strconv.Atoi(value)
		case "active_group_id":
			state.ActiveGroupID = value
		case "ui_settings":
			if strings.TrimSpace(value) != "" {
				err = decodeSettings([]byte(value), &state.Settings)
			}
		}
		if err != nil {
			rows.Close()
			return state, err
		}
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return state, err
	}
	rows, err = tx.Query("SELECT id, name FROM groups ORDER BY position, rowid")
	if err != nil {
		return state, err
	}
	groupIndex := map[string]int{}
	for rows.Next() {
		g := model.Group{Apps: []model.Entry{}}
		if err = rows.Scan(&g.ID, &g.Name); err != nil {
			rows.Close()
			return state, err
		}
		groupIndex[g.ID] = len(state.Groups)
		state.Groups = append(state.Groups, g)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return state, err
	}
	columns, err := appColumns(tx)
	if err != nil {
		return state, err
	}
	env, icon := "''", "''"
	if columns["env_vars"] {
		env = "env_vars"
	}
	if columns["icon"] {
		icon = "icon"
	}
	rows, err = tx.Query("SELECT id, group_id, name, path, args, " + env + ", " + icon + ", added_at FROM apps ORDER BY position, rowid")
	if err != nil {
		return state, err
	}
	for rows.Next() {
		var e model.Entry
		var group, rawEnv string
		if err = rows.Scan(&e.ID, &group, &e.Name, &e.Path, &e.Args, &rawEnv, &e.Icon, &e.AddedAt); err != nil {
			rows.Close()
			return state, err
		}
		if strings.TrimSpace(rawEnv) != "" {
			if err = json.Unmarshal([]byte(rawEnv), &e.EnvVars); err != nil {
				rows.Close()
				return state, fmt.Errorf("invalid env_vars: %w", err)
			}
		}
		index, ok := groupIndex[group]
		if !ok {
			rows.Close()
			return state, fmt.Errorf("orphan entry %q", e.ID)
		}
		state.Groups[index].Apps = append(state.Groups[index].Apps, e)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return state, err
	}
	// The old launcher falls back to the first group when active_group_id is absent or stale.
	if state.Group(state.ActiveGroupID) == nil && len(state.Groups) > 0 {
		state.ActiveGroupID = state.Groups[0].ID
	}
	return state, validate(state)
}

func appColumns(tx *sql.Tx) (map[string]bool, error) {
	rows, err := tx.Query("PRAGMA table_info(apps)")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	columns := map[string]bool{}
	for rows.Next() {
		var cid, notnull, pk int
		var name, kind string
		var defaultValue any
		if err = rows.Scan(&cid, &name, &kind, &notnull, &defaultValue, &pk); err != nil {
			return nil, err
		}
		columns[name] = true
	}
	return columns, rows.Err()
}

func validate(state model.State) error {
	if err := state.Validate(); err != nil {
		return err
	}
	if state.Group(state.ActiveGroupID) == nil {
		return errors.New("active group does not exist")
	}
	// JSON encoding also rejects non-finite settings values before a transaction can change data.
	_, err := json.Marshal(state)
	return err
}

func (s *Store) Save(state model.State) error {
	if err := validate(state); err != nil {
		return err
	}
	settings, err := json.Marshal(state.Settings)
	if err != nil {
		return err
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.Exec(schema); err != nil {
		return err
	}
	columns, err := appColumns(tx)
	if err != nil {
		return err
	}
	for _, column := range []string{"env_vars", "icon"} {
		if !columns[column] {
			if _, err = tx.Exec("ALTER TABLE apps ADD COLUMN " + column + " TEXT NOT NULL DEFAULT ''"); err != nil {
				return err
			}
		}
	}
	if _, err = tx.Exec("DELETE FROM apps; DELETE FROM groups"); err != nil {
		return err
	}
	for key, value := range map[string]string{"version": strconv.Itoa(state.Version), "active_group_id": state.ActiveGroupID, "ui_settings": string(settings)} {
		if _, err = tx.Exec("INSERT INTO meta(key,value) VALUES(?,?) ON CONFLICT(key) DO UPDATE SET value=excluded.value", key, value); err != nil {
			return err
		}
	}
	for i, group := range state.Groups {
		if _, err = tx.Exec("INSERT INTO groups(id,name,position) VALUES(?,?,?)", group.ID, group.Name, i); err != nil {
			return err
		}
		for j, entry := range group.Apps {
			env := ""
			if len(entry.EnvVars) > 0 {
				raw, marshalErr := json.Marshal(entry.EnvVars)
				if marshalErr != nil {
					return marshalErr
				}
				env = string(raw)
			}
			if _, err = tx.Exec("INSERT INTO apps(id,group_id,name,path,args,env_vars,icon,position,added_at) VALUES(?,?,?,?,?,?,?,?,?)", entry.ID, group.ID, entry.Name, entry.Path, entry.Args, env, entry.Icon, j, entry.AddedAt); err != nil {
				return err
			}
		}
	}
	return tx.Commit()
}

func createDatabase(path string, state model.State) error {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	temp, err := os.CreateTemp(filepath.Dir(path), ".launcher-*.db")
	if err != nil {
		return err
	}
	tempPath := temp.Name()
	defer os.Remove(tempPath)
	if err = temp.Close(); err != nil {
		return err
	}
	db, err := connect(tempPath, "rw")
	if err != nil {
		return err
	}
	s := &Store{db: db, path: tempPath}
	err = s.Save(state)
	closeErr := s.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	// Publishing by hard link never replaces a database created by another process.
	return os.Link(tempPath, path)
}
