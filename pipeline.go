package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

func runPipeline(cmd *cobra.Command, args []string) {
	if err := InitDB(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: could not init database: %v\n", err)
		os.Exit(1)
	}

	groupBy, _ := cmd.Flags().GetString("by")

	printFunnel()
	fmt.Println()
	printTimeline()
	fmt.Println()

	if groupBy == "industry" {
		printByIndustry()
	} else if groupBy == "role" {
		printByRole()
	} else {
		printByIndustry()
		fmt.Println()
		printByRole()
	}

	fmt.Println()
	printActive()
}

func printFunnel() {
	fmt.Println(color.New(color.Bold, color.Underline).Sprint("Pipeline Funnel"))
	fmt.Println(strings.Repeat("─", 50))

	rows, err := db.Query(`
		SELECT status, COUNT(*), ROUND(AVG(score),0)
		FROM jobs
		GROUP BY status
		ORDER BY CASE status
			WHEN 'new' THEN 1
			WHEN 'applied' THEN 2
			WHEN 'screening' THEN 3
			WHEN 'interview' THEN 4
			WHEN 'offer' THEN 5
			WHEN 'rejected' THEN 6
		END`)
	if err != nil {
		return
	}
	defer rows.Close()

	var total int
	type row struct {
		status   string
		count    int
		avgScore int
	}
	var data []row

	for rows.Next() {
		var r row
		rows.Scan(&r.status, &r.count, &r.avgScore)
		total += r.count
		data = append(data, r)
	}

	fmt.Printf("  Total: %d jobs\n\n", total)
	maxBar := 30
	for _, r := range data {
		barLen := r.count
		if total > maxBar {
			barLen = r.count * maxBar / total
			if barLen == 0 && r.count > 0 {
				barLen = 1
			}
		}
		barStr := strings.Repeat("█", barLen)
		colorFn := color.New(color.FgWhite).SprintFunc()
		switch r.status {
		case "new":
			colorFn = color.New(color.FgBlue).SprintFunc()
		case "applied":
			colorFn = color.New(color.FgCyan).SprintFunc()
		case "screening":
			colorFn = color.New(color.FgYellow).SprintFunc()
		case "interview":
			colorFn = color.New(color.FgMagenta).SprintFunc()
		case "offer":
			colorFn = color.New(color.FgGreen).SprintFunc()
		case "rejected":
			colorFn = color.New(color.FgRed).SprintFunc()
		}
		fmt.Printf("  %-12s %s %s (avg score: %d)\n", colorFn(r.status), colorFn(barStr), color.CyanString("%d", r.count), r.avgScore)
	}
}

func printTimeline() {
	fmt.Println(color.New(color.Bold, color.Underline).Sprint("Rejection Turnaround"))
	fmt.Println(strings.Repeat("─", 50))

	rows, err := db.Query(`
		SELECT company, title, score, applied_at, rejected_at,
			EXTRACT(DAY FROM rejected_at - applied_at)::INTEGER as days
		FROM jobs
		WHERE status='rejected' AND applied_at IS NOT NULL AND rejected_at IS NOT NULL
		ORDER BY rejected_at DESC`)
	if err != nil {
		return
	}
	defer rows.Close()

	var totalDays, count int
	for rows.Next() {
		var company, title, appliedAt, rejectedAt string
		var score, days int
		rows.Scan(&company, &title, &score, &appliedAt, &rejectedAt, &days)
		totalDays += days
		count++
		fmt.Printf("  %s at %s — %s (%d/100, %d days)\n",
			truncate(title, 40), company, color.RedString("%d days", days), score, days)
	}

	if count > 0 {
		fmt.Printf("\n  Average turnaround: %s\n", color.YellowString("%d days", totalDays/count))
	}
}

func printByIndustry() {
	fmt.Println(color.New(color.Bold, color.Underline).Sprint("By Industry"))
	fmt.Println(strings.Repeat("─", 50))

	rows, err := db.Query(`
		SELECT COALESCE(industry,'unknown'), COUNT(*), ROUND(AVG(score),0),
			SUM(CASE WHEN status='rejected' THEN 1 ELSE 0 END),
			SUM(CASE WHEN status='applied' THEN 1 ELSE 0 END),
			SUM(CASE WHEN status IN ('screening','interview') THEN 1 ELSE 0 END)
		FROM jobs
		WHERE industry IS NOT NULL
		GROUP BY industry
		ORDER BY COUNT(*) DESC`)
	if err != nil {
		return
	}
	defer rows.Close()

	fmt.Printf("  %-14s %5s %5s %5s %5s %5s\n", "Industry", "Total", "Avg", "Rej", "Pend", "Intv")
	fmt.Printf("  %-14s %5s %5s %5s %5s %5s\n", "──────────", "─────", "─────", "─────", "─────", "─────")
	for rows.Next() {
		var industry string
		var total, avg, rejected, pending, interviews int
		rows.Scan(&industry, &total, &avg, &rejected, &pending, &interviews)
		rejStr := fmt.Sprintf("%5d", rejected)
		if rejected > 0 {
			rejStr = fmt.Sprintf("%s%s", strings.Repeat(" ", 5-len(fmt.Sprintf("%d", rejected))), color.RedString("%d", rejected))
		}
		fmt.Printf("  %-14s %5d %5d %s %5d %5d\n",
			industry, total, avg, rejStr, pending, interviews)
	}
}

func printByRole() {
	fmt.Println(color.New(color.Bold, color.Underline).Sprint("By Role Type"))
	fmt.Println(strings.Repeat("─", 50))

	rows, err := db.Query(`
		SELECT COALESCE(role_type,'unknown'), COUNT(*), ROUND(AVG(score),0),
			SUM(CASE WHEN status='rejected' THEN 1 ELSE 0 END),
			SUM(CASE WHEN status='applied' THEN 1 ELSE 0 END),
			SUM(CASE WHEN status IN ('screening','interview') THEN 1 ELSE 0 END)
		FROM jobs
		WHERE role_type IS NOT NULL
		GROUP BY role_type
		ORDER BY COUNT(*) DESC`)
	if err != nil {
		return
	}
	defer rows.Close()

	fmt.Printf("  %-16s %5s %5s %5s %5s %5s\n", "Role", "Total", "Avg", "Rej", "Pend", "Intv")
	fmt.Printf("  %-16s %5s %5s %5s %5s %5s\n", "────────────", "─────", "─────", "─────", "─────", "─────")
	for rows.Next() {
		var roleType string
		var total, avg, rejected, pending, interviews int
		rows.Scan(&roleType, &total, &avg, &rejected, &pending, &interviews)
		rejStr := fmt.Sprintf("%5d", rejected)
		if rejected > 0 {
			rejStr = fmt.Sprintf("%s%s", strings.Repeat(" ", 5-len(fmt.Sprintf("%d", rejected))), color.RedString("%d", rejected))
		}
		fmt.Printf("  %-16s %5d %5d %s %5d %5d\n",
			roleType, total, avg, rejStr, pending, interviews)
	}
}

func printActive() {
	fmt.Println(color.New(color.Bold, color.Underline).Sprint("Active Applications (oldest first)"))
	fmt.Println(strings.Repeat("─", 50))

	rows, err := db.Query(`
		SELECT company, title, score, applied_at,
			EXTRACT(DAY FROM NOW() - applied_at)::INTEGER as days_waiting
		FROM jobs
		WHERE status='applied' AND applied_at IS NOT NULL
		ORDER BY applied_at ASC`)
	if err != nil {
		return
	}
	defer rows.Close()

	for rows.Next() {
		var company, title, appliedAt string
		var score, days int
		rows.Scan(&company, &title, &score, &appliedAt, &days)
		dayColor := color.GreenString
		if days > 14 {
			dayColor = color.YellowString
		}
		if days > 21 {
			dayColor = color.RedString
		}
		fmt.Printf("  %s at %s — %d/100, waiting %s\n",
			truncate(title, 40), company, score, dayColor("%d days", days))
	}
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}
