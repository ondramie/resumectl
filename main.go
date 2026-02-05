package main

import (
	"os"

	"github.com/joho/godotenv"
	"github.com/spf13/cobra"
)

func init() {
	godotenv.Load()
}

var (
	resumePath      string
	jobFile         string
	companyName     string
	maxIterations   int
	targetScore     int
	modelName       string
	withCoverLetter bool
)

type JobInfo struct {
	Company     string
	Title       string
	ReqID       string
	Description string
}

type MatchResult struct {
	Score         int      `json:"score"`
	StrongMatches []string `json:"strong_matches"`
	Gaps          []string `json:"gaps"`
	TailoredLatex string   `json:"tailored_latex"`
}

func main() {
	var rootCmd = &cobra.Command{
		Use:   "resumectl",
		Short: "Self-custodial job hunting. You own your data.",
	}

	var matchCmd = &cobra.Command{
		Use:   "match [job-url]",
		Short: "Match resume against a job posting URL or file",
		Args:  cobra.MaximumNArgs(1),
		Run:   runMatch,
	}
	matchCmd.Flags().StringVarP(&resumePath, "resume", "r", "resume.template.tex", "Path to resume LaTeX file")
	matchCmd.Flags().StringVarP(&jobFile, "file", "f", "", "Path to job description text file (instead of URL)")
	matchCmd.Flags().StringVarP(&companyName, "company", "c", "", "Override company name")
	matchCmd.Flags().IntVarP(&maxIterations, "iterations", "i", 3, "Max iterations to improve score")
	matchCmd.Flags().IntVarP(&targetScore, "target", "t", 85, "Target score to stop iterating")
	matchCmd.Flags().StringVarP(&modelName, "model", "m", "claude-sonnet-4-20250514", "Anthropic model to use")
	matchCmd.Flags().BoolVar(&withCoverLetter, "cover-letter", false, "Also generate a cover letter")
	rootCmd.AddCommand(matchCmd)

	var listCmd = &cobra.Command{
		Use:   "list",
		Short: "List saved jobs from database",
		Run:   runList,
	}
	listCmd.Flags().String("status", "", "Filter by status (new, applied, screening, interview, offer, rejected)")
	listCmd.Flags().Int("min-score", 0, "Filter by minimum score")
	rootCmd.AddCommand(listCmd)

	var pdfCmd = &cobra.Command{
		Use:   "pdf [results-dir]",
		Short: "Compile resume.tex to PDF using tectonic",
		Args:  cobra.ExactArgs(1),
		Run:   runPdf,
	}
	rootCmd.AddCommand(pdfCmd)

	scanCmd.Flags().StringVarP(&resumePath, "resume", "r", "resume.template.tex", "Path to resume LaTeX file")
	rootCmd.AddCommand(scanCmd)

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
