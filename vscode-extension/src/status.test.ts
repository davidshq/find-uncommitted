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

  it("returns local for errors", () => {
    assert.equal(
      tierFromOutcomes([
        { kind: "error", folder: "/a", message: "boom", stderr: "detail" },
      ]),
      "local"
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
});
