import { FolderOutcome } from "./types";

export type StatusTier =
  | "hidden"
  | "clear"
  | "local"
  | "cross"
  | "setup"
  | "checking";

export function tierFromOutcomes(outcomes: FolderOutcome[]): StatusTier {
  if (outcomes.some((o) => o.kind === "missing_binary")) {
    return "setup";
  }
  if (outcomes.some((o) => o.kind === "attention" && o.elevated)) {
    return "cross";
  }
  if (outcomes.some((o) => o.kind === "attention")) {
    return "local";
  }
  if (outcomes.some((o) => o.kind === "error")) {
    return "local";
  }
  const checked = outcomes.filter(
    (o) => o.kind === "clear" || o.kind === "attention"
  );
  if (checked.length === 0) {
    return "hidden";
  }
  return "clear";
}

export function firstOtherMachine(outcomes: FolderOutcome[]): string | undefined {
  for (const o of outcomes) {
    if (o.kind !== "attention") {
      continue;
    }
    for (const s of o.result.situations ?? []) {
      for (const m of s.machines ?? []) {
        const local = (o.result.machines ?? []).find((x) => x.local)?.id;
        if (m && m !== local) {
          return m;
        }
      }
    }
    for (const m of o.result.machines ?? []) {
      if (!m.local && (m.is_dirty || m.has_unpushed)) {
        return m.id;
      }
    }
  }
  return undefined;
}

export function tooltipFromOutcomes(outcomes: FolderOutcome[]): string {
  const parts: string[] = ["Find Uncommitted"];
  for (const o of outcomes) {
    if (o.kind === "clear" || o.kind === "attention") {
      const project = o.result.project ?? o.folder;
      parts.push(project);
      for (const s of o.result.situations ?? []) {
        if (s.nudge) {
          parts.push(`→ ${s.nudge}`);
        }
      }
      if ((o.result.situations ?? []).length === 0) {
        parts.push("→ ok");
      }
    } else if (o.kind === "error") {
      parts.push(`${o.folder}: ${o.message}`);
    } else if (o.kind === "missing_binary") {
      parts.push(o.message);
    }
  }
  return parts.join("\n");
}
