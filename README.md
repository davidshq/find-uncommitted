# Find Uncommitted

A Go application that scans your hard drive for git repositories and reports on their status - whether there are unstaged, staged, or untracked files that need to be committed.

## Features

- 🔍 **Recursive scanning**: Automatically finds all git repositories in the specified directory
- ⚡ **Concurrent processing**: Uses goroutines to check repository status in parallel
- 📊 **Detailed reporting**: Shows branch name, unstaged changes, staged changes, and untracked files
- 🚫 **Smart filtering**: Skips system directories and common build folders to improve performance
- 📈 **Summary statistics**: Provides a count of clean vs. dirty repositories
- 🛠️ **Ownership issue detection**: Identifies and provides guidance for Git ownership problems
- 🔧 **Debug mode**: Optional debug output for troubleshooting

## Usage

### Windows
```bash
# Scan a specific directory
./find-uncommitted.exe C:\somedirectory

# Scan the entire C: drive (may take a while)
./find-uncommitted.exe C:\

# Scan current directory
./find-uncommitted.exe .

# Enable debug output
./find-uncommitted.exe --debug C:\somedirectory
```

### Linux/macOS
```bash
# Scan a specific directory
./find-uncommitted /home/username/projects

# Scan the entire home directory (may take a while)
./find-uncommitted /home/username

# Scan current directory
./find-uncommitted .

# Enable debug output
./find-uncommitted --debug /home/username/projects
```

## Output Example

### Windows
```
Scanning for git repositories in: C:\Users\YourName\Documents
This may take a while depending on the size of your drive...

Found 3 git repositories:

📁 C:\Users\YourName\Documents\my-project
   Branch: main
   ✅ Clean

📁 C:\Users\YourName\Documents\work-project
   Branch: feature/new-feature
   ⚠️  Has uncommitted changes:
      • Unstaged changes
      • Untracked files

📁 C:\Users\YourName\Documents\old-project
   Branch: develop
   ⚠️  Has uncommitted changes:
      • Staged changes

Summary: 1 clean repositories, 2 repositories with uncommitted changes, 0 repositories with errors
```

### Linux/macOS
```
Scanning for git repositories in: /home/username/projects
This may take a while depending on the size of your drive...

Found 3 git repositories:

📁 /home/username/projects/my-project
   Branch: main
   ✅ Clean

📁 /home/username/projects/work-project
   Branch: feature/new-feature
   ⚠️  Has uncommitted changes:
      • Unstaged changes
      • Untracked files

📁 /home/username/projects/old-project
   Branch: develop
   ⚠️  Has uncommitted changes:
      • Staged changes

Summary: 1 clean repositories, 2 repositories with uncommitted changes, 0 repositories with errors
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
4. **Concurrent Processing**: Uses goroutines to check multiple repositories simultaneously
5. **Results Display**: Shows a formatted report with emojis and clear status indicators
6. **Error Handling**: Provides specific guidance for common Git issues like ownership problems

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