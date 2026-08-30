import * as vscode from "vscode";
import {
  StatusTier,
  firstOtherMachine,
  tierFromOutcomes,
  tooltipFromOutcomes,
} from "./tiers";
import { FolderOutcome } from "./types";

export { tierFromOutcomes, type StatusTier };

export function applyStatusBar(
  item: vscode.StatusBarItem,
  tier: StatusTier,
  outcomes: FolderOutcome[],
  hideWhenClear: boolean
): void {
  switch (tier) {
    case "checking":
      item.text = "$(sync~spin) FU";
      item.tooltip = "Find Uncommitted: checking…";
      item.backgroundColor = undefined;
      item.show();
      return;
    case "setup": {
      const msg =
        outcomes.find((o) => o.kind === "missing_binary")?.message ??
        "find-uncommitted binary not found.\nInstall the CLI or set findUncommitted.binaryPath.";
      item.text = "$(warning) FU · setup";
      item.tooltip = msg;
      item.backgroundColor = new vscode.ThemeColor(
        "statusBarItem.warningBackground"
      );
      item.show();
      return;
    }
    case "cross": {
      const other = firstOtherMachine(outcomes);
      item.text = other
        ? `$(warning) FU · ${other}`
        : "$(warning) FU · other machine";
      item.tooltip = tooltipFromOutcomes(outcomes);
      item.backgroundColor = new vscode.ThemeColor(
        "statusBarItem.warningBackground"
      );
      item.show();
      return;
    }
    case "local":
      item.text = "FU · dirty";
      item.tooltip = tooltipFromOutcomes(outcomes);
      item.backgroundColor = undefined;
      item.show();
      return;
    case "clear":
      item.text = "FU · ok";
      item.tooltip = tooltipFromOutcomes(outcomes);
      item.backgroundColor = undefined;
      if (hideWhenClear) {
        item.hide();
      } else {
        item.show();
      }
      return;
    case "hidden":
      item.hide();
      return;
  }
}
