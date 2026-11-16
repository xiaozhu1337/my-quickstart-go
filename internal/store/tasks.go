package store

import (
	"database/sql"

	"my-quickstart-go/internal/model"
)

const taskSchema = `CREATE TABLE IF NOT EXISTS scheduled_tasks (
 id TEXT PRIMARY KEY, name TEXT NOT NULL, daily_time TEXT NOT NULL,
 path TEXT NOT NULL, args TEXT NOT NULL, enabled INTEGER NOT NULL,
 last_scheduled_date TEXT NOT NULL, last_run_at INTEGER NOT NULL,
 last_status TEXT NOT NULL, last_error TEXT NOT NULL, position INTEGER NOT NULL
);`

func loadTasks(tx *sql.Tx) ([]model.ScheduledTask, error) {
	var exists int
	if err := tx.QueryRow("SELECT count(*) FROM sqlite_master WHERE type='table' AND name='scheduled_tasks'").Scan(&exists); err != nil {
		return nil, err
	}
	if exists == 0 {
		return nil, nil
	}
	rows, err := tx.Query(`SELECT id, name, daily_time, path, args, enabled,
 last_scheduled_date, last_run_at, last_status, last_error FROM scheduled_tasks ORDER BY position, rowid`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var tasks []model.ScheduledTask
	for rows.Next() {
		var task model.ScheduledTask
		if err := rows.Scan(&task.ID, &task.Name, &task.Time, &task.Path, &task.Args, &task.Enabled,
			&task.LastScheduledDate, &task.LastRunAt, &task.LastStatus, &task.LastError); err != nil {
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
	if _, err := tx.Exec("DELETE FROM scheduled_tasks"); err != nil {
		return err
	}
	stmt, err := tx.Prepare(`INSERT INTO scheduled_tasks
 (id, name, daily_time, path, args, enabled, last_scheduled_date, last_run_at, last_status, last_error, position)
 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return err
	}
	defer stmt.Close()
	for position, task := range tasks {
		if _, err := stmt.Exec(task.ID, task.Name, task.Time, task.Path, task.Args, task.Enabled,
			task.LastScheduledDate, task.LastRunAt, task.LastStatus, task.LastError, position); err != nil {
			return err
		}
	}
	return nil
}
