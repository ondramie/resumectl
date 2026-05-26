package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

func runPrep(cmd *cobra.Command, args []string) {
	query := args[0]

	var job *JobInfo
	var err error

	if strings.HasPrefix(query, "http://") || strings.HasPrefix(query, "https://") {
		fmt.Fprintf(os.Stderr, "Fetching job description...\n")
		job, err = fetchJobDescription(query)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error fetching job: %v\n", err)
			os.Exit(1)
		}
	} else {
		if err := InitDB(); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		dbJob, err := FindJobByQuery(query)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "Found: [%d] %s — %s\n", dbJob.ID, dbJob.Company, dbJob.Title)

		outputDir, err := findLatestOutputDir(dbJob.ID)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: no match run found for this job (run 'resumectl match' first): %v\n", err)
			os.Exit(1)
		}

		jobDesc, err := os.ReadFile(filepath.Join(outputDir, "job.txt"))
		if err != nil {
			fmt.Fprintf(os.Stderr, "error reading cached job description: %v\n", err)
			os.Exit(1)
		}

		job = &JobInfo{
			Company:     dbJob.Company,
			Title:       dbJob.Title,
			Description: string(jobDesc),
		}
	}

	fmt.Fprintf(os.Stderr, "Generating prep doc for %s — %s...\n", job.Company, job.Title)

	resume, err := os.ReadFile(resumePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error reading resume: %v\n", err)
		os.Exit(1)
	}

	doc, err := generatePrepDoc(job, string(resume))
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	fmt.Println(doc)
}

func findLatestOutputDir(jobID int) (string, error) {
	var outputDir string
	err := db.QueryRow(`
		SELECT output_dir FROM match_runs
		WHERE job_id = $1
		ORDER BY created_at DESC LIMIT 1`, jobID).Scan(&outputDir)
	return outputDir, err
}

func generatePrepDoc(job *JobInfo, resume string) (string, error) {
	apiKey := os.Getenv("ANTHROPIC_API_KEY")
	if apiKey == "" {
		return "", fmt.Errorf("ANTHROPIC_API_KEY not set")
	}

	prompt := fmt.Sprintf(`Generate a thorough interview prep document in Markdown for this candidate.

RESUME:
%s

JOB POSTING (%s — %s):
%s

Write the document with exactly these sections:

# Interview Prep: %s — %s

## What %s Does
2-3 paragraphs: what the company does, their business model, who their customers are, their stage/scale, what makes them distinct. Be specific — pull from the job description.

## How You Can Help Them
Specific ways this candidate's background maps to their actual needs. Reference real resume line items against real job requirements. Be concrete, not generic.

## What to Focus On
The 3-5 most important things to emphasize in this interview — what will matter most to them. Ordered by importance.

## Stories to Tell
6-8 specific STAR stories from the resume most relevant to this role. For each:
- **[Story title]**: Situation/context → what you did → the result (numbers if present in resume).
Make them interview-ready.

## Questions to Ask
10 thoughtful questions — mix of role-specific, team/org, and strategic. Show genuine curiosity about their problems.

## Watch Out For
Likely concerns or gaps they'll probe (based on the JD vs resume), and how to address them directly if asked.`, resume, job.Company, job.Title, job.Description, job.Company, job.Title, job.Company)

	reqBody := map[string]interface{}{
		"model":      "claude-sonnet-4-20250514",
		"max_tokens": 4096,
		"messages": []map[string]string{
			{"role": "user", "content": prompt},
		},
	}

	jsonBody, _ := json.Marshal(reqBody)

	req, _ := http.NewRequest("POST", "https://api.anthropic.com/v1/messages", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := (&http.Client{}).Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("API error %d: %s", resp.StatusCode, string(body))
	}

	var apiResp struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return "", err
	}
	if len(apiResp.Content) == 0 {
		return "", fmt.Errorf("empty response")
	}

	return apiResp.Content[0].Text, nil
}
