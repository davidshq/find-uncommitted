import { spawn } from "child_process";
import * as fs from "fs";
import * as path from "path";
import { CheckJSONResult, FolderOutcome, isElevated } from "./types";

/** Default kill deadline for a single `check --json` subprocess. */
export const CHECK_TIMEOUT_MS = 30_000;

const BINARY_NAMES =
  process.platform === "win32"
    ? ["find-uncommitted.exe", "find-uncommitted"]
    : ["find-uncommitted"];

export type BinaryResolve =
  | { ok: true; path: string }
  | { ok: false; reason: "configured_missing"; configuredPath: string }
  | { ok: false; reason: "not_on_path" };

/**
 * Resolve the CLI binary: setting override, then PATH lookup.
 * Requires a regular file that is executable (Windows: file exists).
 */
export function resolveBinary(configuredPath: string): BinaryResolve {
  const trimmed = configuredPath.trim();
  if (trimmed) {
    if (isRunnableFile(trimmed)) {
      return { ok: true, path: trimmed };
    }
    return { ok: false, reason: "configured_missing", configuredPath: trimmed };
  }
  const pathEnv = process.env.PATH ?? "";
  const dirs = pathEnv.split(path.delimiter);
  for (const dir of dirs) {
    if (!dir) {
      continue;
    }
    for (const name of BINARY_NAMES) {
      const candidate = path.join(dir, name);
      if (isRunnableFile(candidate)) {
        return { ok: true, path: candidate };
      }
    }
  }
  return { ok: false, reason: "not_on_path" };
}

export function missingBinaryMessage(resolved: Exclude<BinaryResolve, { ok: true }>): string {
  if (resolved.reason === "configured_missing") {
    return `Configured findUncommitted.binaryPath is missing or not executable: ${resolved.configuredPath}`;
  }
  return "find-uncommitted binary not found on PATH. Install the CLI or set findUncommitted.binaryPath (GUI apps often lack shell PATH).";
}

export function isRunnableFile(filePath: string): boolean {
  try {
    const st = fs.statSync(filePath);
    if (!st.isFile()) {
      return false;
    }
    if (process.platform === "win32") {
      return true;
    }
    fs.accessSync(filePath, fs.constants.X_OK);
    return true;
  } catch {
    return false;
  }
}

export interface RunCheckOptions {
  binary: string;
  folder: string;
  signal?: AbortSignal;
  /** Kill the child after this many ms (default CHECK_TIMEOUT_MS). */
  timeoutMs?: number;
}

export interface RunCheckRaw {
  exitCode: number;
  stdout: string;
  stderr: string;
  timedOut?: boolean;
}

/**
 * Run `find-uncommitted --json check <folder>` asynchronously.
 * Does not mutate git state — check is read-only from the extension's perspective.
 */
export function runCheckJson(opts: RunCheckOptions): Promise<RunCheckRaw> {
  const timeoutMs = opts.timeoutMs ?? CHECK_TIMEOUT_MS;
  return new Promise((resolve, reject) => {
    if (opts.signal?.aborted) {
      reject(Object.assign(new Error("aborted"), { name: "AbortError" }));
      return;
    }
    const child = spawn(opts.binary, ["--json", "check", opts.folder], {
      env: process.env,
      windowsHide: true,
    });
    let stdout = "";
    let stderr = "";
    let timedOut = false;
    let settled = false;

    const timer = setTimeout(() => {
      timedOut = true;
      child.kill("SIGKILL");
    }, timeoutMs);

    const finish = (fn: () => void) => {
      if (settled) {
        return;
      }
      settled = true;
      clearTimeout(timer);
      opts.signal?.removeEventListener("abort", onAbort);
      fn();
    };

    const onAbort = () => {
      child.kill("SIGKILL");
    };
    opts.signal?.addEventListener("abort", onAbort);

    child.stdout?.on("data", (chunk: Buffer) => {
      stdout += chunk.toString("utf8");
    });
    child.stderr?.on("data", (chunk: Buffer) => {
      stderr += chunk.toString("utf8");
    });
    child.on("error", (err) => {
      finish(() => reject(err));
    });
    child.on("close", (code) => {
      finish(() => {
        if (opts.signal?.aborted && !timedOut) {
          reject(Object.assign(new Error("aborted"), { name: "AbortError" }));
          return;
        }
        if (timedOut) {
          resolve({
            exitCode: 1,
            stdout,
            stderr:
              (stderr ? stderr + "\n" : "") +
              `check timed out after ${timeoutMs}ms`,
            timedOut: true,
          });
          return;
        }
        resolve({ exitCode: code ?? 1, stdout, stderr });
      });
    });
  });
}

export function parseResult(stdout: string): CheckJSONResult | undefined {
  const trimmed = stdout.trim();
  if (!trimmed) {
    return undefined;
  }
  try {
    return JSON.parse(trimmed) as CheckJSONResult;
  } catch {
    return undefined;
  }
}

/**
 * Map CLI exit + JSON to a folder outcome. Exit 1 for non-git is quiet skip.
 * Exit 2 counts as attention even when stdout is not valid JSON.
 */
export function outcomeFromRun(
  folder: string,
  raw: RunCheckRaw
): FolderOutcome {
  const result = parseResult(raw.stdout);
  if (raw.exitCode === 0) {
    return {
      kind: "clear",
      folder,
      result: result ?? {
        schemaVersion: 1,
        ok: true,
        attention: false,
      },
    };
  }
  if (raw.exitCode === 2) {
    // Go's flag package also exits 2 for unknown flags (e.g. stale binary
    // without --json). That is not Attention — surface as setup/error.
    if (!result) {
      const errBlob = `${raw.stderr}\n${raw.stdout}`.toLowerCase();
      if (
        errBlob.includes("flag provided but not defined") ||
        errBlob.includes("usage of ") ||
        errBlob.includes("unknown check flag")
      ) {
        return {
          kind: "error",
          folder,
          message:
            "CLI rejected --json (binary too old?). Rebuild find-uncommitted or update findUncommitted.binaryPath.",
          stderr: raw.stderr.trim() || undefined,
        };
      }
      // No parseable JSON: keep attention and elevate — fail loud rather than
      // mislabel cross-machine work as quiet local dirty.
      const fallback: CheckJSONResult = {
        schemaVersion: 1,
        ok: false,
        attention: true,
        situations: [
          {
            kind: "other_machine_work",
            nudge: raw.stdout.trim()
              ? raw.stdout.trim().slice(0, 500)
              : "attention (unparseable check output)",
          },
        ],
      };
      return {
        kind: "attention",
        folder,
        result: fallback,
        elevated: true,
      };
    }
    return {
      kind: "attention",
      folder,
      result,
      elevated: isElevated(result),
    };
  }
  if (raw.exitCode === 1) {
    const errText = (result?.error ?? raw.stderr ?? "").toLowerCase();
    if (
      errText.includes("not inside a git work tree") ||
      errText.includes("not a git")
    ) {
      return { kind: "not_git", folder };
    }
    return {
      kind: "error",
      folder,
      message:
        result?.error ||
        raw.stderr.trim() ||
        (raw.timedOut ? "check timed out" : `exit ${raw.exitCode}`),
      result,
      stderr: raw.stderr.trim() || undefined,
    };
  }
  return {
    kind: "error",
    folder,
    message: result?.error || raw.stderr.trim() || `exit ${raw.exitCode}`,
    result,
    stderr: raw.stderr.trim() || undefined,
  };
}
