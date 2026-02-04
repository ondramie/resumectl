package main

import (
	"database/sql"
	"os"
	"path/filepath"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

var db *sql.DB

// Job represents a row in the jobs table
type Job struct {
	ID        int
	URL       string
	Company   string
	Title     string
	Score     int
	Status    string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// InitDB creates the database and tables if they don't exist
func InitDB() error {
	// Create ~/.resume-tailor/ directory
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	dataDir := filepath.Join(homeDir, ".resume-tailor")
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return err
	}

	dbPath := filepath.Join(dataDir, "data.db")
	db, err = sql.Open("sqlite3", dbPath)
	if err != nil {
		return err
	}

	// Create tables
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS jobs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			url TEXT UNIQUE,
			company TEXT,
			title TEXT,
			score INTEGER,
			status TEXT DEFAULT 'new',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)
	`)
	return err
}

// SaveJob inserts or updates a job in the database
func SaveJob(url, company, title string, score int) error {
	_, err := db.Exec(`
		INSERT INTO jobs (url, company, title, score, status)
		VALUES (?, ?, ?, ?, 'new')
		ON CONFLICT(url) DO UPDATE SET
			score = excluded.score,
			updated_at = CURRENT_TIMESTAMP
	`, url, company, title, score)
	return err
}

// ListJobs returns jobs filtered by status and min score
func ListJobs(status string, minScore int) ([]Job, error) {
	query := "SELECT id, url, company, title, score, status, created_at, updated_at FROM jobs WHERE 1=1"
	args := []interface{}{}

	if status != "" {
		query += " AND status = ?"
		args = append(args, status)
	}
	if minScore > 0 {
		query += " AND score >= ?"
		args = append(args, minScore)
	}
	query += " ORDER BY created_at DESC"

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var jobs []Job
	for rows.Next() {
		var j Job
		if err := rows.Scan(&j.ID, &j.URL, &j.Company, &j.Title, &j.Score, &j.Status, &j.CreatedAt, &j.UpdatedAt); err != nil {
			return nil, err
		}
		jobs = append(jobs, j)
	}
	return jobs, nil
}
