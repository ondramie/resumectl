package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

var (
	scanQuery    string
	scanBoard    string
	scanMaxAge   int
	scanLocation string
)

func init() {
	scanCmd.Flags().StringVarP(&scanQuery, "query", "q", "", "Search keywords (comma-separated)")
	scanCmd.Flags().StringVarP(&scanBoard, "board", "b", "web3", "Job board (web3)")
	scanCmd.Flags().IntVar(&scanMaxAge, "max-age", 90, "Maximum job age in days")
	scanCmd.Flags().StringVarP(&scanLocation, "location", "l", "", "Filter by location (remote, usa, or any text)")
	scanCmd.MarkFlagRequired("query")
}

var scanCmd = &cobra.Command{
	Use:   "scan",
	Short: "Search job boards and score against resume",
	Run:   runScan,
}

type ScanResult struct {
	Title    string
	Company  string
	URL      string
	Age      string
	AgeDays  int
	Location string
	Salary   string
	Score    int
}

func runScan(cmd *cobra.Command, args []string) {
	keywords := strings.Split(scanQuery, ",")
	for i := range keywords {
		keywords[i] = strings.TrimSpace(keywords[i])
	}

	resume, err := os.ReadFile(resumePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading resume: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Searching %s for: %s\n", scanBoard, strings.Join(keywords, ", "))

	var jobs []ScanResult

	switch scanBoard {
	case "web3":
		jobs, err = scanWeb3Career(keywords, scanMaxAge)
	case "remoteok":
		jobs, err = scanRemoteOK(keywords, scanMaxAge)
	case "all":
		jobs, err = scanAllBoards(keywords, scanMaxAge)
	default:
		fmt.Fprintf(os.Stderr, "Unknown board: %s\nAvailable: web3, remoteok, all\n", scanBoard)
		os.Exit(1)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "Error scanning: %v\n", err)
		os.Exit(1)
	}

	if len(jobs) == 0 {
		fmt.Println("No jobs found matching criteria.")
		return
	}

	if scanLocation != "" {
		jobs = filterByLocation(jobs, scanLocation)
		if len(jobs) == 0 {
			fmt.Println("No jobs found matching location criteria.")
			return
		}
	}

	fmt.Printf("Found %d jobs under %d days old, scoring...\n\n", len(jobs), scanMaxAge)

	for i := range jobs {
		score, err := quickScore(string(resume), jobs[i].Title, jobs[i].Company)
		if err != nil {
			fmt.Fprintf(os.Stderr, "  Warning: could not score %s: %v\n", jobs[i].Title, err)
			continue
		}
		jobs[i].Score = score
	}

	sortByScore(jobs)
	printScanResults(jobs)
}

func scanAllBoards(keywords []string, maxAgeDays int) ([]ScanResult, error) {
	var all []ScanResult
	seen := make(map[string]bool)

	web3Jobs, err := scanWeb3Career(keywords, maxAgeDays)
	if err != nil {
		fmt.Printf("  web3.career: %v\n", err)
	} else {
		fmt.Printf("  web3.career: %d jobs\n", len(web3Jobs))
		for _, j := range web3Jobs {
			if !seen[j.URL] {
				seen[j.URL] = true
				all = append(all, j)
			}
		}
	}

	remoteJobs, err := scanRemoteOK(keywords, maxAgeDays)
	if err != nil {
		fmt.Printf("  remoteok: %v\n", err)
	} else {
		fmt.Printf("  remoteok: %d jobs\n", len(remoteJobs))
		for _, j := range remoteJobs {
			if !seen[j.URL] {
				seen[j.URL] = true
				all = append(all, j)
			}
		}
	}

	return all, nil
}

func filterByLocation(jobs []ScanResult, filter string) []ScanResult {
	filters := strings.Split(strings.ToLower(filter), ",")
	for i := range filters {
		filters[i] = strings.TrimSpace(filters[i])
	}

	var filtered []ScanResult

	for _, j := range jobs {
		loc := strings.ToLower(j.Location)

		match := false
		for _, f := range filters {
			switch f {
			case "remote":
				match = strings.Contains(loc, "remote")
			case "usa", "us":
				match = strings.Contains(loc, "united states") ||
					strings.Contains(loc, ", us") ||
					strings.Contains(loc, "usa") ||
					strings.Contains(loc, "new york") ||
					strings.Contains(loc, "san francisco") ||
					strings.Contains(loc, "los angeles") ||
					strings.Contains(loc, "seattle") ||
					strings.Contains(loc, "austin") ||
					strings.Contains(loc, "denver") ||
					strings.Contains(loc, "chicago") ||
					strings.Contains(loc, "miami") ||
					strings.Contains(loc, "boston")
			default:
				match = strings.Contains(loc, f)
			}
			if match {
				break
			}
		}

		if match {
			filtered = append(filtered, j)
		}
	}

	return filtered
}

func sortByScore(jobs []ScanResult) {
	for i := 0; i < len(jobs)-1; i++ {
		for j := i + 1; j < len(jobs); j++ {
			if jobs[j].Score > jobs[i].Score {
				jobs[i], jobs[j] = jobs[j], jobs[i]
			}
		}
	}
}

func printScanResults(jobs []ScanResult) {
	fmt.Printf("%-6s %-5s %-18s %-30s %-20s %s\n", "Score", "Age", "Company", "Title", "Location", "URL")
	fmt.Println(strings.Repeat("─", 120))

	for _, j := range jobs {
		company := j.Company
		if len(company) > 18 {
			company = company[:15] + "..."
		}
		title := j.Title
		if len(title) > 30 {
			title = title[:27] + "..."
		}
		location := j.Location
		if len(location) > 20 {
			location = location[:17] + "..."
		}

		scoreStr := fmt.Sprintf("%d", j.Score)
		if j.Score >= 80 {
			scoreStr = color.GreenString("%d", j.Score)
		} else if j.Score >= 60 {
			scoreStr = color.YellowString("%d", j.Score)
		} else {
			scoreStr = color.RedString("%d", j.Score)
		}

		fmt.Printf("%-6s %-5s %-18s %-30s %-20s %s\n", scoreStr, j.Age, company, title, location, j.URL)
	}
}
