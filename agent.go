package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/davidshq/find-uncommitted/internal/gitexec"
)

// AgentConfig configures the autonomous publish loop.
type AgentConfig struct {
	ScanRoot     string
	StateRepoDir string
	MachineID    string
	Interval     time.Duration
	TickTimeout  time.Duration
	MaxWorkers   int
	RedactPaths  bool
	DirtyOnly    bool
	Sync         SyncConfig
	LockPath     string
}

// DefaultAgentInterval / DefaultStaleTTL / DefaultHeartbeat and their string
// forms live in duration.go so flags and runtime share one source of truth.

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

func (cfg AgentConfig) tickTimeout() time.Duration {
	if cfg.TickTimeout > 0 {
		return cfg.TickTimeout
	}
	return DefaultAgentTickTimeout
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

	fmt.Printf("Agent started for machine %q (check interval %s, heartbeat %s, tick timeout %s, state repo %s)\n",
		cfg.MachineID, cfg.Interval, cfg.Sync.heartbeat(), cfg.tickTimeout(), cfg.StateRepoDir)

	ticker := time.NewTicker(cfg.Interval)
	defer ticker.Stop()

	var streak tickFailStreak
	// Immediate publish on startup, then wait between ticks on the ticker.
	for {
		tickCtx, cancel := context.WithTimeout(ctx, cfg.tickTimeout())
		tickErr := runAgentTick(tickCtx, cfg)
		cancel()
		if msg := streak.record(tickErr, time.Now()); msg != "" {
			fmt.Fprintln(os.Stderr, msg)
		}

		select {
		case <-ctx.Done():
			fmt.Println("Agent stopped.")
			return nil
		case <-ticker.C:
		}
	}
}

// tickFailStreak escalates consecutive failed ticks. Each failure already logs
// a warning, but a deterministic failure looks identical to transient noise:
// XPS logged the same "will retry later" pair 1,861 times over 31h unnoticed.
type tickFailStreak struct {
	count int
	since time.Time
}

const (
	agentStuckEscalateAfter = 5  // consecutive failed ticks before the first ERROR
	agentStuckRepeatEvery   = 60 // then repeat every N failed ticks
)

// record returns a line to log (or "") for this tick's outcome.
func (s *tickFailStreak) record(err error, now time.Time) string {
	if err == nil {
		if s.count == 0 {
			return ""
		}
		msg := fmt.Sprintf("agent recovered after %d consecutive failed ticks (since %s)",
			s.count, s.since.Format(time.RFC3339))
		*s = tickFailStreak{}
		return msg
	}
	if s.count == 0 {
		s.since = now
	}
	s.count++
	if s.count < agentStuckEscalateAfter || (s.count-agentStuckEscalateAfter)%agentStuckRepeatEvery != 0 {
		return ""
	}
	return fmt.Sprintf("ERROR: agent has failed %d consecutive ticks since %s; peers see a stale snapshot for this machine. "+
		"Last error: %v. Run `find-uncommitted doctor`; recovery runbook: docs/plan-state-repo-publish-snowball.md#recovery-runbook",
		s.count, s.since.Format(time.RFC3339), oneLine(err.Error(), 300))
}

// oneLine flattens multi-line git stderr for a single log line. Git lists the
// offending paths on the lines after "would be overwritten by checkout:", so
// truncating at the first newline would drop exactly the actionable part.
func oneLine(s string, max int) string {
	s = strings.Join(strings.Fields(s), " ")
	if r := []rune(s); len(r) > max {
		return string(r[:max]) + " …"
	}
	return s
}

// runAgentTick scans and publishes once. Returns the error that stopped the
// publish (already logged), or nil when the tick published or had nothing to do.
func runAgentTick(ctx context.Context, cfg AgentConfig) error {
	warn := func(format string, args ...any) {
		fmt.Fprintf(os.Stderr, "warning: "+format+"\n", args...)
	}

	lock, err := acquireStateRepoSyncLockBlocking(ctx, cfg.StateRepoDir)
	if err != nil {
		warn("%v", err)
		return err
	}
	defer lock.Release()

	if err := pullStateRepoLocked(ctx, cfg.Sync); err != nil {
		warn("%v", err)
		// Only abort when the tick/parent context is done. A nested per-command
		// timeout is treated like any other pull failure: keep going so a local
		// snapshot write can still land.
		if err := ctx.Err(); err != nil {
			warn("agent tick aborted: %v", err)
			return err
		}
		// Continue: local write still useful even if pull failed.
	}

	snap, committed, err := publishAgentSnapshot(ctx, cfg)
	if err != nil {
		warn("%v", err)
		if err := ctx.Err(); err != nil {
			warn("agent tick aborted: %v", err)
		}
		return err
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
	return nil
}

// smokePublishOnce runs one scan+publish for install-scheduler verification.
// Returns the on-disk snapshot path on success so install can print proof the file landed.
func smokePublishOnce(cfg AgentConfig) (string, error) {
	cfg.Sync.StateRepoDir = cfg.StateRepoDir
	cfg.Sync.MachineID = cfg.MachineID

	ctx, cancel := context.WithTimeout(context.Background(), cfg.tickTimeout())
	defer cancel()

	lock, err := acquireStateRepoSyncLockBlocking(ctx, cfg.StateRepoDir)
	if err != nil {
		return "", err
	}
	defer lock.Release()

	if err := pullStateRepoLocked(ctx, cfg.Sync); err != nil {
		// Pull may fail offline; still require a successful local publish.
		fmt.Fprintf(os.Stderr, "warning: %v\n", err)
	}
	if _, _, err := publishAgentSnapshot(ctx, cfg); err != nil {
		return "", err
	}
	return SnapshotFilePath(cfg.StateRepoDir, cfg.MachineID), nil
}

// publishAgentSnapshot scans and publishes one machine snapshot.
func publishAgentSnapshot(ctx context.Context, cfg AgentConfig) (MachineSnapshot, bool, error) {
	started := time.Now()
	repos, err := findGitReposContext(ctx, cfg.ScanRoot, cfg.StateRepoDir)
	if err != nil {
		// A missing/unmounted root must not publish an empty "all clear"
		// snapshot; keep the last good one so peers see it age instead.
		return MachineSnapshot{}, false, fmt.Errorf("agent scan skipped, not publishing: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return MachineSnapshot{}, false, fmt.Errorf("agent scan cancelled: %w", err)
	}
	results := checkRepoStatuses(ctx, repos, cfg.DirtyOnly, cfg.MaxWorkers)
	if err := ctx.Err(); err != nil {
		return MachineSnapshot{}, false, fmt.Errorf("agent scan cancelled: %w", err)
	}
	snap := BuildMachineSnapshot(cfg.MachineID, cfg.ScanRoot, results, started, cfg.RedactPaths)
	committed, err := PublishLocalSnapshot(ctx, cfg.Sync, snap)
	return snap, committed, err
}

// checkRepoStatuses checks each repo path concurrently and optionally filters clean repos.
// When ctx is cancelled, no further repos are scheduled so a timed-out agent tick
// does not keep spawning git for the rest of a large scan root.
func checkRepoStatuses(ctx context.Context, repos []string, dirtyOnlyFilter bool, maxWorkers int) []RepoSnapshot {
	if len(repos) == 0 {
		return nil
	}

	maxWorkers = gitexec.RepoCheckWorkerCount(maxWorkers, len(repos))

	jobs := make(chan string)
	statusChan := make(chan RepoSnapshot, len(repos))
	var wg sync.WaitGroup

	for i := 0; i < maxWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for repoPath := range jobs {
				if ctx.Err() != nil {
					// Skip without calling checkRepoStatus so cancelled ticks do not
					// start more git processes for already-queued paths.
					continue
				}
				statusChan <- checkRepoStatus(ctx, repoPath)
			}
		}()
	}

feed:
	for _, repo := range repos {
		select {
		case <-ctx.Done():
			break feed
		case jobs <- repo:
		}
	}
	close(jobs)

	go func() {
		wg.Wait()
		close(statusChan)
	}()

	var results []RepoSnapshot
	for status := range statusChan {
		if dirtyOnlyFilter && !snapshotNeedsAttention(status) {
			continue
		}
		results = append(results, status)
	}
	sort.Slice(results, func(i, j int) bool {
		return results[i].Path < results[j].Path
	})
	return results
}
