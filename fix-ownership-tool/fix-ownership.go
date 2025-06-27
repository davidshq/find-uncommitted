package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

var debugMode bool

func main() {
	flag.BoolVar(&debugMode, "debug", false, "Enable debug output")
	flag.Parse()

	args := flag.Args()
	if len(args) < 1 {
		fmt.Println("Usage: go run fix-ownership.go [--debug] <directory_to_scan>")
		fmt.Println("This will find git repositories with ownership issues and fix them.")
		os.Exit(1)
	}

	rootDir := args[0]
	fmt.Printf("Scanning for git repositories in: %s\n", rootDir)
	fmt.Println("This will automatically fix ownership issues...")
	fmt.Println()

	repos := findGitRepos(rootDir)

	if len(repos) == 0 {
		fmt.Println("No git repositories found.")
		return
	}

	fmt.Printf("Found %d git repositories. Checking for ownership issues...\n\n", len(repos))

	fixedCount := 0
	for _, repo := range repos {
		if hasOwnershipIssue(repo) {
			fmt.Printf("Fixing ownership for: %s\n", repo)
			if fixOwnership(repo) {
				fixedCount++
				fmt.Printf("✅ Fixed: %s\n", repo)
			} else {
				fmt.Printf("❌ Failed to fix: %s\n", repo)
			}
		} else if debugMode {
			fmt.Printf("✅ No ownership issue: %s\n", repo)
		}
	}

	fmt.Printf("\nFixed ownership for %d repositories.\n", fixedCount)
}

func findGitRepos(rootDir string) []string {
	var repos []string

	err := filepath.Walk(rootDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			if debugMode {
				fmt.Printf("[DEBUG] Skipping (error accessing): %s\n", path)
			}
			return nil
		}

		if info.IsDir() {
			if debugMode {
				fmt.Printf("[DEBUG] Visiting: %s\n", path)
			}

			// Check if this is a .git directory FIRST
			if filepath.Base(path) == ".git" {
				if debugMode {
					fmt.Printf("[DEBUG] Found .git directory: %s\n", path)
				}
				repoPath := filepath.Dir(path)
				repos = append(repos, repoPath)
				return filepath.SkipDir
			}

			// Then check if directory should be skipped
			base := filepath.Base(path)
			if strings.HasPrefix(base, ".") ||
				base == "node_modules" ||
				base == "vendor" ||
				base == "bin" ||
				base == "obj" ||
				strings.Contains(path, "\\Windows\\") ||
				strings.Contains(path, "\\Program Files\\") ||
				strings.Contains(path, "\\Program Files (x86)\\") {
				if debugMode {
					fmt.Printf("[DEBUG] Skipping directory: %s\n", path)
				}
				return filepath.SkipDir
			}
		}

		return nil
	})

	if err != nil {
		fmt.Printf("Error scanning directory: %v\n", err)
	}

	return repos
}

func hasOwnershipIssue(repoPath string) bool {
	_, err := exec.Command("git", "-C", repoPath, "rev-parse", "--git-dir").Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			errOutput := string(exitErr.Stderr)
			return strings.Contains(errOutput, "dubious ownership")
		}
	}
	return false
}

func fixOwnership(repoPath string) bool {
	// Convert Windows path to forward slashes for git
	gitPath := strings.ReplaceAll(repoPath, "\\", "/")

	cmd := exec.Command("git", "config", "--global", "--add", "safe.directory", gitPath)
	err := cmd.Run()
	return err == nil
}
