package main

import (
	"net/url"
	"regexp"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

func stripHTML(html string) string {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		re := regexp.MustCompile(`<[^>]*>`)
		return re.ReplaceAllString(html, "")
	}
	text := doc.Text()
	text = regexp.MustCompile(`\s+`).ReplaceAllString(text, " ")
	return strings.TrimSpace(text)
}

func extractReqIDFromURL(rawURL string) string {
	if u, err := url.Parse(rawURL); err == nil {
		for _, key := range []string{"pid", "req", "id", "jid", "gh_jid"} {
			if v := u.Query().Get(key); v != "" {
				return v
			}
		}
	}

	patterns := []*regexp.Regexp{
		regexp.MustCompile(`/jobs/([a-f0-9]{8}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{12})`),
		regexp.MustCompile(`/jobs/(\d+)`),
		regexp.MustCompile(`/job/([a-f0-9]{8}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{12})`),
		regexp.MustCompile(`/job/(\d+)`),
		regexp.MustCompile(`/positions/(\d+)`),
	}

	for _, p := range patterns {
		if matches := p.FindStringSubmatch(rawURL); matches != nil {
			return matches[1]
		}
	}

	return ""
}

func extractCompanyFromURL(rawURL string) string {
	patterns := []struct {
		re    *regexp.Regexp
		group int
	}{
		{regexp.MustCompile(`apply\.workable\.com/([^/]+)/`), 1},
		{regexp.MustCompile(`/companies/([^/]+)/jobs/`), 1},
		{regexp.MustCompile(`ats\.rippling\.com/([^/]+)/jobs/`), 1},
		{regexp.MustCompile(`boards\.greenhouse\.io/([^/]+)/`), 1},
		{regexp.MustCompile(`jobs\.lever\.co/([^/]+)/`), 1},
		{regexp.MustCompile(`://([^.]+)\.workday\.com`), 1},
		{regexp.MustCompile(`careers\.([^.]+)\.com`), 1},
		{regexp.MustCompile(`://([^.]+)\.bamboohr\.com`), 1},
		{regexp.MustCompile(`jobs\.ashbyhq\.com/([^/]+)`), 1},
	}

	for _, p := range patterns {
		if matches := p.re.FindStringSubmatch(rawURL); matches != nil {
			return matches[p.group]
		}
	}

	re := regexp.MustCompile(`https?://(?:www\.)?([^/]+)`)
	if matches := re.FindStringSubmatch(rawURL); matches != nil {
		parts := strings.Split(matches[1], ".")
		if len(parts) > 0 {
			return parts[0]
		}
	}
	return "unknown"
}

func filterFalseGaps(gaps []string, resume string) []string {
	resumeLower := strings.ToLower(resume)
	stopWords := map[string]bool{
		"experience": true, "the": true, "and": true, "for": true,
		"with": true, "from": true, "that": true, "this": true,
		"not": true, "but": true, "are": true, "was": true,
		"has": true, "have": true, "had": true, "been": true,
		"web": true, "framework": true, "limited": true,
		"explicit": true, "mentioned": true, "listed": true,
		"skills": true, "knowledge": true, "proficiency": true,
		"expertise": true, "background": true, "strong": true,
		"direct": true, "specific": true, "dedicated": true,
		"streaming": true, "processing": true, "development": true,
		"engineering": true, "production": true, "building": true,
	}
	techPattern := regexp.MustCompile(`[A-Z][a-zA-Z]*(?:\.[a-zA-Z]+)*|[a-z]+(?:\.[a-zA-Z]+)+`)
	var filtered []string
	for _, gap := range gaps {
		techs := techPattern.FindAllString(gap, -1)
		found := false
		for _, tech := range techs {
			techLower := strings.ToLower(tech)
			if len(techLower) < 2 || stopWords[techLower] {
				continue
			}
			if strings.Contains(resumeLower, techLower) {
				found = true
				break
			}
		}
		if !found {
			filtered = append(filtered, gap)
		}
	}
	return filtered
}

func postProcessLatex(latex string) string {
	tabularRe := regexp.MustCompile(`(?m)^.*\\begin\{tabular\}.*$`)
	latex = tabularRe.ReplaceAllString(latex,
		`\begin{tabular}{ @{} >{\bfseries}l @{\hspace{3ex}} >{\raggedright\arraybackslash}p{0.68\textwidth} }`)

	if !strings.Contains(latex, `\hyphenpenalty`) {
		latex = strings.Replace(latex,
			`\usepackage[left=0.75in,top=0.6in,right=0.75in,bottom=0.6in]{geometry}`,
			`\usepackage[left=0.75in,top=0.6in,right=0.75in,bottom=0.6in]{geometry}`+"\n"+
				`\tolerance=1`+"\n"+
				`\emergencystretch=\maxdimen`+"\n"+
				`\hyphenpenalty=10000`+"\n"+
				`\hbadness=10000`,
			1)
	}

	return latex
}
