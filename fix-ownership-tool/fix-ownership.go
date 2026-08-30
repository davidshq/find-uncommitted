package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"find-uncommitted/internal/discover"
)

var debugMode bool

func main() {
	flag.BoolVar(&debugMode, "debug", false, "Enable debug output")
	flag.Parse()

	args := flag.Args()
	if len(args) < 1 {
		fmt.Println("Usage: fix-ownership [--debug] <directory_to_scan>")
		fmt.Println("This will find git repositories with ownership issues and fix them.")
		os.Exit(1)
	}

	rootDir := args[0]
	fmt.Printf("Scanning for git repositories in: %s\n", rootDir)
	fmt.Println("This will automatically fix ownership issues...")
	fmt.Println()

	repos := discover.FindGitRepos(rootDir, discover.WalkOptions{Debug: debugMode})

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
	gitPath := strings.ReplaceAll(repoPath, "\\", "/")

	existing, err := exec.Command("git", "config", "--global", "--get-all", "safe.directory").Output()
	if err == nil {
		for _, line := range strings.Split(strings.ReplaceAll(string(existing), "\r\n", "\n"), "\n") {
			if strings.TrimSpace(line) == gitPath {
				return true
			}
		}
	}

	cmd := exec.Command("git", "config", "--global", "--add", "safe.directory", gitPath)
	err = cmd.Run()
	return err == nil
}
