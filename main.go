package main

import (
	"encoding/csv"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
)

type RepoStatus struct {
	Path                 string
	HasUnstaged          bool
	HasStaged            bool
	HasUntracked         bool
	HasUnpushed          bool
	HasUntrackedUpstream bool
	Branch               string
	IsDirty              bool
	IsClean              bool
	Error                string
}

var debugMode bool
var dirtyOnly bool
var outputFile string

func main() {
	flag.BoolVar(&debugMode, "debug", false, "Enable debug output")
	flag.BoolVar(&dirtyOnly, "dirty-only", false, "Show only repositories with uncommitted changes")
	flag.StringVar(&outputFile, "output", "", "Save results to CSV file (e.g., --output results.csv)")
	flag.Parse()

	args := flag.Args()
	if len(args) < 1 {
		fmt.Println("Usage: go run main.go [--debug] [--dirty-only] [--output filename.csv] <directory_to_scan>")
		fmt.Println("Example: go run main.go C:\\")
		fmt.Println("Example: go run main.go --debug C:\\")
		fmt.Println("Example: go run main.go --dirty-only C:\\")
		fmt.Println("Example: go run main.go --output results.csv C:\\")
		os.Exit(1)
	}

	rootDir := args[0]
	fmt.Printf("Scanning for git repositories in: %s\n", rootDir)
	if dirtyOnly {
		fmt.Println("Showing only repositories with uncommitted changes...")
	}
	if outputFile != "" {
		fmt.Printf("Results will be saved to: %s\n", outputFile)
	}
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

	maxWorkers := runtime.NumCPU() * 4
	if maxWorkers < 4 {
		maxWorkers = 4
	}
	sem := make(chan struct{}, maxWorkers)

	for _, repo := range repos {
		wg.Add(1)
		go func(repoPath string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
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
		// Filter out clean repositories if --dirty-only flag is set
		if dirtyOnly && status.Error == "" && !status.IsDirty {
			continue
		}
		results = append(results, status)
	}

	// Display results in tabular format
	displayRepoStatusTable(results)

	// Export to CSV if requested
	if outputFile != "" {
		err := exportToCSV(results, outputFile)
		if err != nil {
			fmt.Printf("Error saving to CSV: %v\n", err)
		} else {
			fmt.Printf("Results saved to: %s\n", outputFile)
		}
	}

	// Summary
	cleanCount := 0
	dirtyCount := 0
	unpushedCount := 0
	untrackedUpstreamCount := 0
	errorCount := 0
	for _, status := range results {
		if status.Error != "" {
			errorCount++
		} else if status.IsDirty {
			dirtyCount++
		} else if status.HasUntrackedUpstream {
			untrackedUpstreamCount++
		} else if status.HasUnpushed {
			unpushedCount++
		} else {
			cleanCount++
		}
	}

	if dirtyOnly {
		fmt.Printf("\nSummary: %d repositories with uncommitted changes, %d repositories with errors\n", dirtyCount, errorCount)
	} else {
		fmt.Printf("\nSummary: %d clean repositories, %d repositories with uncommitted changes, %d repositories with unpushed commits, %d repositories with untracked upstream, %d repositories with errors\n",
			cleanCount, dirtyCount, unpushedCount, untrackedUpstreamCount, errorCount)
	}
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

	// Determine upstream tracking status first.
	_, upstreamErr := exec.Command("git", "-C", repoPath, "rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{u}").Output()
	if upstreamErr != nil {
		status.HasUntrackedUpstream = true
	} else {
		// Check for unpushed commits only when an upstream exists.
		unpushed, err := exec.Command("git", "-C", repoPath, "rev-list", "--count", "@{u}..HEAD").Output()
		if err != nil {
			if debugMode {
				fmt.Printf("[DEBUG] Failed to check unpushed commits in %s: %v\n", repoPath, err)
			}
		} else {
			count, parseErr := strconv.Atoi(strings.TrimSpace(string(unpushed)))
			if parseErr != nil {
				if debugMode {
					fmt.Printf("[DEBUG] Failed to parse unpushed count in %s: %v\n", repoPath, parseErr)
				}
			} else if count > 0 {
				status.HasUnpushed = true
			}
		}
	}

	// Dirty means working tree changes only.
	status.IsDirty = status.HasUnstaged || status.HasStaged || status.HasUntracked
	status.IsClean = !status.IsDirty && !status.HasUnpushed && !status.HasUntrackedUpstream

	return status
}

func displayRepoStatusTable(results []RepoStatus) {
	// Get working directory for relative paths
	wd, _ := os.Getwd()

	const pathColWidth = 50
	const branchColWidth = 20
	const statusColWidth = 18

	// Print table header
	fmt.Printf("%-*s %-*s %-*s %s\n", pathColWidth, "Repository", branchColWidth, "Branch", statusColWidth, "Status", "Changes")
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
		} else if status.IsDirty {
			statusText = "⚠️  Dirty"
			changesText = strings.Join(getChangesText(status), ", ")
		} else if status.HasUntrackedUpstream {
			statusText = "🔗 Untracked Upstream"
			changesText = "untracked-upstream"
		} else if status.HasUnpushed {
			statusText = "⬆️ Unpushed"
			changesText = "unpushed"
		} else {
			statusText = "✅ Clean"
			changesText = "-"
		}

		// Truncate long branch names
		branch := status.Branch
		if len(branch) > 17 {
			branch = branch[:14] + "..."
		}

		fmt.Printf("%-*s %-*s %-*s %s\n", pathColWidth, relPath, branchColWidth, branch, statusColWidth, statusText, changesText)
	}
}

func exportToCSV(results []RepoStatus, filename string) error {
	file, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("failed to create CSV file: %v", err)
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	// Write header
	header := []string{"Repository", "Branch", "Status", "Changes"}
	if err := writer.Write(header); err != nil {
		return fmt.Errorf("failed to write header to CSV: %v", err)
	}

	// Write data rows
	for _, status := range results {
		// Get relative path for cleaner display
		wd, _ := os.Getwd()
		relPath, _ := filepath.Rel(wd, status.Path)
		if relPath == "." {
			relPath = status.Path
		}

		branch := status.Branch

		// Determine status and changes
		var statusText string
		if status.Error != "" {
			statusText = "Error: " + status.Error
		} else if status.IsDirty {
			statusText = "Dirty"
		} else if status.HasUntrackedUpstream {
			statusText = "UntrackedUpstream"
		} else if status.HasUnpushed {
			statusText = "Unpushed"
		} else {
			statusText = "Clean"
		}

		row := []string{
			relPath,
			branch,
			statusText,
			strings.Join(getChangesText(status), ", "),
		}
		if err := writer.Write(row); err != nil {
			return fmt.Errorf("failed to write row to CSV: %v", err)
		}
	}
	return nil
}

func getChangesText(status RepoStatus) []string {
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
	if status.HasUntrackedUpstream {
		changes = append(changes, "untracked-upstream")
	}
	return changes
}
