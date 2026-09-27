package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	exitDoctorOK   = 0
	exitDoctorFail = 1
)

// doctorReport is the structured result of an operability check.
type doctorReport struct {
	lines  []string
	failed bool
}

func (r *doctorReport) ok(format string, args ...any) {
	r.lines = append(r.lines, "OK   "+fmt.Sprintf(format, args...))
}

func (r *doctorReport) warn(format string, args ...any) {
	r.lines = append(r.lines, "WARN "+fmt.Sprintf(format, args...))
}

func (r *doctorReport) fail(format string, args ...any) {
	r.failed = true
	r.lines = append(r.lines, "FAIL "+fmt.Sprintf(format, args...))
}

func (r *doctorReport) info(format string, args ...any) {
	r.lines = append(r.lines, "     "+fmt.Sprintf(format, args...))
}

func (r *doctorReport) write(w io.Writer) {
	for _, line := range r.lines {
		fmt.Fprintln(w, line)
	}
}

// DoctorInput carries resolved runtime values for the doctor command.
type DoctorInput struct {
	ConfigPath   string
	ConfigExists bool
	Resolved     ResolvedSettings
	MachineID    string
	StateRepo    string
	ScanRoot     string
	Interval     string
	Heartbeat    string
	StaleTTL     string
	StaleTTLDur  time.Duration
	MaxWorkers   int
	TickTimeout  string
	Now          time.Time
}

// SchedulerStatus is the OS scheduler registration/health probe result.
type SchedulerStatus struct {
	Supported        bool
	Installed        bool
	Running          bool
	Name             string
	Detail           string
	NotRunningIsFail bool // true for always-on user services (systemd); false for at-logon tasks
}

// runDoctorMode inspects config, locks, last publish, scheduler, and state-repo health.
// Exit 0 when no FAIL lines; exit 1 when any check failed.
func runDoctorMode(in DoctorInput) int {
	if in.Now.IsZero() {
		in.Now = time.Now().UTC()
	}
	report := collectDoctorReport(context.Background(), in)
	report.write(os.Stdout)
	if report.failed {
		return exitDoctorFail
	}
	return exitDoctorOK
}

func collectDoctorReport(ctx context.Context, in DoctorInput) doctorReport {
	var r doctorReport

	switch {
	case in.ConfigPath == "":
		r.warn("config path unavailable")
	case in.ConfigExists:
		r.ok("config path: %s", in.ConfigPath)
	default:
		r.warn("config path missing: %s (run --install-scheduler or create sticky config)", in.ConfigPath)
	}

	r.info("machine_id=%s (%s)", in.MachineID, formatConfigSource(in.Resolved.MachineIDSource, "hostname"))
	if in.ScanRoot != "" {
		r.info("scan_root=%s (%s)", in.ScanRoot, formatConfigSource(in.Resolved.ScanRootSource, "default"))
	} else {
		r.warn("scan_root unset (interactive scan and agent need a directory or sticky scan_root)")
	}
	r.info("interval=%s heartbeat=%s stale_ttl=%s tick_timeout=%s max_workers=%d",
		in.Interval, in.Heartbeat, in.StaleTTL, in.TickTimeout, resolvedMaxWorkers(in.MaxWorkers))

	if in.StateRepo == "" {
		r.warn("state_repo unset (local-only mode; cross-machine sync disabled)")
	} else {
		if err := validateStateRepo(in.StateRepo); err != nil {
			r.fail("state_repo invalid: %v", err)
		} else {
			r.ok("state_repo: %s", in.StateRepo)
			checkStateRepoRemote(ctx, &r, in.StateRepo)
			checkSyncLock(&r, in.StateRepo)
			checkLastPublish(&r, in)
		}
	}

	checkAgentLock(&r, in.MachineID)
	checkScheduler(&r)

	hb := mustParseDoctorDuration(in.Heartbeat)
	if in.StateRepo != "" && staleTTLTooShort(hb, in.StaleTTLDur) {
		r.warn("stale_ttl (%s) < 2× heartbeat (%s); healthy machines may appear stale", in.StaleTTL, in.Heartbeat)
	}

	return r
}

func mustParseDoctorDuration(s string) time.Duration {
	d, err := time.ParseDuration(s)
	if err != nil {
		return 0
	}
	return d
}

func checkStateRepoRemote(ctx context.Context, r *doctorReport, stateRepo string) {
	out, stderr, err := runGit(ctx, stateRepo, "remote", "get-url", "origin")
	if err != nil {
		detail := strings.TrimSpace(stderr)
		if detail == "" {
			detail = err.Error()
		}
		r.warn("state_repo has no usable origin remote (%s)", detail)
		return
	}
	r.ok("state_repo origin: %s", strings.TrimSpace(out))
}

func checkSyncLock(r *doctorReport, stateRepo string) {
	lock, err := tryAcquireStateRepoSyncLock(stateRepo)
	if err != nil {
		if errors.Is(err, ErrStateRepoBusy) {
			r.ok("state-repo sync lock: held (agent or CLI sync in progress)")
			return
		}
		r.warn("state-repo sync lock: %v", err)
		return
	}
	lock.Release()
	r.ok("state-repo sync lock: free")
}

func checkLastPublish(r *doctorReport, in DoctorInput) {
	path := SnapshotFilePath(in.StateRepo, in.MachineID)
	snap, err := ReadMachineSnapshot(path)
	if err != nil {
		if os.IsNotExist(err) {
			r.warn("last publish: no snapshot yet for this machine (%s)", path)
			return
		}
		r.warn("last publish: %v", err)
		return
	}
	age := in.Now.Sub(snap.UpdatedAt).Truncate(time.Second)
	line := fmt.Sprintf("last publish: %s (%s ago, %d repos)",
		snap.UpdatedAt.Format(time.RFC3339), age, len(snap.Repos))
	if in.StaleTTLDur > 0 && in.Now.Sub(snap.UpdatedAt) > in.StaleTTLDur {
		r.warn("%s — stale vs stale_ttl %s", line, in.StaleTTL)
		return
	}
	r.ok("%s", line)
}

func checkAgentLock(r *doctorReport, machineID string) {
	path := lockPathFor("", machineID)
	f, err := os.OpenFile(path, os.O_RDWR, 0o644)
	if err != nil {
		if os.IsNotExist(err) {
			r.info("agent lock: not held (%s)", path)
			return
		}
		r.warn("agent lock: %v", err)
		return
	}
	defer f.Close()

	pid := readLockPID(f)
	if lockErr := lockFileExclusive(f); lockErr != nil {
		if lockWouldBlock(lockErr) {
			if pid > 0 {
				r.ok("agent lock: held by pid %d (%s)", pid, path)
			} else {
				r.ok("agent lock: held (%s)", path)
			}
			return
		}
		r.warn("agent lock: %v", lockErr)
		return
	}
	_ = unlockFile(f)
	if pid > 0 {
		r.warn("agent lock file present (stale pid %d?) but lock is free: %s", pid, path)
		return
	}
	r.info("agent lock: not held (%s)", path)
}

func readLockPID(f *os.File) int {
	data, err := io.ReadAll(f)
	if err != nil {
		return 0
	}
	_, _ = f.Seek(0, 0)
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return 0
	}
	return pid
}

// probeSchedulerStatus is the scheduler probe used by doctor; tests may override it.
var probeSchedulerStatus = schedulerStatus

func checkScheduler(r *doctorReport) {
	status, err := probeSchedulerStatus()
	if err != nil {
		r.warn("scheduler: %v", err)
		return
	}
	if !status.Supported {
		r.info("scheduler: not supported on this OS")
		return
	}
	if !status.Installed {
		r.warn("scheduler: not installed (%s)", status.Name)
		return
	}
	if status.Running {
		r.ok("scheduler: installed and running (%s)", status.Detail)
		return
	}
	msg := fmt.Sprintf("scheduler: installed but not running (%s)", status.Detail)
	if status.NotRunningIsFail {
		r.fail("%s", msg)
		return
	}
	r.warn("%s", msg)
}
