import assert from "node:assert/strict";
import { describe, it } from "node:test";
import { formatCheckMachineCell } from "./types";

describe("formatCheckMachineCell", () => {
  it("matches CLI Dirty cell with changes", () => {
    assert.equal(
      formatCheckMachineCell({
        id: "x",
        local: true,
        branch: "main",
        is_dirty: true,
        has_unstaged: true,
        has_staged: true,
      }),
      "Dirty on main (unstaged, staged)"
    );
  });

  it("matches CLI Clean cell", () => {
    assert.equal(
      formatCheckMachineCell({
        id: "x",
        local: false,
        branch: "main",
        is_clean: true,
      }),
      "Clean on main"
    );
  });

  it("matches CLI Unpushed with count", () => {
    assert.equal(
      formatCheckMachineCell({
        id: "x",
        local: true,
        branch: "feat",
        has_unpushed: true,
        ahead_count: 3,
      }),
      "Unpushed on feat (unpushed:3)"
    );
  });

  it("matches CLI Empty", () => {
    assert.equal(
      formatCheckMachineCell({
        id: "x",
        local: true,
        branch: "main",
        is_empty: true,
      }),
      "Empty on main (no commits yet)"
    );
  });
});
