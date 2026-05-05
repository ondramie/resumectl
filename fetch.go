package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"

	"github.com/PuerkitoBio/goquery"
	gopdf "github.com/ledongthuc/pdf"
)

type atsHandler func(u *url.URL, pathParts []string) (*JobInfo, error)

var atsRoutes = map[string]atsHandler{
	"boards.greenhouse.io":     handleGreenhouse,
	"job-boards.greenhouse.io": handleGreenhouse,
	"jobs.lever.co":            handleLever,
	"jobs.ashbyhq.com":         handleAshby,
	"ats.rippling.com":         handleRippling,
	"apply.workable.com":       handleWorkable,
	"jobs.gem.com":             handleGem,
}

func handleGreenhouse(u *url.URL, parts []string) (*JobInfo, error) {
	if token := u.Query().Get("token"); token != "" {
		job, err := fetchGreenhouseEmbed(token)
		if err != nil {
			return nil, err
		}
		if job.Company == "unknown" {
			if forCompany := u.Query().Get("for"); forCompany != "" {
				job.Company = forCompany
			}
		}
		return job, nil
	}
	for i, p := range parts {
		if p == "jobs" && i+1 < len(parts) && i > 0 {
			return fetchGreenhouseJob(parts[i-1], parts[i+1])
		}
	}
	return nil, fmt.Errorf("could not parse Greenhouse URL: %s", u.String())
}

func handleLever(u *url.URL, parts []string) (*JobInfo, error) {
	if len(parts) >= 2 {
		return fetchLeverJob(parts[0], parts[1])
	}
	return nil, fmt.Errorf("could not parse Lever URL: %s", u.String())
}

func handleAshby(u *url.URL, parts []string) (*JobInfo, error) {
	if len(parts) >= 2 {
		return fetchAshbyJob(parts[0], parts[1])
	}
	return nil, fmt.Errorf("could not parse Ashby URL: %s", u.String())
}

func handleRippling(u *url.URL, parts []string) (*JobInfo, error) {
	return fetchGenericJob(u.String())
}

func handleWorkable(u *url.URL, parts []string) (*JobInfo, error) {
	if len(parts) >= 3 && parts[1] == "j" {
		return fetchWorkableJob(parts[0], parts[2])
	}
	return nil, fmt.Errorf("could not parse Workable URL: %s", u.String())
}

func fetchWorkableJob(company, shortcode string) (*JobInfo, error) {
	apiURL := fmt.Sprintf("https://apply.workable.com/api/v2/accounts/%s/jobs/%s", company, shortcode)

	resp, err := http.Get(apiURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("Workable API error: HTTP %d", resp.StatusCode)
	}

	var job struct {
		Title        string   `json:"title"`
		Department   []string `json:"department"`
		Workplace    string   `json:"workplace"`
		Description  string   `json:"description"`
		Requirements string   `json:"requirements"`
		Benefits     string   `json:"benefits"`
		Location     struct {
			City    string `json:"city"`
			Country string `json:"country"`
		} `json:"location"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&job); err != nil {
		return nil, err
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Title: %s\n", job.Title))
	sb.WriteString(fmt.Sprintf("Company: %s\n", company))
	if job.Location.City != "" || job.Location.Country != "" {
		sb.WriteString(fmt.Sprintf("Location: %s, %s\n", job.Location.City, job.Location.Country))
	}
	if job.Workplace != "" {
		sb.WriteString(fmt.Sprintf("Workplace: %s\n", job.Workplace))
	}
	if len(job.Department) > 0 {
		sb.WriteString(fmt.Sprintf("Department: %s\n", strings.Join(job.Department, ", ")))
	}
	sb.WriteString(fmt.Sprintf("\n%s\n", stripHTML(job.Description)))
	if job.Requirements != "" {
		sb.WriteString(fmt.Sprintf("\nRequirements:\n%s\n", stripHTML(job.Requirements)))
	}
	if job.Benefits != "" {
		sb.WriteString(fmt.Sprintf("\nBenefits:\n%s\n", stripHTML(job.Benefits)))
	}

	return &JobInfo{
		Company:     company,
		Title:       job.Title,
		ReqID:       shortcode,
		Description: sb.String(),
	}, nil
}

func extractPDFText(filePath string) (string, error) {
	f, r, err := gopdf.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("error parsing PDF: %v", err)
	}
	defer f.Close()

	var sb strings.Builder
	for i := 1; i <= r.NumPage(); i++ {
		p := r.Page(i)
		if p.V.IsNull() {
			continue
		}
		text, err := p.GetPlainText(nil)
		if err != nil {
			continue
		}
		sb.WriteString(text)
	}
	return strings.TrimSpace(sb.String()), nil
}

func fetchPDFJob(pdfURL string) (*JobInfo, error) {
	resp, err := http.Get(pdfURL)
	if err != nil {
		return nil, fmt.Errorf("error downloading PDF: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("PDF download error: HTTP %d", resp.StatusCode)
	}

	tmpFile, err := os.CreateTemp("", "resumectl-*.pdf")
	if err != nil {
		return nil, err
	}
	defer os.Remove(tmpFile.Name())
	defer tmpFile.Close()

	if _, err := io.Copy(tmpFile, resp.Body); err != nil {
		return nil, err
	}
	tmpFile.Close()

	f, r, err := gopdf.Open(tmpFile.Name())
	if err != nil {
		return nil, fmt.Errorf("error parsing PDF: %v", err)
	}
	defer f.Close()

	var sb strings.Builder
	for i := 1; i <= r.NumPage(); i++ {
		p := r.Page(i)
		if p.V.IsNull() {
			continue
		}
		text, err := p.GetPlainText(nil)
		if err != nil {
			continue
		}
		sb.WriteString(text)
	}

	content := strings.TrimSpace(sb.String())
	if len(content) < 100 {
		return nil, fmt.Errorf("PDF text extraction returned too little content (%d chars)", len(content))
	}

	company := extractCompanyFromURL(pdfURL)
	reqID := extractReqIDFromURL(pdfURL)

	return &JobInfo{
		Company:     company,
		Title:       reqID,
		ReqID:       reqID,
		Description: content,
	}, nil
}

func fetchJobDescription(rawURL string) (*JobInfo, error) {
	rawURL = strings.ReplaceAll(rawURL, `\`, "")

	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("invalid URL: %v", err)
	}

	var pathParts []string
	for _, p := range strings.Split(u.Path, "/") {
		if p != "" {
			pathParts = append(pathParts, p)
		}
	}

	if strings.HasSuffix(strings.ToLower(u.Path), ".pdf") {
		return fetchPDFJob(rawURL)
	}

	if handler, ok := atsRoutes[u.Hostname()]; ok {
		return handler(u, pathParts)
	}

	if ashbyJID := u.Query().Get("ashby_jid"); ashbyJID != "" {
		company := extractCompanyFromURL(rawURL)
		fmt.Printf("  Detected Ashby job ID, trying API for %s...\n", company)
		candidates := []string{company}
		stripped := strings.TrimPrefix(company, "hello")
		stripped = strings.TrimPrefix(stripped, "get")
		stripped = strings.TrimPrefix(stripped, "try")
		stripped = strings.TrimSuffix(stripped, "hq")
		if stripped != company {
			candidates = append(candidates, stripped)
		}
		for _, c := range candidates {
			if job, err := fetchAshbyJob(c, ashbyJID); err == nil {
				return job, nil
			}
		}
	}

	if ghJobID := u.Query().Get("gh_jid"); ghJobID != "" {
		company := extractCompanyFromURL(rawURL)
		fmt.Printf("  Detected Greenhouse job ID, trying API for %s...\n", company)
		if job, err := fetchGreenhouseJob(company, ghJobID); err == nil {
			return job, nil
		}
	}

	job, err := fetchGenericJob(rawURL)
	if err != nil {
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

	title := strings.TrimSpace(doc.Find("h1.app-title").Text())
	if title == "" {
		title = strings.TrimSpace(doc.Find("h1").First().Text())
	}

	companyText := strings.TrimSpace(doc.Find("span.company-name").Text())
	company := strings.TrimPrefix(companyText, "at ")
	company = strings.TrimSpace(strings.Split(company, "\n")[0])
	if company == "" {
		company = "unknown"
	}

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

	if len(content) > 15000 {
		content = content[:15000]
	}

	companySafe := strings.ToLower(strings.ReplaceAll(company, " ", ""))

	return &JobInfo{
		Company:     companySafe,
		Title:       title,
		ReqID:       jobID,
		Description: content,
	}, nil
}

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

	content := stripHTML(job.Content)

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

	if job.DescriptionPlain != "" {
		sb.WriteString(fmt.Sprintf("\n%s\n", job.DescriptionPlain))
	} else {
		sb.WriteString(fmt.Sprintf("\n%s\n", stripHTML(job.Description)))
	}

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

func handleGem(u *url.URL, parts []string) (*JobInfo, error) {
	if len(parts) < 2 {
		return nil, fmt.Errorf("could not parse Gem URL: %s", u.String())
	}
	company, jobID := parts[0], parts[1]
	jobURL := u.String()

	boardContent, err := fetchViaJina("https://jobs.gem.com/" + company)
	title := ""
	if err == nil {
		for _, line := range strings.Split(boardContent, "\n") {
			if strings.Contains(line, jobID) {
				if start := strings.Index(line, "["); start != -1 {
					if end := strings.Index(line[start:], "]"); end != -1 {
						text := line[start+1 : start+end]
						if sep := strings.Index(text, " San Francisco"); sep != -1 {
							title = strings.TrimSpace(text[:sep])
						} else if sep := strings.Index(text, " •"); sep != -1 {
							title = strings.TrimSpace(text[:sep])
						} else {
							title = strings.TrimSpace(text)
						}
					}
				}
				break
			}
		}
	}

	jobContent, err := fetchViaJina(jobURL)
	if err != nil {
		return nil, fmt.Errorf("could not fetch Gem job: %v", err)
	}

	if idx := strings.Index(jobContent, "Markdown Content:"); idx != -1 {
		jobContent = strings.TrimSpace(jobContent[idx+len("Markdown Content:"):])
	}

	if title == "" {
		title = jobID
	}

	return &JobInfo{
		Company:     company,
		Title:       title,
		ReqID:       jobID,
		Description: jobContent,
	}, nil
}

func extractJinaTitle(jinaContent string) string {
	for _, line := range strings.SplitN(jinaContent, "\n", 10) {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "Title:") {
			return strings.TrimSpace(strings.TrimPrefix(line, "Title:"))
		}
	}
	return ""
}

func fetchViaJina(pageURL string) (string, error) {
	req, err := http.NewRequest("GET", "https://r.jina.ai/"+pageURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "text/plain")

	resp, err := (&http.Client{}).Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("Jina HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(body)), nil
}

func fetchGenericJob(jobURL string) (*JobInfo, error) {
	req, err := http.NewRequest("GET", jobURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	req.Header.Set("DNT", "1")
	req.Header.Set("Connection", "keep-alive")
	req.Header.Set("Upgrade-Insecure-Requests", "1")
	req.Header.Set("Sec-Fetch-Dest", "document")
	req.Header.Set("Sec-Fetch-Mode", "navigate")
	req.Header.Set("Sec-Fetch-Site", "none")
	req.Header.Set("Sec-Fetch-User", "?1")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == 403 {
		return nil, fmt.Errorf("HTTP 403 — this site blocks automated requests.\nCopy the job description from your browser and run:\n  resumectl match --file job.txt --company %s", extractCompanyFromURL(jobURL))
	}
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, err
	}

	title := doc.Find("title").First().Text()
	if title == "" {
		title = doc.Find("h1").First().Text()
	}
	title = strings.TrimSpace(title)

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
				content = stripHTML(desc)
			}
			if t, ok := ld["title"].(string); ok && t != "" {
				title = t
			}
		}
	})

	if content == "" {
		doc.Find("script, style, nav, header, footer").Remove()
		var text strings.Builder
		doc.Find("body").Each(func(i int, s *goquery.Selection) {
			text.WriteString(s.Text())
		})
		content = text.String()
	}

	content = regexp.MustCompile(`\s+`).ReplaceAllString(content, " ")
	content = strings.TrimSpace(content)

	if len(content) < 200 {
		fmt.Println("  Page appears JavaScript-rendered, trying Jina reader...")
		if jinaContent, err := fetchViaJina(jobURL); err == nil && len(jinaContent) > 200 {
			if t := extractJinaTitle(jinaContent); t != "" {
				title = t
			}
			if idx := strings.Index(jinaContent, "Markdown Content:"); idx != -1 {
				content = strings.TrimSpace(jinaContent[idx+len("Markdown Content:"):])
			} else {
				content = jinaContent
			}
		}
	}

	if len(content) > 15000 {
		content = content[:15000]
	}

	company := extractCompanyFromURL(jobURL)

	return &JobInfo{
		Company:     company,
		Title:       title,
		ReqID:       extractReqIDFromURL(jobURL),
		Description: content,
	}, nil
}
