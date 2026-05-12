package queue

import (
	"database/sql"
	"encoding/json"

	_ "github.com/mattn/go-sqlite3"
)

type Job struct {
	ID         int
	OrderNum   string
	ProjectID  int
	DocumentID int
	Polygons   []int
	Lang       string
	Token      string
	Attempts   int
}

type Queue struct {
	db *sql.DB
}

func New(path string) *Queue {
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		panic(err)
	}
	db.SetMaxOpenConns(1)

	db.Exec(`PRAGMA journal_mode = WAL;`)
	db.Exec(`PRAGMA busy_timeout = 5000;`)
	q := &Queue{db: db}
	q.init()
	return q
}

func (q *Queue) init() {
	schema := `
    CREATE TABLE IF NOT EXISTS jobs (
        id INTEGER PRIMARY KEY AUTOINCREMENT,
        order_num TEXT UNIQUE,
        project_id INTEGER,
        document_id INTEGER,
        polygons TEXT,
        lang TEXT,
        token TEXT,
        status TEXT,
        attempts INTEGER DEFAULT 0,
        last_error TEXT,
        created_at DATETIME DEFAULT CURRENT_TIMESTAMP
    );`

	_, err := q.db.Exec(schema)
	if err != nil {
		panic(err)
	}
}

func (q *Queue) AddWithToken(orderNum string, projectID int, documentID int, polygons []int, lang string, token string) {
	p, _ := json.Marshal(polygons)

	_, err := q.db.Exec(`
        INSERT OR IGNORE INTO jobs (order_num, project_id, document_id, polygons, lang, token, status)
        VALUES (?, ?, ?, ?, ?, ?, 'pending')`,
		orderNum, projectID, documentID, string(p), lang, token,
	)

	if err != nil {
		panic(err)
	}
}

func (q *Queue) FetchBatch(limit int) ([]Job, error) {
	tx, err := q.db.Begin()
	if err != nil {
		return nil, err
	}

	rows, err := tx.Query(`
        SELECT id, order_num, project_id, document_id, polygons, lang, token, attempts
        FROM jobs
        WHERE status = 'pending'
        LIMIT ?`, limit)

	if err != nil {
		tx.Rollback()
		return nil, err
	}
	defer rows.Close()

	var jobs []Job
	for rows.Next() {
		var j Job
		var polygonsStr string

		rows.Scan(
			&j.ID,
			&j.OrderNum,
			&j.ProjectID,
			&j.DocumentID,
			&polygonsStr,
			&j.Lang,
			&j.Token,
			&j.Attempts,
		)

		json.Unmarshal([]byte(polygonsStr), &j.Polygons)
		jobs = append(jobs, j)
	}

	for _, j := range jobs {
		tx.Exec(`UPDATE jobs SET status = 'processing' WHERE id = ?`, j.ID)
	}

	err = tx.Commit()
	if err != nil {
		return nil, err
	}

	return jobs, nil
}

func (q *Queue) MarkDone(id int) {
	q.db.Exec(`UPDATE jobs SET status='done' WHERE id=?`, id)
}

func (q *Queue) MarkFailed(id int, errMsg string) {
	q.db.Exec(`
        UPDATE jobs 
        SET status='failed', attempts=attempts+1, last_error=? 
        WHERE id=?`, errMsg, id)
}
