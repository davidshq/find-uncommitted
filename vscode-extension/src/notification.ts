import { CROSS_MACHINE_KINDS, FolderOutcome } from "./types";

/**
 * How to surface cross-machine attention.
 * Default is the usual VS Code warning notification (`showWarningMessage`).
 * Legacy setting value `"banner"` is accepted as an alias of `"notification"`.
 */
export type AttentionDisplay = "notification" | "statusBar";

export const NOTIFICATION_ACTIONS = {
  showDetails: "Show Details",
  openSettings: "Open Settings",
  dismiss: "Dismiss",
} as const;

/** Normalize config / legacy values to AttentionDisplay. */
export function parseAttentionDisplay(raw: string | undefined): AttentionDisplay {
  if (raw === "statusBar") {
    return "statusBar";
  }
  // "notification", legacy "banner", unset, or unknown → notification
  return "notification";
}

/**
 * Stable fingerprint of elevated cross-machine situations across folders.
 * `undefined` means there is no cross-machine episode (no notification).
 */
export function episodeFingerprint(
  outcomes: FolderOutcome[]
): string | undefined {
  const parts: string[] = [];
  for (const o of outcomes) {
    if (o.kind !== "attention" || !o.elevated) {
      continue;
    }
    for (const s of o.result.situations ?? []) {
      if (!CROSS_MACHINE_KINDS.has(s.kind)) {
        continue;
      }
      const machines = [...(s.machines ?? [])].sort().join(",");
      parts.push(`${o.folder}|${s.kind}|${s.nudge}|${machines}`);
    }
  }
  if (parts.length === 0) {
    return undefined;
  }
  return parts.sort().join("\n");
}

/**
 * Whether to show the warning notification for this check result.
 * Once an episode fingerprint has been shown (or dismissed), refreshes with the
 * same fingerprint must not re-spam; a clear or materially different set may.
 */
export function shouldShowAttentionNotification(opts: {
  display: AttentionDisplay;
  fingerprint: string | undefined;
  lastShownFingerprint: string | undefined;
}): boolean {
  if (opts.display !== "notification") {
    return false;
  }
  if (!opts.fingerprint) {
    return false;
  }
  if (opts.fingerprint === opts.lastShownFingerprint) {
    return false;
  }
  return true;
}

/** Short message body for the notification (CLI nudge text). */
export function notificationMessage(outcomes: FolderOutcome[]): string {
  const nudges: string[] = [];
  for (const o of outcomes) {
    if (o.kind !== "attention" || !o.elevated) {
      continue;
    }
    const project = o.result.project ?? o.folder;
    for (const s of o.result.situations ?? []) {
      if (!CROSS_MACHINE_KINDS.has(s.kind) || !s.nudge.trim()) {
        continue;
      }
      nudges.push(`${project}: ${s.nudge}`);
    }
  }
  if (nudges.length === 0) {
    return "Find Uncommitted: other machine needs attention";
  }
  if (nudges.length === 1) {
    return `Find Uncommitted — ${nudges[0]}`;
  }
  return `Find Uncommitted — ${nudges[0]} (+${nudges.length - 1} more)`;
}

/**
 * Tracks which cross-machine episode was already surfaced so refreshes do not
 * re-show the notification. Pure state helper — UI lives in the extension host.
 */
export class NotificationEpisodeTracker {
  private lastShownFingerprint: string | undefined;

  get lastShown(): string | undefined {
    return this.lastShownFingerprint;
  }

  /**
   * Decide whether to show; if yes, mark the fingerprint as shown immediately
   * so overlapping async notifications for the same episode are suppressed.
   */
  takeShowDecision(
    display: AttentionDisplay,
    outcomes: FolderOutcome[]
  ): { show: boolean; message: string; fingerprint: string } | { show: false } {
    const fingerprint = episodeFingerprint(outcomes);
    if (!fingerprint) {
      // Cleared: allow a future episode to notify again.
      this.lastShownFingerprint = undefined;
      return { show: false };
    }
    const show = shouldShowAttentionNotification({
      display,
      fingerprint,
      lastShownFingerprint: this.lastShownFingerprint,
    });
    if (!show) {
      return { show: false };
    }
    this.lastShownFingerprint = fingerprint;
    return {
      show: true,
      message: notificationMessage(outcomes),
      fingerprint,
    };
  }

  /** Explicit dismiss — same as having shown this fingerprint. */
  dismiss(fingerprint: string): void {
    this.lastShownFingerprint = fingerprint;
  }

  /** Clear episode memory (e.g. user switched display mode back to notification). */
  reset(): void {
    this.lastShownFingerprint = undefined;
  }
}
