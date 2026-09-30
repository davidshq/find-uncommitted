# TODO

## State-repo worktree support

`rebaseInProgress` / `abortRebaseIfInProgress` (`gitsync.go`) only probe `stateRepoDir/.git/rebase-merge|rebase-apply`. That misses linked worktrees and `.git` files, so a mid-rebase state clone in a worktree layout won’t self-abort.

- Resolve the real git dir via `git rev-parse --git-path` (or equivalent) before checking rebase markers.
- Cover worktree layouts in sync/rebase recovery tests.
- Confirm other state-repo paths (locks, snapshot files) behave correctly when the state clone is a worktree.

## Installer
- It can look like it stalled, may want to add progress report bar.