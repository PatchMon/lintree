package main

import (
	"fmt"
	"lintree/internal/ui"
	"lintree/internal/version"
	"os"
)

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "-v", "--version", "version":
			fmt.Print(version.Full())
			// Check for updates
			if latest, available, err := version.CheckForUpdate(); err == nil {
				if available {
					fmt.Printf("\n  Update available: %s → %s\n", version.Version, latest)
					fmt.Printf("  Run: %s\n", version.UpdateCommand())
				} else {
					fmt.Println("\n  You're on the latest version.")
				}
			}
			return
		case "-h", "--help", "help":
			printUsage()
			return
		}
	}

	root := "/"
	if len(os.Args) > 1 {
		root = os.Args[1]
	}

	info, err := os.Stat(root)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n", err)
		os.Exit(1)
	}
	if !info.IsDir() {
		fmt.Fprintf(os.Stderr, "Error: %s is not a directory\n", root)
		os.Exit(1)
	}

	if err := ui.Run(root); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n", err)
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Print(`lintree - Terminal disk usage visualizer

Usage:
  lintree [path]       Scan and visualize disk usage (default: /)
  lintree -v           Show version and check for updates
  lintree -h           Show this help

Controls:
  ↑↓ / jk              Navigate cells
  ←→ / hl              Spatial movement
  Enter / l             Drill into directory
  Backspace / h         Go back
  ?                     Help overlay
  q / Ctrl+C            Quit

Website: https://lintree.sh
`)
}
