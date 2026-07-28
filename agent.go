package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"sync"
	"syscall"
	"time"
)

// AgentConfig configures the autonomous publish loop.
type AgentConfig struct {
	ScanRoot     string
	StateRepoDir string
	MachineID    string
	Interval     time.Duration
	RedactPaths  bool
	DirtyOnly    bool
	Sync         SyncConfig
	LockPath     string
}

// DefaultAgentInterval is the default publish cadence.
const DefaultAgentInterval = 30 * time.Second

// DefaultStaleTTL marks remote snapshots stale when older than this.
const DefaultStaleTTL = 5 * time.Minute

// agentLock holds an exclusive flock-style lock file for the agent process.
type agentLock struct {
	file *os.File
	path string
}

func lockPathFor(stateRepoDir, machineID string) string {
	base := filepath.Join(os.TempDir(), "find-uncommitted")
	_ = os.MkdirAll(base, 0o755)
	return filepath.Join(base, sanitizeMachineID(machineID)+".agent.lock")
}

func acquireAgentLock(path string) (*agentLock, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("create lock directory: %w", err)
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return nil, fmt.Errorf("open lock file: %w", err)
	}
	if err := lockFileExclusive(f); err != nil {
		_ = f.Close()
		return nil, fmt.Errorf("another agent appears to be running (lock %s): %w", path, err)
	}
	_, _ = f.Seek(0, 0)
	_ = f.Truncate(0)
	_, _ = fmt.Fprintf(f, "%d\n", os.Getpid())
	return &agentLock{file: f, path: path}, nil
}

func (l *agentLock) Release() {
	if l == nil || l.file == nil {
		return
	}
	_ = unlockFile(l.file)
	_ = l.file.Close()
	_ = os.Remove(l.path)
}

// RunAgentLoop publishes snapshots on an interval until interrupted.
func RunAgentLoop(cfg AgentConfig) error {
	if cfg.Interval <= 0 {
		cfg.Interval = DefaultAgentInterval
	}
	if cfg.LockPath == "" {
		cfg.LockPath = lockPathFor(cfg.StateRepoDir, cfg.MachineID)
	}
	cfg.Sync.StateRepoDir = cfg.StateRepoDir
	cfg.Sync.MachineID = cfg.MachineID

	lock, err := acquireAgentLock(cfg.LockPath)
	if err != nil {
		return err
	}
	defer lock.Release()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	fmt.Printf("Agent started for machine %q (interval %s, state repo %s)\n",
		cfg.MachineID, cfg.Interval, cfg.StateRepoDir)

	// Immediate publish on startup, then wait between ticks.
	for {
		runAgentTick(cfg)
		select {
		case <-ctx.Done():
			fmt.Println("Agent stopped.")
			return nil
		case <-time.After(cfg.Interval):
		}
	}
}

func runAgentTick(cfg AgentConfig) {
	warn := func(format string, args ...any) {
		fmt.Fprintf(os.Stderr, "warning: "+format+"\n", args...)
	}

	if err := PullStateRepo(cfg.Sync); err != nil {
		warn("%v", err)
		// Continue: local write still useful even if pull failed.
	}

	snap, committed, err := publishAgentSnapshot(cfg)
	if err != nil {
		warn("%v", err)
		return
	}
	if debugMode {
		if committed {
			fmt.Printf("[DEBUG] published/pushed snapshot (%d repos)\n", len(snap.Repos))
		} else {
			fmt.Printf("[DEBUG] snapshot unchanged and in sync\n")
		}
	} else if committed {
		fmt.Printf("Published snapshot at %s (%d repos)\n",
			snap.UpdatedAt.Format(time.RFC3339), len(snap.Repos))
	}
}

// smokePublishOnce runs one scan+publish for --install-scheduler verification.
// Returns the on-disk snapshot path on success so install can print proof the file landed.
func smokePublishOnce(cfg AgentConfig) (string, error) {
	cfg.Sync.StateRepoDir = cfg.StateRepoDir
	cfg.Sync.MachineID = cfg.MachineID

	if err := PullStateRepo(cfg.Sync); err != nil {
		// Pull may fail offline; still require a successful local publish.
		fmt.Fprintf(os.Stderr, "warning: %v\n", err)
	}
	if _, _, err := publishAgentSnapshot(cfg); err != nil {
		return "", err
	}
	return SnapshotFilePath(cfg.StateRepoDir, cfg.MachineID), nil
}

// publishAgentSnapshot scans and publishes one machine snapshot.
func publishAgentSnapshot(cfg AgentConfig) (MachineSnapshot, bool, error) {
	started := time.Now()
	repos := findGitRepos(cfg.ScanRoot, cfg.StateRepoDir)
	results := checkRepoStatuses(repos, cfg.DirtyOnly)
	snap := BuildMachineSnapshot(cfg.MachineID, cfg.ScanRoot, results, started, cfg.RedactPaths)
	committed, err := PublishLocalSnapshot(cfg.Sync, snap)
	return snap, committed, err
}

// checkRepoStatuses checks each repo path concurrently and optionally filters clean repos.
func checkRepoStatuses(repos []string, dirtyOnlyFilter bool) []RepoStatus {
	if len(repos) == 0 {
		return nil
	}

	maxWorkers := runtime.NumCPU() * 4
	if maxWorkers < 4 {
		maxWorkers = 4
	}
	if maxWorkers > len(repos) {
		maxWorkers = len(repos)
	}

	jobs := make(chan string)
	statusChan := make(chan RepoStatus, len(repos))
	var wg sync.WaitGroup

	for i := 0; i < maxWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for repoPath := range jobs {
				statusChan <- checkRepoStatus(repoPath)
			}
		}()
	}

	for _, repo := range repos {
		jobs <- repo
	}
	close(jobs)

	go func() {
		wg.Wait()
		close(statusChan)
	}()

	var results []RepoStatus
	for status := range statusChan {
		if dirtyOnlyFilter && !repoNeedsAttention(status) {
			continue
		}
		results = append(results, status)
	}
	return results
}

func repoNeedsAttention(status RepoStatus) bool {
	return status.Error != "" || status.IsDirty || status.HasUnpushed || status.HasBehind || status.HasUntrackedUpstream
}

func snapshotNeedsAttention(repo RepoSnapshot) bool {
	return repo.Error != "" || repo.IsDirty || repo.HasUnpushed || repo.HasBehind || repo.HasUntrackedUpstream
}
