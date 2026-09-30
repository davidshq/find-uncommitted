import assert from "node:assert/strict";
import { describe, it } from "node:test";
import { tierFromOutcomes } from "./tiers";
import { FolderOutcome, formatDetails } from "./types";

const clear: FolderOutcome = {
  kind: "clear",
  folder: "/a",
  result: { schemaVersion: 1, ok: true, attention: false, project: "a" },
};

const localAtt: FolderOutcome = {
  kind: "attention",
  folder: "/a",
  elevated: false,
  result: {
    schemaVersion: 1,
    ok: false,
    attention: true,
    project: "a",
    situations: [{ kind: "local_dirty", nudge: "commit" }],
  },
};

const crossAtt: FolderOutcome = {
  kind: "attention",
  folder: "/a",
  elevated: true,
  result: {
    schemaVersion: 1,
    ok: false,
    attention: true,
    project: "a",
    situations: [{ kind: "other_machine_work", nudge: "desktop dirty" }],
  },
};

describe("tierFromOutcomes", () => {
  it("returns setup for missing binary", () => {
    assert.equal(
      tierFromOutcomes([
        { kind: "missing_binary", message: "nope" },
      ]),
      "setup"
    );
  });

  it("prefers cross over local", () => {
    assert.equal(tierFromOutcomes([localAtt, crossAtt]), "cross");
  });

  it("returns local for quiet attention", () => {
    assert.equal(tierFromOutcomes([localAtt]), "local");
  });

  it("returns error tier for check failures (not dirty)", () => {
    assert.equal(
      tierFromOutcomes([
        { kind: "error", folder: "/a", message: "boom", stderr: "detail" },
      ]),
      "error"
    );
  });

  it("prefers error over local dirty", () => {
    assert.equal(
      tierFromOutcomes([
        localAtt,
        { kind: "error", folder: "/b", message: "timed out" },
      ]),
      "error"
    );
  });

  it("prefers cross over error", () => {
    assert.equal(
      tierFromOutcomes([
        crossAtt,
        { kind: "error", folder: "/b", message: "timed out" },
      ]),
      "cross"
    );
  });

  it("returns clear when checked folders are ok", () => {
    assert.equal(tierFromOutcomes([clear]), "clear");
  });

  it("returns hidden when only non-git folders", () => {
    assert.equal(
      tierFromOutcomes([{ kind: "not_git", folder: "/docs" }]),
      "hidden"
    );
  });
});

describe("formatDetails", () => {
  it("includes stderr on errors", () => {
    const text = formatDetails([
      {
        kind: "error",
        folder: "/a",
        message: "timed out",
        stderr: "check timed out after 30000ms",
      },
    ]);
    assert.match(text, /timed out/);
    assert.match(text, /stderr:/);
  });

  it("renders attention nudges", () => {
    const text = formatDetails([crossAtt]);
    assert.match(text, /desktop dirty/);
  });

  it("lists local machine first, one per line", () => {
    const text = formatDetails([
      {
        kind: "attention",
        folder: "/a",
        elevated: true,
        result: {
          schemaVersion: 1,
          ok: false,
          attention: true,
          project: "github.com/acme/app",
          machines: [
            {
              id: "DMHP",
              local: false,
              stale: true,
              branch: "main",
              is_clean: true,
            },
            {
              id: "XPS-8950",
              local: true,
              branch: "main",
              is_dirty: true,
              has_unstaged: true,
            },
          ],
          situations: [{ kind: "local_dirty", nudge: "commit or stash" }],
        },
      },
    ]);
    const lines = text.split("\n");
    assert.equal(lines[0], "github.com/acme/app");
    assert.match(lines[1], /^  XPS-8950\*: Dirty on main \(unstaged\)/);
    assert.match(lines[2], /^  DMHP \(stale\): Clean on main/);
    assert.match(lines[3], /commit or stash/);
  });

  it("prepends Checked when checkedAt is provided", () => {
    const checkedAt = new Date(2026, 8, 29, 15, 11, 42);
    const text = formatDetails([clear], checkedAt);
    const lines = text.split("\n");
    assert.equal(lines[0], "Checked: 2026-09-29 15:11:42");
    assert.equal(lines[1], "");
    assert.equal(lines[2], "a");
  });

  it("annotates local checked and remote published times on machine lines", () => {
    const checkedAt = new Date(2026, 8, 29, 15, 11, 42);
    const now = checkedAt;
    const text = formatDetails(
      [
        {
          kind: "attention",
          folder: "/a",
          elevated: true,
          result: {
            schemaVersion: 1,
            ok: false,
            attention: true,
            project: "github.com/acme/app",
            machines: [
              {
                id: "XPS",
                local: true,
                branch: "main",
                is_dirty: true,
                has_unstaged: true,
              },
              {
                id: "DMHP",
                local: false,
                stale: true,
                branch: "main",
                is_clean: true,
                updated_at: "2026-09-28T12:00:00Z",
              },
            ],
            situations: [{ kind: "other_machine_work", nudge: "resolve" }],
          },
        },
      ],
      checkedAt,
      now
    );
    const lines = text.split("\n");
    assert.equal(lines[0], "Checked: 2026-09-29 15:11:42");
    assert.match(
      lines[3],
      /^  XPS\*: Dirty on main \(unstaged\) · checked 2026-09-29 15:11:42$/
    );
    assert.match(
      lines[4],
      /^  DMHP \(stale\): Clean on main · published .+ \(.* ago\)$/
    );
  });
});
