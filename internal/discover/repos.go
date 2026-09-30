package discover

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// WalkOptions configures repository discovery under a scan root.
type WalkOptions struct {
	Debug    bool
	Excludes []string
	// Context, when set, aborts the walk early on cancel/deadline so large
	// scan roots do not ignore agent tick timeouts during discovery.
	Context context.Context
}

// FindGitRepos walks rootDir and returns paths to git repositories.
// A directory containing a .git entry is treated as a repo root: .git is a
// directory in a normal clone and a file in a linked worktree or submodule.
//
// A root that is missing, unreadable or not a directory is an error: zero repos
// from a bad root would otherwise publish an empty snapshot that peers read as
// "all clear". A symlinked root is followed (filepath.Walk alone never descends
// it), and repo paths are reported under rootDir as the user spelled it.
func FindGitRepos(rootDir string, opts WalkOptions) ([]string, error) {
	var repos []string
	excludes := normalizeExcludes(opts.Excludes)
	root := filepath.Clean(rootDir)

	walkRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return nil, fmt.Errorf("scan root %q: %w", rootDir, err)
	}
	info, err := os.Stat(walkRoot)
	if err != nil {
		return nil, fmt.Errorf("scan root %q: %w", rootDir, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("scan root %q is not a directory", rootDir)
	}
	// Map a path under the resolved root back under the requested root so
	// exclusions (e.g. the state repo) and published paths stay unchanged.
	displayPath := func(p string) string {
		if walkRoot == root {
			return p
		}
		if rel, err := filepath.Rel(walkRoot, p); err == nil {
			return filepath.Join(root, rel)
		}
		return p
	}

	var rootErr error
	err = filepath.Walk(walkRoot, func(path string, info os.FileInfo, err error) error {
		if opts.Context != nil {
			if cerr := opts.Context.Err(); cerr != nil {
				return cerr
			}
		}
		if err != nil {
			// Stat passed but the root can't be listed (permissions, stale
			// mount): that's an unknown, not zero repos.
			if path == walkRoot {
				rootErr = fmt.Errorf("scan root %q: %w", rootDir, err)
				return rootErr
			}
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
			repoPath := displayPath(filepath.Dir(path))
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
		if filepath.Clean(path) != walkRoot && shouldSkipDir(path) {
			if opts.Debug {
				fmt.Printf("[DEBUG] Skipping directory: %s\n", path)
			}
			return filepath.SkipDir
		}

		return nil
	})

	if rootErr != nil {
		return nil, rootErr
	}
	if err != nil && !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
		fmt.Printf("Error scanning directory: %v\n", err)
	}

	return repos, nil
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
