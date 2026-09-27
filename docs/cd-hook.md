# Shell `cd` hook (Fork A)

Hand-rolled wrappers that run `find-uncommitted check` after you change into a directory. They print **only when Attention fires** (exit code `2`) — clean trees and non-git paths stay silent.

This is intentional integration tax, not an installer: copy or source the examples yourself. Prompt / menubar polish comes later, once this habit is boring.

## Prerequisites

1. A built binary on `PATH`, or set `FIND_UNCOMMITTED_BIN`:

   ```bash
   # Linux
   mkdir -p binaries && go build -o binaries/find-uncommitted .
   export FIND_UNCOMMITTED_BIN="$PWD/binaries/find-uncommitted"

   # Windows (PowerShell)
   mkdir binaries -Force; go build -o binaries/find-uncommitted.exe .
   $env:FIND_UNCOMMITTED_BIN = "$PWD\binaries\find-uncommitted.exe"
   ```

2. Sticky config / `--state-repo` if you want cross-machine correlation (same as interactive `check`). For a local-only nudge while testing:

   ```bash
   export FIND_UNCOMMITTED_CD_ARGS=--no-remote
   ```

## Linux / macOS (bash or zsh)

Example script: [`examples/cd-hook/cd-hook.bash`](../examples/cd-hook/cd-hook.bash)

```bash
# ~/.bashrc or ~/.zshrc
export FIND_UNCOMMITTED_BIN="$HOME/bin/find-uncommitted"   # if not on PATH
# export FIND_UNCOMMITTED_CD_ARGS=--no-remote             # optional
source /path/to/find-uncommitted/examples/cd-hook/cd-hook.bash
```

- **bash** wraps builtin `cd`.
- **zsh** registers a `chpwd` hook (covers `cd`, `pushd`, etc.).
- Same work tree is not re-checked when you `cd` into a subdirectory; leaving the tree clears that cache so re-entry runs `check` again.
- Disable temporarily: `export FIND_UNCOMMITTED_CD_HOOK=0`.

### Try it

```bash
source examples/cd-hook/cd-hook.bash
cd /tmp                    # silent (not a git work tree)
cd ~/repos/some-dirty-repo # prints check summary + nudges when Attention
```

## Windows (PowerShell)

Example script: [`examples/cd-hook/cd-hook.ps1`](../examples/cd-hook/cd-hook.ps1)

```powershell
# In $PROFILE (e.g. Documents\PowerShell\Microsoft.PowerShell_profile.ps1)
$env:FIND_UNCOMMITTED_BIN = "C:\path\to\binaries\find-uncommitted.exe"
# $env:FIND_UNCOMMITTED_CD_ARGS = "--no-remote"   # optional
. "C:\path\to\find-uncommitted\examples\cd-hook\cd-hook.ps1"
```

Wraps interactive `cd`, `Push-Location`, and `Pop-Location` (not every `Set-Location` call). Same silence / Attention / same-tree rules as the bash hook.

Disable temporarily: `$env:FIND_UNCOMMITTED_CD_HOOK = "0"`.

### Try it

```powershell
. .\examples\cd-hook\cd-hook.ps1
cd $env:TEMP
cd C:\repos\some-dirty-repo
```

## Behavior cheat sheet

| Situation | Exit from `check` | Hook output |
|-----------|-------------------|-------------|
| Not a git work tree | (skipped; no spawn) | Silent |
| Clean / nothing to flag | `0` | Silent |
| Attention (local dirty, other machine, …) | `2` | Prints `check` human summary |
| Hard error | `1` | Silent (does not break `cd`) |

## Smoke test

From the repo root (Linux; requires `bash` + a built binary):

```bash
./examples/cd-hook/test-cd-hook.sh
```

On Windows with PowerShell 7+:

```powershell
./examples/cd-hook/test-cd-hook.ps1
```

## Related

- CLI docs: [README — Check one repo](../README.md#check-one-repo-pre-flight)
- Product context: [strategic-directions.md](strategic-directions.md) (Fork A)
- Priorities: [developer-review-findings.md](developer-review-findings.md)
