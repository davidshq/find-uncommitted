## 1. Error formatting helper

- [x] 1.1 Add `formatGitError(stderr, err)` (trim stderr, prefer first `fatal:` line, optional length cap, fall back to `err`)
- [x] 1.2 Use `formatGitError` in `appendRepoCheckError` and other repo-check error paths that currently emit only `exit status N`
- [x] 1.3 Add unit tests for `formatGitError` (stderr present, empty stderr, multiline stderr, fatal prefix trimming)

## 2. Upstream and empty-repo classification

- [x] 2.1 Add `isEmptyRepository` detection (no commits yet) after successful `rev-parse --git-dir`
- [x] 2.2 Extend upstream-check branch: `no upstream configured` → untracked upstream; empty repo → non-error empty status; else → error with `formatGitError`
- [x] 2.3 Short-circuit ahead/behind/upstream checks for empty repos
- [x] 2.4 Add unit tests for upstream classification (no upstream, empty repo / `hutsell/won` shape, unknown fatal with stderr)

## 3. Invalid-repo and timeout distinction

- [x] 3.1 Include stderr detail in non-ownership `rev-parse --git-dir` failures via `formatGitError`
- [x] 3.2 Verify timeout/cancellation path still uses distinct wording and is not rewritten by `formatGitError`
- [x] 3.3 Add regression test: timeout/WaitDelay never reported as "Not a valid git repository"

## 4. Attention and display

- [x] 4.1 Represent empty repos in `RepoStatus` / `RepoSnapshot` (e.g. `IsEmpty` or derived clean empty state)
- [x] 4.2 Update `repoStatusText` / inventory to show empty status without error icon where appropriate
- [x] 4.3 Ensure `DetectSituations` / `SituationLocalError` skips empty repos (no fix-local-git-error nudge)
- [x] 4.4 Add situation tests for empty repo and detailed upstream fatal

## 5. Documentation

- [x] 5.1 Update README troubleshooting: git stderr in errors, empty repos, exit 128 is not one specific problem
- [x] 5.2 Run `go test ./...` and manual smoke on `c:\code\hutsell\won` plus a repo with real upstream fatal
