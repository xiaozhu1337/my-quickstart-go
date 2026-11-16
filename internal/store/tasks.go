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
	if _, err := tx.Exec("DELETE FROM scheduled_tasks"); err != nil {
		return err
	}
	stmt, err := tx.Prepare(`INSERT INTO scheduled_tasks
 (id, name, daily_time, path, args, enabled, last_scheduled_date, last_run_at, last_status, last_error, position, python_path, command)
 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return err
	}
	defer stmt.Close()
	for position, task := range tasks {
		if _, err := stmt.Exec(task.ID, task.Name, task.Time, task.Path, task.Args, task.Enabled,
			task.LastScheduledDate, task.LastRunAt, task.LastStatus, task.LastError, position, task.PythonPath, task.Command); err != nil {
			return err
		}
	}
	return nil
}

func hasTaskPythonPath(tx *sql.Tx) (bool, error) {
	return hasTaskColumn(tx, "python_path")
}

func hasTaskColumn(tx *sql.Tx, name string) (bool, error) {
	var count int
	err := tx.QueryRow("SELECT count(*) FROM pragma_table_info('scheduled_tasks') WHERE name = ?", name).Scan(&count)
	return count != 0, err
}
