package main

import (
	"context"
	"encoding/csv"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// RepoStatus is the live scan result for one local git repository.
// Ahead/behind counts use already-known upstream tracking refs (no fetch).
type RepoStatus struct {
	Path                 string
	Origin               string // normalized remote.origin.url; empty if none
	HasUnstaged          bool
	HasStaged            bool
	HasUntracked         bool
	HasUnpushed          bool
	HasBehind            bool
	HasUntrackedUpstream bool
	AheadCount           int    // commits ahead of upstream (0 if none/unknown)
	BehindCount          int    // commits behind upstream (0 if none/unknown)
	HeadSHA              string // short HEAD SHA for cross-machine tip comparison
	Branch               string
	IsDirty              bool
	IsClean              bool
	Error                string
}

var debugMode bool
var dirtyOnly bool
var outputFile string

func main() {
	var (
		stateRepo      string
		agentMode      bool
		intervalStr    string
		heartbeatStr   string
		staleTTLStr    string
		tickTimeoutStr string
		machineID      string
		installSched   bool
		uninstallSched bool
		redactPaths    bool
		skipRemote     bool
	)

	flag.BoolVar(&debugMode, "debug", false, "Enable debug output")
	flag.BoolVar(&dirtyOnly, "dirty-only", false, "Show only projects/repos needing attention (dirty, unpushed, behind, untracked upstream, cross-machine cues, or errors)")
	flag.StringVar(&outputFile, "output", "", "Save results to CSV file (e.g., --output results.csv)")
	flag.StringVar(&stateRepo, "state-repo", "", "Local path to private Git state repository for cross-machine sync")
	flag.BoolVar(&agentMode, "agent", false, "Run as background agent publishing machine snapshots")
	flag.StringVar(&intervalStr, "interval", DefaultIntervalString, "Agent check interval (default 2m)")
	flag.StringVar(&heartbeatStr, "heartbeat", DefaultHeartbeatString, "Agent liveness commit interval when status unchanged (default 15m)")
	flag.StringVar(&staleTTLStr, "stale-ttl", DefaultStaleTTLString, "Mark machine snapshots stale after this duration (default 30m)")
	flag.StringVar(&tickTimeoutStr, "tick-timeout", DefaultTickTimeoutString, "Agent per-tick deadline for pull, scan, and publish (default 2m)")
	flag.StringVar(&machineID, "machine-id", "", "Machine identifier (default: hostname)")
	flag.BoolVar(&installSched, "install-scheduler", false, "Install OS scheduler to run the agent at login")
	flag.BoolVar(&uninstallSched, "uninstall-scheduler", false, "Remove OS scheduler registration")
	flag.BoolVar(&redactPaths, "redact-paths", false, "Redact full paths in published snapshots (keep basename)")
	flag.BoolVar(&skipRemote, "no-remote", false, "Skip loading other machines' snapshots even if state repo is configured")
	flag.Parse()

	if uninstallSched {
		if err := uninstallScheduler(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		return
	}

	flagSet := map[string]bool{}
	flag.Visit(func(f *flag.Flag) {
		flagSet[f.Name] = true
	})

	args := flag.Args()
	var rootDirArg string
	if len(args) >= 1 {
		rootDirArg = args[0]
	}

	configPath, err := DefaultConfigPath()
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not resolve config path: %v\n", err)
		configPath = ""
	}
	fileCfg, err := LoadUserConfig(configPath)
	if err != nil {
		// Unreadable/corrupt sticky config must not silently fall back to defaults.
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	resolved := ResolveSettings(FlagOverrides{
		StateRepo:      stateRepo,
		StateRepoSet:   flagSet["state-repo"],
		ScanRoot:       rootDirArg,
		ScanRootSet:    rootDirArg != "",
		MachineID:      machineID,
		MachineIDSet:   flagSet["machine-id"],
		Interval:       intervalStr,
		IntervalSet:    flagSet["interval"],
		Heartbeat:      heartbeatStr,
		HeartbeatSet:   flagSet["heartbeat"],
		StaleTTL:       staleTTLStr,
		StaleTTLSet:    flagSet["stale-ttl"],
		RedactPaths:    redactPaths,
		RedactPathsSet: flagSet["redact-paths"],
	}, fileCfg, os.Getenv)

	stateRepo = resolved.StateRepo
	machineID = resolved.MachineID
	if resolved.Interval != "" {
		intervalStr = resolved.Interval
	}
	if resolved.Heartbeat != "" {
		heartbeatStr = resolved.Heartbeat
	}
	if resolved.StaleTTL != "" {
		staleTTLStr = resolved.StaleTTL
	}
	redactPaths = resolved.RedactPaths

	if machineID == "" {
		host, err := os.Hostname()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error resolving hostname for machine id: %v\n", err)
			os.Exit(1)
		}
		machineID = host
	}

	interval, err := time.ParseDuration(intervalStr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Invalid --interval: %v\n", err)
		os.Exit(1)
	}
	heartbeat, err := time.ParseDuration(heartbeatStr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Invalid heartbeat: %v\n", err)
		os.Exit(1)
	}
	staleTTL, err := time.ParseDuration(staleTTLStr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Invalid --stale-ttl: %v\n", err)
		os.Exit(1)
	}
	tickTimeout, err := time.ParseDuration(tickTimeoutStr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Invalid --tick-timeout: %v\n", err)
		os.Exit(1)
	}
	if tickTimeout <= 0 {
		fmt.Fprintln(os.Stderr, "Invalid --tick-timeout: must be positive")
		os.Exit(1)
	}
	if stateRepo != "" {
		warnStaleTTLMismatch(heartbeat, staleTTL)
	}

	var rootDir string
	if resolved.ScanRoot != "" {
		rootDir = resolved.ScanRoot
		if abs, err := filepath.Abs(rootDir); err == nil {
			rootDir = abs
		}
	}
	if rootDir == "" {
		printUsage()
		os.Exit(1)
	}
	if stateRepo != "" {
		if abs, err := filepath.Abs(stateRepo); err == nil {
			stateRepo = abs
		}
	}

	if installSched {
		if stateRepo == "" {
			fmt.Fprintln(os.Stderr, "Error: --install-scheduler requires --state-repo (or sticky config / FIND_UNCOMMITTED_STATE_REPO)")
			os.Exit(1)
		}
		if err := validateStateRepo(stateRepo); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		if configPath != "" {
			sticky := UserConfig{
				StateRepo:   stateRepo,
				ScanRoot:    rootDir,
				Interval:    intervalStr,
				Heartbeat:   heartbeatStr,
				StaleTTL:    staleTTLStr,
				RedactPaths: redactPaths,
			}
			// Only persist machine_id when explicitly set; blank means hostname at runtime.
			if flagSet["machine-id"] {
				sticky.MachineID = machineID
			}
			if err := SaveUserConfig(configPath, sticky); err != nil {
				fmt.Fprintf(os.Stderr, "Error writing sticky config: %v\n", err)
				os.Exit(1)
			}
			fmt.Printf("Wrote sticky config: %s\n", configPath)
		}
		exe, err := resolveExecutable()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error resolving executable: %v\n", err)
			os.Exit(1)
		}
		printSchedulerPrereqs()
		// Smoke publish before OS registration so a broken state repo/credentials
		// fails install loudly (and avoids racing Linux enable --now).
		snapPath, err := smokePublishOnce(AgentConfig{
			ScanRoot:     rootDir,
			StateRepoDir: stateRepo,
			MachineID:    machineID,
			TickTimeout:  tickTimeout,
			RedactPaths:  redactPaths,
			Sync: SyncConfig{
				StateRepoDir: stateRepo,
				MachineID:    machineID,
				Heartbeat:    heartbeat,
			},
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: smoke publish failed (scheduler not installed): %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Smoke publish OK: %s\n", snapPath)
		if err := installScheduler(exe); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("Migration: existing unit-only installs should re-run --install-scheduler once so sticky config is created.")
		return
	}

	if agentMode {
		if stateRepo == "" {
			fmt.Fprintln(os.Stderr, "Error: --agent requires --state-repo (or sticky config / FIND_UNCOMMITTED_STATE_REPO)")
			os.Exit(1)
		}
		if err := validateStateRepo(stateRepo); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		if flagSet["state-repo"] && configPath != "" {
			persistMachineID := ""
			if flagSet["machine-id"] {
				persistMachineID = machineID
			}
			if err := EnsureConfigFromAgent(configPath, stateRepo, rootDir, persistMachineID, intervalStr, heartbeatStr, staleTTLStr, redactPaths); err != nil {
				fmt.Fprintf(os.Stderr, "warning: could not write sticky config: %v\n", err)
			}
		}
		printPrivacyNotice()
		err := RunAgentLoop(AgentConfig{
			ScanRoot:     rootDir,
			StateRepoDir: stateRepo,
			MachineID:    machineID,
			Interval:     interval,
			TickTimeout:  tickTimeout,
			RedactPaths:  redactPaths,
			DirtyOnly:    false, // agent always publishes full status set
			Sync: SyncConfig{
				StateRepoDir: stateRepo,
				MachineID:    machineID,
				Heartbeat:    heartbeat,
			},
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, "Agent error: %v\n", err)
			os.Exit(1)
		}
		return
	}

	// Normal scan mode
	fmt.Printf("Scanning for git repositories in: %s\n", rootDir)
	if dirtyOnly {
		fmt.Println("Showing only projects needing attention (local or cross-machine situations)...")
	}
	if outputFile != "" {
		fmt.Printf("Results will be saved to: %s\n", outputFile)
	}
	fmt.Println("This may take a while depending on the size of your drive...")
	fmt.Println()

	repos := findGitRepos(rootDir, stateRepo)
	if len(repos) == 0 {
		fmt.Println("No git repositories found.")
	} else {
		fmt.Printf("Found %d git repositories:\n\n", len(repos))
	}

	// When remotes may load, keep clean local repos so cross-machine situations
	// (other-machine work, branch mismatch) can still be detected; --dirty-only
	// filters the Attention/inventory presentation instead.
	willTryRemote := stateRepo != "" && !skipRemote
	scanCtx := context.Background()
	results := checkRepoStatuses(scanCtx, repos, dirtyOnly && !willTryRemote)

	var remote []LoadedSnapshot
	remoteOK := false
	if stateRepo != "" && !skipRemote {
		if resolved.StateRepoSource == SourceConfig {
			fmt.Fprintf(os.Stderr, "using state repo from config (%s); pass --no-remote for local only\n", stateRepo)
		}
		// Invalid/missing state repo is a hard error when remotes are requested.
		// Soft-degrading to local-only trains distrust of aggregate views.
		if err := validateStateRepo(stateRepo); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			fmt.Fprintln(os.Stderr, "Fix state_repo in sticky config / env / --state-repo, or pass --no-remote for a local-only scan.")
			os.Exit(1)
		}
		if err := PullStateRepoReadOnly(scanCtx, SyncConfig{StateRepoDir: stateRepo, MachineID: machineID}); err != nil {
			fmt.Fprintf(os.Stderr, "warning: %v\n", err)
		}
		// Try on-disk snapshots even when pull fails.
		remote, err = LoadAllMachineSnapshots(stateRepo, staleTTL, time.Now().UTC())
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: failed loading remote snapshots: %v\n", err)
		} else {
			warnCorruptSnapshots(remote)
			remoteOK = true
		}
	}

	if len(results) == 0 && len(remote) == 0 {
		return
	}

	// Aggregate UI when state repo validated or at least one snapshot loaded.
	useAggregate := remoteOK
	var rows []AggregateRow
	if useAggregate {
		rows = BuildAggregateRows(machineID, results, remote)
		situations := DetectSituations(rows)
		if dirtyOnly {
			keys := ProjectKeysWithSituations(situations)
			// Also keep load-error rows and single-repo local attention without cross-machine cues.
			rows = FilterRowsByProjectKeys(rows, keys)
			// Re-detect so Attention matches filtered inventory (drop orphaned stale-only noise).
			situations = DetectSituations(rows)
		}
		DisplayAttention(situations)
		fmt.Println("Full inventory:")
		displayAggregateTable(rows, false) // already situation-filtered when dirty-only
		printStaleMachineSummary(remote, staleTTL)
	} else {
		if dirtyOnly {
			var filtered []RepoStatus
			for _, r := range results {
				if repoNeedsAttention(r) {
					filtered = append(filtered, r)
				}
			}
			results = filtered
		}
		situations := DetectLocalSituations(machineID, results)
		DisplayAttention(situations)
		fmt.Println("Full inventory:")
		displayRepoStatusTable(results)
	}

	if outputFile != "" {
		var err error
		if useAggregate {
			err = exportAggregateToCSV(rows, outputFile, false)
		} else {
			err = exportToCSV(results, outputFile)
		}
		if err != nil {
			fmt.Printf("Error saving to CSV: %v\n", err)
		} else {
			fmt.Printf("Results saved to: %s\n", outputFile)
		}
	}

	if useAggregate {
		printAggregateSummary(rows, dirtyOnly)
	} else {
		printSummary(results)
	}
}

func printUsage() {
	fmt.Println("Usage: find-uncommitted [flags] [directory_to_scan]")
	fmt.Println()
	fmt.Println("Examples:")
	fmt.Println("  find-uncommitted C:\\code")
	fmt.Println("  find-uncommitted --dirty-only --output results.csv C:\\code")
	fmt.Println("  find-uncommitted --state-repo D:\\state-repo C:\\code")
	fmt.Println("  find-uncommitted --agent --state-repo D:\\state-repo --interval 2m C:\\code")
	fmt.Println("  find-uncommitted --install-scheduler --state-repo D:\\state-repo C:\\code")
	fmt.Println()
	fmt.Println("After --install-scheduler, sticky config enables aggregate remotes on bare scans.")
	fmt.Println("Install smoke-publishes one snapshot so you can confirm the state repo works.")
	fmt.Println("Scan root may come from config when the directory argument is omitted.")
	fmt.Println("Cross-machine sync requires a private Git repository. See README for privacy notes.")
}

func printPrivacyNotice() {
	fmt.Println("Privacy: snapshots may include repository paths and branch names.")
	fmt.Println("Use a private state repository. Consider --redact-paths to limit path detail.")
}

func printSchedulerPrereqs() {
	fmt.Println("Scheduler prerequisites:")
	fmt.Println("  - Built binary available (not `go run`)")
	fmt.Println("  - Private Git state repo cloned locally and accessible offline-tolerant")
	fmt.Println("  - Git credentials configured for non-interactive pull/push")
	if runtime.GOOS == "linux" {
		fmt.Println("  - systemd user session; enable lingering for headless: loginctl enable-linger $USER")
	}
	if runtime.GOOS == "windows" {
		fmt.Println("  - Permission to create scheduled tasks for the current user")
	}
}

func validateStateRepo(dir string) error {
	info, err := os.Stat(dir)
	if err != nil {
		return fmt.Errorf("state repo path %q: %w", dir, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("state repo path %q is not a directory", dir)
	}
	gitDir := filepath.Join(dir, ".git")
	if _, err := os.Stat(gitDir); err != nil {
		return fmt.Errorf("state repo %q does not look like a git repository (missing .git)", dir)
	}
	return nil
}

func printSummary(results []RepoStatus) {
	cleanCount := 0
	dirtyCount := 0
	unpushedCount := 0
	behindCount := 0
	untrackedUpstreamCount := 0
	errorCount := 0
	for _, status := range results {
		if status.Error != "" {
			errorCount++
		} else if status.IsDirty {
			dirtyCount++
		} else if status.HasUntrackedUpstream {
			untrackedUpstreamCount++
		} else if status.HasBehind {
			behindCount++
		} else if status.HasUnpushed {
			unpushedCount++
		} else {
			cleanCount++
		}
	}

	if dirtyOnly {
		attention := dirtyCount + unpushedCount + behindCount + untrackedUpstreamCount
		fmt.Printf("\nSummary: %d repositories needing attention, %d repositories with errors\n", attention, errorCount)
	} else {
		fmt.Printf("\nSummary: %d clean repositories, %d repositories with uncommitted changes, %d repositories with unpushed commits, %d repositories behind upstream, %d repositories with untracked upstream, %d repositories with errors\n",
			cleanCount, dirtyCount, unpushedCount, behindCount, untrackedUpstreamCount, errorCount)
	}
}

func findGitRepos(rootDir string, excludeRepos ...string) []string {
	var repos []string
	excludes := make([]string, 0, len(excludeRepos))
	for _, e := range excludeRepos {
		if e == "" {
			continue
		}
		abs, err := filepath.Abs(e)
		if err != nil {
			abs = filepath.Clean(e)
		}
		excludes = append(excludes, abs)
	}

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
				if shouldExcludeRepo(repoPath, excludes) {
					if debugMode {
						fmt.Printf("[DEBUG] Excluding state/sync repo: %s\n", repoPath)
					}
					return filepath.SkipDir
				}
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

func shouldExcludeRepo(repoPath string, excludes []string) bool {
	abs, err := filepath.Abs(repoPath)
	if err != nil {
		abs = filepath.Clean(repoPath)
	}
	for _, ex := range excludes {
		if abs == ex {
			return true
		}
	}
	return false
}

func checkRepoStatus(ctx context.Context, repoPath string) RepoStatus {
	status := RepoStatus{
		Path: repoPath,
	}

	if err := ctx.Err(); err != nil {
		status.Error = fmt.Sprintf("git check cancelled: %v", err)
		return status
	}

	// First check if this is a valid git repository
	_, stderr, err := runGit(ctx, repoPath, "rev-parse", "--git-dir")
	if err != nil {
		if isGitContextErr(ctx, err) {
			status.Error = fmt.Sprintf("git timed out or cancelled: %v", err)
			return status
		}
		// Check if it's a dubious ownership error
		if strings.Contains(stderr, "dubious ownership") {
			status.Error = "Git ownership issue - run: git config --global --add safe.directory " + strings.ReplaceAll(repoPath, "\\", "/")
			return status
		}
		status.Error = "Not a valid git repository"
		return status
	}

	// Get current branch
	branch, stderr, err := runGit(ctx, repoPath, "branch", "--show-current")
	if err != nil {
		if isGitContextErr(ctx, err) {
			status.Error = fmt.Sprintf("git timed out or cancelled: %v", err)
			return status
		}
		// Check if it's a detached HEAD state (exit code 1)
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			// Try to get the commit hash instead
			commit, _, commitErr := runGit(ctx, repoPath, "rev-parse", "--short", "HEAD")
			if commitErr == nil {
				status.Branch = fmt.Sprintf("detached HEAD (%s)", strings.TrimSpace(commit))
			} else if isGitContextErr(ctx, commitErr) {
				status.Error = fmt.Sprintf("git timed out or cancelled: %v", commitErr)
				return status
			} else {
				status.Branch = "detached HEAD"
				status.Error = fmt.Sprintf("Branch issue: %v", err)
			}
		} else {
			_ = stderr
			status.Branch = "unknown"
			status.Error = fmt.Sprintf("Branch issue: %v", err)
		}
		// Don't return here, continue checking other status
	} else {
		status.Branch = strings.TrimSpace(branch)
	}

	// Capture normalized origin for cross-machine project correlation.
	status.Origin = repoOriginURL(ctx, repoPath)

	// Check for unstaged changes
	unstaged, _, err := runGit(ctx, repoPath, "diff", "--name-only")
	if err != nil {
		if isGitContextErr(ctx, err) {
			status.Error = fmt.Sprintf("git timed out or cancelled: %v", err)
			return status
		}
		if status.Error == "" {
			status.Error = fmt.Sprintf("Failed to check unstaged changes: %v", err)
		} else {
			status.Error += fmt.Sprintf("; unstaged check failed: %v", err)
		}
		return status
	}
	status.HasUnstaged = len(strings.TrimSpace(unstaged)) > 0

	// Check for staged changes
	staged, _, err := runGit(ctx, repoPath, "diff", "--cached", "--name-only")
	if err != nil {
		if isGitContextErr(ctx, err) {
			status.Error = fmt.Sprintf("git timed out or cancelled: %v", err)
			return status
		}
		if status.Error == "" {
			status.Error = fmt.Sprintf("Failed to check staged changes: %v", err)
		} else {
			status.Error += fmt.Sprintf("; staged check failed: %v", err)
		}
		return status
	}
	status.HasStaged = len(strings.TrimSpace(staged)) > 0

	// Check for untracked files
	untracked, _, err := runGit(ctx, repoPath, "ls-files", "--others", "--exclude-standard")
	if err != nil {
		if isGitContextErr(ctx, err) {
			status.Error = fmt.Sprintf("git timed out or cancelled: %v", err)
			return status
		}
		if status.Error == "" {
			status.Error = fmt.Sprintf("Failed to check untracked files: %v", err)
		} else {
			status.Error += fmt.Sprintf("; untracked check failed: %v", err)
		}
		return status
	}
	status.HasUntracked = len(strings.TrimSpace(untracked)) > 0

	// Determine upstream tracking status first.
	_, upStderr, upstreamErr := runGit(ctx, repoPath, "rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{u}")
	if upstreamErr != nil {
		if isGitContextErr(ctx, upstreamErr) {
			status.Error = fmt.Sprintf("git timed out or cancelled: %v", upstreamErr)
			return status
		}
		errOutput := strings.ToLower(upStderr + upstreamErr.Error())
		if strings.Contains(errOutput, "no upstream configured") {
			status.HasUntrackedUpstream = true
		} else {
			status.Error = fmt.Sprintf("Failed to check upstream tracking: %v", upstreamErr)
		}
	} else {
		// Ahead/behind against cached upstream refs only (no fetch).
		status.AheadCount = revListCount(ctx, repoPath, "@{u}..HEAD")
		status.BehindCount = revListCount(ctx, repoPath, "HEAD..@{u}")
		status.HasUnpushed = status.AheadCount > 0
		status.HasBehind = status.BehindCount > 0
	}

	status.HeadSHA = shortHeadSHA(ctx, repoPath)

	// Dirty means working tree changes only.
	status.IsDirty = status.HasUnstaged || status.HasStaged || status.HasUntracked
	status.IsClean = !status.IsDirty && !status.HasUnpushed && !status.HasBehind && !status.HasUntrackedUpstream

	return status
}

// revListCount runs `git rev-list --count <range>` and returns 0 on any failure.
func revListCount(ctx context.Context, repoPath, revRange string) int {
	out, _, err := runGit(ctx, repoPath, "rev-list", "--count", revRange)
	if err != nil {
		if debugMode {
			fmt.Printf("[DEBUG] Failed rev-list %s in %s: %v\n", revRange, repoPath, err)
		}
		return 0
	}
	count, err := strconv.Atoi(strings.TrimSpace(out))
	if err != nil {
		if debugMode {
			fmt.Printf("[DEBUG] Failed to parse rev-list count in %s: %v\n", repoPath, err)
		}
		return 0
	}
	return count
}

// shortHeadSHA returns a short HEAD commit hash, or empty when unavailable.
func shortHeadSHA(ctx context.Context, repoPath string) string {
	out, _, err := runGit(ctx, repoPath, "rev-parse", "--short", "HEAD")
	if err != nil {
		return ""
	}
	return strings.TrimSpace(out)
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
		} else if status.HasBehind && status.HasUnpushed {
			statusText = "↕️ Diverged"
			changesText = strings.Join(getChangesText(status), ", ")
		} else if status.HasBehind {
			statusText = "⬇️ Behind"
			changesText = strings.Join(getChangesText(status), ", ")
		} else if status.HasUnpushed {
			statusText = "⬆️ Unpushed"
			changesText = strings.Join(getChangesText(status), ", ")
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

	header := []string{"Repository", "Origin", "Branch", "Status", "Changes"}
	if err := writer.Write(header); err != nil {
		return fmt.Errorf("failed to write header to CSV: %v", err)
	}

	for _, status := range results {
		wd, _ := os.Getwd()
		relPath, _ := filepath.Rel(wd, status.Path)
		if relPath == "." {
			relPath = status.Path
		}

		branch := status.Branch

		var statusText string
		if status.Error != "" {
			statusText = "Error: " + status.Error
		} else if status.IsDirty {
			statusText = "Dirty"
		} else if status.HasUntrackedUpstream {
			statusText = "UntrackedUpstream"
		} else if status.HasBehind && status.HasUnpushed {
			statusText = "Diverged"
		} else if status.HasBehind {
			statusText = "Behind"
		} else if status.HasUnpushed {
			statusText = "Unpushed"
		} else {
			statusText = "Clean"
		}

		row := []string{
			relPath,
			status.Origin,
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
		if status.AheadCount > 0 {
			changes = append(changes, fmt.Sprintf("unpushed:%d", status.AheadCount))
		} else {
			changes = append(changes, "unpushed")
		}
	}
	if status.HasBehind {
		if status.BehindCount > 0 {
			changes = append(changes, fmt.Sprintf("behind:%d", status.BehindCount))
		} else {
			changes = append(changes, "behind")
		}
	}
	if status.HasUntrackedUpstream {
		changes = append(changes, "untracked-upstream")
	}
	return changes
}
