import assert from "node:assert/strict";
import { describe, it } from "node:test";
import {
  NotificationEpisodeTracker,
  notificationMessage,
  episodeFingerprint,
  parseAttentionDisplay,
  shouldShowAttentionNotification,
} from "./notification";
import { FolderOutcome } from "./types";

const localAtt: FolderOutcome = {
  kind: "attention",
  folder: "/a",
  elevated: false,
  result: {
    schemaVersion: 1,
    ok: false,
    attention: true,
    project: "a",
    situations: [{ kind: "local_dirty", nudge: "commit locally" }],
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
    situations: [
      {
        kind: "other_machine_work",
        nudge: "desktop dirty",
        machines: ["laptop", "desktop"],
      },
    ],
  },
};

const crossAttAlt: FolderOutcome = {
  kind: "attention",
  folder: "/a",
  elevated: true,
  result: {
    schemaVersion: 1,
    ok: false,
    attention: true,
    project: "a",
    situations: [
      {
        kind: "branch_mismatch",
        nudge: "branches differ",
        machines: ["desktop"],
      },
    ],
  },
};

const clear: FolderOutcome = {
  kind: "clear",
  folder: "/a",
  result: { schemaVersion: 1, ok: true, attention: false, project: "a" },
};

describe("parseAttentionDisplay", () => {
  it("defaults unknown and legacy banner to notification", () => {
    assert.equal(parseAttentionDisplay(undefined), "notification");
    assert.equal(parseAttentionDisplay("banner"), "notification");
    assert.equal(parseAttentionDisplay("notification"), "notification");
    assert.equal(parseAttentionDisplay("statusBar"), "statusBar");
  });
});

describe("episodeFingerprint", () => {
  it("returns undefined for local-only attention", () => {
    assert.equal(episodeFingerprint([localAtt]), undefined);
  });

  it("returns a stable string for cross-machine attention", () => {
    const fp = episodeFingerprint([crossAtt]);
    assert.ok(fp);
    assert.match(fp!, /other_machine_work/);
    assert.match(fp!, /desktop dirty/);
    assert.match(fp!, /desktop,laptop/);
  });

  it("changes when the attention set changes", () => {
    assert.notEqual(
      episodeFingerprint([crossAtt]),
      episodeFingerprint([crossAttAlt])
    );
  });
});

describe("shouldShowAttentionNotification", () => {
  const fp = episodeFingerprint([crossAtt])!;

  it("shows for notification mode with a new fingerprint", () => {
    assert.equal(
      shouldShowAttentionNotification({
        display: "notification",
        fingerprint: fp,
        lastShownFingerprint: undefined,
      }),
      true
    );
  });

  it("does not show in statusBar mode", () => {
    assert.equal(
      shouldShowAttentionNotification({
        display: "statusBar",
        fingerprint: fp,
        lastShownFingerprint: undefined,
      }),
      false
    );
  });

  it("does not show without a fingerprint (local / clear)", () => {
    assert.equal(
      shouldShowAttentionNotification({
        display: "notification",
        fingerprint: undefined,
        lastShownFingerprint: undefined,
      }),
      false
    );
  });

  it("does not re-show the same episode", () => {
    assert.equal(
      shouldShowAttentionNotification({
        display: "notification",
        fingerprint: fp,
        lastShownFingerprint: fp,
      }),
      false
    );
  });
});

describe("notificationMessage", () => {
  it("includes project and nudge", () => {
    assert.match(notificationMessage([crossAtt]), /desktop dirty/);
    assert.match(notificationMessage([crossAtt]), /a:/);
  });
});

describe("NotificationEpisodeTracker", () => {
  it("shows once per episode then suppresses refresh spam", () => {
    const tracker = new NotificationEpisodeTracker();
    const first = tracker.takeShowDecision("notification", [crossAtt]);
    assert.equal(first.show, true);
    const second = tracker.takeShowDecision("notification", [crossAtt]);
    assert.equal(second.show, false);
  });

  it("never shows for local-only or statusBar mode", () => {
    const tracker = new NotificationEpisodeTracker();
    assert.equal(
      tracker.takeShowDecision("notification", [localAtt]).show,
      false
    );
    assert.equal(
      tracker.takeShowDecision("statusBar", [crossAtt]).show,
      false
    );
  });

  it("shows again after clear then new attention", () => {
    const tracker = new NotificationEpisodeTracker();
    assert.equal(
      tracker.takeShowDecision("notification", [crossAtt]).show,
      true
    );
    assert.equal(tracker.takeShowDecision("notification", [clear]).show, false);
    assert.equal(
      tracker.takeShowDecision("notification", [crossAtt]).show,
      true
    );
  });

  it("shows again when the attention set changes", () => {
    const tracker = new NotificationEpisodeTracker();
    assert.equal(
      tracker.takeShowDecision("notification", [crossAtt]).show,
      true
    );
    assert.equal(
      tracker.takeShowDecision("notification", [crossAttAlt]).show,
      true
    );
  });

  it("reset allows re-show of the same episode", () => {
    const tracker = new NotificationEpisodeTracker();
    assert.equal(
      tracker.takeShowDecision("notification", [crossAtt]).show,
      true
    );
    tracker.reset();
    assert.equal(
      tracker.takeShowDecision("notification", [crossAtt]).show,
      true
    );
  });

  it("dismiss marks the episode as shown", () => {
    const tracker = new NotificationEpisodeTracker();
    const fp = episodeFingerprint([crossAtt])!;
    tracker.dismiss(fp);
    assert.equal(
      tracker.takeShowDecision("notification", [crossAtt]).show,
      false
    );
  });
});
