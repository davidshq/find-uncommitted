# Find Uncommitted

A Go application that scans your hard drive for git repositories and reports on their status - whether there are unstaged, staged, or untracked files that need to be committed.

## Features

- 🔍 **Recursive scanning**: Automatically finds all git repositories in the specified directory
- ⚡ **Concurrent processing**: Uses goroutines to check repository status in parallel
- 📊 **Detailed reporting**: Shows branch name, unstaged changes, staged changes, and untracked files
- 🚫 **Smart filtering**: Skips system directories and common build folders to improve performance
- 📈 **Summary statistics**: Provides a count of clean vs. dirty repositories

## Usage

```bash
# Scan a specific directory
go run main.go C:\Users\YourName\Documents

# Scan the entire C: drive (may take a while)
go run main.go C:\

# Scan current directory
go run main.go .
```

## Output Example

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

Summary: 1 clean repositories, 2 repositories with uncommitted changes
```

## Requirements

- Go 1.21 or later
- Git installed and accessible from command line

## Building

```bash
# Build the executable
go build -o find-uncommitted.exe main.go

# Run the executable
./find-uncommitted.exe C:\
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

## Performance Notes

- The application skips common system directories and build folders to improve scanning speed
- Concurrent processing means checking many repositories won't take proportionally longer
- Large drives may take several minutes to scan completely 