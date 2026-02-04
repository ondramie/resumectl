package main

import (
	"regexp"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

func stripHTML(html string) string {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		// Fallback: crude regex strip
		re := regexp.MustCompile(`<[^>]*>`)
		return re.ReplaceAllString(html, "")
	}
	text := doc.Text()
	// Clean up whitespace
	text = regexp.MustCompile(`\s+`).ReplaceAllString(text, " ")
	return strings.TrimSpace(text)
}

func extractReqIDFromURL(rawURL string) string {
	patterns := []*regexp.Regexp{
		regexp.MustCompile(`/jobs/([a-f0-9-]+)`),         // rippling, lever
		regexp.MustCompile(`/jobs/(\d+)`),                // greenhouse
		regexp.MustCompile(`[?&]pid=(\d+)`),              // netflix
		regexp.MustCompile(`/job/([a-f0-9-]+)`),          // generic
		regexp.MustCompile(`[?&](?:req|id|jid)=([^&]+)`), // query params
	}

	for _, p := range patterns {
		if matches := p.FindStringSubmatch(rawURL); matches != nil {
			return matches[1]
		}
	}

	return ""
}

func extractCompanyFromURL(rawURL string) string {
	// Known ATS patterns: extract company from path
	patterns := []struct {
		re    *regexp.Regexp
		group int
	}{
		{regexp.MustCompile(`ats\.rippling\.com/([^/]+)/jobs/`), 1},
		{regexp.MustCompile(`boards\.greenhouse\.io/([^/]+)/`), 1},
		{regexp.MustCompile(`jobs\.lever\.co/([^/]+)/`), 1},
		{regexp.MustCompile(`([^.]+)\.workday\.com`), 1},
		{regexp.MustCompile(`careers\.([^.]+)\.com`), 1},
		{regexp.MustCompile(`([^.]+)\.bamboohr\.com`), 1},
		{regexp.MustCompile(`jobs\.ashbyhq\.com/([^/]+)`), 1},
	}

	for _, p := range patterns {
		if matches := p.re.FindStringSubmatch(rawURL); matches != nil {
			return matches[p.group]
		}
	}

	// Fallback: domain name
	re := regexp.MustCompile(`https?://(?:www\.)?([^/]+)`)
	if matches := re.FindStringSubmatch(rawURL); matches != nil {
		parts := strings.Split(matches[1], ".")
		if len(parts) > 0 {
			return parts[0]
		}
	}
	return "unknown"
}

func postProcessLatex(latex string) string {
	// Fix tabular column spec to prevent overflow — replace the whole line
	tabularRe := regexp.MustCompile(`(?m)^.*\\begin\{tabular\}.*$`)
	latex = tabularRe.ReplaceAllString(latex,
		`\begin{tabular}{ @{} >{\bfseries}l @{\hspace{3ex}} >{\raggedright\arraybackslash}p{0.68\textwidth} }`)

	// Ensure hyphenation settings are present
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
