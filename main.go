package main

import (
	"flag"
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
	HasUnpushed  bool
	Branch       string
	IsClean      bool
	Error        string
}

var debugMode bool

func main() {
	flag.BoolVar(&debugMode, "debug", false, "Enable debug output")
	flag.Parse()

	args := flag.Args()
	if len(args) < 1 {
		fmt.Println("Usage: go run main.go [--debug] <directory_to_scan>")
		fmt.Println("Example: go run main.go C:\\")
		fmt.Println("Example: go run main.go --debug C:\\")
		os.Exit(1)
	}

	rootDir := args[0]
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

	// Display results in tabular format
	displayRepoStatusTable(results)

	// Summary
	cleanCount := 0
	dirtyCount := 0
	errorCount := 0
	for _, status := range results {
		if status.Error != "" {
			errorCount++
		} else if status.IsClean {
			cleanCount++
		} else {
			dirtyCount++
		}
	}

	fmt.Printf("\nSummary: %d clean repositories, %d repositories with uncommitted changes, %d repositories with errors\n", cleanCount, dirtyCount, errorCount)
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

func checkRepoStatus(repoPath string) RepoStatus {
	status := RepoStatus{
		Path: repoPath,
	}

	// First check if this is a valid git repository
	_, err := exec.Command("git", "-C", repoPath, "rev-parse", "--git-dir").Output()
	if err != nil {
		// Check if it's a dubious ownership error
		if exitErr, ok := err.(*exec.ExitError); ok {
			errOutput := string(exitErr.Stderr)
			if strings.Contains(errOutput, "dubious ownership") {
				status.Error = "Git ownership issue - run: git config --global --add safe.directory " + strings.ReplaceAll(repoPath, "\\", "/")
				return status
			}
		}
		status.Error = "Not a valid git repository"
		return status
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

	// Check for unpushed commits
	unpushed, err := exec.Command("git", "-C", repoPath, "rev-list", "--count", "@{u}..HEAD").Output()
	if err != nil {
		// If there's no upstream branch, check if there are any commits at all
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 128 {
			// No upstream branch, check if we have any commits
			commitCount, commitErr := exec.Command("git", "-C", repoPath, "rev-list", "--count", "HEAD").Output()
			if commitErr == nil {
				count := strings.TrimSpace(string(commitCount))
				if count != "0" {
					status.HasUnpushed = true
				}
			}
		} else {
			// Other error, log it but don't fail the entire check
			if debugMode {
				fmt.Printf("[DEBUG] Failed to check unpushed commits in %s: %v\n", repoPath, err)
			}
		}
	} else {
		count := strings.TrimSpace(string(unpushed))
		if count != "0" {
			status.HasUnpushed = true
		}
	}

	// Determine if repository is clean
	status.IsClean = !status.HasUnstaged && !status.HasStaged && !status.HasUntracked && !status.HasUnpushed

	return status
}

func displayRepoStatusTable(results []RepoStatus) {
	// Get working directory for relative paths
	wd, _ := os.Getwd()

	// Print table header
	fmt.Printf("%-45s %-15s %-8s %s\n", "Repository", "Branch", "Status", "Changes")
	fmt.Println(strings.Repeat("-", 90))

	// Print each repository as a table row
	for _, status := range results {
		// Get relative path for cleaner display
		relPath, _ := filepath.Rel(wd, status.Path)
		if relPath == "." {
			relPath = status.Path
		}

		// Truncate long paths
		if len(relPath) > 42 {
			relPath = "..." + relPath[len(relPath)-39:]
		}

		// Determine status and changes
		var statusText, changesText string
		if status.Error != "" {
			statusText = "❌ Error"
			changesText = status.Error
		} else if status.IsClean {
			statusText = "✅ Clean"
			changesText = "-"
		} else {
			statusText = "⚠️  Dirty"
			var changes []string
			if status.HasUnstaged {
				changes = append(changes, "unstaged")
			}
			if status.HasStaged {
				changes = append(changes, "staged")
			}
			if status.HasUntracked {
				changes = append(changes, "untracked")
			}
			if status.HasUnpushed {
				changes = append(changes, "unpushed")
			}
			changesText = strings.Join(changes, ", ")
		}

		// Truncate long branch names
		branch := status.Branch
		if len(branch) > 17 {
			branch = branch[:14] + "..."
		}

		fmt.Printf("%-50s %-20s %-10s %s\n", relPath, branch, statusText, changesText)
	}
}
