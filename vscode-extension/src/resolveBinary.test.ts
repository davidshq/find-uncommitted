import assert from "node:assert/strict";
import * as fs from "node:fs";
import * as os from "node:os";
import * as path from "node:path";
import { describe, it } from "node:test";
import {
  isRunnableFile,
  missingBinaryMessage,
  resolveBinary,
} from "./check";

describe("resolveBinary", () => {
  it("reports configured_missing when path does not exist", () => {
    const r = resolveBinary("/no/such/find-uncommitted-binary-xyz");
    assert.equal(r.ok, false);
    if (!r.ok) {
      assert.equal(r.reason, "configured_missing");
      assert.match(missingBinaryMessage(r), /Configured/);
    }
  });

  it("accepts a configured executable file", () => {
    const dir = fs.mkdtempSync(path.join(os.tmpdir(), "fu-bin-"));
    const bin = path.join(dir, "find-uncommitted");
    fs.writeFileSync(bin, "#!/bin/sh\necho ok\n", { mode: 0o755 });
    try {
      const r = resolveBinary(bin);
      assert.equal(r.ok, true);
      if (r.ok) {
        assert.equal(r.path, bin);
      }
    } finally {
      fs.rmSync(dir, { recursive: true, force: true });
    }
  });

  it("rejects a non-file configured path", () => {
    const dir = fs.mkdtempSync(path.join(os.tmpdir(), "fu-dir-"));
    try {
      const r = resolveBinary(dir);
      assert.equal(r.ok, false);
      if (!r.ok) {
        assert.equal(r.reason, "configured_missing");
      }
    } finally {
      fs.rmSync(dir, { recursive: true, force: true });
    }
  });

  it("rejects a non-executable configured file", () => {
    if (process.platform === "win32") {
      return;
    }
    const dir = fs.mkdtempSync(path.join(os.tmpdir(), "fu-nox-"));
    const bin = path.join(dir, "find-uncommitted");
    fs.writeFileSync(bin, "#!/bin/sh\necho ok\n", { mode: 0o644 });
    try {
      const r = resolveBinary(bin);
      assert.equal(r.ok, false);
      if (!r.ok) {
        assert.equal(r.reason, "configured_missing");
      }
    } finally {
      fs.rmSync(dir, { recursive: true, force: true });
    }
  });
});

describe("isRunnableFile", () => {
  it("is false for missing paths", () => {
    assert.equal(isRunnableFile("/definitely/missing/fu"), false);
  });
});
