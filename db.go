package main

import (
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

var db *sql.DB

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

func InitDB() error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	dataDir := filepath.Join(homeDir, ".resumectl")
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return err
	}

	dbPath := filepath.Join(dataDir, "data.db")
	db, err = sql.Open("sqlite3", dbPath)
	if err != nil {
		return err
	}

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
	if err != nil {
		return err
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS match_runs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			job_id INTEGER REFERENCES jobs(id),
			score INTEGER,
			strong_matches TEXT,
			gaps TEXT,
			source_resume_hash TEXT,
			tailored_resume_hash TEXT,
			output_dir TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)
	`)
	return err
}

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

func SaveMatchRun(jobURL string, score int, strongMatches, gaps []string, sourceHash, tailoredHash, outputDir string) error {
	var jobID int64
	err := db.QueryRow("SELECT id FROM jobs WHERE url = ?", jobURL).Scan(&jobID)
	if err != nil {
		return err
	}

	matchesJSON, _ := json.Marshal(strongMatches)
	gapsJSON, _ := json.Marshal(gaps)

	_, err = db.Exec(`
		INSERT INTO match_runs (job_id, score, strong_matches, gaps, source_resume_hash, tailored_resume_hash, output_dir)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, jobID, score, string(matchesJSON), string(gapsJSON), sourceHash, tailoredHash, outputDir)
	return err
}

func contentHash(content string) string {
	return fmt.Sprintf("%x", sha256.Sum256([]byte(content)))[:12]
}

func FindJobByQuery(query string) (*Job, error) {
	row := db.QueryRow(`
		SELECT id, url, company, title, score, status, created_at, updated_at
		FROM jobs
		WHERE url = ? OR LOWER(company) LIKE '%' || LOWER(?) || '%' OR LOWER(title) LIKE '%' || LOWER(?) || '%'
		ORDER BY created_at DESC LIMIT 1`, query, query, query)
	var j Job
	err := row.Scan(&j.ID, &j.URL, &j.Company, &j.Title, &j.Score, &j.Status, &j.CreatedAt, &j.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("no job found matching %q", query)
	}
	return &j, err
}

func UpdateJobStatus(id int, status string) error {
	validStatuses := map[string]bool{"new": true, "applied": true, "screening": true, "interview": true, "offer": true, "rejected": true, "withdrawn": true}
	if !validStatuses[status] {
		return fmt.Errorf("invalid status %q: must be one of new, applied, screening, interview, offer, rejected, withdrawn", status)
	}
	now := time.Now()
	var err error
	switch status {
	case "applied":
		_, err = db.Exec(`UPDATE jobs SET status=?, applied_at=?, updated_at=? WHERE id=?`, status, now, now, id)
	case "rejected":
		_, err = db.Exec(`UPDATE jobs SET status=?, rejected_at=?, updated_at=? WHERE id=?`, status, now, now, id)
	default:
		_, err = db.Exec(`UPDATE jobs SET status=?, updated_at=? WHERE id=?`, status, now, id)
	}
	return err
}

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
