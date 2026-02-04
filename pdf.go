package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

func runPdf(cmd *cobra.Command, args []string) {
	dir := args[0]

	// Check if directory exists
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "Error: directory %s does not exist\n", dir)
		os.Exit(1)
	}

	// Check if resume.tex exists
	resumeTexPath := filepath.Join(dir, "resume.tex")
	if _, err := os.Stat(resumeTexPath); os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "Error: %s does not exist\n", resumeTexPath)
		os.Exit(1)
	}

	// Copy .cls files from current directory to target directory
	clsFiles, _ := filepath.Glob("*.cls")
	for _, cls := range clsFiles {
		src, err := os.ReadFile(cls)
		if err != nil {
			continue
		}
		dst := filepath.Join(dir, filepath.Base(cls))
		os.WriteFile(dst, src, 0644)
	}

	// Run tectonic
	fmt.Printf("Compiling %s...\n", resumeTexPath)
	execCmd := exec.Command("tectonic", "resume.tex")
	execCmd.Dir = dir
	execCmd.Stdout = os.Stdout
	execCmd.Stderr = os.Stderr

	if err := execCmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error running tectonic: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("%s PDF compiled: %s/resume.pdf\n", color.GreenString("✓"), dir)
}
