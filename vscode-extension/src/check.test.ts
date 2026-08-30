import assert from "node:assert/strict";
import { describe, it } from "node:test";
import { outcomeFromRun, parseResult } from "./check";
import { isElevated } from "./types";

describe("parseResult", () => {
  it("parses valid JSON", () => {
    const r = parseResult('{"schemaVersion":1,"ok":true,"attention":false}');
    assert.equal(r?.ok, true);
  });

  it("returns undefined for garbage", () => {
    assert.equal(parseResult("not json"), undefined);
  });
});

describe("outcomeFromRun", () => {
  it("maps exit 0 to clear", () => {
    const o = outcomeFromRun("/repo", {
      exitCode: 0,
      stdout: '{"schemaVersion":1,"ok":true,"attention":false,"project":"app"}',
      stderr: "",
    });
    assert.equal(o.kind, "clear");
    if (o.kind === "clear") {
      assert.equal(o.result.project, "app");
    }
  });

  it("maps exit 2 with JSON to attention and elevated for cross-machine", () => {
    const o = outcomeFromRun("/repo", {
      exitCode: 2,
      stdout: JSON.stringify({
        schemaVersion: 1,
        ok: false,
        attention: true,
        situations: [
          {
            kind: "other_machine_work",
            nudge: "unfinished on desktop",
            machines: ["desktop"],
          },
        ],
      }),
      stderr: "",
    });
    assert.equal(o.kind, "attention");
    if (o.kind === "attention") {
      assert.equal(o.elevated, true);
      assert.equal(o.result.situations?.[0]?.kind, "other_machine_work");
    }
  });

  it("maps exit 2 without JSON to elevated attention (not quiet local)", () => {
    const o = outcomeFromRun("/repo", {
      exitCode: 2,
      stdout: "laptop*: clean  ·  desktop: dirty\n→ unfinished work on desktop",
      stderr: "",
    });
    assert.equal(o.kind, "attention");
    if (o.kind === "attention") {
      assert.equal(o.elevated, true);
      assert.match(o.result.situations?.[0]?.nudge ?? "", /desktop/);
    }
  });

  it("maps exit 2 empty stdout to elevated attention with fallback nudge", () => {
    const o = outcomeFromRun("/repo", {
      exitCode: 2,
      stdout: "",
      stderr: "",
    });
    assert.equal(o.kind, "attention");
    if (o.kind === "attention") {
      assert.equal(o.elevated, true);
      assert.match(o.result.situations?.[0]?.nudge ?? "", /unparseable/);
    }
  });

  it("maps exit 2 from stale CLI (unknown --json) as error not attention", () => {
    const o = outcomeFromRun("/repo", {
      exitCode: 2,
      stdout: "",
      stderr:
        "flag provided but not defined: -json\nUsage of /path/find-uncommitted:\n  -agent\n",
    });
    assert.equal(o.kind, "error");
    if (o.kind === "error") {
      assert.match(o.message, /too old|Rebuild/i);
    }
  });

  it("maps local dirty JSON as quiet attention", () => {
    const o = outcomeFromRun("/repo", {
      exitCode: 2,
      stdout: JSON.stringify({
        schemaVersion: 1,
        ok: false,
        attention: true,
        situations: [{ kind: "local_dirty", nudge: "commit or stash" }],
      }),
      stderr: "",
    });
    assert.equal(o.kind, "attention");
    if (o.kind === "attention") {
      assert.equal(o.elevated, false);
    }
  });

  it("skips non-git exit 1 quietly", () => {
    const o = outcomeFromRun("/tmp/nogit", {
      exitCode: 1,
      stdout: JSON.stringify({
        schemaVersion: 1,
        ok: false,
        attention: false,
        error: '"/tmp/nogit" is not inside a git work tree: fatal: ...',
      }),
      stderr: "",
    });
    assert.equal(o.kind, "not_git");
  });

  it("maps other exit 1 to error and keeps stderr", () => {
    const o = outcomeFromRun("/repo", {
      exitCode: 1,
      stdout: "",
      stderr: "state repo boom",
      timedOut: true,
    });
    assert.equal(o.kind, "error");
    if (o.kind === "error") {
      assert.match(o.message, /boom|timed out/);
      assert.equal(o.stderr, "state repo boom");
    }
  });
});

describe("isElevated", () => {
  it("is false for local-only situations", () => {
    assert.equal(
      isElevated({
        schemaVersion: 1,
        ok: false,
        attention: true,
        situations: [{ kind: "local_unpushed", nudge: "push" }],
      }),
      false
    );
  });

  it("is true for tip_mismatch", () => {
    assert.equal(
      isElevated({
        schemaVersion: 1,
        ok: false,
        attention: true,
        situations: [{ kind: "tip_mismatch", nudge: "tips differ" }],
      }),
      true
    );
  });
});
