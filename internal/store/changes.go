package store

import (
	"database/sql"
	"encoding/json"
	"slices"

	"my-quickstart-go/internal/model"
)

type Fields uint8

const (
	Groups Fields = 1 << iota
	ActiveGroup
	Settings
	Tasks
	All = Groups | ActiveGroup | Settings | Tasks
)

// SaveChanges commits the differences between immutable snapshots from the same database.
func (s *Store) SaveChanges(before, after model.State, fields Fields) error {
	if fields&Groups != 0 {
		if err := validate(after); err != nil {
			return err
		}
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	const meta = "INSERT INTO meta(key,value) VALUES(?,?) ON CONFLICT(key) DO UPDATE SET value=excluded.value"
	if fields&Settings != 0 {
		b, err := json.Marshal(after.Settings)
		if err != nil {
			return err
		}
		if _, err = tx.Exec(meta, "ui_settings", string(b)); err != nil {
			return err
		}
	}
	if fields&ActiveGroup != 0 {
		if _, err = tx.Exec(meta, "active_group_id", after.ActiveGroupID); err != nil {
			return err
		}
	}
	if fields&Groups != 0 {
		if err = writeGroupChanges(tx, before.Groups, after.Groups); err != nil {
			return err
		}
	}
	if fields&Tasks != 0 {
		if err = ensureTaskColumns(tx); err != nil {
			return err
		}
		if err = writeTaskChanges(tx, before.Tasks, after.Tasks); err != nil {
			return err
		}
	}
	return tx.Commit()
}

type entryPosition struct {
	group string
	index int
	entry model.Entry
}

func writeGroupChanges(tx *sql.Tx, before, after []model.Group) error {
	if err := ensureAppColumns(tx); err != nil {
		return err
	}
	groups := make(map[string]int, len(before))
	entries := map[string]entryPosition{}
	for i, g := range before {
		groups[g.ID] = i
		for j, e := range g.Apps {
			entries[e.ID] = entryPosition{g.ID, j, e}
		}
	}
	for i, g := range after {
		index, exists := groups[g.ID]
		if !exists || index != i || before[index].Name != g.Name {
			_, err := tx.Exec("INSERT INTO groups(id,name,position) VALUES(?,?,?) ON CONFLICT(id) DO UPDATE SET name=excluded.name,position=excluded.position", g.ID, g.Name, i)
			if err != nil {
				return err
			}
		}
		delete(groups, g.ID)
		for j, e := range g.Apps {
			old := entries[e.ID]
			delete(entries, e.ID)
			if old.group == g.ID && old.index == j && sameEntry(old.entry, e) {
				continue
			}
			env := ""
			if len(e.EnvVars) > 0 {
				raw, err := json.Marshal(e.EnvVars)
				if err != nil {
					return err
				}
				env = string(raw)
			}
			_, err := tx.Exec(`INSERT INTO apps(id,group_id,name,path,args,env_vars,icon,position,added_at)
VALUES(?,?,?,?,?,?,?,?,?) ON CONFLICT(id) DO UPDATE SET group_id=excluded.group_id,name=excluded.name,path=excluded.path,
args=excluded.args,env_vars=excluded.env_vars,icon=excluded.icon,position=excluded.position,added_at=excluded.added_at`,
				e.ID, g.ID, e.Name, e.Path, e.Args, env, e.Icon, j, e.AddedAt)
			if err != nil {
				return err
			}
		}
	}
	for id := range entries {
		if _, err := tx.Exec("DELETE FROM apps WHERE id=?", id); err != nil {
			return err
		}
	}
	for id := range groups {
		if _, err := tx.Exec("DELETE FROM groups WHERE id=?", id); err != nil {
			return err
		}
	}
	return nil
}

func sameEntry(a, b model.Entry) bool {
	return a.ID == b.ID && a.Name == b.Name && a.Path == b.Path && a.Args == b.Args && a.Icon == b.Icon && a.AddedAt == b.AddedAt && slices.Equal(a.EnvVars, b.EnvVars)
}
