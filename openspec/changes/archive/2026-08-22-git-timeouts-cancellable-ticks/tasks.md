## 1. Context-aware git execution

- [x] 1.1 Add default timeout constants and a shared `runGit(ctx, dir, args...)` / update `GitRunner` to accept `context.Context`
- [x] 1.2 Implement `ExecGitRunner` with `exec.CommandContext`, per-command deadline, `GIT_TERMINAL_PROMPT=0`, and nil stdin
- [x] 1.3 Update all gitsync callers of `GitRunner.Run` for the new signature

## 2. Scan path timeouts

- [x] 2.1 Thread `context.Context` through `checkRepoStatus`, `revListCount`, `shortHeadSHA`, and `repoOriginURL`
- [x] 2.2 Update `checkRepoStatuses` / interactive scan / agent publish to pass contexts; record timeout as repo `Error`

## 3. Cancellable agent ticks

- [x] 3.1 Replace agent `time.After` wait with `time.NewTicker`
- [x] 3.2 Wrap each `runAgentTick` in `context.WithTimeout` from the signal parent context; pass tick ctx through pull/scan/publish
- [x] 3.3 On tick/git timeout, log non-fatal warning and continue the loop

## 4. Tests and docs

- [x] 4.1 Add tests for git runner cancel/timeout and agent tick deadline abort behavior
- [x] 4.2 Update README agent/sync docs for timeouts and ticker behavior
- [x] 4.3 Run `go test ./...` and fix regressions
