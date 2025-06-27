package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
)

type RepoStatus struct {
	Path         string
	HasUnstaged  bool
	HasStaged    bool
	HasUntracked bool
	Branch       string
	IsClean      bool
	Error        string
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: go run main.go <directory_to_scan>")
		fmt.Println("Example: go run main.go C:\\")
		os.Exit(1)
	}

	rootDir := os.Args[1]
	fmt.Printf("Scanning for git repositories in: %s\n", rootDir)
	fmt.Println("This may take a while depending on the size of your drive...")
	fmt.Println()

	repos := findGitRepos(rootDir)

	if len(repos) == 0 {
		fmt.Println("No git repositories found.")
		return
	}

	fmt.Printf("Found %d git repositories:\n\n", len(repos))

	// Check status of each repository concurrently
	var wg sync.WaitGroup
	statusChan := make(chan RepoStatus, len(repos))

	for _, repo := range repos {
		wg.Add(1)
		go func(repoPath string) {
			defer wg.Done()
			status := checkRepoStatus(repoPath)
			statusChan <- status
		}(repo)
	}

	// Close channel when all goroutines complete
	go func() {
		wg.Wait()
		close(statusChan)
	}()

	// Collect and display results
	var results []RepoStatus
	for status := range statusChan {
		results = append(results, status)
	}

	// Display results
	for _, status := range results {
		displayRepoStatus(status)
	}

	// Summary
	cleanCount := 0
	dirtyCount := 0
	for _, status := range results {
		if status.IsClean {
			cleanCount++
		} else {
			dirtyCount++
		}
	}

	fmt.Printf("\nSummary: %d clean repositories, %d repositories with uncommitted changes\n", cleanCount, dirtyCount)
}

func findGitRepos(rootDir string) []string {
	var repos []string

	err := filepath.Walk(rootDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			fmt.Printf("[DEBUG] Skipping (error accessing): %s\n", path)
			return nil
		}

		if info.IsDir() {
			fmt.Printf("[DEBUG] Visiting: %s\n", path)

			// Check if this is a .git directory FIRST
			if filepath.Base(path) == ".git" {
				fmt.Printf("[DEBUG] Found .git directory: %s\n", path)
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
				fmt.Printf("[DEBUG] Skipping directory: %s\n", path)
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

func checkRepoStatus(repoPath string) RepoStatus {
	status := RepoStatus{
		Path: repoPath,
	}

	// Get current branch
	branch, err := exec.Command("git", "-C", repoPath, "branch", "--show-current").Output()
	if err != nil {
		// Check if it's a detached HEAD state
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			// Try to get the commit hash instead
			commit, commitErr := exec.Command("git", "-C", repoPath, "rev-parse", "--short", "HEAD").Output()
			if commitErr == nil {
				status.Branch = fmt.Sprintf("detached HEAD (%s)", strings.TrimSpace(string(commit)))
			} else {
				status.Branch = "detached HEAD"
				status.Error = fmt.Sprintf("Branch issue: %v", err)
			}
		} else {
			status.Branch = "unknown"
			status.Error = fmt.Sprintf("Branch issue: %v", err)
		}
		// Don't return here, continue checking other status
	} else {
		status.Branch = strings.TrimSpace(string(branch))
	}

	// Check for unstaged changes
	unstaged, err := exec.Command("git", "-C", repoPath, "diff", "--name-only").Output()
	if err != nil {
		if status.Error == "" {
			status.Error = fmt.Sprintf("Failed to check unstaged changes: %v", err)
		} else {
			status.Error += fmt.Sprintf("; unstaged check failed: %v", err)
		}
		return status
	}
	status.HasUnstaged = len(strings.TrimSpace(string(unstaged))) > 0

	// Check for staged changes
	staged, err := exec.Command("git", "-C", repoPath, "diff", "--cached", "--name-only").Output()
	if err != nil {
		if status.Error == "" {
			status.Error = fmt.Sprintf("Failed to check staged changes: %v", err)
		} else {
			status.Error += fmt.Sprintf("; staged check failed: %v", err)
		}
		return status
	}
	status.HasStaged = len(strings.TrimSpace(string(staged))) > 0

	// Check for untracked files
	untracked, err := exec.Command("git", "-C", repoPath, "ls-files", "--others", "--exclude-standard").Output()
	if err != nil {
		if status.Error == "" {
			status.Error = fmt.Sprintf("Failed to check untracked files: %v", err)
		} else {
			status.Error += fmt.Sprintf("; untracked check failed: %v", err)
		}
		return status
	}
	status.HasUntracked = len(strings.TrimSpace(string(untracked))) > 0

	// Determine if repository is clean
	status.IsClean = !status.HasUnstaged && !status.HasStaged && !status.HasUntracked

	return status
}

func displayRepoStatus(status RepoStatus) {
	// Get relative path for cleaner display
	wd, _ := os.Getwd()
	relPath, _ := filepath.Rel(wd, status.Path)
	if relPath == "." {
		relPath = status.Path
	}

	fmt.Printf("📁 %s\n", relPath)
	fmt.Printf("   Branch: %s\n", status.Branch)

	if status.Error != "" {
		fmt.Printf("   ❌ Error: %s\n", status.Error)
	} else if status.IsClean {
		fmt.Printf("   ✅ Clean\n")
	} else {
		fmt.Printf("   ⚠️  Has uncommitted changes:\n")
		if status.HasUnstaged {
			fmt.Printf("      • Unstaged changes\n")
		}
		if status.HasStaged {
			fmt.Printf("      • Staged changes\n")
		}
		if status.HasUntracked {
			fmt.Printf("      • Untracked files\n")
		}
	}
	fmt.Println()
}
