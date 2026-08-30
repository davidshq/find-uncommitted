package discover

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// WalkOptions configures repository discovery under a scan root.
type WalkOptions struct {
	Debug    bool
	Excludes []string
}

// FindGitRepos walks rootDir and returns paths to git repositories.
// A directory containing a .git entry is treated as a repo root: .git is a
// directory in a normal clone and a file in a linked worktree or submodule.
func FindGitRepos(rootDir string, opts WalkOptions) []string {
	var repos []string
	excludes := normalizeExcludes(opts.Excludes)
	root := filepath.Clean(rootDir)

	err := filepath.Walk(rootDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			if opts.Debug {
				fmt.Printf("[DEBUG] Skipping (error accessing): %s\n", path)
			}
			return nil
		}

		// Match .git before the file/dir split: a .git *file* marks a linked
		// worktree or submodule, which is exactly where unfinished work hides.
		if filepath.Base(path) == ".git" {
			if opts.Debug {
				kind := "file (worktree/submodule)"
				if info.IsDir() {
					kind = "directory"
				}
				fmt.Printf("[DEBUG] Found .git %s: %s\n", kind, path)
			}
			repoPath := filepath.Dir(path)
			if shouldExcludeRepo(repoPath, excludes) {
				if opts.Debug {
					fmt.Printf("[DEBUG] Excluding state/sync repo: %s\n", repoPath)
				}
			} else {
				repos = append(repos, repoPath)
			}
			if info.IsDir() {
				return filepath.SkipDir
			}
			// SkipDir from a file would skip the rest of the containing
			// directory, hiding sibling entries; a file has nothing to descend.
			return nil
		}

		if !info.IsDir() {
			return nil
		}

		if opts.Debug {
			fmt.Printf("[DEBUG] Visiting: %s\n", path)
		}

		// Never skip the scan root itself. The user asked for it explicitly, so
		// a hidden root (~/.dotfiles) must not silently yield zero repos.
		if filepath.Clean(path) != root && shouldSkipDir(path) {
			if opts.Debug {
				fmt.Printf("[DEBUG] Skipping directory: %s\n", path)
			}
			return filepath.SkipDir
		}

		return nil
	})

	if err != nil {
		fmt.Printf("Error scanning directory: %v\n", err)
	}

	return repos
}

func normalizeExcludes(excludeRepos []string) []string {
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
	return excludes
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

func shouldSkipDir(path string) bool {
	base := filepath.Base(path)
	return strings.HasPrefix(base, ".") ||
		base == "node_modules" ||
		base == "vendor" ||
		base == "bin" ||
		base == "obj" ||
		strings.Contains(path, "\\Windows\\") ||
		strings.Contains(path, "\\Program Files\\") ||
		strings.Contains(path, "\\Program Files (x86)\\")
}
