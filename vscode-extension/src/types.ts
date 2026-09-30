/**
 * schemaVersion 1 contract from `find-uncommitted --json check`.
 * Keep in sync with CheckJSONResult in the Go CLI.
 */
export interface CheckJSONResult {
  schemaVersion: number;
  ok: boolean;
  attention: boolean;
  error?: string;
  project?: string;
  machines?: CheckJSONMachine[];
  situations?: CheckJSONSituation[];
}

export interface CheckJSONMachine {
  id: string;
  local: boolean;
  stale?: boolean;
  /** Remote snapshot publish time (RFC3339). Omitted for live local rows. */
  updated_at?: string;
  path?: string;
  origin?: string;
  branch?: string;
  has_unstaged?: boolean;
  has_staged?: boolean;
  has_untracked?: boolean;
  has_unpushed?: boolean;
  has_behind?: boolean;
  has_untracked_upstream?: boolean;
  ahead_count?: number;
  behind_count?: number;
  head_sha?: string;
  is_dirty?: boolean;
  is_clean?: boolean;
  is_empty?: boolean;
  error?: string;
}

export interface CheckJSONSituation {
  kind: string;
  nudge: string;
  machines?: string[];
  stale?: boolean;
}

/** Situation kinds that elevate the status bar (cross-machine). */
export const CROSS_MACHINE_KINDS = new Set([
  "other_machine_work",
  "branch_mismatch",
  "tip_mismatch",
  "stale_evidence",
]);

export type FolderOutcome =
  | { kind: "clear"; result: CheckJSONResult; folder: string }
  | { kind: "attention"; result: CheckJSONResult; folder: string; elevated: boolean }
  | { kind: "not_git"; folder: string }
  | {
      kind: "error";
      folder: string;
      message: string;
      result?: CheckJSONResult;
      stderr?: string;
    }
  | { kind: "missing_binary"; message: string };

export function isElevated(result: CheckJSONResult): boolean {
  return (result.situations ?? []).some((s) => CROSS_MACHINE_KINDS.has(s.kind));
}

/** Local wall-clock stamp for Output channel freshness lines. */
export function formatCheckedAt(checkedAt: Date): string {
  const pad = (n: number) => String(n).padStart(2, "0");
  const y = checkedAt.getFullYear();
  const mo = pad(checkedAt.getMonth() + 1);
  const d = pad(checkedAt.getDate());
  const h = pad(checkedAt.getHours());
  const mi = pad(checkedAt.getMinutes());
  const s = pad(checkedAt.getSeconds());
  return `${y}-${mo}-${d} ${h}:${mi}:${s}`;
}

/** Compact relative age matching CLI formatCompactAge (e.g. "45s", "12m", "3h20m", "2d"). */
export function formatCompactAge(ms: number): string {
  let d = Math.abs(ms);
  d = Math.floor(d / 1000) * 1000;
  const sec = Math.floor(d / 1000);
  if (sec < 60) {
    return `${sec}s`;
  }
  const min = Math.floor(sec / 60);
  if (min < 60) {
    return `${min}m`;
  }
  const hours = Math.floor(min / 60);
  if (hours < 48) {
    const remMin = min % 60;
    return remMin === 0 ? `${hours}h` : `${hours}h${remMin}m`;
  }
  return `${Math.floor(hours / 24)}d`;
}

/**
 * Annotate a remote machine line with publish wall-clock + relative age.
 * Returns "" when updated_at is missing/unparseable (live local rows).
 */
export function formatPublishedSuffix(
  updatedAt: string | undefined,
  now: Date = new Date()
): string {
  if (!updatedAt?.trim()) {
    return "";
  }
  const t = new Date(updatedAt);
  if (Number.isNaN(t.getTime())) {
    return "";
  }
  return ` · published ${formatCheckedAt(t)} (${formatCompactAge(now.getTime() - t.getTime())} ago)`;
}

/** Annotate a live local machine line with this check’s wall-clock time. */
export function formatCheckedSuffix(checkedAt: Date | undefined): string {
  if (!checkedAt) {
    return "";
  }
  return ` · checked ${formatCheckedAt(checkedAt)}`;
}

/**
 * Human-readable Output channel body for the latest check.
 * When `checkedAt` is set, a “Checked” header is prepended and local machine
 * lines include that time. Remote machine lines include snapshot publish time.
 */
export function formatDetails(
  outcomes: FolderOutcome[],
  checkedAt?: Date,
  now: Date = new Date()
): string {
  const lines: string[] = [];
  if (checkedAt) {
    lines.push(`Checked: ${formatCheckedAt(checkedAt)}`);
    lines.push("");
  }
  for (const o of outcomes) {
    if (o.kind === "missing_binary") {
      lines.push(o.message);
      continue;
    }
    if (o.kind === "not_git") {
      lines.push(`${o.folder}: (not a git work tree — skipped)`);
      continue;
    }
    if (o.kind === "error") {
      lines.push(`${o.folder}: error — ${o.message}`);
      if (o.result?.error && o.result.error !== o.message) {
        lines.push(`  ${o.result.error}`);
      }
      if (o.stderr && o.stderr !== o.message) {
        lines.push(`  stderr: ${o.stderr}`);
      }
      continue;
    }
    const r = o.result;
    const project = r.project ?? o.folder;
    lines.push(project);
    const machines = [...(r.machines ?? [])].sort((a, b) => {
      if (a.local !== b.local) {
        return a.local ? -1 : 1;
      }
      return a.id.localeCompare(b.id);
    });
    for (const m of machines) {
      lines.push(`  ${formatMachineLine(m, now, checkedAt)}`);
    }
    if (machines.length === 0) {
      lines.push("  (no machine status)");
    }
    if ((r.situations ?? []).length === 0) {
      lines.push("→ ok");
    } else {
      for (const s of r.situations ?? []) {
        if (s.nudge.trim()) {
          lines.push(`→ ${s.nudge}`);
        }
      }
    }
    lines.push("");
  }
  return lines.join("\n").trimEnd();
}

function formatMachineLine(
  m: CheckJSONMachine,
  now: Date = new Date(),
  checkedAt?: Date
): string {
  let id = m.id;
  if (m.local) {
    id += "*";
  }
  if (m.stale) {
    id += " (stale)";
  }
  // Match CLI printCheckSummary / formatCheckMachineCell (plain repoStatusText).
  let cell = formatCheckMachineCell(m);
  if (m.local) {
    cell += formatCheckedSuffix(checkedAt);
  } else {
    cell += formatPublishedSuffix(m.updated_at, now);
  }
  return `${id}: ${cell}`;
}

/** Mirrors Go formatCheckMachineCell / repoStatusText(plain) + branch assembly. */
export function formatCheckMachineCell(m: CheckJSONMachine): string {
  const [st, ch] = plainRepoStatusAndChanges(m);
  let cell = st;
  if (m.branch) {
    cell = `${st} on ${m.branch}`;
  }
  if (ch && ch !== "-") {
    cell = `${cell} (${ch})`;
  }
  return cell;
}

/** Mirrors Go repoStatusText(repo, true). */
function plainRepoStatusAndChanges(m: CheckJSONMachine): [string, string] {
  if (m.error) {
    return ["Error", m.error];
  }
  const changes = snapshotChangesList(m).join(", ");
  if (m.is_dirty) {
    return ["Dirty", changes];
  }
  if (m.is_empty) {
    return ["Empty", "no commits yet"];
  }
  if (m.has_untracked_upstream) {
    return ["UntrackedUpstream", "untracked-upstream"];
  }
  if (m.has_behind && m.has_unpushed) {
    return ["Diverged", changes];
  }
  if (m.has_behind) {
    return ["Behind", changes];
  }
  if (m.has_unpushed) {
    return ["Unpushed", changes];
  }
  return ["Clean", "-"];
}

/** Mirrors Go snapshotChangesText. */
function snapshotChangesList(m: CheckJSONMachine): string[] {
  const changes: string[] = [];
  if (m.has_unstaged) {
    changes.push("unstaged");
  }
  if (m.has_staged) {
    changes.push("staged");
  }
  if (m.has_untracked) {
    changes.push("untracked");
  }
  if (m.has_unpushed) {
    changes.push(
      m.ahead_count && m.ahead_count > 0
        ? `unpushed:${m.ahead_count}`
        : "unpushed"
    );
  }
  if (m.has_behind) {
    changes.push(
      m.behind_count && m.behind_count > 0
        ? `behind:${m.behind_count}`
        : "behind"
    );
  }
  if (m.has_untracked_upstream) {
    changes.push("untracked-upstream");
  }
  return changes;
}
