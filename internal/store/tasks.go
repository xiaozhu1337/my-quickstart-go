package store

import (
	"database/sql"
	"strings"

	"my-quickstart-go/internal/model"
)

const taskSchema = `CREATE TABLE IF NOT EXISTS scheduled_tasks (
 id TEXT PRIMARY KEY, name TEXT NOT NULL, daily_time TEXT NOT NULL,
 path TEXT NOT NULL, args TEXT NOT NULL, enabled INTEGER NOT NULL,
 last_scheduled_date TEXT NOT NULL, last_run_at INTEGER NOT NULL,
 last_status TEXT NOT NULL, last_error TEXT NOT NULL, position INTEGER NOT NULL,
 python_path TEXT NOT NULL DEFAULT '', command TEXT NOT NULL DEFAULT '', at_boot INTEGER NOT NULL DEFAULT 0,
 window_mode TEXT NOT NULL DEFAULT '', last_boot_id TEXT NOT NULL DEFAULT ''
);`

var optionalTaskColumns = []struct{ name, kind, fallback string }{
	{"python_path", "TEXT", "''"}, {"command", "TEXT", "''"}, {"at_boot", "INTEGER", "0"},
	{"window_mode", "TEXT", "''"}, {"last_boot_id", "TEXT", "''"},
}

func taskColumns(tx *sql.Tx) (map[string]bool, error) {
	rows, err := tx.Query("SELECT name FROM pragma_table_info('scheduled_tasks')")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	cols := map[string]bool{}
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		cols[name] = true
	}
	return cols, rows.Err()
}

func loadTasks(tx *sql.Tx) ([]model.ScheduledTask, error) {
	cols, err := taskColumns(tx)
	if err != nil || len(cols) == 0 {
		return nil, err
	}
	names := make([]string, 0, len(optionalTaskColumns))
	for _, col := range optionalTaskColumns {
		if cols[col.name] {
			names = append(names, col.name)
		} else {
			names = append(names, col.fallback)
		}
	}
	rows, err := tx.Query(`SELECT id, name, daily_time, path, args, enabled,
 last_scheduled_date, last_run_at, last_status, last_error, ` + strings.Join(names, ", ") + ` FROM scheduled_tasks ORDER BY position, rowid`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var tasks []model.ScheduledTask
	for rows.Next() {
		var task model.ScheduledTask
		if err := rows.Scan(&task.ID, &task.Name, &task.Time, &task.Path, &task.Args, &task.Enabled,
			&task.LastScheduledDate, &task.LastRunAt, &task.LastStatus, &task.LastError, &task.PythonPath, &task.Command, &task.AtBoot, &task.WindowMode, &task.LastBootID); err != nil {
			return nil, err
		}
		tasks = append(tasks, task)
	}
	return tasks, rows.Err()
}

func saveTasks(tx *sql.Tx, tasks []model.ScheduledTask) error {
	if err := ensureTaskColumns(tx); err != nil {
		return err
	}
	if _, err := tx.Exec("DELETE FROM scheduled_tasks"); err != nil {
		return err
	}
	return writeTaskChanges(tx, nil, tasks)
}

const taskInsert = `INSERT INTO scheduled_tasks
 (id, name, daily_time, path, args, enabled, last_scheduled_date, last_run_at, last_status, last_error, position, python_path, command, at_boot, window_mode, last_boot_id)
 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

func writeTaskChanges(tx *sql.Tx, previous, tasks []model.ScheduledTask) error {
	old := make(map[string]int, len(previous))
	for i, t := range previous {
		old[t.ID] = i
	}
	stmt, err := tx.Prepare(taskInsert + " ON CONFLICT(id) DO UPDATE SET " + taskUpdates)
	if err != nil {
		return err
	}
	defer stmt.Close()
	for position, task := range tasks {
		index, exists := old[task.ID]
		delete(old, task.ID)
		if exists && index == position && previous[index] == task {
			continue
		}
		if err := task.Validate(); err != nil {
			return err
		}
		if _, err := stmt.Exec(task.ID, task.Name, task.Time, task.Path, task.Args, task.Enabled,
			task.LastScheduledDate, task.LastRunAt, task.LastStatus, task.LastError, position, task.PythonPath, task.Command, task.AtBoot, task.WindowMode, task.LastBootID); err != nil {
			return err
		}
	}
	for id := range old {
		if _, err := tx.Exec("DELETE FROM scheduled_tasks WHERE id=?", id); err != nil {
			return err
		}
	}
	return nil
}

const taskUpdates = `name=excluded.name,daily_time=excluded.daily_time,path=excluded.path,args=excluded.args,enabled=excluded.enabled,last_scheduled_date=excluded.last_scheduled_date,last_run_at=excluded.last_run_at,last_status=excluded.last_status,last_error=excluded.last_error,position=excluded.position,python_path=excluded.python_path,command=excluded.command,at_boot=excluded.at_boot,window_mode=excluded.window_mode,last_boot_id=excluded.last_boot_id`

func ensureTaskColumns(tx *sql.Tx) error {
	if _, err := tx.Exec(taskSchema); err != nil {
		return err
	}
	cols, err := taskColumns(tx)
	if err != nil {
		return err
	}
	for _, col := range optionalTaskColumns {
		if !cols[col.name] {
			if _, err := tx.Exec("ALTER TABLE scheduled_tasks ADD COLUMN " + col.name + " " + col.kind + " NOT NULL DEFAULT " + col.fallback); err != nil {
				return err
			}
		}
	}
	return nil
}
