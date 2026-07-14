# Find Uncommitted

A Go application that scans your hard drive for git repositories and reports on their status - whether there are unstaged, staged, or untracked files that need to be committed.

## Features

- 🔍 **Recursive scanning**: Automatically finds all git repositories in the specified directory
- ⚡ **Concurrent processing**: Uses goroutines to check repository status in parallel
- 📊 **Detailed reporting**: Shows branch name, unstaged changes, staged changes, untracked files, and unpushed commits
- 🚫 **Smart filtering**: Skips system directories and common build folders to improve performance
- 📈 **Summary statistics**: Provides a count of clean vs. dirty repositories
- 🎯 **Dirty-only mode**: Option to show only repositories needing attention (dirty, unpushed, untracked upstream, errors)
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

- Each machine writes only its own file: `machines/<machine-id>.json`
- Background `--agent` mode pulls, scans, writes, and push/rebases on an interval (default **30s**)
- Normal scans with `--state-repo` pull (`--ff-only`) and merge other machines’ snapshots
- Snapshots older than `--stale-ttl` (default **5m**) are labeled **stale**
- Unchanged status does not create commits every tick; a heartbeat commit keeps freshness without constant chatty history
- Snapshot filenames include a short hash so sanitized machine ids cannot collide
- Agent exits cleanly on Ctrl+C / SIGTERM

### Privacy warning

Snapshots can include repository **paths** and **branch names**. Keep the state repository private. Use `--redact-paths` if you want basenames only in published JSON. Agent logs intentionally avoid dumping full snapshot payloads.

### Setup

1. Create a **private** empty Git repo and clone it locally (example: `D:\find-uncommitted-state`).
2. Configure non-interactive `git pull` / `git push` credentials for that clone.
3. Start the agent (or install the scheduler):

```bash
# Run agent in the foreground (default interval 30s)
./find-uncommitted --agent --state-repo /path/to/state-clone /path/to/scan/root

# Install OS scheduler (Windows Task Scheduler at logon, or Linux systemd user service)
./find-uncommitted --install-scheduler --state-repo /path/to/state-clone /path/to/scan/root

# Remove scheduler registration
./find-uncommitted --uninstall-scheduler
```

Linux notes:
- Uses a systemd **user** service that keeps the long-running agent alive
- For headless sessions: `loginctl enable-linger $USER`

Windows notes:
- Registers an **at-logon** scheduled task that starts the agent process
- The 30s cadence is owned by the agent loop (not a 30s Task Scheduler trigger)

### Aggregate CLI view

```bash
./find-uncommitted --state-repo /path/to/state-clone /path/to/scan/root
./find-uncommitted --state-repo /path/to/state-clone --stale-ttl 5m --machine-id my-laptop /path/to/scan/root
```

Local rows are marked with `*` on the machine column. Stale machines are annotated in the table and summarized after output.

Useful flags:

| Flag | Meaning |
|------|---------|
| `--state-repo` | Local clone of the private sync Git repo |
| `--agent` | Background publish loop |
| `--interval` | Publish interval (default `30s`) |
| `--stale-ttl` | Staleness threshold (default `5m`) |
| `--machine-id` | Override hostname-based machine id |
| `--redact-paths` | Publish basename-only paths |
| `--no-remote` | Local scan only even if `--state-repo` is set |
| `--install-scheduler` / `--uninstall-scheduler` | OS autostart integration |

## Output Example

The tool now displays results in a clean tabular format:

```
Scanning for git repositories in: /home/username/projects
This may take a while depending on the size of your drive...

Found 24 git repositories:

Repository                                    Branch          Status   Changes
------------------------------------------------------------------------------------------
../my-project                                 main            ✅ Clean    -
../work-project                               feature/new...  ⚠️  Dirty  unstaged, untracked
../old-project                                develop         ⚠️  Dirty  staged
../notes-project                              master          ⚠️  Dirty  unpushed

Summary: 21 clean repositories, 3 repositories with uncommitted changes, 0 repositories with errors
```

The output shows:
- **Repository**: Path to the git repository (truncated for readability)
- **Branch**: Current branch name (truncated if too long)
- **Status**: One of:
   - ✅ Clean
   - ⚠️ Dirty (working tree/index changes)
   - ⬆️ Unpushed (ahead of upstream)
   - 🔗 Untracked Upstream (branch has no configured upstream)
   - ❌ Error
- **Changes**: Specific types of changes detected:
  - `unstaged`: Modified files not yet staged
  - `staged`: Files staged for commit
  - `untracked`: New files not tracked by git
  - `unpushed`: Commits that haven't been pushed to remote
   - `untracked-upstream`: Branch has no upstream tracking configuration

## Dirty-Only Mode

Use the `--dirty-only` flag to show only repositories that need attention:

```bash
./find-uncommitted --dirty-only /home/username/projects
```

This will filter out clean repositories and keep those that need attention:
- Unstaged / staged / untracked working-tree changes
- Unpushed commits
- Untracked upstream
- Git errors

This is particularly useful when you want to quickly identify which repositories need attention without scrolling through a long list of clean repositories.

## CSV Export

Use the `--output` flag to save results to a CSV file for further analysis:

```bash
./find-uncommitted --output results.csv /home/username/projects
```

The CSV file will contain the following columns:
- **Repository**: Path to the git repository
- **Branch**: Current branch name
- **Status**: Clean, Dirty, Unpushed, UntrackedUpstream, or Error with details
- **Changes**: Comma-separated list of change types (unstaged, staged, untracked, unpushed, untracked-upstream)

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
# Build the main executable
go build -o find-uncommitted.exe main.go

# Build the ownership fixer
cd fix-ownership-tool
go build -o fix-ownership.exe fix-ownership.go
cd ..
```

### Linux/macOS
```bash
# Build the main executable
go build -o find-uncommitted main.go

# Build the ownership fixer
cd fix-ownership-tool
go build -o fix-ownership fix-ownership.go
cd ..
```

### Cross-platform build
```bash
# Build for Windows from Linux/macOS
GOOS=windows GOARCH=amd64 go build -o find-uncommitted.exe main.go
cd fix-ownership-tool
GOOS=windows GOARCH=amd64 go build -o fix-ownership.exe fix-ownership.go
cd ..

# Build for Linux from Windows
GOOS=linux GOARCH=amd64 go build -o find-uncommitted main.go
cd fix-ownership-tool
GOOS=linux GOARCH=amd64 go build -o fix-ownership fix-ownership.go
cd ..
```

## How it works

1. **Directory Scanning**: Uses `filepath.Walk` to recursively scan the specified directory
2. **Git Detection**: Looks for `.git` directories to identify git repositories
3. **Status Checking**: For each repository found, runs git commands to check:
   - Current branch
   - Unstaged changes (`git diff --name-only`)
   - Staged changes (`git diff --cached --name-only`)
   - Untracked files (`git ls-files --others --exclude-standard`)
   - Unpushed commits (`git rev-list --count @{u}..HEAD`)
4. **Concurrent Processing**: Uses goroutines to check multiple repositories simultaneously
5. **Results Filtering**: Optionally filters out clean repositories when using `--dirty-only` flag
6. **Results Display**: Shows a formatted report with emojis and clear status indicators
7. **CSV Export**: Optionally saves results to a CSV file for external analysis
8. **Error Handling**: Provides specific guidance for common Git issues like ownership problems

## Performance Notes

- The application skips common system directories and build folders to improve scanning speed
- Concurrent processing means checking many repositories won't take proportionally longer
- Large drives may take several minutes to scan completely
- Debug mode adds output but may slow down processing slightly

## Troubleshooting

### Debug Mode
Use the `--debug` flag to see detailed information about directory scanning and repository detection.

### Ownership Issues
If you see ownership errors, run the fix-ownership tool first, then run the main tool again.

### Timing Issues
If the fix-ownership tool doesn't seem to work immediately, try running it with the `--debug` flag or wait a few seconds before running the main tool again. 