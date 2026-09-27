# Auto-run setup

Install the OS scheduler so the cross-machine **agent** starts automatically and keeps publishing snapshots in the background. macOS is not supported yet (`install-scheduler` declines on other platforms).

The agent never runs `git commit` / `push` / `pull` on your code repos — it only scans and publishes status into your private **state** repo.

## When the agent runs (session vs always-on)

Default install is **while you have a user session**, not “whenever the machine is on.” Remote edits alone do not start the agent.

| Situation | Linux | Windows |
|-----------|-------|---------|
| Interactive desktop / RDP logged in | Runs (user systemd service) | Runs (Task Scheduler **at logon**) |
| SSH session as that user | Runs while the session (and user manager) is up | Often runs for the life of that logon; not after logout |
| Logged out / at login screen | **Stops** unless linger is enabled (below) | **Stops** — no supported always-on mode yet |
| SMB / SFTP / another PC editing files with **no** logon for this user | No agent tick | No agent tick |

**Why:** the agent must `git pull` / `git push` your private **state** repo non-interactively. That usually needs *your* SSH agent or credential helper in a real user session. A true “always, even when logged out” Windows task runs in a different context and often cannot see those credentials — so this release stays session-scoped on Windows.

**Practical effect:** if you change repos over SMB while nobody is logged in, disk state updates but **no snapshot is published** until the agent runs again after logon (or until a Linux linger service is already up). Other machines keep the last heartbeat, then mark the host stale after `stale_ttl`.

There is **no** `install-scheduler` option today for “session only” vs “always.” On Linux the always-on path is OS linger (documented below), not a separate scheduler mode. A Windows always-on option is deferred until there is a clear credential story (e.g. deploy key for the state repo only).

## Prerequisites

1. **Built binary** (not `go run`). From the repo root:

   ```bash
   mkdir -p binaries
   # Linux
   go build -o binaries/find-uncommitted .
   # Windows
   go build -o binaries/find-uncommitted.exe .
   ```

2. A **private** empty Git repository, cloned locally (this is the sync bus).

3. **Non-interactive** `git pull` / `git push` credentials for that clone (SSH key agent, credential helper, etc.). The agent sets `GIT_TERMINAL_PROMPT=0`, so interactive prompts will fail.

4. A **scan root** — the directory tree of repos you want monitored (e.g. `~/repos` or `C:\repos`).

## Install

```bash
# Linux
./binaries/find-uncommitted --state-repo /path/to/state-clone install-scheduler /path/to/scan/root

# Windows (PowerShell or cmd)
.\binaries\find-uncommitted.exe --state-repo D:\find-uncommitted-state install-scheduler C:\repos
```

On Windows, creating/updating the **at-logon** task often requires an elevated prompt (`Access is denied` without it). Sticky config and smoke publish still run; re-run the elevated task registration if install stops at that step.

What install does, in order:

1. Writes sticky config (`config.toml`) with `state_repo`, `scan_root`, cadence knobs, and a stable `machine_id` if unset.
2. Smoke-publishes one snapshot so you can confirm the state repo works (install aborts if this fails).
3. Registers the OS auto-start integration and starts the agent (Linux) or prepares it for next logon (Windows).

Foreground alternative (no OS registration):

```bash
./binaries/find-uncommitted --state-repo /path/to/state-clone agent /path/to/scan/root
```

### Sticky config locations

| Platform | Path |
|----------|------|
| Linux | `~/.config/find-uncommitted/config.toml` (or `$XDG_CONFIG_HOME/...`) |
| Windows | `%AppData%\find-uncommitted\config.toml` |

After install, bare scans pick up remotes from this file:

```bash
./binaries/find-uncommitted
./binaries/find-uncommitted /path/to/scan/root
```

## Linux (systemd user service)

Install creates and enables:

- Unit: `~/.config/systemd/user/find-uncommitted-agent.service`
- `ExecStart`: `<absolute-path-to-binary> agent`
- `Restart=on-failure` (crash recovery)

Scan roots and intervals come from sticky config, not from the unit file.

**Session-only (default):** without linger, the user systemd instance stops when your last login session ends — including after SSH disconnect if nothing else holds a session. That matches “only while logged in.”

### Always-on / headless (linger)

To keep the agent up after logout and across reboot **without** an active login (SSH/SMB-only boxes):

```bash
loginctl enable-linger $USER
```

**Scope:** linger is **per user, not per app**. It lets this account’s entire `systemd --user` instance run without a login — so **every enabled user service** for that account can stay up (this agent, Syncthing, Podman, etc.), not only find-uncommitted. Other OS users are unchanged. Apps that are not systemd user units are unaffected.

Then confirm linger and the service without relying on an interactive desktop:

```bash
# Linger enabled for this account?
loginctl show-user "$USER" -p Linger
# → Linger=yes

# User manager running because of linger (works from a system shell / other account):
systemctl is-active "user@$(id -u).service"
# Or for another username: systemctl is-active user@1000.service
```

Check the **agent unit** while you are not at that machine’s desktop:

```bash
# Easiest: SSH in as the same user (linger keeps systemd --user up after “logout”)
ssh you@that-host 'systemctl --user is-active find-uncommitted-agent.service'

# From root / another local account on the box:
systemctl --user -M youruser@ is-active find-uncommitted-agent.service
```

If `systemctl --user` is awkward, skip it and confirm a fresh snapshot under `machines/` in the state clone from another computer after a tick (or after reboot with linger on).

Linger is an OS setting, not something `install-scheduler` toggles. Disable with `loginctl disable-linger $USER` if you want session-only again (that stops headless user services for the account, not only this agent). Ensure non-interactive Git credentials for the state clone still work in that lingering context (e.g. key files readable without an unlocked SSH agent).

### Verify

```bash
systemctl --user status find-uncommitted-agent.service
systemctl --user is-active find-uncommitted-agent.service
journalctl --user -u find-uncommitted-agent.service -n 50 --no-pager
```

You should also see a new/updated file under `machines/` in the state clone after a successful tick.

Quick operability report (config, locks, last publish, service health):

```bash
./binaries/find-uncommitted doctor
./binaries/find-uncommitted --print-config
```

### Uninstall

```bash
./binaries/find-uncommitted uninstall-scheduler
```

That disables/stops the unit and removes the unit file. Sticky config is left in place so interactive scans still work.

## Windows (Task Scheduler)

Install creates:

- Task name: `FindUncommittedAgent`
- Trigger: **at logon** for the current user (`LogonTrigger`, limited rights)
- Action: `<absolute-path-to-exe> agent` (no `.cmd` / VBS wrapper)
- Settings: `MultipleInstancesPolicy=IgnoreNew` (so `schtasks /Run` is not stuck **Queued**), `ExecutionTimeLimit` disabled (agent may run indefinitely), task marked hidden, `RestartOnFailure` every `1m` up to `999` attempts (crash recovery while the logon session is still up)

At agent start on Windows, if this process is the only one on the console (typical for Task Scheduler), the agent **detaches that console** so no cmd window stays open. Interactive `agent` in an existing terminal keeps logging visible. Git subprocesses and cancel helpers are created with no console window.

Cadence (check interval / heartbeat) is owned by the agent loop inside the process, not by a repeating Task Scheduler trigger.

**Session-only (only supported mode):** the task runs in your interactive logon session. It does **not** run at the login screen, after full logout, or when the only access is SMB/SFTP with no Windows logon for this user. There is no linger equivalent in the current installer. For an always-on Windows box you hit mainly over the network, leave a user session logged in (or run `agent` under a host mechanism you trust with state-repo credentials).

If the agent process exits with a failure while you are still logged in, Task Scheduler restarts it (same idea as Linux `Restart=on-failure`). Clean exit (code 0) does not restart. Re-run `install-scheduler` after upgrading so older tasks pick up recovery settings and invoke soft command `agent` (not `--agent`).

After upgrading the binary or moving it, re-run `install-scheduler` so the task path is rewritten.

Install registers the task for **next logon**; it does not start the agent immediately. To start without logging off:

```powershell
schtasks /Run /TN FindUncommittedAgent
# or
Start-Process .\binaries\find-uncommitted.exe -ArgumentList "agent" -WindowStyle Hidden
```

Confirm a single process (not a swarm of agents):

```powershell
Get-CimInstance Win32_Process -Filter "Name='find-uncommitted.exe'" |
  Select-Object ProcessId, CommandLine
```

A healthy tick may briefly show several `git` children (bounded by `max_workers`, default 8). Those should drain when the tick finishes or is cancelled; they must not keep climbing across ticks. On Windows, git and cancel helpers are started with no console window so the agent does not flash a terminal each tick.

### Verify

```powershell
schtasks /Query /TN FindUncommittedAgent /V /FO LIST
schtasks /Run /TN FindUncommittedAgent
```

Or open **Task Scheduler** → Task Scheduler Library → `FindUncommittedAgent`.

Confirm a snapshot under `machines\` in the state clone after a tick.

Quick operability report (config, locks, last publish, task/service health):

```powershell
.\binaries\find-uncommitted.exe doctor
.\binaries\find-uncommitted.exe --print-config
```

### Uninstall

```powershell
.\binaries\find-uncommitted.exe uninstall-scheduler
```

Removes the scheduled task. Sticky config remains.

## After moving or rebuilding the binary

The unit/task points at the **absolute path** resolved at install time. If you rebuild into a new location (or rename `binaries/`), re-run `install-scheduler` with the same `--state-repo` and scan root so the unit/task is rewritten.

## Optional cadence

Defaults: check every `2m`, heartbeat every `15m`, stale after `30m`. Override at install time with flags, or edit sticky config afterward (see [README sticky config](../README.md#sticky-config-recommended)).

## Privacy

Snapshots may include repository paths, branch names, and normalized `origin` URLs. Keep the state repository **private**. Use `--redact-paths` at install/agent time if you want less path detail.
