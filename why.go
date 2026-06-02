package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

func runWhy(_ *cobra.Command, args []string) {
	query := args[0]

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

	resume, err := os.ReadFile(resumePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error reading resume: %v\n", err)
		os.Exit(1)
	}

	job := &JobInfo{
		Company:     dbJob.Company,
		Title:       dbJob.Title,
		Description: string(jobDesc),
	}

	fmt.Fprintf(os.Stderr, "Generating answer for %s...\n", job.Company)

	answer, err := generateWhyAnswer(job, string(resume))
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	fmt.Println(answer)
}

func generateWhyAnswer(job *JobInfo, resume string) (string, error) {
	apiKey := os.Getenv("ANTHROPIC_API_KEY")
	if apiKey == "" {
		return "", fmt.Errorf("ANTHROPIC_API_KEY not set")
	}

	prompt := fmt.Sprintf(`Write a short answer (2-3 paragraphs) to the application question "Why do you want to join %s?" for this candidate applying to %s — %s.

RESUME:
%s

JOB POSTING:
%s

Rules:
- Ground it in the candidate's actual work — reference real things they've built, not abstract enthusiasm
- Connect specific technical challenges at %s to specific experience from the resume
- No corporate buzzwords, no mission-statement parroting, no "I'm passionate about..."
- The "why" must be earned through specifics, not declared
- Sound like a senior engineer talking to another engineer, not a cover letter
- 2-3 short paragraphs, tight and direct
- Do not use the word "passionate"
- Output only the answer text, no preamble, no explanation`, job.Company, job.Company, job.Title, resume, job.Description, job.Company)

	reqBody := map[string]interface{}{
		"model":      "claude-sonnet-4-20250514",
		"max_tokens": 1024,
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
