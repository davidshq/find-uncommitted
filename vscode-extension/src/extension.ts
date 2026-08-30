import * as vscode from "vscode";
import {
  missingBinaryMessage,
  outcomeFromRun,
  resolveBinary,
  runCheckJson,
} from "./check";
import { applyStatusBar, tierFromOutcomes } from "./status";
import { FolderOutcome, formatDetails } from "./types";

let statusBar: vscode.StatusBarItem;
let output: vscode.OutputChannel;
let lastOutcomes: FolderOutcome[] = [];
let generation = 0;
let abortController: AbortController | undefined;
let refreshTimer: ReturnType<typeof setInterval> | undefined;
let openDebounce: ReturnType<typeof setTimeout> | undefined;

export function activate(context: vscode.ExtensionContext): void {
  output = vscode.window.createOutputChannel("Find Uncommitted");
  statusBar = vscode.window.createStatusBarItem(
    vscode.StatusBarAlignment.Right,
    100
  );
  statusBar.command = "findUncommitted.showDetails";
  context.subscriptions.push(output, statusBar);

  context.subscriptions.push(
    vscode.commands.registerCommand("findUncommitted.checkWorkspace", () =>
      refresh("check").then((ok) => {
        if (ok) {
          output.show(true);
        }
      })
    ),
    vscode.commands.registerCommand("findUncommitted.refresh", () =>
      refresh("refresh")
    ),
    vscode.commands.registerCommand("findUncommitted.showDetails", () => {
      showDetails();
    })
  );

  context.subscriptions.push(
    vscode.workspace.onDidChangeWorkspaceFolders(() => scheduleCheckOnOpen()),
    vscode.workspace.onDidChangeConfiguration((e) => {
      if (e.affectsConfiguration("findUncommitted")) {
        setupRefreshInterval();
        void refresh("config");
      }
    })
  );

  setupRefreshInterval();
  scheduleCheckOnOpen();

  context.subscriptions.push({
    dispose: () => {
      if (openDebounce) {
        clearTimeout(openDebounce);
      }
      if (refreshTimer) {
        clearInterval(refreshTimer);
      }
      abortController?.abort();
    },
  });
}

export function deactivate(): void {
  // disposed via subscriptions
}

function cfg() {
  return vscode.workspace.getConfiguration("findUncommitted");
}

function scheduleCheckOnOpen(): void {
  if (!cfg().get<boolean>("checkOnOpen", true)) {
    return;
  }
  if (openDebounce) {
    clearTimeout(openDebounce);
  }
  openDebounce = setTimeout(() => {
    void refresh("open");
  }, 400);
}

function setupRefreshInterval(): void {
  if (refreshTimer) {
    clearInterval(refreshTimer);
    refreshTimer = undefined;
  }
  const minutes = cfg().get<number>("refreshIntervalMinutes", 0);
  if (minutes > 0) {
    refreshTimer = setInterval(() => {
      void refresh("timer");
    }, minutes * 60_000);
  }
}

async function refresh(_reason: string): Promise<boolean> {
  const myGen = ++generation;
  abortController?.abort();
  abortController = new AbortController();
  const signal = abortController.signal;

  const hideWhenClear = cfg().get<boolean>("hideWhenClear", false);
  applyStatusBar(statusBar, "checking", [], hideWhenClear);

  const binaryPath = cfg().get<string>("binaryPath", "");
  const resolved = resolveBinary(binaryPath);
  if (!resolved.ok) {
    const outcome: FolderOutcome = {
      kind: "missing_binary",
      message: missingBinaryMessage(resolved),
    };
    lastOutcomes = [outcome];
    if (myGen !== generation) {
      return false;
    }
    applyStatusBar(statusBar, "setup", lastOutcomes, hideWhenClear);
    output.clear();
    output.appendLine(outcome.message);
    return true;
  }

  const folders = vscode.workspace.workspaceFolders ?? [];
  const outcomes: FolderOutcome[] = [];

  for (const folder of folders) {
    if (signal.aborted || myGen !== generation) {
      return false;
    }
    try {
      const raw = await runCheckJson({
        binary: resolved.path,
        folder: folder.uri.fsPath,
        signal,
      });
      if (signal.aborted || myGen !== generation) {
        return false;
      }
      outcomes.push(outcomeFromRun(folder.uri.fsPath, raw));
    } catch (err) {
      if ((err as { name?: string }).name === "AbortError") {
        return false;
      }
      outcomes.push({
        kind: "error",
        folder: folder.uri.fsPath,
        message: err instanceof Error ? err.message : String(err),
      });
    }
  }

  if (myGen !== generation) {
    return false;
  }

  lastOutcomes = outcomes;
  const tier = tierFromOutcomes(outcomes);
  applyStatusBar(statusBar, tier, outcomes, hideWhenClear);

  output.clear();
  output.appendLine(formatDetails(outcomes));
  return true;
}

function showDetails(): void {
  if (lastOutcomes.length === 0) {
    void refresh("details").then((ok) => {
      if (!ok) {
        return;
      }
      output.clear();
      output.appendLine(formatDetails(lastOutcomes));
      output.show(true);
    });
    return;
  }
  output.clear();
  output.appendLine(formatDetails(lastOutcomes));
  output.show(true);
}
