package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/davidshq/find-uncommitted/internal/discover"
	"github.com/davidshq/find-uncommitted/internal/gitexec"
)

var debugMode bool
var dirtyOnly bool
var outputFile string

func main() {
	// Detach as early as possible for Task Scheduler launches so a console
	// window does not flash before flag parsing / config I/O.
	if argsHasAgentMode(os.Args[1:]) {
		detachAgentConsoleIfOwned()
	}

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
		maxWorkers     int
	)

	flag.BoolVar(&debugMode, "debug", false, "Enable debug output")
	flag.BoolVar(&dirtyOnly, "dirty-only", false, "Show only projects/repos needing attention (dirty, unpushed, behind, untracked upstream, cross-machine cues, or errors)")
	var showInventory bool
	flag.BoolVar(&showInventory, "inventory", false, "Print path-centric Full inventory (and Attention nudges) instead of the default Project × Machine matrix")
	var verboseOut bool
	flag.BoolVar(&verboseOut, "verbose", false, "Same as --inventory: path table + Attention (audit trail)")
	flag.StringVar(&outputFile, "output", "", "Save results to CSV file (e.g., --output results.csv)")
	flag.StringVar(&stateRepo, "state-repo", "", "Local path to private Git state repository for cross-machine sync")
	flag.StringVar(&intervalStr, "interval", DefaultIntervalString, fmt.Sprintf("Agent check interval (default %s)", DefaultIntervalString))
	flag.StringVar(&heartbeatStr, "heartbeat", DefaultHeartbeatString, fmt.Sprintf("Agent liveness commit interval when status unchanged (default %s)", DefaultHeartbeatString))
	flag.StringVar(&staleTTLStr, "stale-ttl", DefaultStaleTTLString, fmt.Sprintf("Mark machine snapshots stale after this duration (default %s)", DefaultStaleTTLString))
	flag.StringVar(&tickTimeoutStr, "tick-timeout", DefaultTickTimeoutString, fmt.Sprintf("Agent per-tick deadline for pull, scan, and publish (default %s)", DefaultTickTimeoutString))
	flag.StringVar(&machineID, "machine-id", "", "Machine identifier (default: hostname)")
	flag.BoolVar(&redactPaths, "redact-paths", false, "Redact full paths in published snapshots (keep basename)")
	flag.BoolVar(&skipRemote, "no-remote", false, "Skip loading other machines' snapshots even if state repo is configured")
	flag.IntVar(&maxWorkers, "max-workers", 0, fmt.Sprintf("Max parallel repo checks (default %d; 0 = default)", gitexec.DefaultMaxWorkers))
	var jsonOutput bool
	flag.BoolVar(&jsonOutput, "json", false, "With check: print machine-readable JSON to stdout (human text remains default)")
	var printConfig bool
	flag.BoolVar(&printConfig, "print-config", false, "Print resolved settings with sources and exit")
	flag.Usage = printUsage
	flag.Parse()
	if verboseOut {
		showInventory = true
	}

	flagSet := map[string]bool{}
	flag.Visit(func(f *flag.Flag) {
		flagSet[f.Name] = true
	})

	args := flag.Args()
	checkMode := false
	checkPath := ""
	doctorMode := false
	var rootDirArg string
	softMode, softRest, softErr := parseSoftCommand(args)
	if softErr != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", softErr)
		switch softMode {
		case softDoctor:
			fmt.Fprintln(os.Stderr, "Usage: find-uncommitted [flags] doctor")
		case softAgent:
			fmt.Fprintln(os.Stderr, "Usage: find-uncommitted [flags] agent [directory_to_scan]")
		case softInstallScheduler:
			fmt.Fprintln(os.Stderr, "Usage: find-uncommitted [flags] install-scheduler [directory_to_scan]")
		case softUninstallScheduler:
			fmt.Fprintln(os.Stderr, "Usage: find-uncommitted [flags] uninstall-scheduler")
		}
		os.Exit(1)
	}
	switch softMode {
	case softCheck:
		checkMode = true
		parsedPath, checkJSON, err := parseCheckArgs(softRest)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		checkPath = parsedPath
		if checkJSON {
			jsonOutput = true
		}
	case softDoctor:
		doctorMode = true
	case softAgent:
		agentMode = true
		if len(softRest) >= 1 {
			rootDirArg = softRest[0]
		}
	case softInstallScheduler:
		installSched = true
		if len(softRest) >= 1 {
			rootDirArg = softRest[0]
		}
	case softUninstallScheduler:
		uninstallSched = true
	case softNone:
		if len(softRest) >= 1 {
			rootDirArg = softRest[0]
		}
	}

	if uninstallSched {
		if err := uninstallScheduler(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		return
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
		MaxWorkers:     maxWorkers,
		MaxWorkersSet:  flagSet["max-workers"],
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
	maxWorkers = resolved.MaxWorkers

	if machineID == "" {
		host, err := os.Hostname()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error resolving hostname for machine id: %v\n", err)
			os.Exit(1)
		}
		machineID = host
	}
	// Hostname-only identity collides on cloned VMs; generate and persist a stable id
	// only when installing the scheduler or starting the agent (not on every bare scan).
	persistMachineIDInConfig := false
	if shouldPersistStableMachineID(resolved, fileCfg, flagSet, installSched, agentMode) {
		machineID = GenerateStableMachineID(machineID)
		persistMachineIDInConfig = true
	} else if flagSet["machine-id"] {
		persistMachineIDInConfig = true
	}

	interval, err := parseDurationFlag("--interval", intervalStr)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	heartbeat, err := parseDurationFlag("heartbeat", heartbeatStr)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	staleTTL, err := parseDurationFlag("--stale-ttl", staleTTLStr)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	tickTimeout, err := parsePositiveDurationFlag("--tick-timeout", tickTimeoutStr)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	// Skip on check/doctor/print-config: those paths should stay signal, not config lectures.
	if stateRepo != "" && !checkMode && !doctorMode && !printConfig {
		warnStaleTTLMismatch(heartbeat, staleTTL)
	}

	if stateRepo != "" {
		if abs, err := filepath.Abs(stateRepo); err == nil {
			stateRepo = abs
		}
	}

	if printConfig {
		configExists := configPath != "" && ConfigFileExists(configPath)
		printConfigToStdout(configPath, configExists, resolved, machineID, intervalStr, heartbeatStr, staleTTLStr, maxWorkers, tickTimeoutStr, flagSet["tick-timeout"])
		return
	}

	// Path-scoped pre-flight: no scan-root required.
	if checkMode {
		os.Exit(runCheckMode(context.Background(), checkPath, machineID, stateRepo, skipRemote, staleTTL, jsonOutput))
	}

	var rootDir string
	if resolved.ScanRoot != "" {
		rootDir = resolved.ScanRoot
		if abs, err := filepath.Abs(rootDir); err == nil {
			rootDir = abs
		}
	}

	if doctorMode {
		os.Exit(runDoctorMode(DoctorInput{
			ConfigPath:   configPath,
			ConfigExists: configPath != "" && ConfigFileExists(configPath),
			Resolved:     resolved,
			MachineID:    machineID,
			StateRepo:    stateRepo,
			ScanRoot:     rootDir,
			Interval:     intervalStr,
			Heartbeat:    heartbeatStr,
			StaleTTL:     staleTTLStr,
			StaleTTLDur:  staleTTL,
			MaxWorkers:   maxWorkers,
			TickTimeout:  tickTimeoutStr,
		}))
	}

	if rootDir == "" {
		printUsage()
		os.Exit(1)
	}

	if installSched {
		requireStateRepo(stateRepo, "install-scheduler")
		validateStateRepoOrExit(stateRepo)
		if configPath != "" {
			sticky := stickyConfigFromRun(stateRepo, rootDir, machineID, intervalStr, heartbeatStr, staleTTLStr, redactPaths, gitexec.ResolvedMaxWorkers(maxWorkers))
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
		snapPath, err := smokePublishOnce(newAgentConfig(rootDir, stateRepo, machineID, interval, tickTimeout, maxWorkers, redactPaths, heartbeat, false))
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: smoke publish failed (scheduler not installed): %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Smoke publish OK: %s\n", snapPath)
		if err := installScheduler(exe); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("Migration: re-run install-scheduler after upgrading so the unit/task uses soft command agent.")
		return
	}

	if agentMode {
		// On Windows, Task Scheduler allocates a console for this binary; detach
		// when we own it so auto-run stays invisible. Interactive terminals are kept.
		detachAgentConsoleIfOwned()
		requireStateRepo(stateRepo, "agent")
		validateStateRepoOrExit(stateRepo)
		if flagSet["state-repo"] && configPath != "" {
			if err := EnsureConfigFromAgent(configPath, stateRepo, rootDir, machineID, intervalStr, heartbeatStr, staleTTLStr, redactPaths); err != nil {
				fmt.Fprintf(os.Stderr, "warning: could not write sticky config: %v\n", err)
			}
		}
		if persistMachineIDInConfig && configPath != "" {
			if err := EnsureMachineIDInConfig(configPath, machineID); err != nil {
				fmt.Fprintf(os.Stderr, "warning: could not persist machine_id: %v\n", err)
			}
		}
		printPrivacyNotice()
		err := RunAgentLoop(newAgentConfig(rootDir, stateRepo, machineID, interval, tickTimeout, maxWorkers, redactPaths, heartbeat, false))
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
	results := checkRepoStatuses(scanCtx, repos, dirtyOnly && !willTryRemote, maxWorkers)

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
		// Try on-disk snapshots even when pull fails.
		remote, remoteOK = loadRemoteSnapshots(scanCtx, stateRepo, machineID, staleTTL)
	}

	if len(results) == 0 && len(remote) == 0 {
		return
	}

	// Always build aggregate rows so the default Project × Machine matrix (and
	// optional path inventory) share one code path for local-only and remote.
	rows := BuildAggregateRows(machineID, results, remote)
	situations := DetectSituations(rows)
	if dirtyOnly {
		keys := ProjectKeysWithSituations(situations)
		rows = FilterRowsByProjectKeys(rows, keys)
		situations = DetectSituations(rows)
	}

	if showInventory {
		// Opt-in audit trail: Attention + path table (former default).
		DisplayAttention(situations)
		fmt.Println("Full inventory:")
		displayAggregateTable(rows, false) // already situation-filtered when dirty-only
	} else {
		// Default morning view: matrix only (no Attention + inventory stack).
		displayProjectMachineMatrix(rows)
	}
	if remoteOK {
		printStaleMachineSummary(remote, staleTTL)
	}

	if outputFile != "" {
		if err := exportAggregateToCSV(rows, outputFile, false); err != nil {
			fmt.Printf("Error saving to CSV: %v\n", err)
		} else {
			fmt.Printf("Results saved to: %s\n", outputFile)
		}
	}

	printAggregateSummary(rows, dirtyOnly)
}

func printUsage() {
	fmt.Println("Usage: find-uncommitted [flags] [directory_to_scan]")
	fmt.Println("       find-uncommitted [flags] check [--json] <path>")
	fmt.Println("       find-uncommitted [flags] doctor")
	fmt.Println("       find-uncommitted [flags] agent [directory_to_scan]")
	fmt.Println("       find-uncommitted [flags] install-scheduler [directory_to_scan]")
	fmt.Println("       find-uncommitted [flags] uninstall-scheduler")
	fmt.Println()
	fmt.Println("Examples:")
	fmt.Println("  find-uncommitted C:\\code")
	fmt.Println("  find-uncommitted --dirty-only --output results.csv C:\\code")
	fmt.Println("  find-uncommitted --inventory C:\\code")
	fmt.Println("  find-uncommitted --state-repo D:\\state-repo C:\\code")
	fmt.Println("  find-uncommitted --state-repo D:\\state-repo --interval 2m agent C:\\code")
	fmt.Println("  find-uncommitted --state-repo D:\\state-repo install-scheduler C:\\code")
	fmt.Println("  find-uncommitted uninstall-scheduler")
	fmt.Println("  find-uncommitted check ~/repos/work-project")
	fmt.Println("  find-uncommitted --json check .")
	fmt.Println("  find-uncommitted --no-remote check .")
	fmt.Println("  find-uncommitted --print-config")
	fmt.Println("  find-uncommitted doctor")
	fmt.Println()
	fmt.Println("Commands:")
	fmt.Println("  (default)             Interactive tree scan → Project x Machine matrix")
	fmt.Println("  check [--json] <path> Pre-flight one repo (exit 0=ok, 2=attention, 1=error)")
	fmt.Println("  doctor                Operability report (config, locks, scheduler, state repo)")
	fmt.Println("  agent                 Background publish loop (needs state_repo + scan root)")
	fmt.Println("  install-scheduler     OS autostart + sticky config + smoke publish")
	fmt.Println("  uninstall-scheduler   Remove OS autostart registration")
	fmt.Println()
	fmt.Println("--inventory / --verbose  Path-centric Full inventory + Attention (former default).")
	fmt.Println("  --json  With check: machine-readable JSON on stdout (schemaVersion 1); warnings stay on stderr.")
	fmt.Println("--print-config  Print resolved settings with sources (flag/env/config/default) and exit.")
	fmt.Println("After install-scheduler, sticky config enables aggregate remotes on bare scans.")
	fmt.Println("Install smoke-publishes one snapshot so you can confirm the state repo works.")
	fmt.Println("Scan root may come from config when the directory argument is omitted.")
	fmt.Println("Re-run install-scheduler after upgrading so OS units invoke soft command agent (not --agent).")
	fmt.Println("Cross-machine sync requires a private Git repository. See README for privacy notes.")
	fmt.Println()
	fmt.Println("Flags:")
	flag.PrintDefaults()
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
		fmt.Println("  - systemd user session; for headless use loginctl enable-linger $USER (per-user: all enabled user services can stay up)")
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

func findGitRepos(rootDir string, excludeRepos ...string) []string {
	return findGitReposContext(context.Background(), rootDir, excludeRepos...)
}

func findGitReposContext(ctx context.Context, rootDir string, excludeRepos ...string) []string {
	return discover.FindGitRepos(rootDir, discover.WalkOptions{
		Debug:    debugMode,
		Excludes: excludeRepos,
		Context:  ctx,
	})
}

func gitCancelledError(err error) string {
	return fmt.Sprintf("git timed out or cancelled: %v", err)
}

func setGitCancelled(ctx context.Context, status *RepoSnapshot, err error) bool {
	if gitexec.IsContextErr(ctx, err) {
		status.Error = gitCancelledError(err)
		return true
	}
	return false
}

// gitWorkingTreeNonempty runs a git command whose nonempty stdout means a working-tree signal.
// ok is false when the status check should abort (cancel or hard error already recorded).
func gitWorkingTreeNonempty(ctx context.Context, repoPath string, status *RepoSnapshot, failPrefix, shortMsg string, args ...string) (ok, nonempty bool) {
	out, stderr, err := gitexec.Run(ctx, repoPath, args...)
	if err != nil {
		if setGitCancelled(ctx, status, err) {
			return false, false
		}
		appendRepoCheckError(status, stderr, err, failPrefix, shortMsg)
		return false, false
	}
	return true, len(strings.TrimSpace(out)) > 0
}

func checkRepoStatus(ctx context.Context, repoPath string) RepoSnapshot {
	status := RepoSnapshot{
		Path: repoPath,
	}

	if err := ctx.Err(); err != nil {
		status.Error = fmt.Sprintf("git check cancelled: %v", err)
		return status
	}

	// First check if this is a valid git repository
	_, stderr, err := gitexec.Run(ctx, repoPath, "rev-parse", "--git-dir")
	if err != nil {
		if setGitCancelled(ctx, &status, err) {
			return status
		}
		// Check if it's a dubious ownership error
		if strings.Contains(stderr, "dubious ownership") {
			status.Error = "Git ownership issue - run: git config --global --add safe.directory " + strings.ReplaceAll(repoPath, "\\", "/")
			return status
		}
		status.Error = invalidRepositoryError(stderr, err)
		return status
	}

	// Get current branch
	branch, stderr, err := gitexec.Run(ctx, repoPath, "branch", "--show-current")
	if err != nil {
		if setGitCancelled(ctx, &status, err) {
			return status
		}
		// Check if it's a detached HEAD state (exit code 1)
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			// Try to get the commit hash instead
			commit, _, commitErr := gitexec.Run(ctx, repoPath, "rev-parse", fmt.Sprintf("--short=%d", shortHeadSHALen), "HEAD")
			if commitErr == nil {
				status.Branch = fmt.Sprintf("detached HEAD (%s)", strings.TrimSpace(commit))
			} else if setGitCancelled(ctx, &status, commitErr) {
				return status
			} else {
				status.Branch = "detached HEAD"
				status.Error = fmt.Sprintf("Branch issue: %s", gitexec.FormatError(stderr, err))
			}
		} else {
			status.Branch = "unknown"
			status.Error = fmt.Sprintf("Branch issue: %s", gitexec.FormatError(stderr, err))
		}
		// Don't return here, continue checking other status
	} else {
		status.Branch = strings.TrimSpace(branch)
	}

	status.IsEmpty = repoIsEmpty(ctx, repoPath)

	// Capture normalized origin for cross-machine project correlation.
	status.Origin = repoOriginURL(ctx, repoPath)

	ok, nonempty := gitWorkingTreeNonempty(ctx, repoPath, &status, "Failed to check unstaged changes", "unstaged check failed", "diff", "--name-only")
	if !ok {
		return status
	}
	status.HasUnstaged = nonempty

	ok, nonempty = gitWorkingTreeNonempty(ctx, repoPath, &status, "Failed to check staged changes", "staged check failed", "diff", "--cached", "--name-only")
	if !ok {
		return status
	}
	status.HasStaged = nonempty

	ok, nonempty = gitWorkingTreeNonempty(ctx, repoPath, &status, "Failed to check untracked files", "untracked check failed", "ls-files", "--others", "--exclude-standard")
	if !ok {
		return status
	}
	status.HasUntracked = nonempty

	if status.IsEmpty {
		status.HeadSHA = ""
	} else {
		status.HeadSHA = shortHeadSHA(ctx, repoPath)
	}

	// Upstream tracking (skip for empty repositories).
	if !status.IsEmpty {
		_, upStderr, upstreamErr := gitexec.Run(ctx, repoPath, "rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{u}")
		if upstreamErr != nil {
			if setGitCancelled(ctx, &status, upstreamErr) {
				return status
			}
			// Empty-repo detection already ran via repoIsEmpty (HEAD). Do not
			// reclassify upstream "unknown revision" (e.g. deleted @{u}) as empty.
			untrackedUpstream, repoErr := classifyUpstreamFailure(upStderr, upstreamErr)
			if repoErr != "" {
				status.Error = repoErr
			} else if untrackedUpstream {
				status.HasUntrackedUpstream = true
			}
		} else {
			// Ahead/behind against cached upstream refs only (no fetch).
			fillAheadBehind(ctx, &status, repoPath)
		}
	}

	// Dirty means working tree changes only.
	status.IsDirty = status.HasUnstaged || status.HasStaged || status.HasUntracked
	status.IsClean = status.Error == "" && !status.IsDirty && !status.HasUnpushed && !status.HasBehind && !status.HasUntrackedUpstream

	return status
}

// fillAheadBehind sets AheadCount/BehindCount (and flags) from rev-list.
// Failures set Error — never treat unknown counts as zero/clean.
func fillAheadBehind(ctx context.Context, status *RepoSnapshot, repoPath string) {
	if n, stderr, err := revListCount(ctx, repoPath, "@{u}..HEAD"); err != nil {
		if setGitCancelled(ctx, status, err) {
			return
		}
		appendRepoCheckError(status, stderr, err, "Failed to count commits ahead of upstream", "ahead count failed")
	} else {
		status.AheadCount = n
		status.HasUnpushed = n > 0
	}
	if n, stderr, err := revListCount(ctx, repoPath, "HEAD..@{u}"); err != nil {
		if setGitCancelled(ctx, status, err) {
			return
		}
		appendRepoCheckError(status, stderr, err, "Failed to count commits behind upstream", "behind count failed")
	} else {
		status.BehindCount = n
		status.HasBehind = n > 0
	}
}

// revListCount runs `git rev-list --count <range>`. On failure it returns err
// (and stderr) so callers can surface unknown ahead/behind instead of 0.
func revListCount(ctx context.Context, repoPath, revRange string) (count int, stderr string, err error) {
	out, stderr, err := gitexec.Run(ctx, repoPath, "rev-list", "--count", revRange)
	if err != nil {
		if debugMode {
			fmt.Printf("[DEBUG] Failed rev-list %s in %s: %v\n", revRange, repoPath, err)
		}
		return 0, stderr, err
	}
	count, err = strconv.Atoi(strings.TrimSpace(out))
	if err != nil {
		if debugMode {
			fmt.Printf("[DEBUG] Failed to parse rev-list count in %s: %v\n", repoPath, err)
		}
		return 0, "", fmt.Errorf("unexpected rev-list output %q: %w", out, err)
	}
	return count, "", nil
}

// shortHeadSHALen is the fixed abbrev length for published HeadSHA values.
// Using an explicit length (not bare --short) avoids core.abbrev mismatches
// across machines that would otherwise look like tip_mismatch.
const shortHeadSHALen = 12

// shortHeadSHA returns a fixed-length short HEAD commit hash, or empty when unavailable.
func shortHeadSHA(ctx context.Context, repoPath string) string {
	out, _, err := gitexec.Run(ctx, repoPath, "rev-parse", fmt.Sprintf("--short=%d", shortHeadSHALen), "HEAD")
	if err != nil {
		return ""
	}
	return strings.TrimSpace(out)
}
