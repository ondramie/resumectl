package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type RemoteOKJob struct {
	ID       string   `json:"id"`
	Position string   `json:"position"`
	Company  string   `json:"company"`
	Location string   `json:"location"`
	Tags     []string `json:"tags"`
	URL      string   `json:"url"`
	Epoch    int64    `json:"epoch"`
}

func scanRemoteOK(keywords []string, maxAgeDays int) ([]ScanResult, error) {
	req, err := http.NewRequest("GET", "https://remoteok.com/api", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7)")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	var jobs []RemoteOKJob
	if err := json.NewDecoder(resp.Body).Decode(&jobs); err != nil {
		return nil, err
	}

	keywordsLower := make([]string, len(keywords))
	for i, k := range keywords {
		keywordsLower[i] = strings.ToLower(strings.TrimSpace(k))
	}

	var results []ScanResult
	now := time.Now()
	cutoff := now.AddDate(0, 0, -maxAgeDays)

	for _, j := range jobs {
		if j.ID == "" || j.Position == "" {
			continue
		}

		posted := time.Unix(j.Epoch, 0)
		if posted.Before(cutoff) {
			continue
		}

		posLower := strings.ToLower(j.Position)
		tagsLower := strings.ToLower(strings.Join(j.Tags, " "))
		match := false
		for _, kw := range keywordsLower {
			if strings.Contains(posLower, kw) || strings.Contains(tagsLower, kw) {
				match = true
				break
			}
		}
		if !match {
			continue
		}

		age := now.Sub(posted)
		ageStr := formatAge(age)

		loc := j.Location
		if loc == "" {
			loc = "Remote"
		}

		results = append(results, ScanResult{
			Title:    j.Position,
			Company:  j.Company,
			URL:      j.URL,
			Age:      ageStr,
			AgeDays:  int(age.Hours() / 24),
			Location: loc,
		})
	}

	return results, nil
}

func formatAge(d time.Duration) string {
	hours := int(d.Hours())
	if hours < 24 {
		return fmt.Sprintf("%dh", hours)
	}
	days := hours / 24
	if days < 7 {
		return fmt.Sprintf("%dd", days)
	}
	weeks := days / 7
	return fmt.Sprintf("%dw", weeks)
}
