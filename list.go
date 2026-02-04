package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

func runList(cmd *cobra.Command, args []string) {
	if err := InitDB(); err != nil {
		fmt.Fprintf(os.Stderr, "Error initializing database: %v\n", err)
		os.Exit(1)
	}

	status, _ := cmd.Flags().GetString("status")
	minScore, _ := cmd.Flags().GetInt("min-score")

	jobs, err := ListJobs(status, minScore)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error listing jobs: %v\n", err)
		os.Exit(1)
	}

	if len(jobs) == 0 {
		fmt.Println("No jobs found.")
		return
	}

	// Print table header
	fmt.Printf("%-4s %-20s %-30s %-6s %-10s\n", "ID", "Company", "Title", "Score", "Status")
	fmt.Println(strings.Repeat("-", 75))

	for _, j := range jobs {
		title := j.Title
		if len(title) > 30 {
			title = title[:27] + "..."
		}
		company := j.Company
		if len(company) > 20 {
			company = company[:17] + "..."
		}

		// Color score
		scoreStr := fmt.Sprintf("%d", j.Score)
		if j.Score >= 80 {
			scoreStr = color.GreenString("%d", j.Score)
		} else if j.Score >= 60 {
			scoreStr = color.YellowString("%d", j.Score)
		} else {
			scoreStr = color.RedString("%d", j.Score)
		}

		fmt.Printf("%-4d %-20s %-30s %-6s %-10s\n", j.ID, company, title, scoreStr, j.Status)
	}
}
