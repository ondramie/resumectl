package main

import (
	"crypto/sha256"
	"database/sql"
	"embed"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	_ "github.com/lib/pq"
)

//go:embed migrations/*.sql
var migrations embed.FS

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
	dbURL := os.Getenv("NEON_DB_URL")
	if dbURL == "" {
		return fmt.Errorf("NEON_DB_URL not set")
	}

	var err error
	db, err = sql.Open("postgres", dbURL)
	if err != nil {
		return err
	}

	src, err := iofs.New(migrations, "migrations")
	if err != nil {
		return err
	}
	driver, err := postgres.WithInstance(db, &postgres.Config{})
	if err != nil {
		return err
	}
	m, err := migrate.NewWithInstance("iofs", src, "postgres", driver)
	if err != nil {
		return err
	}
	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return err
	}
	return nil
}

func SaveJob(url, company, title string, score int) error {
	_, err := db.Exec(`
		INSERT INTO jobs (url, company, title, score, status)
		VALUES ($1, $2, $3, $4, 'new')
		ON CONFLICT(url) DO UPDATE SET
			score = EXCLUDED.score,
			updated_at = NOW()
	`, url, company, title, score)
	return err
}

func SaveMatchRun(jobURL string, score int, strongMatches, gaps []string, sourceHash, tailoredHash, outputDir string) error {
	var jobID int64
	err := db.QueryRow("SELECT id FROM jobs WHERE url = $1", jobURL).Scan(&jobID)
	if err != nil {
		return err
	}

	matchesJSON, _ := json.Marshal(strongMatches)
	gapsJSON, _ := json.Marshal(gaps)

	_, err = db.Exec(`
		INSERT INTO match_runs (job_id, score, strong_matches, gaps, source_resume_hash, tailored_resume_hash, output_dir)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
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
		WHERE url = $1 OR LOWER(company) LIKE '%' || LOWER($2) || '%' OR LOWER(title) LIKE '%' || LOWER($3) || '%'
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
	var err error
	switch status {
	case "applied":
		_, err = db.Exec(`UPDATE jobs SET status=$1, applied_at=NOW(), updated_at=NOW() WHERE id=$2`, status, id)
	case "rejected":
		_, err = db.Exec(`UPDATE jobs SET status=$1, rejected_at=NOW(), updated_at=NOW() WHERE id=$2`, status, id)
	default:
		_, err = db.Exec(`UPDATE jobs SET status=$1, updated_at=NOW() WHERE id=$2`, status, id)
	}
	return err
}

func ListJobs(status string, minScore int) ([]Job, error) {
	query := "SELECT id, url, company, title, score, status, created_at, updated_at FROM jobs WHERE 1=1"
	args := []interface{}{}
	i := 1

	if status != "" {
		query += fmt.Sprintf(" AND status = $%d", i)
		args = append(args, status)
		i++
	}
	if minScore > 0 {
		query += fmt.Sprintf(" AND score >= $%d", i)
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
