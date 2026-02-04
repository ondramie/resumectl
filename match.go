package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

func runMatch(cmd *cobra.Command, args []string) {
	if err := InitDB(); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not init database: %v\n", err)
	}

	resume, err := os.ReadFile(resumePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading resume: %v\n", err)
		os.Exit(1)
	}

	var job *JobInfo

	if jobFile != "" {
		fmt.Println("Loading job description from file...")
		content, err := os.ReadFile(jobFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error reading job file: %v\n", err)
			os.Exit(1)
		}

		baseName := strings.TrimSuffix(filepath.Base(jobFile), filepath.Ext(jobFile))
		title := strings.ReplaceAll(baseName, "-", " ")

		company := companyName
		if company == "" && len(args) > 0 {
			company = args[0]
		}
		if company == "" {
			company = "unknown"
		}

		job = &JobInfo{
			Company:     company,
			Title:       title,
			ReqID:       baseName,
			Description: string(content),
		}
	} else if len(args) > 0 {
		fmt.Println("Fetching job description...")
		job, err = fetchJobDescription(args[0])
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error fetching job: %v\n", err)
			os.Exit(1)
		}
	} else {
		fmt.Fprintf(os.Stderr, "Error: must provide either a job URL or --file\n")
		os.Exit(1)
	}

	fmt.Printf("Job: %s at %s\n", job.Title, job.Company)

	currentResume := string(resume)
	var bestResult *MatchResult
	var bestScore int

	for iteration := 1; iteration <= maxIterations; iteration++ {
		fmt.Printf("\n%s Iteration %d/%d\n", color.CyanString("→"), iteration, maxIterations)
		fmt.Println("Analyzing and tailoring...")

		result, err := analyzeAndTailor(currentResume, job.Description)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error analyzing: %v\n", err)
			os.Exit(1)
		}

		printIterationResult(iteration, result)

		if result.Score > bestScore {
			bestScore = result.Score
			bestResult = result
		}

		if result.Score >= targetScore {
			fmt.Printf("\n%s Target score %d reached!\n", color.GreenString("✓"), targetScore)
			break
		}

		if iteration > 1 && result.Score <= bestScore-5 {
			fmt.Printf("\n%s Score not improving, stopping.\n", color.YellowString("⚠"))
			break
		}

		currentResume = result.TailoredLatex
	}

	fmt.Println()
	fmt.Println(color.New(color.Bold, color.Underline).Sprint("Final Results"))
	fmt.Println(strings.Repeat("═", 50))
	if bestResult == nil {
		fmt.Fprintf(os.Stderr, "Error: no valid results produced. The job description may be empty or the API returned invalid responses.\n")
		os.Exit(1)
	}
	printResult(bestResult)

	if db != nil {
		var jobURL string
		if len(args) > 0 {
			jobURL = args[0]
		} else if jobFile != "" {
			jobURL = "file://" + jobFile
		}
		if jobURL != "" {
			if err := SaveJob(jobURL, job.Company, job.Title, bestResult.Score); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: could not save to database: %v\n", err)
			} else {
				fmt.Printf("%s Saved to database\n", color.GreenString("✓"))
			}
		}
	}

	outputDir := generateOutputDir(job)
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Error creating output dir: %v\n", err)
		os.Exit(1)
	}

	resumeOut := filepath.Join(outputDir, "resume.tex")
	if err := os.WriteFile(resumeOut, []byte(bestResult.TailoredLatex), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "Error saving tailored resume: %v\n", err)
		os.Exit(1)
	}

	jobOut := filepath.Join(outputDir, "job.txt")
	os.WriteFile(jobOut, []byte(job.Description), 0644)

	reportOut := filepath.Join(outputDir, "report.txt")
	report := fmt.Sprintf("Score: %d/100\n\nStrong Matches:\n", bestResult.Score)
	for _, m := range bestResult.StrongMatches {
		report += fmt.Sprintf("  - %s\n", m)
	}
	report += "\nGaps:\n"
	for _, g := range bestResult.Gaps {
		report += fmt.Sprintf("  - %s\n", g)
	}
	os.WriteFile(reportOut, []byte(report), 0644)

	if withCoverLetter {
		fmt.Println()
		fmt.Println(color.New(color.Bold).Sprint("Cover Letter"))
		fmt.Println(strings.Repeat("─", 50))
		if len(bestResult.Gaps) > 0 {
			fmt.Println("The following gaps were identified:")
			for _, g := range bestResult.Gaps {
				fmt.Printf("  %s %s\n", color.YellowString("•"), g)
			}
		}
		fmt.Println()
		fmt.Println("Add any context to strengthen your letter")
		fmt.Println("(e.g. side projects, motivation, relocation plans)")
		fmt.Printf("Press Enter to skip: ")

		reader := bufio.NewReader(os.Stdin)
		line, _ := reader.ReadString('\n')
		userContext := strings.TrimSpace(line)

		fmt.Println("Generating cover letter...")
		coverLetter, err := generateCoverLetter(bestResult.TailoredLatex, job.Description, bestResult, userContext)
		if err != nil {
			fmt.Printf("%s Cover letter generation failed: %v\n", color.YellowString("⚠"), err)
		} else {
			coverOut := filepath.Join(outputDir, "cover-letter.txt")
			os.WriteFile(coverOut, []byte(coverLetter), 0644)
			fmt.Printf("%s Cover letter saved\n", color.GreenString("✓"))
		}
	}

	fmt.Printf("\n%s Results saved to: %s/\n", color.GreenString("✓"), outputDir)
	fmt.Printf("  resume.tex  - tailored resume\n")
	fmt.Printf("  job.txt     - job description\n")
	fmt.Printf("  report.txt  - match report\n")
	if withCoverLetter {
		fmt.Printf("  cover-letter.txt - cover letter\n")
	}
	fmt.Println()
	fmt.Println("To compile PDF (after reviewing/editing resume.tex):")
	fmt.Printf("  resumectl pdf %s\n", outputDir)
}

func generateOutputDir(job *JobInfo) string {
	sanitize := func(s string) string {
		s = strings.ToLower(s)
		s = regexp.MustCompile(`[^a-z0-9]+`).ReplaceAllString(s, "-")
		s = strings.Trim(s, "-")
		return s
	}

	company := sanitize(job.Company)
	reqID := sanitize(job.ReqID)
	if reqID == "" {
		reqID = sanitize(job.Title)
	}

	return filepath.Join("results", company, reqID)
}

func analyzeAndTailor(resume, jobDescription string) (*MatchResult, error) {
	apiKey := os.Getenv("ANTHROPIC_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("ANTHROPIC_API_KEY not set")
	}

	prompt := fmt.Sprintf(`Analyze this resume against the job description and create a tailored version.

RESUME (LaTeX):
%s

JOB DESCRIPTION:
%s

Instructions:
1. Score the match 0-100 based on actual skill/experience alignment
2. Identify strong matches (skills/experience that align well)
3. Identify gaps (required skills/experience that are truly missing)
   - Use common sense inference: if the resume lists Elixir + Postgres, the candidate obviously has ORM experience (Ecto). If they list Python + Postgres, they have used SQLAlchemy or similar. Do NOT flag commonly implied skills as gaps.
   - Only flag a gap if the skill/experience is genuinely absent and cannot be reasonably inferred from the listed technologies and experience.
4. Create a tailored LaTeX resume that:
   - Reorders bullet points to emphasize relevant experience first
   - Adjusts the Technical section to highlight relevant skills
   - Does NOT add false information
   - Keeps exact same LaTeX structure

Respond with ONLY valid JSON (no markdown):
{
  "score": <0-100>,
  "strong_matches": ["match1", "match2", ...],
  "gaps": ["gap1", "gap2", ...],
  "tailored_latex": "<complete LaTeX document>"
}`, resume, jobDescription)

	reqBody := map[string]interface{}{
		"model":      modelName,
		"max_tokens": 8000,
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

	text := apiResp.Content[0].Text
	text = strings.TrimSpace(text)
	if strings.HasPrefix(text, "```") {
		if idx := strings.Index(text, "\n"); idx != -1 {
			text = text[idx+1:]
		}
		if idx := strings.LastIndex(text, "```"); idx != -1 {
			text = text[:idx]
		}
		text = strings.TrimSpace(text)
	}

	var result MatchResult
	if err := json.Unmarshal([]byte(text), &result); err != nil {
		return nil, fmt.Errorf("parse error: %v\nRaw: %s", err, text[:500])
	}

	result.TailoredLatex = postProcessLatex(result.TailoredLatex)

	return &result, nil
}

func generateCoverLetter(resume, jobDescription string, matchResult *MatchResult, userContext string) (string, error) {
	apiKey := os.Getenv("ANTHROPIC_API_KEY")
	if apiKey == "" {
		return "", fmt.Errorf("ANTHROPIC_API_KEY not set")
	}

	var matchInfo strings.Builder
	matchInfo.WriteString("MATCH ANALYSIS:\n")
	matchInfo.WriteString(fmt.Sprintf("Score: %d/100\n\n", matchResult.Score))
	if len(matchResult.StrongMatches) > 0 {
		matchInfo.WriteString("Strong Matches:\n")
		for _, m := range matchResult.StrongMatches {
			matchInfo.WriteString(fmt.Sprintf("- %s\n", m))
		}
	}
	if len(matchResult.Gaps) > 0 {
		matchInfo.WriteString("\nGaps:\n")
		for _, g := range matchResult.Gaps {
			matchInfo.WriteString(fmt.Sprintf("- %s\n", g))
		}
	}

	contextSection := ""
	if userContext != "" {
		contextSection = fmt.Sprintf("\nADDITIONAL CONTEXT FROM CANDIDATE:\n%s\n", userContext)
	}

	prompt := fmt.Sprintf(`Write a professional cover letter based on this resume, job description, and match analysis.

RESUME:
%s

JOB DESCRIPTION:
%s

%s
%s
Instructions:
- Read the job description carefully — identify exactly what they are looking for
- For each key requirement, show how the candidate's experience directly delivers it
- Use specific achievements and metrics from the resume as evidence
- Structure the letter around what the job wants, not around the resume chronology
- If additional context was provided by the candidate, weave it in naturally
- Do NOT mention weaknesses, gaps, or missing skills — focus entirely on what the candidate brings
- Use a confident but natural tone — not generic or overly formal
- Write 3-4 concise paragraphs
- Do NOT fabricate any experience or skills not in the resume or additional context
- Return ONLY the cover letter text, no JSON or markdown wrapping`, resume, jobDescription, matchInfo.String(), contextSection)

	reqBody := map[string]interface{}{
		"model":      modelName,
		"max_tokens": 2000,
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
		return "", err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("API error %d: %s", resp.StatusCode, string(body))
	}

	var apiResp struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return "", err
	}

	if len(apiResp.Content) == 0 {
		return "", fmt.Errorf("empty response")
	}

	return apiResp.Content[0].Text, nil
}

func printIterationResult(iteration int, r *MatchResult) {
	scoreColor := color.RedString
	if r.Score >= 80 {
		scoreColor = color.GreenString
	} else if r.Score >= 60 {
		scoreColor = color.YellowString
	}
	fmt.Printf("  Score: %s\n", scoreColor("%d/100", r.Score))
}

func printResult(r *MatchResult) {
	fmt.Println()
	fmt.Println(color.New(color.Bold, color.Underline).Sprint("Match Analysis"))
	fmt.Println(strings.Repeat("─", 50))

	scoreColor := color.RedString
	if r.Score >= 80 {
		scoreColor = color.GreenString
	} else if r.Score >= 60 {
		scoreColor = color.YellowString
	}
	fmt.Printf("\nScore: %s\n", scoreColor("%d/100", r.Score))

	if len(r.StrongMatches) > 0 {
		fmt.Printf("\n%s\n", color.GreenString("Strong Matches:"))
		for _, m := range r.StrongMatches {
			fmt.Printf("  %s %s\n", color.GreenString("✓"), m)
		}
	}

	if len(r.Gaps) > 0 {
		fmt.Printf("\n%s\n", color.RedString("Gaps:"))
		for _, g := range r.Gaps {
			fmt.Printf("  %s %s\n", color.RedString("✗"), g)
		}
	}
}
