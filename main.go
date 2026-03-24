package main

import (
	"fmt"
	"lintree/internal/ui"
	"lintree/internal/version"
	"os"
	"runtime/debug"
)

func main() {
	// Soft memory limit. Too low = GC thrashes CPU. Too high = unbounded growth.
	debug.SetMemoryLimit(1024 * 1024 * 1024) // 1GB

	fast := false
	var args []string

	// Parse flags
	for _, arg := range os.Args[1:] {
		switch arg {
		case "-fast", "--fast":
			fast = true
		default:
			args = append(args, arg)
		}
	}

	if len(args) > 0 {
		switch args[0] {
		case "-v", "--version", "version":
			fmt.Print(version.Full())
			if latest, available, err := version.CheckForUpdate(); err == nil {
				if available {
					fmt.Printf("\n  Update available: %s -> %s\n", version.Version, latest)
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

	root := "."
	if len(args) > 0 {
		root = args[0]
	}

	info, err := os.Stat(root) //nolint:gosec // user-provided path is intentional
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n", err)
		os.Exit(1)
	}
	if !info.IsDir() {
		fmt.Fprintf(os.Stderr, "Error: %s is not a directory\n", root)
		os.Exit(1)
	}

	if err := ui.Run(root, fast); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n", err)
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Print(`lintree - Terminal disk usage visualizer

Usage:
  lintree [path]       Scan and visualize disk usage (default: current directory)
  lintree -fast [path] Fast scan mode (more workers, higher CPU usage)
  lintree -v           Show version and check for updates
  lintree -h           Show this help

Controls:
  arrows / jk          Navigate cells
  arrows / hl          Spatial movement
  Enter / l            Drill into directory
  Backspace / h        Go back
  ?                    Help overlay
  q / Ctrl+C           Quit

Website: https://lintree.sh
`)
}
