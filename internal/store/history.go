package store

import (
	"database/sql"
	"time"
)

type HistoryPause struct {
	Start  int64
	End    sql.NullInt64
	Reason string
}

type HistoryTask struct {
	ID       int64
	Name     string
	Start    int64
	End      sql.NullInt64
	Quality  sql.NullString
	Focus    sql.NullString
	Category string
	Pauses   []HistoryPause
}

// History returns tasks overlapping the range with all their pauses. Open
// tasks count as running until now.
func (s *Store) History(from, until time.Time, now int64) ([]HistoryTask, error) {
	rows, err := s.db.Query(`
 SELECT t.id, t.name, t.started_at, t.ended_at, t.quality, t.focus, COALESCE(c.name, '-'),
        p.started_at, p.ended_at, p.reason
 FROM tasks t LEFT JOIN categories c ON c.id = t.category_id LEFT JOIN pauses p ON p.task_id = t.id
 WHERE t.started_at < ?
   AND (COALESCE(t.ended_at, ?) > ? OR t.started_at >= ?)
 ORDER BY t.started_at, t.id, p.started_at, p.id
 `, until.Unix(), now, from.Unix(), from.Unix())

	if err != nil {
		return nil, err
	}

	defer rows.Close()
	var tasks []HistoryTask

	for rows.Next() {
		var task HistoryTask
		var pauseStart, pauseEnd sql.NullInt64
		var reason sql.NullString

		if err := rows.Scan(&task.ID, &task.Name, &task.Start, &task.End, &task.Quality, &task.Focus, &task.Category, &pauseStart, &pauseEnd, &reason); err != nil {
			return nil, err
		}

		if len(tasks) == 0 || tasks[len(tasks)-1].ID != task.ID {
			tasks = append(tasks, task)
		}

		if pauseStart.Valid {
			last := &tasks[len(tasks)-1]
			last.Pauses = append(last.Pauses, HistoryPause{pauseStart.Int64, pauseEnd, reason.String})
		}
	}

	return tasks, rows.Err()
}
