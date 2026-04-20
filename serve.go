package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

var serveToken string
var servePort int

func runServe(cmd *cobra.Command, args []string) {
	if err := InitDB(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: could not init database: %v\n", err)
		os.Exit(1)
	}

	if serveToken == "" {
		serveToken = os.Getenv("RESUMECTL_API_TOKEN")
	}
	if serveToken == "" {
		fmt.Fprintf(os.Stderr, "Error: --token flag or RESUMECTL_API_TOKEN env var required\n")
		os.Exit(1)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/match", authMiddleware(handleMatch))
	mux.HandleFunc("/pipeline", authMiddleware(handlePipeline))
	mux.HandleFunc("/pdf/", authMiddleware(handlePDFDownload))
	mux.HandleFunc("/health", handleHealth)

	addr := fmt.Sprintf(":%d", servePort)
	fmt.Printf("%s Server listening on %s\n", color.GreenString("✓"), addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func authMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if !strings.HasPrefix(auth, "Bearer ") || strings.TrimPrefix(auth, "Bearer ") != serveToken {
			http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
			return
		}
		next(w, r)
	}
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func handleMatch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		URL     string `json:"url"`
		File    string `json:"file"`
		Company string `json:"company"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
		return
	}

	if req.URL == "" && req.File == "" {
		http.Error(w, `{"error":"url or file required"}`, http.StatusBadRequest)
		return
	}

	var job *JobInfo
	var err error

	if req.URL != "" {
		job, err = fetchJobDescription(req.URL)
		if err != nil {
			http.Error(w, fmt.Sprintf(`{"error":"fetch failed: %s"}`, err), http.StatusBadGateway)
			return
		}
	}

	if req.Company != "" && job != nil {
		job.Company = req.Company
	}

	descLen := len(strings.TrimSpace(job.Description))
	if descLen < 200 {
		http.Error(w, fmt.Sprintf(`{"error":"job description too short (%d chars)"}`, descLen), http.StatusUnprocessableEntity)
		return
	}

	templates, _ := findTemplates()
	if len(templates) == 0 {
		http.Error(w, `{"error":"no resume templates found"}`, http.StatusInternalServerError)
		return
	}

	bestTemplate := templates[0]
	if len(templates) > 1 {
		bestTemplate = selectBestTemplateFromList(templates, job)
	}

	resume, err := os.ReadFile(bestTemplate)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"could not read template: %s"}`, err), http.StatusInternalServerError)
		return
	}

	result, err := analyzeAndTailor(string(resume), job.Description)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"analysis failed: %s"}`, err), http.StatusInternalServerError)
		return
	}

	if db != nil {
		SaveJob(req.URL, job.Company, job.Title, result.Score)
	}

	outputDir := generateOutputDir(job)
	os.MkdirAll(outputDir, 0755)
	os.WriteFile(outputDir+"/resume.tex", []byte(result.TailoredLatex), 0644)
	os.WriteFile(outputDir+"/job.txt", []byte(job.Description), 0644)

	report := fmt.Sprintf("Score: %d/100\n\nStrong Matches:\n", result.Score)
	for _, m := range result.StrongMatches {
		report += fmt.Sprintf("  - %s\n", m)
	}
	report += "\nGaps:\n"
	for _, g := range result.Gaps {
		report += fmt.Sprintf("  - %s\n", g)
	}
	os.WriteFile(outputDir+"/report.txt", []byte(report), 0644)

	clsFiles, _ := filepath.Glob("*.cls")
	if len(clsFiles) == 0 {
		home, _ := os.UserHomeDir()
		clsFiles, _ = filepath.Glob(filepath.Join(home, ".resumectl", "*.cls"))
	}
	for _, cls := range clsFiles {
		src, _ := os.ReadFile(cls)
		os.WriteFile(filepath.Join(outputDir, filepath.Base(cls)), src, 0644)
	}

	pdfURL := ""
	cmd := exec.Command("tectonic", "resume.tex")
	cmd.Dir = outputDir
	if err := cmd.Run(); err == nil {
		pdfURL = fmt.Sprintf("/pdf/%s/resume.pdf", outputDir)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"score":          result.Score,
		"company":        job.Company,
		"title":          job.Title,
		"strong_matches": result.StrongMatches,
		"gaps":           result.Gaps,
		"template_used":  bestTemplate,
		"output_dir":     outputDir,
		"pdf_url":        pdfURL,
	})
}

func handlePipeline(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	type funnelRow struct {
		Status   string `json:"status"`
		Count    int    `json:"count"`
		AvgScore int    `json:"avg_score"`
	}

	type activeRow struct {
		Company     string `json:"company"`
		Title       string `json:"title"`
		Score       int    `json:"score"`
		AppliedAt   string `json:"applied_at"`
		DaysWaiting int    `json:"days_waiting"`
	}

	var funnel []funnelRow
	rows, err := db.Query(`SELECT status, COUNT(*), ROUND(AVG(score),0) FROM jobs GROUP BY status`)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var r funnelRow
			rows.Scan(&r.Status, &r.Count, &r.AvgScore)
			funnel = append(funnel, r)
		}
	}

	var active []activeRow
	rows2, err := db.Query(`
		SELECT company, title, score, applied_at,
			EXTRACT(DAY FROM NOW() - applied_at)::INTEGER
		FROM jobs
		WHERE status='applied' AND applied_at IS NOT NULL
		ORDER BY applied_at ASC`)
	if err == nil {
		defer rows2.Close()
		for rows2.Next() {
			var r activeRow
			rows2.Scan(&r.Company, &r.Title, &r.Score, &r.AppliedAt, &r.DaysWaiting)
			active = append(active, r)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"funnel": funnel,
		"active": active,
	})
}

func handlePDFDownload(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/pdf/")
	if !strings.HasSuffix(path, ".pdf") || strings.Contains(path, "..") {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	data, err := os.ReadFile(path)
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"resume.pdf\""))
	w.Write(data)
}

func findTemplates() ([]string, error) {
	templates, err := filepath.Glob("resume.template*.tex")
	if err != nil || len(templates) == 0 {
		home, _ := os.UserHomeDir()
		templates, err = filepath.Glob(home + "/.resumectl/resume.template*.tex")
	}
	return templates, err
}

func selectBestTemplateFromList(templates []string, job *JobInfo) string {
	var results []scoredTemplate
	for _, t := range templates {
		resume, err := os.ReadFile(t)
		if err != nil {
			continue
		}
		score, err := quickScore(string(resume), job.Description, job.Company)
		if err != nil {
			continue
		}
		results = append(results, scoredTemplate{t, score})
	}

	if len(results) == 0 {
		return templates[0]
	}

	best := results[0]
	tied := false
	for _, r := range results[1:] {
		if r.score > best.score {
			best = r
			tied = false
		} else if r.score == best.score {
			tied = true
		}
	}

	if tied {
		winner, err := compareTemplates(results, job.Description)
		if err == nil {
			best = winner
		}
	}

	return best.path
}
