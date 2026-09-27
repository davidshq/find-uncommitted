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

export function formatDetails(outcomes: FolderOutcome[]): string {
  const lines: string[] = [];
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
      lines.push(`  ${formatMachineLine(m)}`);
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

function formatMachineLine(m: CheckJSONMachine): string {
  let id = m.id;
  if (m.local) {
    id += "*";
  }
  if (m.stale) {
    id += " (stale)";
  }
  // Match CLI printCheckSummary / formatCheckMachineCell (plain repoStatusText).
  return `${id}: ${formatCheckMachineCell(m)}`;
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
