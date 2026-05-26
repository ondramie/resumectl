package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

type scoredTemplate struct {
	path  string
	label string
	score int
}

func templateLabel(path string) string {
	base := strings.TrimSuffix(filepath.Base(path), ".tex")
	parts := strings.SplitN(base, ".", 3)
	if len(parts) == 3 {
		return parts[2]
	}
	return base
}

func compareTemplates(templates []scoredTemplate, jobTitle, jobDescription string) (scoredTemplate, error) {
	apiKey := os.Getenv("ANTHROPIC_API_KEY")
	if apiKey == "" {
		return templates[0], fmt.Errorf("ANTHROPIC_API_KEY not set")
	}

	var sb strings.Builder
	for i, t := range templates {
		resume, err := os.ReadFile(t.path)
		if err != nil {
			continue
		}
		sb.WriteString(fmt.Sprintf("=== RESUME %d (focus: %s) ===\n%s\n\n", i+1, t.label, string(resume)))
	}

	prompt := fmt.Sprintf(`You are selecting the best resume variant for a job application.
Job title: "%s"

Each resume is labeled with its focus area. Select based on:
1. Role type alignment — does the resume's focus match the job's primary domain? (weighted heavily)
2. Skill keyword overlap
3. How the experience is framed

Output ONLY: {"winner":N} where N is the resume number (1 or 2).

%s
=== JOB DESCRIPTION ===
%s`, jobTitle, sb.String(), jobDescription)

	reqBody := map[string]interface{}{
		"model":      "claude-haiku-4-5-20251001",
		"max_tokens": 50,
		"messages": []map[string]string{
			{"role": "user", "content": prompt},
		},
	}

	jsonBody, _ := json.Marshal(reqBody)

	req, _ := http.NewRequest("POST", "https://api.anthropic.com/v1/messages", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return templates[0], err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != 200 {
		return templates[0], fmt.Errorf("API error %d: %s", resp.StatusCode, string(body))
	}

	var apiResp struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return templates[0], err
	}

	if len(apiResp.Content) == 0 {
		return templates[0], fmt.Errorf("empty response")
	}

	text := strings.TrimSpace(apiResp.Content[0].Text)
	start := strings.Index(text, "{")
	end := strings.LastIndex(text, "}")
	if start == -1 || end == -1 {
		return templates[0], fmt.Errorf("no JSON in response")
	}

	var result struct {
		Winner int `json:"winner"`
	}
	if err := json.Unmarshal([]byte(text[start:end+1]), &result); err != nil {
		return templates[0], err
	}

	idx := result.Winner - 1
	if idx >= 0 && idx < len(templates) {
		return templates[idx], nil
	}
	return templates[0], nil
}

func scoreTemplate(resume, jobTitle, jobDescription, label string) (int, error) {
	apiKey := os.Getenv("ANTHROPIC_API_KEY")
	if apiKey == "" {
		return 0, fmt.Errorf("ANTHROPIC_API_KEY not set")
	}

	prompt := fmt.Sprintf(`Score how well this resume variant matches the job. This resume has a "%s" focus.
Weight your score equally between: (1) role type alignment — does the resume's focus match the job title "%s"? and (2) skill/keyword overlap with the job description.
Output ONLY: {"score":N} where N is 0-100.

RESUME:
%s

JOB DESCRIPTION:
%s`, label, jobTitle, resume, jobDescription)

	reqBody := map[string]interface{}{
		"model":      "claude-haiku-4-5-20251001",
		"max_tokens": 50,
		"messages": []map[string]string{
			{"role": "user", "content": prompt},
		},
	}

	jsonBody, _ := json.Marshal(reqBody)

	req, _ := http.NewRequest("POST", "https://api.anthropic.com/v1/messages", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != 200 {
		return 0, fmt.Errorf("API error %d: %s", resp.StatusCode, string(body))
	}

	var apiResp struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return 0, err
	}

	if len(apiResp.Content) == 0 {
		return 0, fmt.Errorf("empty response")
	}

	text := strings.TrimSpace(apiResp.Content[0].Text)
	start := strings.Index(text, "{")
	end := strings.LastIndex(text, "}")
	if start == -1 || end == -1 || end <= start {
		re := regexp.MustCompile(`\b(\d{1,3})\b`)
		if matches := re.FindStringSubmatch(text); matches != nil {
			score, _ := strconv.Atoi(matches[1])
			if score >= 0 && score <= 100 {
				return score, nil
			}
		}
		return 0, fmt.Errorf("no score found in: %s", text)
	}

	var result struct {
		Score int `json:"score"`
	}
	if err := json.Unmarshal([]byte(text[start:end+1]), &result); err != nil {
		return 0, fmt.Errorf("parse error: %v", err)
	}

	return result.Score, nil
}

func quickScore(resume, jobDescription, company string) (int, error) {
	apiKey := os.Getenv("ANTHROPIC_API_KEY")
	if apiKey == "" {
		return 0, fmt.Errorf("ANTHROPIC_API_KEY not set")
	}

	prompt := fmt.Sprintf(`Score how well this resume matches the job description. Output ONLY: {"score":N} where N is 0-100.

RESUME:
%s

JOB DESCRIPTION:
%s`, resume, jobDescription)

	reqBody := map[string]interface{}{
		"model":      "claude-haiku-4-5-20251001",
		"max_tokens": 50,
		"messages": []map[string]string{
			{"role": "user", "content": prompt},
		},
	}

	jsonBody, _ := json.Marshal(reqBody)

	req, _ := http.NewRequest("POST", "https://api.anthropic.com/v1/messages", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != 200 {
		return 0, fmt.Errorf("API error %d: %s", resp.StatusCode, string(body))
	}

	var apiResp struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return 0, err
	}

	if len(apiResp.Content) == 0 {
		return 0, fmt.Errorf("empty response")
	}

	text := strings.TrimSpace(apiResp.Content[0].Text)

	start := strings.Index(text, "{")
	end := strings.LastIndex(text, "}")
	if start == -1 || end == -1 || end <= start {
		re := regexp.MustCompile(`\b(\d{1,3})\b`)
		if matches := re.FindStringSubmatch(text); matches != nil {
			score, _ := strconv.Atoi(matches[1])
			if score >= 0 && score <= 100 {
				return score, nil
			}
		}
		preview := text
		if len(preview) > 50 {
			preview = preview[:50]
		}
		return 0, fmt.Errorf("no score found in: %s", preview)
	}

	jsonStr := text[start : end+1]
	var result struct {
		Score int `json:"score"`
	}
	if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
		return 0, fmt.Errorf("parse error: %v", err)
	}

	return result.Score, nil
}
