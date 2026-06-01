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
	input := args[0]

	if _, err := os.Stat(input); os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "Error: %s does not exist\n", input)
		os.Exit(1)
	}

	var dir string
	var texFile string

	if filepath.Ext(input) == ".tex" {
		dir = filepath.Dir(input)
		texFile = filepath.Base(input)
	} else {
		dir = input
		texFile = "resume.tex"
	}

	resumeTexPath := filepath.Join(dir, texFile)
	if _, err := os.Stat(resumeTexPath); os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "Error: %s does not exist\n", resumeTexPath)
		os.Exit(1)
	}

	clsFiles, _ := filepath.Glob("*.cls")
	for _, cls := range clsFiles {
		src, err := os.ReadFile(cls)
		if err != nil {
			continue
		}
		dst := filepath.Join(dir, filepath.Base(cls))
		os.WriteFile(dst, src, 0644)
	}

	fmt.Printf("Compiling %s...\n", resumeTexPath)
	execCmd := exec.Command("tectonic", texFile)
	execCmd.Dir = dir
	execCmd.Stdout = os.Stdout
	execCmd.Stderr = os.Stderr

	if err := execCmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error running tectonic: %v\n", err)
		os.Exit(1)
	}

	pdfName := texFile[:len(texFile)-len(filepath.Ext(texFile))] + ".pdf"
	fmt.Printf("%s PDF compiled: %s/%s\n", color.GreenString("✓"), dir, pdfName)
}
