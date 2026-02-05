package main

import (
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

func scanWeb3Career(keywords []string, maxAgeDays int) ([]ScanResult, error) {
	var allResults []ScanResult
	seen := make(map[string]bool)

	for _, keyword := range keywords {
		keyword = strings.TrimSpace(strings.ToLower(keyword))
		keyword = strings.ReplaceAll(keyword, " ", "-")
		searchURL := fmt.Sprintf("https://web3.career/%s-jobs", keyword)

		results, err := fetchWeb3CareerPage(searchURL, maxAgeDays)
		if err != nil {
			fmt.Printf("  Warning: %s - %v\n", keyword, err)
			continue
		}

		for _, r := range results {
			if !seen[r.URL] {
				seen[r.URL] = true
				allResults = append(allResults, r)
			}
		}
	}

	return allResults, nil
}

func fetchWeb3CareerPage(searchURL string, maxAgeDays int) ([]ScanResult, error) {

	req, err := http.NewRequest("GET", searchURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36")

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

	var results []ScanResult

	doc.Find("tr[data-jobid]").Each(func(i int, s *goquery.Selection) {
		tds := s.Find("td")
		if tds.Length() < 4 {
			return
		}

		title := strings.TrimSpace(tds.Eq(0).Text())
		company := strings.TrimSpace(tds.Eq(1).Text())
		ageText := strings.TrimSpace(tds.Eq(2).Text())
		location := strings.TrimSpace(tds.Eq(3).Text())

		if title == "" {
			return
		}

		jobID, _ := s.Attr("data-jobid")
		slug := strings.ToLower(strings.ReplaceAll(title, " ", "-"))
		slug = regexp.MustCompile(`[^a-z0-9-]`).ReplaceAllString(slug, "")
		jobURL := fmt.Sprintf("https://web3.career/%s-%s/%s", slug, strings.ToLower(strings.ReplaceAll(company, " ", "")), jobID)

		href, exists := tds.Eq(0).Find("a").Attr("href")
		if exists && href != "" {
			if !strings.HasPrefix(href, "http") {
				jobURL = "https://web3.career" + href
			} else {
				jobURL = href
			}
		}

		ageDays := parseAgeDays(ageText)
		if ageDays < 0 {
			ageDays = 0
		}
		if ageDays > maxAgeDays {
			return
		}

		salary := ""
		tds.Each(func(j int, td *goquery.Selection) {
			text := strings.TrimSpace(td.Text())
			if strings.Contains(text, "$") && strings.Contains(text, "k") {
				salary = text
			}
		})

		results = append(results, ScanResult{
			Title:    title,
			Company:  company,
			URL:      jobURL,
			Age:      ageText,
			AgeDays:  ageDays,
			Location: location,
			Salary:   salary,
		})
	})

	if len(results) == 0 {
		results, err = scanWeb3CareerFallback(doc, maxAgeDays)
		if err != nil {
			return nil, err
		}
	}

	return results, nil
}

func scanWeb3CareerFallback(doc *goquery.Document, maxAgeDays int) ([]ScanResult, error) {
	var results []ScanResult

	doc.Find("a[href*='/']").Each(func(i int, s *goquery.Selection) {
		href, exists := s.Attr("href")
		if !exists {
			return
		}

		matched, _ := regexp.MatchString(`/[a-z-]+-[a-z-]+/\d+$`, href)
		if !matched {
			return
		}

		title := strings.TrimSpace(s.Text())
		if title == "" || len(title) < 10 || len(title) > 100 {
			return
		}

		jobURL := href
		if !strings.HasPrefix(href, "http") {
			jobURL = "https://web3.career" + href
		}

		parent := s.Parent().Parent()
		ageText := ""
		ageDays := 0
		parent.Find("*").Each(func(j int, el *goquery.Selection) {
			text := strings.TrimSpace(el.Text())
			if days := parseAgeDays(text); days >= 0 && ageText == "" {
				ageText = text
				ageDays = days
			}
		})

		if ageDays > maxAgeDays {
			return
		}

		results = append(results, ScanResult{
			Title:   title,
			Company: "unknown",
			URL:     jobURL,
			Age:     ageText,
			AgeDays: ageDays,
		})
	})

	return results, nil
}

func extractCompanyFromJobRow(s *goquery.Selection) string {
	text := s.Text()
	parts := strings.Split(text, "\n")
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if len(p) > 2 && len(p) < 50 && !strings.Contains(p, "$") && !strings.Contains(p, "ago") {
			return p
		}
	}
	return "unknown"
}

func parseAgeDays(text string) int {
	text = strings.ToLower(strings.TrimSpace(text))

	patterns := []struct {
		re   *regexp.Regexp
		mult int
	}{
		{regexp.MustCompile(`^(\d+)d$`), 1},
		{regexp.MustCompile(`^(\d+)\s*days?`), 1},
		{regexp.MustCompile(`^(\d+)w$`), 7},
		{regexp.MustCompile(`^(\d+)\s*weeks?`), 7},
		{regexp.MustCompile(`^(\d+)m$`), 30},
		{regexp.MustCompile(`^(\d+)\s*months?`), 30},
		{regexp.MustCompile(`^(\d+)h$`), 0},
		{regexp.MustCompile(`^(\d+)\s*hours?`), 0},
	}

	for _, p := range patterns {
		if matches := p.re.FindStringSubmatch(text); matches != nil {
			n, _ := strconv.Atoi(matches[1])
			return n * p.mult
		}
	}

	return -1
}
