package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

// atsHandler routes a parsed URL to the appropriate fetcher
type atsHandler func(u *url.URL, pathParts []string) (*JobInfo, error)

// atsRoutes maps hostnames to their handlers
var atsRoutes = map[string]atsHandler{
	"boards.greenhouse.io":     handleGreenhouse,
	"job-boards.greenhouse.io": handleGreenhouse,
	"jobs.lever.co":            handleLever,
	"jobs.ashbyhq.com":         handleAshby,
	"ats.rippling.com":         handleRippling,
}

func handleGreenhouse(u *url.URL, parts []string) (*JobInfo, error) {
	// Embed URL: /embed/job_app?token=ID
	if token := u.Query().Get("token"); token != "" {
		return fetchGreenhouseEmbed(token)
	}
	// Standard: /{company}/jobs/{id}
	for i, p := range parts {
		if p == "jobs" && i+1 < len(parts) && i > 0 {
			return fetchGreenhouseJob(parts[i-1], parts[i+1])
		}
	}
	return nil, fmt.Errorf("could not parse Greenhouse URL: %s", u.String())
}

func handleLever(u *url.URL, parts []string) (*JobInfo, error) {
	// /{company}/{id}
	if len(parts) >= 2 {
		return fetchLeverJob(parts[0], parts[1])
	}
	return nil, fmt.Errorf("could not parse Lever URL: %s", u.String())
}

func handleAshby(u *url.URL, parts []string) (*JobInfo, error) {
	// /{company}/{id}
	if len(parts) >= 2 {
		return fetchAshbyJob(parts[0], parts[1])
	}
	return nil, fmt.Errorf("could not parse Ashby URL: %s", u.String())
}

func handleRippling(u *url.URL, parts []string) (*JobInfo, error) {
	// /{company}/jobs/{id}
	return fetchGenericJob(u.String())
}

func fetchJobDescription(rawURL string) (*JobInfo, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("invalid URL: %v", err)
	}

	// Split path into non-empty segments
	var pathParts []string
	for _, p := range strings.Split(u.Path, "/") {
		if p != "" {
			pathParts = append(pathParts, p)
		}
	}

	// Route by hostname
	if handler, ok := atsRoutes[u.Hostname()]; ok {
		return handler(u, pathParts)
	}

	// Check for gh_jid param (Greenhouse embedded on company sites)
	if ghJobID := u.Query().Get("gh_jid"); ghJobID != "" {
		company := strings.Split(u.Hostname(), ".")[0]
		fmt.Printf("  Detected Greenhouse job ID, trying API for %s...\n", company)
		if job, err := fetchGreenhouseJob(company, ghJobID); err == nil {
			return job, nil
		}
	}

	// Fallback to generic HTML scraping
	job, err := fetchGenericJob(rawURL)
	if err != nil {
		// If generic fails (e.g. 403), try Greenhouse API as fallback
		company := extractCompanyFromURL(rawURL)
		reqID := extractReqIDFromURL(rawURL)
		if reqID != "" {
			fmt.Printf("  Generic scrape failed (%v), trying Greenhouse API...\n", err)
			if ghJob, ghErr := fetchGreenhouseJob(company, reqID); ghErr == nil {
				return ghJob, nil
			}
		}
		return nil, err
	}

	// If generic returned empty/short description, try Greenhouse
	if len(job.Description) < 100 {
		company := extractCompanyFromURL(rawURL)
		reqID := extractReqIDFromURL(rawURL)
		if reqID != "" {
			fmt.Printf("  Job description looks empty, trying Greenhouse API...\n")
			if ghJob, ghErr := fetchGreenhouseJob(company, reqID); ghErr == nil {
				return ghJob, nil
			}
		}
	}

	return job, nil
}

func fetchGreenhouseEmbed(jobID string) (*JobInfo, error) {
	embedURL := fmt.Sprintf("https://boards.greenhouse.io/embed/job_app?token=%s", jobID)

	req, err := http.NewRequest("GET", embedURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("Greenhouse embed error: HTTP %d", resp.StatusCode)
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, err
	}

	// Extract title from h1
	title := strings.TrimSpace(doc.Find("h1.app-title").Text())
	if title == "" {
		title = strings.TrimSpace(doc.Find("h1").First().Text())
	}

	// Extract company from "at CompanyName" span
	companyText := strings.TrimSpace(doc.Find("span.company-name").Text())
	company := strings.TrimPrefix(companyText, "at ")
	company = strings.TrimSpace(strings.Split(company, "\n")[0])
	if company == "" {
		company = "unknown"
	}

	// Extract job description from the page body
	doc.Find("script, style, nav, header, footer, form, .application-form").Remove()
	var text strings.Builder
	doc.Find("#content, .job-post, .job__description, body").First().Each(func(i int, s *goquery.Selection) {
		text.WriteString(s.Text())
	})

	content := text.String()
	content = regexp.MustCompile(`\s+`).ReplaceAllString(content, " ")
	content = strings.TrimSpace(content)

	if content == "" {
		return nil, fmt.Errorf("could not extract job description from Greenhouse embed")
	}

	// Truncate if too long
	if len(content) > 15000 {
		content = content[:15000]
	}

	// Sanitize company name for use as directory
	companySafe := strings.ToLower(strings.ReplaceAll(company, " ", ""))

	return &JobInfo{
		Company:     companySafe,
		Title:       title,
		ReqID:       jobID,
		Description: content,
	}, nil
}

// GreenhouseJob represents the Greenhouse API response
type GreenhouseJob struct {
	Title    string `json:"title"`
	Content  string `json:"content"`
	Location struct {
		Name string `json:"name"`
	} `json:"location"`
	Departments []struct {
		Name string `json:"name"`
	} `json:"departments"`
}

func fetchGreenhouseJob(company, jobID string) (*JobInfo, error) {
	apiURL := fmt.Sprintf("https://boards-api.greenhouse.io/v1/boards/%s/jobs/%s", company, jobID)

	resp, err := http.Get(apiURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("Greenhouse API error: HTTP %d", resp.StatusCode)
	}

	var job GreenhouseJob
	if err := json.NewDecoder(resp.Body).Decode(&job); err != nil {
		return nil, err
	}

	// Strip HTML from content
	content := stripHTML(job.Content)

	// Build structured description
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Title: %s\n", job.Title))
	sb.WriteString(fmt.Sprintf("Company: %s\n", company))
	if job.Location.Name != "" {
		sb.WriteString(fmt.Sprintf("Location: %s\n", job.Location.Name))
	}
	if len(job.Departments) > 0 {
		sb.WriteString(fmt.Sprintf("Department: %s\n", job.Departments[0].Name))
	}
	sb.WriteString(fmt.Sprintf("\n%s", content))

	return &JobInfo{
		Company:     company,
		Title:       job.Title,
		ReqID:       jobID,
		Description: sb.String(),
	}, nil
}

// LeverJob represents the Lever API response
type LeverJob struct {
	Text       string `json:"text"`
	Categories struct {
		Location   string `json:"location"`
		Team       string `json:"team"`
		Department string `json:"department"`
	} `json:"categories"`
	Description      string `json:"description"`
	DescriptionPlain string `json:"descriptionPlain"`
	Lists            []struct {
		Text    string `json:"text"`
		Content string `json:"content"`
	} `json:"lists"`
}

func fetchLeverJob(company, jobID string) (*JobInfo, error) {
	apiURL := fmt.Sprintf("https://api.lever.co/v0/postings/%s/%s", company, jobID)

	resp, err := http.Get(apiURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("Lever API error: HTTP %d", resp.StatusCode)
	}

	var job LeverJob
	if err := json.NewDecoder(resp.Body).Decode(&job); err != nil {
		return nil, err
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Title: %s\n", job.Text))
	sb.WriteString(fmt.Sprintf("Company: %s\n", company))
	if job.Categories.Location != "" {
		sb.WriteString(fmt.Sprintf("Location: %s\n", job.Categories.Location))
	}
	if job.Categories.Team != "" {
		sb.WriteString(fmt.Sprintf("Team: %s\n", job.Categories.Team))
	}
	if job.Categories.Department != "" {
		sb.WriteString(fmt.Sprintf("Department: %s\n", job.Categories.Department))
	}

	// Use plain description if available, otherwise strip HTML
	if job.DescriptionPlain != "" {
		sb.WriteString(fmt.Sprintf("\n%s\n", job.DescriptionPlain))
	} else {
		sb.WriteString(fmt.Sprintf("\n%s\n", stripHTML(job.Description)))
	}

	// Add lists (requirements, qualifications, etc.)
	for _, list := range job.Lists {
		sb.WriteString(fmt.Sprintf("\n%s:\n%s\n", list.Text, stripHTML(list.Content)))
	}

	return &JobInfo{
		Company:     company,
		Title:       job.Text,
		ReqID:       jobID,
		Description: sb.String(),
	}, nil
}

func fetchAshbyJob(company, jobID string) (*JobInfo, error) {
	// Try board API first
	apiURL := fmt.Sprintf("https://api.ashbyhq.com/posting-api/job-board/%s", company)
	resp, err := http.Get(apiURL)
	if err == nil && resp.StatusCode == 200 {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		var board struct {
			Jobs []struct {
				ID               string `json:"id"`
				Title            string `json:"title"`
				DescriptionPlain string `json:"descriptionPlain"`
			} `json:"jobs"`
		}
		if err := json.Unmarshal(body, &board); err == nil {
			for _, job := range board.Jobs {
				if job.ID == jobID {
					return &JobInfo{
						Company:     company,
						Title:       job.Title,
						ReqID:       jobID,
						Description: job.DescriptionPlain,
					}, nil
				}
			}
		}
	} else if resp != nil {
		resp.Body.Close()
	}

	// Fallback: scrape the page (Ashby puts full description in meta tag)
	pageURL := fmt.Sprintf("https://jobs.ashbyhq.com/%s/%s", company, jobID)
	resp, err = http.Get(pageURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("Ashby page error: HTTP %d", resp.StatusCode)
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, err
	}

	title := doc.Find("title").First().Text()
	title = strings.TrimSpace(title)

	// Ashby puts the full job description in the meta description tag
	description, _ := doc.Find(`meta[name="description"]`).Attr("content")
	if description == "" {
		return nil, fmt.Errorf("could not extract job description from Ashby page")
	}

	return &JobInfo{
		Company:     company,
		Title:       title,
		ReqID:       jobID,
		Description: description,
	}, nil
}

func fetchGenericJob(jobURL string) (*JobInfo, error) {
	req, err := http.NewRequest("GET", jobURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, err
	}

	// Try to extract title
	title := doc.Find("title").First().Text()
	if title == "" {
		title = doc.Find("h1").First().Text()
	}
	title = strings.TrimSpace(title)

	// Try JSON-LD structured data first (common on Phenom, Workday, etc.)
	var content string
	doc.Find(`script[type="application/ld+json"]`).Each(func(i int, s *goquery.Selection) {
		if content != "" {
			return
		}
		var ld map[string]interface{}
		if err := json.Unmarshal([]byte(s.Text()), &ld); err != nil {
			return
		}
		if ld["@type"] == "JobPosting" {
			if desc, ok := ld["description"].(string); ok && len(desc) > 100 {
				// Strip HTML from description
				content = stripHTML(desc)
			}
			if t, ok := ld["title"].(string); ok && t != "" {
				title = t
			}
		}
	})

	// Fallback: extract body text
	if content == "" {
		doc.Find("script, style, nav, header, footer").Remove()
		var text strings.Builder
		doc.Find("body").Each(func(i int, s *goquery.Selection) {
			text.WriteString(s.Text())
		})
		content = text.String()
	}

	// Clean up whitespace
	content = regexp.MustCompile(`\s+`).ReplaceAllString(content, " ")
	content = strings.TrimSpace(content)

	// Truncate if too long
	if len(content) > 15000 {
		content = content[:15000]
	}

	// Extract company from URL as fallback
	company := extractCompanyFromURL(jobURL)

	return &JobInfo{
		Company:     company,
		Title:       title,
		ReqID:       extractReqIDFromURL(jobURL),
		Description: content,
	}, nil
}
