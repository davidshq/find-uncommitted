# Find Uncommitted

A Go application that scans your hard drive for git repositories and reports on their status - whether there are unstaged, staged, or untracked files that need to be committed.

## Features

- 🔍 **Recursive scanning**: Automatically finds all git repositories in the specified directory
- ⚡ **Concurrent processing**: Uses goroutines to check repository status in parallel
- 📊 **Detailed reporting**: Shows branch name, unstaged/staged/untracked changes, unpushed commits, and behind-upstream status
- 💡 **Attention nudges**: Soft suggestions (commit, push, pull, branch mismatch, other-machine work) — never auto-runs git on your repos
- ✈️ **`check <path>` pre-flight**: Compact project × machine status for one repo (scriptable exit codes + `--json`)
- 🧩 **Editor extension**: VS Code / Cursor thin client — see [vscode-extension/](vscode-extension/)
- 🚫 **Smart filtering**: Skips system directories and common build folders to improve performance
- 📈 **Summary statistics**: Provides a count of clean vs. dirty repositories
- 🎯 **Dirty-only mode**: Option to show only projects needing attention (including cross-machine situations)
- 📄 **CSV export**: Save results to a CSV file for further analysis
- 🛠️ **Ownership issue detection**: Identifies and provides guidance for Git ownership problems
- 🔧 **Debug mode**: Optional debug output for troubleshooting
- 🔄 **Cross-machine sync**: Background agent publishes per-machine snapshots to a private Git state repo and the CLI shows aggregate (with stale labeling)

## Usage

### Windows
```bash
# Scan a specific directory
./find-uncommitted.exe C:\somedirectory

# Scan the entire C: drive (may take a while)
./find-uncommitted.exe C:\

# Scan current directory
./find-uncommitted.exe .

# Show only repositories with uncommitted changes
./find-uncommitted.exe --dirty-only C:\somedirectory

# Enable debug output
./find-uncommitted.exe --debug C:\somedirectory

# Save results to CSV file
./find-uncommitted.exe --output results.csv C:\somedirectory

# Combine flags
./find-uncommitted.exe --dirty-only --output dirty-repos.csv C:\somedirectory
```

### Linux/macOS
```bash
# Scan a specific directory
./find-uncommitted /home/username/projects

# Scan the entire home directory (may take a while)
./find-uncommitted /home/username

# Scan current directory
./find-uncommitted .

# Show only repositories with uncommitted changes
./find-uncommitted --dirty-only /home/username/projects

# Enable debug output
./find-uncommitted --debug /home/username/projects

# Save results to CSV file
./find-uncommitted --output results.csv /home/username/projects

# Combine flags
./find-uncommitted --dirty-only --output dirty-repos.csv /home/username/projects
```

## Cross-machine state sync

Use a **private** Git repository as a sync bus so each machine publishes its latest known uncommitted state and any machine can display an aggregate view.

### How it works

- Each machine writes only its own file: `machines/<sanitized>-<hash>.json` (short hash of the raw machine id avoids collisions after path-unsafe characters are sanitized)
- Each repo entry includes a normalized **`origin`** URL (when configured) so the same project can be correlated across machines even when local paths differ; SSH and HTTPS remotes canonicalize to the same key
- Aggregate rows sort by that identity (origin, or path basename for local-only repos) so copies of one project land together
- Background `--agent` mode pulls, scans, writes, and push/rebases on a check interval (default **2m**)
- Each agent tick has a **2m deadline**; every individual git subprocess has a **30s** deadline (`CommandContext`). Hung credential prompts or stuck git abort the tick with a warning instead of stalling forever while the process looks healthy
- Agent git invocations set `GIT_TERMINAL_PROMPT=0` so interactive credential waits fail fast
- Interactive scans load remotes when a state repo is resolved from `--state-repo`, `FIND_UNCOMMITTED_STATE_REPO`, or sticky TOML config (unless `--no-remote`)
- Agent and interactive CLI coordinate on the state clone with a flock on `.find-uncommitted-sync.lock`; if the agent is publishing, the CLI skips `git pull` and uses on-disk snapshots
- On `--install-scheduler`, a stable `machine_id` is generated and saved when none is configured (hostname + random suffix) so cloned VMs do not silently share an id
- Snapshots older than `--stale-ttl` (default **30m**) are labeled **stale**
- Unchanged status does not create commits every tick; a **heartbeat** commit (default **15m**, sticky `heartbeat`) refreshes `updated_at` so remote views stay fresh without chatty history. Content changes still publish on the check that detects them
- Agent exits cleanly on Ctrl+C / SIGTERM (mid-tick git work is cancelled)

### Sticky config (recommended)

After `--install-scheduler`, settings are written to a user-scoped TOML file so bare scans include remotes without retyping `--state-repo`:

| Platform | Path |
|----------|------|
| Linux/macOS | `$XDG_CONFIG_HOME/find-uncommitted/config.toml` (usually `~/.config/find-uncommitted/config.toml`) |
| Windows | `%AppData%\find-uncommitted\config.toml` |

Example:

```toml
state_repo = "/path/to/uncommitted-state"
scan_root = "/path/to/repos"
machine_id = "my-laptop-a1b2"  # auto-generated on --install-scheduler if unset
interval = "2m"       # how often to check (scan + publish decision)
heartbeat = "15m"     # liveness commit when status is unchanged
stale_ttl = "30m"     # mark remote snapshots stale after this (keep ≈ 2× heartbeat)
redact_paths = false
max_workers = 8       # parallel repo checks (default 8)
```

To restore the older aggressive profile (frequent checks + chatty heartbeats):

```toml
interval = "30s"
heartbeat = "2m"
stale_ttl = "5m"
```

**Precedence:** CLI flags > environment variables > config file > built-in defaults.

Useful env vars: `FIND_UNCOMMITTED_STATE_REPO`, `FIND_UNCOMMITTED_SCAN_ROOT`, `FIND_UNCOMMITTED_MACHINE_ID`, `FIND_UNCOMMITTED_INTERVAL`, `FIND_UNCOMMITTED_HEARTBEAT`, `FIND_UNCOMMITTED_STALE_TTL`, `FIND_UNCOMMITTED_REDACT_PATHS`, `FIND_UNCOMMITTED_MAX_WORKERS`.

When config supplies `state_repo`, the CLI prints a short stderr notice and aggregates remotes. Use `--no-remote` for a local-only scan. If the configured state clone path is missing or invalid, the scan **exits with an error** (so a bad sticky config cannot silently look like a local-only machine). If the clone is valid but offline/`git pull` fails, the tool warns and still shows local results plus any on-disk snapshots. Corrupt individual snapshot JSON files are skipped with a stderr warning; valid siblings still appear in the aggregate.

`--install-scheduler` runs a one-shot **smoke publish** before registering the OS scheduler, then prints the snapshot path so you can confirm a file landed in the state repo.

**Migration:** If you installed the scheduler before sticky config existed, re-run `--install-scheduler` once (or create the TOML file manually). Until then, interactive scans stay local-only unless you pass `--state-repo`.

If your sticky config still has `stale_ttl = "5m"` from an older install, bump it to `30m` (or at least ~2× `heartbeat`). Newer defaults use a `15m` heartbeat when unset; leaving `stale_ttl` at `5m` makes healthy machines look stale for most of each heartbeat window.

### Privacy warning

Snapshots can include repository **paths**, **branch names**, and normalized **`origin`** URLs (org/repo identity). Keep the state repository private. Use `--redact-paths` if you want basenames only for paths and a stable hash instead of the origin URL (correlation across machines still works because the hash is deterministic). Agent logs intentionally avoid dumping full snapshot payloads.

### Setup

1. Create a **private** empty Git repo and clone it locally (example: `D:\find-uncommitted-state`).
2. Configure non-interactive `git pull` / `git push` credentials for that clone.
3. Start the agent (or install the scheduler):

```bash
# Run agent in the foreground (default check interval 2m, heartbeat 15m)
./find-uncommitted --agent --state-repo /path/to/state-clone /path/to/scan/root

# Install OS scheduler (writes sticky config, smoke-publishes a snapshot, then registers Task Scheduler / systemd)
./find-uncommitted --install-scheduler --state-repo /path/to/state-clone /path/to/scan/root

# After install, bare scans use sticky config (aggregate remotes by default)
./find-uncommitted /path/to/scan/root
./find-uncommitted   # uses scan_root from config when set

# Local-only even with sticky config
./find-uncommitted --no-remote /path/to/scan/root

# Remove scheduler registration
./find-uncommitted --uninstall-scheduler
```

Linux notes:
- Uses a systemd **user** service that keeps the long-running agent alive
- The unit launches `--agent` only; `scan_root`, `state_repo`, `interval`, `heartbeat`, and related settings come from sticky config
- For headless sessions: `loginctl enable-linger $USER`

Windows notes:
- Registers an **at-logon** scheduled task that starts the agent process
- Check / heartbeat cadence is owned by the agent loop (not a Task Scheduler repeat trigger)

### Aggregate CLI view

```bash
# Explicit state repo (also works without sticky config)
./find-uncommitted --state-repo /path/to/state-clone /path/to/scan/root
./find-uncommitted --state-repo /path/to/state-clone --stale-ttl 30m --machine-id my-laptop /path/to/scan/root

# With sticky config already installed
./find-uncommitted /path/to/scan/root
```

### Check one repo (pre-flight)

Path-scoped pre-flight for the repo you are about to work in — same correlation and Attention cues as the aggregate view, without scanning a whole tree:

```bash
./find-uncommitted check ~/repos/work-project
./find-uncommitted check .                  # from inside a repo (or subdirectory)
./find-uncommitted --no-remote check .      # local status only
./find-uncommitted --json check .           # machine-readable JSON (editors / scripts)
./find-uncommitted check --json .           # same; --json may follow check
```

Example:

```
github.com/you/work-project
  laptop*: Dirty on feature/auth (unstaged)
  desktop: Clean on main
→ commit or stash local changes before switching machines
→ other machine desktop has uncommitted work
```

Local machine is listed first (`*`); each machine is on its own line.

With `--json`, stdout is a single JSON object (`schemaVersion: 1`) with `ok`, `attention`, `project`, `machines[]` (including `local` / `stale` and snapshot status fields), and `situations[]` (`kind`, `nudge`, `machines`, `stale`). Non-fatal warnings stay on stderr so stdout remains parseable. On hard errors, exit is `1` and JSON (if emitted) has `ok: false` with an `error` field — never a false “clear” success. Human text remains the default without `--json`.

Exit codes (check mode only):

| Code | Meaning |
|------|---------|
| `0` | Nothing needing attention |
| `2` | One or more Attention situations |
| `1` | Usage error, not a git work tree, or invalid state repo when remotes are required |

Suitable for a shell `cd` hook later: `find-uncommitted check "$PWD"` (install helpers are out of scope). Editor clients should prefer `find-uncommitted --json check <path>`.

### Editor extension (VS Code / Cursor)

A thin client lives in [`vscode-extension/`](vscode-extension/). It shells out to `find-uncommitted --json check` for each workspace folder — same Attention nudges as the CLI, no git mutations, no reimplemented sync. Cross-machine attention defaults to a usual VS Code warning notification plus status bar; set `findUncommitted.attentionDisplay` to `statusBar` for the subtle footer only.

Install the Go binary first, then see [vscode-extension/README.md](vscode-extension/README.md) for compile / VSIX steps and `findUncommitted.binaryPath` when the GUI app’s `PATH` is incomplete.

Output starts with an **Attention** section (soft suggestions only — no git commands are run on your repos), then a **Full inventory** table. Local rows are marked with `*` on the machine column. Stale machines are annotated in the table and summarized after output.

Attention covers:

- Local error, dirty, unpushed, behind upstream, and untracked upstream (behind uses cached tracking refs; no automatic `git fetch`)
- Cross-machine branch mismatch and same-branch tip mismatch (via published short HEAD SHAs) for the same project identity
- Other machine has dirty work (pull will not help) or unpushed commits while local is clean
- Stale remote evidence when a remote attention-worthy snapshot is old

The Full inventory groups rows under each project identity (origin, or parent/basename for local-only repos).

With `--dirty-only`, Attention and inventory are limited to projects that produced at least one situation (clean local clones are still scanned when remotes are enabled so cross-machine cues can fire). Load-error snapshot rows remain visible.

Useful flags:

| Flag | Meaning |
|------|---------|
| `--state-repo` | Local clone of the private sync Git repo |
| `--agent` | Background publish loop |
| `--interval` | Check interval: scan + publish decision (default `2m`) |
| `--heartbeat` | Liveness commit when status unchanged (default `15m`) |
| `--stale-ttl` | Staleness threshold (default `30m`; keep ≈ 2× `heartbeat`) |
| `--tick-timeout` | Per-tick deadline for pull, scan, and publish (default `2m`) |
| `--max-workers` | Max parallel repo checks (default `8`) |
| `--machine-id` | Override hostname-based machine id |
| `--redact-paths` | Publish basename-only paths |
| `--no-remote` | Local scan only even if a state repo is configured |
| `--install-scheduler` / `--uninstall-scheduler` | OS autostart integration (install writes sticky config + smoke-publishes a snapshot) |

## Output Example

The tool prints an Attention list first, then a full inventory table:

```
Attention (suggestions only — no commands are run):
  • github.com/you/work-project
      → commit or stash local changes before switching machines
  • github.com/you/work-project
      → pull before continuing (behind upstream by 2 commit(s))

Full inventory:
Repository                                    Branch          Status   Changes
------------------------------------------------------------------------------------------
../my-project                                 main            ✅ Clean    -
../work-project                               feature/new...  ⚠️  Dirty  unstaged, untracked, behind:2
../old-project                                develop         ⚠️  Dirty  staged
../notes-project                              master          ⬆️ Unpushed  unpushed:1
../stale-clone                                main            ⬇️ Behind   behind:3

Summary: 21 clean repositories, 3 repositories with uncommitted changes, 1 repositories with unpushed commits, 1 repositories behind upstream, 0 repositories with untracked upstream, 0 repositories with errors
```

The output shows:
- **Attention**: Soft nudges for what to do next (never auto-executed)
- **Repository**: Path to the git repository (truncated for readability)
- **Branch**: Current branch name (truncated if too long)
- **Status**: One of:
   - ✅ Clean
   - ⚠️ Dirty (working tree/index changes)
   - ⬆️ Unpushed (ahead of upstream)
   - ⬇️ Behind (behind upstream per cached tracking refs)
   - ↕️ Diverged (both ahead and behind)
   - 🔗 Untracked Upstream (branch has no configured upstream)
   - ❌ Error
- **Changes**: Specific types of changes detected:
  - `unstaged`: Modified files not yet staged
  - `staged`: Files staged for commit
  - `untracked`: New files not tracked by git
  - `unpushed` / `unpushed:N`: Commits that haven't been pushed to remote
  - `behind` / `behind:N`: Commits present on upstream that aren't local yet
  - `untracked-upstream`: Branch has no upstream tracking configuration

## Dirty-Only Mode

Use the `--dirty-only` flag to show only projects that need attention:

```bash
./find-uncommitted --dirty-only /home/username/projects
```

This keeps Attention and inventory limited to:

- Git errors
- Unstaged / staged / untracked working-tree changes
- Unpushed commits
- Behind upstream (cached tracking refs)
- Untracked upstream
- Cross-machine situations (branch/tip mismatch, other-machine work, stale remote evidence) when a state repo is configured

When remotes are enabled, clean local clones are still scanned so cues like "other machine has uncommitted work" can appear; only projects without any situation are hidden.

## CSV Export

Use the `--output` flag to save results to a CSV file for further analysis:

```bash
./find-uncommitted --output results.csv /home/username/projects
```

The CSV file will contain the following columns:
- **Repository**: Path to the git repository
- **Branch**: Current branch name
- **Status**: Clean, Dirty, Unpushed, Behind, Diverged, UntrackedUpstream, or Error with details
- **Changes**: Comma-separated list of change types (unstaged, staged, untracked, unpushed, behind, untracked-upstream)

This is useful for:
- Importing into spreadsheet applications for analysis
- Creating reports for team management
- Tracking repository status over time
- Filtering and sorting results in external tools

You can combine the CSV export with other flags:
```bash
# Export only dirty repositories to CSV
./find-uncommitted --dirty-only --output dirty-repos.csv /home/username/projects

# Export with debug output (debug info won't appear in CSV)
./find-uncommitted --debug --output results.csv /home/username/projects
```

## Git Ownership Issues

If you encounter "dubious ownership" errors, the tool will provide specific guidance:

```
📁 ..\somedirectory
   Branch: unknown
   ❌ Error: Git ownership issue - run: git config --global --add safe.directory C:/somedirectory
```

### Automatic Fix

Use the included ownership fixer tool:

#### Windows
```bash
# Fix ownership issues for all repositories in a directory
./fix-ownership-tool/fix-ownership.exe C:\somedirectory

# With debug output
./fix-ownership-tool/fix-ownership.exe --debug C:\somedirectory
```

#### Linux/macOS
```bash
# Fix ownership issues for all repositories in a directory
./fix-ownership-tool/fix-ownership /home/username/projects

# With debug output
./fix-ownership-tool/fix-ownership --debug /home/username/projects
```

This will automatically run the necessary `git config` commands to resolve ownership issues.

## Requirements

- Go 1.21 or later
- Git installed and accessible from command line

## Building

### Windows
```bash
# Build the main executable (package ".", not main.go — the tool is multi-file)
go build -o find-uncommitted.exe .

# Build the ownership fixer
cd fix-ownership-tool
go build -o fix-ownership.exe .
cd ..
```

### Linux/macOS
```bash
# Build the main executable (package ".", not main.go — the tool is multi-file)
go build -o find-uncommitted .

# Build the ownership fixer
cd fix-ownership-tool
go build -o fix-ownership .
cd ..
```

### Cross-platform build
```bash
# Build for Windows from Linux/macOS
GOOS=windows GOARCH=amd64 go build -o find-uncommitted.exe .
cd fix-ownership-tool
GOOS=windows GOARCH=amd64 go build -o fix-ownership.exe .
cd ..

# Build for Linux from Windows
GOOS=linux GOARCH=amd64 go build -o find-uncommitted .
cd fix-ownership-tool
GOOS=linux GOARCH=amd64 go build -o fix-ownership .
cd ..
```

## How it works

1. **Directory Scanning**: Uses `filepath.Walk` to recursively scan the specified directory
2. **Git Detection**: Looks for `.git` directories to identify git repositories
3. **Status Checking**: For each repository found, runs git commands to check:
   - Current branch and short HEAD SHA
   - Unstaged changes (`git diff --name-only`)
   - Staged changes (`git diff --cached --name-only`)
   - Untracked files (`git ls-files --others --exclude-standard`)
   - Ahead of upstream (`git rev-list --count @{u}..HEAD`) when tracking exists
   - Behind upstream (`git rev-list --count HEAD..@{u}`) against cached tracking refs (no automatic fetch)
4. **Concurrent Processing**: Uses goroutines to check multiple repositories simultaneously
5. **Attention + inventory**: Builds soft situation nudges, then displays a formatted inventory (and optional CSV)
6. **Error Handling**: Provides specific guidance for common Git issues like ownership problems

## Performance Notes

- The application skips common system directories and build folders to improve scanning speed
- Repo status checks run in parallel, capped at **8 workers** by default (`max_workers` in sticky config, `--max-workers`, or `FIND_UNCOMMITTED_MAX_WORKERS`) to avoid overloading git and the filesystem
- Each git subprocess has a **30s** deadline; under heavy load a repo may report `git timed out or cancelled` instead of a false "invalid repository" error
- Large scan roots may take several minutes to scan completely
- Debug mode adds output but may slow down processing slightly

## Troubleshooting

### Debug Mode
Use the `--debug` flag to see detailed information about directory scanning and repository detection.

### Ownership Issues
If you see ownership errors, run the fix-ownership tool first, then run the main tool again.

### Timing Issues
If the fix-ownership tool doesn't seem to work immediately, try running it with the `--debug` flag or wait a few seconds before running the main tool again.

### Git timeouts under load
If many repos are scanned at once (e.g. an entire `code` tree), you may see `git timed out or cancelled` on otherwise healthy repos. Lower parallelism with `max_workers = 4` in sticky config, narrow `scan_root`, or re-run the scan. Timeouts are not the same as broken repositories.

### Git exit status 128 and error detail
Git uses exit code **128** as a generic fatal error — it does not mean one specific problem (missing upstream, empty repo, corrupt `.git`, etc.). find-uncommitted includes git's stderr detail in repository errors when available (for example `no such branch` or `does not have any commits yet`) instead of showing only `exit status 128`.

**Empty repositories** (`git init` with no commits yet) are shown as **Empty** in the inventory and do not produce a "fix local git error" Attention nudge. Repos with real git failures still appear as errors with the fatal message preserved. 