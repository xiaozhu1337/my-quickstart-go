package store

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"

	"my-quickstart-go/internal/model"
)

func legacyDatabasePaths() ([]string, error) {
	var base string
	switch runtime.GOOS {
	case "windows":
		base = os.Getenv("LOCALAPPDATA")
	case "darwin":
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, err
		}
		base = filepath.Join(home, "Library", "Application Support")
	default:
		base = os.Getenv("XDG_DATA_HOME")
		if base == "" {
			home, err := os.UserHomeDir()
			if err != nil {
				return nil, err
			}
			base = filepath.Join(home, ".local", "share")
		}
	}
	paths := []string{
		filepath.Join(base, "my-quickstart", "launcher.db"),
		filepath.Join(base, "com.cat.tauri-app", "launcher.db"),
	}
	if runtime.GOOS == "windows" {
		if roaming := os.Getenv("APPDATA"); roaming != "" {
			paths = append(paths, filepath.Join(roaming, "com.cat.tauri-app", "launcher.db"))
		}
	}
	return paths, nil
}

func readLegacyState(path string) (model.State, error) {
	// SQLite pins a consistent WAL snapshot; read-only access may update lock/SHM metadata.
	db, err := connect(path, "ro")
	if err != nil {
		return model.State{}, err
	}
	defer db.Close()
	tx, err := db.BeginTx(context.Background(), &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return model.State{}, err
	}
	defer tx.Rollback()
	var integrity string
	if err = tx.QueryRow("PRAGMA quick_check").Scan(&integrity); err != nil {
		return model.State{}, err
	}
	if integrity != "ok" {
		return model.State{}, fmt.Errorf("invalid legacy database: %s", integrity)
	}
	// Read rows into State without buffering database files or imposing a file-size limit.
	state, err := load(tx)
	if err != nil {
		return model.State{}, err
	}
	return state, tx.Commit()
}

func decodeJSON(data []byte, value any) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(value); err != nil {
		return err
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return errors.New("settings must contain exactly one JSON value")
	}
	return nil
}

func decodeSettings(data []byte, settings *model.Settings) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	if size, ok := raw["cardSize"]; ok {
		if _, present := raw["cardWidth"]; !present {
			raw["cardWidth"] = size
		}
		delete(raw, "cardSize")
	}
	data, err := json.Marshal(raw)
	if err != nil {
		return err
	}
	return decodeJSON(data, settings)
}
