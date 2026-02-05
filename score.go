package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
)

func quickScore(resume, jobTitle, company string) (int, error) {
	apiKey := os.Getenv("ANTHROPIC_API_KEY")
	if apiKey == "" {
		return 0, fmt.Errorf("ANTHROPIC_API_KEY not set")
	}

	prompt := fmt.Sprintf(`Score resume fit for job "%s" at %s. Output ONLY: {"score":N} where N is 0-100.

%s`, jobTitle, company, resume)

	reqBody := map[string]interface{}{
		"model":      "claude-3-5-haiku-latest",
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
