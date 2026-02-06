package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type SerpAPIResponse struct {
	JobsResults []SerpAPIJob `json:"jobs_results"`
}

type SerpAPIJob struct {
	Title              string `json:"title"`
	CompanyName        string `json:"company_name"`
	Location           string `json:"location"`
	Description        string `json:"description"`
	DetectedExtensions struct {
		PostedAt   string `json:"posted_at"`
		SalaryInfo string `json:"salary"`
	} `json:"detected_extensions"`
	ApplyOptions []struct {
		Link string `json:"link"`
	} `json:"apply_options"`
	JobID string `json:"job_id"`
}

func scanGoogleJobs(keywords []string, maxAgeDays int) ([]ScanResult, error) {
	apiKey := os.Getenv("SERP_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("SERP_API_KEY not set in .env")
	}

	query := strings.Join(keywords, " ")

	params := url.Values{}
	params.Set("engine", "google_jobs")
	params.Set("q", query)
	params.Set("api_key", apiKey)
	params.Set("gl", "us")
	params.Set("hl", "en")
	params.Set("location", "United States")

	if scanLocation != "" {
		locFilter := strings.ToLower(scanLocation)
		if strings.Contains(locFilter, "remote") {
			params.Set("ltype", "1")
		} else if strings.Contains(locFilter, "los angeles") {
			params.Set("location", "Los Angeles, CA")
		} else if strings.Contains(locFilter, "san francisco") || strings.Contains(locFilter, "sf") {
			params.Set("location", "San Francisco, CA")
		} else if strings.Contains(locFilter, "new york") || strings.Contains(locFilter, "nyc") {
			params.Set("location", "New York, NY")
		} else if strings.Contains(locFilter, "usa") || strings.Contains(locFilter, "us") {
			params.Set("location", "United States")
		} else {
			params.Set("location", scanLocation)
		}
	}

	apiURL := "https://serpapi.com/search?" + params.Encode()

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Get(apiURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("SerpAPI HTTP %d", resp.StatusCode)
	}

	var serpResp SerpAPIResponse
	if err := json.NewDecoder(resp.Body).Decode(&serpResp); err != nil {
		return nil, err
	}

	var results []ScanResult

	for _, j := range serpResp.JobsResults {
		ageDays := parsePostedAt(j.DetectedExtensions.PostedAt)
		if ageDays > maxAgeDays {
			continue
		}

		ageStr := j.DetectedExtensions.PostedAt
		if ageStr == "" {
			ageStr = "?"
		}

		jobURL := ""
		if len(j.ApplyOptions) > 0 {
			jobURL = j.ApplyOptions[0].Link
		}

		results = append(results, ScanResult{
			Title:    j.Title,
			Company:  j.CompanyName,
			URL:      jobURL,
			Age:      ageStr,
			AgeDays:  ageDays,
			Location: j.Location,
			Salary:   j.DetectedExtensions.SalaryInfo,
		})
	}

	return results, nil
}

func parsePostedAt(text string) int {
	text = strings.ToLower(text)

	patterns := []struct {
		re   *regexp.Regexp
		mult int
	}{
		{regexp.MustCompile(`(\d+)\s*hour`), 0},
		{regexp.MustCompile(`(\d+)\s*day`), 1},
		{regexp.MustCompile(`(\d+)\s*week`), 7},
		{regexp.MustCompile(`(\d+)\s*month`), 30},
	}

	for _, p := range patterns {
		if matches := p.re.FindStringSubmatch(text); matches != nil {
			n, _ := strconv.Atoi(matches[1])
			return n * p.mult
		}
	}

	return 0
}
