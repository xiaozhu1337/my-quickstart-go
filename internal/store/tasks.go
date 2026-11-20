package store

import (
	"database/sql"

	"my-quickstart-go/internal/model"
)

const taskSchema = `CREATE TABLE IF NOT EXISTS scheduled_tasks (
 id TEXT PRIMARY KEY, name TEXT NOT NULL, daily_time TEXT NOT NULL,
 path TEXT NOT NULL, args TEXT NOT NULL, enabled INTEGER NOT NULL,
 last_scheduled_date TEXT NOT NULL, last_run_at INTEGER NOT NULL,
 last_status TEXT NOT NULL, last_error TEXT NOT NULL, position INTEGER NOT NULL,
 python_path TEXT NOT NULL DEFAULT '', command TEXT NOT NULL DEFAULT ''
);`

func loadTasks(tx *sql.Tx) ([]model.ScheduledTask, error) {
	var exists int
	if err := tx.QueryRow("SELECT count(*) FROM sqlite_master WHERE type='table' AND name='scheduled_tasks'").Scan(&exists); err != nil {
		return nil, err
	}
	if exists == 0 {
		return nil, nil
	}
	hasPython, err := hasTaskPythonPath(tx)
	if err != nil {
		return nil, err
	}
	pythonColumn := "''"
	if hasPython {
		pythonColumn = "python_path"
	}
	hasCommand, err := hasTaskColumn(tx, "command")
	if err != nil {
		return nil, err
	}
	commandColumn := "''"
	if hasCommand {
		commandColumn = "command"
	}
	rows, err := tx.Query(`SELECT id, name, daily_time, path, args, enabled,
 last_scheduled_date, last_run_at, last_status, last_error, ` + pythonColumn + ", " + commandColumn + ` FROM scheduled_tasks ORDER BY position, rowid`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var tasks []model.ScheduledTask
	for rows.Next() {
		var task model.ScheduledTask
		if err := rows.Scan(&task.ID, &task.Name, &task.Time, &task.Path, &task.Args, &task.Enabled,
			&task.LastScheduledDate, &task.LastRunAt, &task.LastStatus, &task.LastError, &task.PythonPath, &task.Command); err != nil {
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
 (id, name, daily_time, path, args, enabled, last_scheduled_date, last_run_at, last_status, last_error, position, python_path, command)
 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

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
			task.LastScheduledDate, task.LastRunAt, task.LastStatus, task.LastError, position, task.PythonPath, task.Command); err != nil {
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

const taskUpdates = `name=excluded.name,daily_time=excluded.daily_time,path=excluded.path,args=excluded.args,enabled=excluded.enabled,last_scheduled_date=excluded.last_scheduled_date,last_run_at=excluded.last_run_at,last_status=excluded.last_status,last_error=excluded.last_error,position=excluded.position,python_path=excluded.python_path,command=excluded.command`

func hasTaskPythonPath(tx *sql.Tx) (bool, error) {
	return hasTaskColumn(tx, "python_path")
}

func hasTaskColumn(tx *sql.Tx, name string) (bool, error) {
	var count int
	err := tx.QueryRow("SELECT count(*) FROM pragma_table_info('scheduled_tasks') WHERE name = ?", name).Scan(&count)
	return count != 0, err
}

func ensureTaskColumns(tx *sql.Tx) error {
	if _, err := tx.Exec(taskSchema); err != nil {
		return err
	}
	hasPython, err := hasTaskPythonPath(tx)
	if err != nil {
		return err
	}
	if !hasPython {
		if _, err := tx.Exec("ALTER TABLE scheduled_tasks ADD COLUMN python_path TEXT NOT NULL DEFAULT ''"); err != nil {
			return err
		}
	}
	hasCommand, err := hasTaskColumn(tx, "command")
	if err != nil {
		return err
	}
	if !hasCommand {
		if _, err := tx.Exec("ALTER TABLE scheduled_tasks ADD COLUMN command TEXT NOT NULL DEFAULT ''"); err != nil {
			return err
		}
	}
	return nil
}
