package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

var syncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Sync job statuses from Gmail",
	Run:   runSync,
}

type statusUpdate struct {
	JobID     int    `json:"job_id"`
	NewStatus string `json:"new_status"`
	Reason    string `json:"reason"`
}

func runSync(cmd *cobra.Command, args []string) {
	if err := InitDB(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	svc, err := getGmailService()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Gmail auth error: %v\n", err)
		os.Exit(1)
	}

	all, err := ListJobs("", 0)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error loading jobs: %v\n", err)
		os.Exit(1)
	}

	var active []Job
	for _, j := range all {
		switch j.Status {
		case "applied", "screening", "interview":
			active = append(active, j)
		}
	}

	if len(active) == 0 {
		fmt.Println("No active applications to sync.")
		return
	}

	fmt.Printf("Syncing %d active applications...\n", len(active))

	query := `(interview OR "job offer" OR "offer letter" OR "not moving forward" OR "we regret" OR "thank you for applying" OR "next steps" OR "phone screen" OR "unfortunately" OR "excited to move forward" OR "hired" OR "not selected") newer_than:90d category:primary`

	fmt.Println("Searching Gmail...")
	emails, err := fetchJobEmails(svc, query)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Gmail search error: %v\n", err)
		os.Exit(1)
	}

	if len(emails) == 0 {
		fmt.Println("No relevant emails found.")
		return
	}

	fmt.Printf("Found %d emails, classifying...\n", len(emails))

	updates, err := classifyEmails(active, emails)
	if err != nil {
		fmt.Fprintf(os.Stderr, "classification error: %v\n", err)
		os.Exit(1)
	}

	changed := 0
	for _, u := range updates {
		job := findJobByID(active, u.JobID)
		if job == nil || u.NewStatus == job.Status {
			continue
		}
		if err := UpdateJobStatus(u.JobID, u.NewStatus); err != nil {
			fmt.Fprintf(os.Stderr, "  error updating %s: %v\n", job.Company, err)
			continue
		}
		statusFn := color.YellowString
		if u.NewStatus == "rejected" {
			statusFn = color.RedString
		} else if u.NewStatus == "interview" || u.NewStatus == "offer" {
			statusFn = color.GreenString
		}
		fmt.Printf("  %s %s → %s (%s)\n", color.CyanString("→"), job.Company, statusFn(u.NewStatus), u.Reason)
		changed++
	}

	if changed == 0 {
		fmt.Println("No status changes detected.")
	}
}

func findJobByID(jobs []Job, id int) *Job {
	for i := range jobs {
		if jobs[i].ID == id {
			return &jobs[i]
		}
	}
	return nil
}

func classifyEmails(jobs []Job, emails []EmailSummary) ([]statusUpdate, error) {
	apiKey := os.Getenv("ANTHROPIC_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("ANTHROPIC_API_KEY not set")
	}

	var jobList strings.Builder
	for _, j := range jobs {
		jobList.WriteString(fmt.Sprintf("- ID %d: %s (%s), current status: %s\n", j.ID, j.Company, j.Title, j.Status))
	}

	var emailList strings.Builder
	for i, e := range emails {
		emailList.WriteString(fmt.Sprintf("[%d] From: %s\n    Subject: %s\n    Preview: %s\n\n", i+1, e.From, e.Subject, e.Snippet))
	}

	prompt := fmt.Sprintf(`You are analyzing job application emails to update statuses.

Active job applications:
%s
Recent emails:
%s
For each email that clearly indicates a status change for one of the active jobs, return an update.

Valid statuses: "screening" (phone screen scheduled), "interview" (technical or onsite interview), "offer" (job offer received), "rejected" (rejection or not moving forward).

Rules:
- Only return an update when you are confident the email matches an active job
- Match company names fuzzily (e.g. "Stripe" matches "stripe", "Stripe Inc")
- Skip ambiguous, automated, or newsletter-style emails
- Do not downgrade status (e.g. don't set "screening" if already "interview")

Return ONLY valid JSON array (no markdown):
[{"job_id": <id>, "new_status": "<status>", "reason": "<one short phrase>"}]

If no updates, return: []`, jobList.String(), emailList.String())

	reqBody := map[string]interface{}{
		"model":      "claude-sonnet-4-20250514",
		"max_tokens": 1000,
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
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("API error %d: %s", resp.StatusCode, string(body))
	}

	var apiResp struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return nil, err
	}
	if len(apiResp.Content) == 0 {
		return nil, fmt.Errorf("empty response")
	}

	text := strings.TrimSpace(apiResp.Content[0].Text)
	if strings.HasPrefix(text, "```") {
		if idx := strings.Index(text, "\n"); idx != -1 {
			text = text[idx+1:]
		}
		if idx := strings.LastIndex(text, "```"); idx != -1 {
			text = text[:idx]
		}
		text = strings.TrimSpace(text)
	}

	var updates []statusUpdate
	if err := json.Unmarshal([]byte(text), &updates); err != nil {
		return nil, fmt.Errorf("parse error: %v\nRaw: %s", err, text)
	}
	return updates, nil
}
