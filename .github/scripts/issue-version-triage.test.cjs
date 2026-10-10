const test = require("node:test");
const assert = require("node:assert/strict");
const fs = require("node:fs");
const path = require("node:path");

const triage = require("./issue-version-triage.cjs");

function createGithub({
  latestVersion = "6.0.1",
  existingLabels = new Set(),
  existingComments = [],
  issues = [],
  currentIssue = null,
} = {}) {
  const calls = {
    createComment: [],
    createLabel: [],
    getLabel: [],
    getLatestRelease: [],
    paginate: [],
    addLabels: [],
    removeLabel: [],
    getIssue: [],
  };

  const github = {
    currentIssue,
    rest: {
      issues: {
        async get(payload) {
          calls.getIssue.push(payload);
          return { data: github.currentIssue };
        },
        async getLabel({ name }) {
          calls.getLabel.push(name);
          if (existingLabels.has(name)) {
            return { data: { name } };
          }
          const error = new Error("Not Found");
          error.status = 404;
          throw error;
        },
        async createLabel(payload) {
          calls.createLabel.push(payload);
          existingLabels.add(payload.name);
          return { data: payload };
        },
        async addLabels(payload) {
          calls.addLabels.push(payload);
          return { data: payload };
        },
        async removeLabel(payload) {
          calls.removeLabel.push(payload);
          return { data: payload };
        },
        async createComment(payload) {
          calls.createComment.push(payload);
          return { data: payload };
        },
        listForRepo: Symbol("listForRepo"),
        listComments: Symbol("listComments"),
      },
      repos: {
        async getLatestRelease() {
          calls.getLatestRelease.push(true);
          return { data: { tag_name: `v${latestVersion}` } };
        },
      },
    },
    async paginate(endpoint) {
      calls.paginate.push(endpoint);
      return endpoint === github.rest.issues.listForRepo ? issues : existingComments;
    },
  };

  return { github, calls };
}

// Existing fixtures describe a current open report unless a test supplies a
// different live read. The production helper still makes the actual GET.
async function syncLabels(args) {
  if (!args.github.currentIssue) {
    args.github.currentIssue = { ...args.context.payload.issue, state: "open" };
  }
  return triage.syncLabels(args);
}

function createContext({ action = "opened", issue, previousBody }) {
  return {
    payload: {
      action,
      issue: { updated_at: "2026-10-08T10:00:00Z", ...issue },
      ...(previousBody === undefined ? {} : { changes: { body: { from: previousBody } } }),
    },
    repo: {
      owner: "rcourtman",
      repo: "Pulse",
    },
  };
}

function createCore() {
  return {
    info() {},
    warning() {},
  };
}

test("syncLabels classifies older bug reports without requesting a retest", async () => {
  const { github, calls } = createGithub({ latestVersion: "6.0.1" });
  const issue = {
    number: 1402,
    title: "Standalone hosts disappear after upgrade",
    body: "## Feedback type\nBug / regression\n\n## Pulse version\n6.0.0-rc.1\n",
    labels: [],
  };

  await syncLabels({
    github,
    context: createContext({ issue }),
    core: createCore(),
  });

  assert.equal(calls.addLabels.length, 1);
  assert.deepEqual(calls.addLabels[0].labels, [
    "affects-6.0.0-rc.1",
    "bug",
  ]);
});

test("syncLabels only adds documentation classification for non-bug v6 feedback", async () => {
  const { github, calls } = createGithub({ latestVersion: "6.0.1" });
  const issue = {
    number: 1415,
    title: "Docs path is wrong",
    body: "## Feedback type\nDocumentation issue\n\n## Pulse version\n6.0.0-rc.1\n",
    labels: [],
  };

  await syncLabels({
    github,
    context: createContext({ issue }),
    core: createCore(),
  });

  assert.equal(calls.addLabels.length, 1);
  assert.deepEqual(calls.addLabels[0].labels, ["documentation"]);
});

test("syncLabels marks declared secondary topics for decomposition", async () => {
  const { github, calls } = createGithub({ latestVersion: "6.4.1" });
  const issue = {
    number: 1796,
    title: "Availability workflow feedback",
    body: [
      "## Problem",
      "Machine availability is hard to scan.",
      "",
      "## Additional actionable topics",
      "The triage bot should preserve secondary requests.",
    ].join("\n"),
    labels: [{ name: "enhancement" }],
  };

  await syncLabels({
    github,
    context: createContext({ issue }),
    core: createCore(),
  });

  assert.deepEqual(calls.createLabel.map((call) => call.name), [
    "needs-decomposition",
  ]);
  assert.deepEqual(calls.addLabels[0].labels, [
    "needs-decomposition",
  ]);
});

test("syncLabels preserves community decomposition on an empty form field", async () => {
  for (const action of ["edited", "reopened"]) {
    const { github, calls } = createGithub({ latestVersion: "6.4.1" });
    const issue = {
      number: 1796,
      title: "Availability workflow feedback",
      body: "## Additional actionable topics\nNone.\n",
      // A later comment may have raised a distinct topic after this form was filed.
      labels: [{ name: "enhancement" }, { name: "needs-decomposition" }],
    };

    await syncLabels({
      github,
      context: createContext({ action, issue }),
      core: createCore(),
    });

    assert.equal(calls.createLabel.length, 0);
    assert.equal(calls.addLabels.length, 0);
    assert.deepEqual(calls.removeLabel, []);
  }
});

test("syncLabels adds newly declared topics but respects a completed disposition", async () => {
  const topicBody = "## Additional actionable topics\nAdd a storage filter.\n";
  const emptyBody = "## Additional actionable topics\nNone.\n";
  for (const { action, previousBody, expectedAdd } of [
    { action: "edited", previousBody: emptyBody, expectedAdd: true },
    { action: "edited", previousBody: topicBody, expectedAdd: false },
    { action: "edited", expectedAdd: false },
    { action: "reopened", expectedAdd: false },
  ]) {
    const { github, calls } = createGithub();
    await syncLabels({
      github,
      context: createContext({ action, previousBody, issue: {
        number: 1796,
        title: "Availability workflow feedback",
        body: topicBody,
        labels: [{ name: "enhancement" }],
      } }),
      core: createCore(),
    });
    assert.deepEqual(calls.addLabels.flatMap(call => call.labels),
      expectedAdd ? ["needs-decomposition"] : []);
  }
});

test("additional topic classification is explicit and fail-quiet for legacy forms", () => {
  const { classifyAdditionalActionableTopics } = triage.internals;

  assert.equal(classifyAdditionalActionableTopics("## Problem\nOne thing\n"), null);
  assert.equal(
    classifyAdditionalActionableTopics("## Additional actionable topics\n_No response_\n"),
    false
  );
  assert.equal(
    classifyAdditionalActionableTopics("## Additional actionable topics\nNone known.\n"),
    false
  );
  assert.equal(
    classifyAdditionalActionableTopics(
      "## Additional actionable topics\n<!<!-- -->-->\n"
    ),
    false
  );
  assert.equal(
    classifyAdditionalActionableTopics(
      "## Additional actionable topics\n- Add a storage filter\n- Reduce log noise\n"
    ),
    true
  );
  assert.equal(
    classifyAdditionalActionableTopics(
      [
        "### Additional actionable topics",
        "### 2. Proxmox VE Agent Install Command",
        "The generated command should expose its security controls.",
        "",
        "### Pulse version",
        "v6.4.1",
      ].join("\n")
    ),
    true
  );
});

test("topic classification reads later paragraphs after an empty first line", () => {
  for (const firstLine of ["None", "<!-- List distinct topics here. -->", "_No response_"]) {
    const body = [
      "### Additional actionable topics", firstLine, "",
      "The second host also loses its Docker inventory.", "",
      "### Pulse version", "6.5.0",
    ].join("\n");
    assert.equal(triage.internals.classifyAdditionalActionableTopics(body), true);
  }
});

test("topic classification stops at form fields but retains topic subheadings", () => {
  const { classifyAdditionalActionableTopics } = triage.internals;
  for (const newline of ["\n", "\r\n"]) {
    for (const followingField of ["Pulse version", "Additional context", "Logs, screenshots, or diagnostics", "Confirmations"]) {
      assert.equal(classifyAdditionalActionableTopics([
        "### Additional actionable topics", "None", "",
        `### ${followingField}`, "An error or a second topic outside this field.",
      ].join(newline)), false);
    }
    assert.equal(classifyAdditionalActionableTopics([
      "### Additional actionable topics", "None", "",
      "### Separate backup problem", "Backups belong to the wrong host.",
      "### Pulse version", "6.5.0",
    ].join(newline)), true);
  }
});

test("hidden template fields cannot declare topics or override feedback type", () => {
  const { classifyAdditionalActionableTopics, classifyV6FeedbackType } = triage.internals;
  const hidden = ["<!--", "### Additional actionable topics", "A second bug.",
    "### Feedback type", "Bug / regression", "-->"].join("\n");
  assert.equal(classifyAdditionalActionableTopics(hidden), null);
  assert.equal(classifyV6FeedbackType(hidden), null);
  assert.equal(classifyAdditionalActionableTopics(
    `${hidden}\n### Additional actionable topics\nNone\n\n### Pulse version\n6.5.0`
  ), false);
  assert.equal(classifyV6FeedbackType(
    `${hidden}\n### Feedback type\nDocumentation issue\n\n### Pulse version\n6.5.0`
  ), "documentation");
  assert.equal(classifyV6FeedbackType(
    "### Feedback type\nDocumentation issue\n\n### Describe the bug\nBug / regression"
  ), "documentation");
});

test("a hidden hint or empty topic field is not an actionable declaration", () => {
  for (const value of ["", "<!-- A second bug. -->", "None\n\n<!-- Another topic. -->"]) {
    assert.equal(triage.internals.classifyAdditionalActionableTopics(
      `### Additional actionable topics\n${value}\n\n### Pulse version\n6.5.0`
    ), false);
  }
});

test("syncLabels surfaces newly added later-line topics without resetting Community state", async () => {
  const previousBody = "### Additional actionable topics\nNone\n\n### Pulse version\n6.5.0";
  const body = "### Additional actionable topics\nNone\n\n" +
    "Backup attribution is also wrong.\n\n### Pulse version\n6.5.0";
  for (const action of ["opened", "edited", "reopened"]) {
    const { github, calls } = createGithub();
    const issue = { number: 2800, title: "Host inventory report", body,
      labels: [{ name: "enhancement" }, { name: "needs-retest-on-latest" }, { name: "needs-human" }] };
    await syncLabels({ github, core: createCore(),
      context: createContext({ action, issue, previousBody }) });
    assert.deepEqual(calls.addLabels.flatMap(call => call.labels),
      action === "reopened" ? [] : ["needs-decomposition"]);
    assert.deepEqual(calls.removeLabel, []);
    assert.deepEqual(calls.createComment, []);
    assert.deepEqual(calls.getLatestRelease, []);
    assert.deepEqual(calls.paginate, []);
    assert.deepEqual(calls.getIssue.map(call => call.issue_number), [2800]);
  }
});

test("syncLabels ignores hidden bug examples and topic hints", async () => {
  const { github, calls } = createGithub();
  const issue = { number: 2801, title: "Documentation wording",
    body: "<!--\n### Feedback type\nBug / regression\n" +
      "### Additional actionable topics\nAnother bug.\n-->\n" +
      "### Feedback type\nDocumentation issue\n\n" +
      "### Additional actionable topics\nNone\n\n### Pulse version\n6.5.0",
    labels: [{ name: "needs-retest-on-latest" }, { name: "needs-decomposition" }] };
  await syncLabels({ github, core: createCore(), context: createContext({ issue }) });
  assert.deepEqual(calls.addLabels.flatMap(call => call.labels), ["documentation"]);
  assert.deepEqual(calls.createLabel.map(call => call.name), []);
  assert.deepEqual(calls.removeLabel, []);
  assert.deepEqual(calls.createComment, []);
  assert.deepEqual(calls.getLatestRelease, []);
  assert.deepEqual(calls.paginate, []);
});

test("fenced examples cannot declare issue metadata", () => {
  const { extractPulseVersion, classifyAdditionalActionableTopics, classifyV6FeedbackType } = triage.internals;
  for (const fence of ["```", "~~~~", "   ````"]) {
    for (const newline of ["\n", "\r\n"]) {
      const body = ["### Existing evidence", `${fence}markdown`,
        "### Pulse version", "6.4.1", "### Feedback type", "Bug / regression",
        "### Additional actionable topics", "A copied example, not a new topic.",
        fence].join(newline);
      assert.equal(extractPulseVersion("No declared version", body), null, body);
      assert.equal(extractPulseVersion("Bug on v6.5.0", body), "6.5.0", body);
      assert.equal(classifyV6FeedbackType(body), null, body);
      assert.equal(classifyAdditionalActionableTopics(body), null, body);
    }
  }
});

test("only matching complete fences end pasted examples", () => {
  const { extractPulseVersion, classifyV6FeedbackType } = triage.internals;
  for (const { open, falseClose } of [
    { open: "````text", falseClose: "```" },
    { open: "```text", falseClose: "~~~" },
    { open: "~~~text", falseClose: "```" },
    { open: "```text", falseClose: "``` still part of the log" },
    { open: "```text", falseClose: "" },
  ]) {
    const body = [open, "Log text", falseClose, "### Pulse version", "6.4.1",
      "### Feedback type", "Regression"].join("\n");
    assert.equal(extractPulseVersion("Unknown", body), null, body);
    assert.equal(classifyV6FeedbackType(body), null, body);
  }
});

test("real declarations after fenced examples remain authoritative", () => {
  const { extractPulseVersion, classifyAdditionalActionableTopics, classifyV6FeedbackType } = triage.internals;
  for (const fence of ["```", "~~~"]) {
    const body = [fence, "Pulse version: 6.4.1", "### Pulse version", "6.4.1",
      "### Feedback type", "Regression", "### Additional actionable topics", "Copied topic", fence,
      "### Feedback type", "Documentation issue", "### Additional actionable topics", "None",
      "### Pulse version", "6.5.0"].join("\n");
    assert.equal(extractPulseVersion("Upgrade from v6.4.5", body), "6.5.0", body);
    assert.equal(classifyV6FeedbackType(body), "documentation", body);
    assert.equal(classifyAdditionalActionableTopics(body), false, body);
  }
});

test("indented log fields cannot declare a legacy version", () => {
  for (const indent of ["    ", "\t"]) {
    const body = `${indent}Pulse version: 6.4.1\n${indent}Agent version: 6.4.1\n`;
    assert.equal(triage.internals.extractPulseVersion("No declared version", body), null, body);
    assert.equal(triage.internals.extractPulseVersion("Bug on v6.5.0", body), "6.5.0", body);
  }
});

test("real version fields preserve fenced values without borrowing later evidence", () => {
  const { extractPulseVersion } = triage.internals;
  for (const field of ["### Pulse version", "Pulse | Version"]) {
    for (const fence of ["````", "~~~"]) {
      assert.equal(extractPulseVersion("Upgrade from v6.4.5", [field, `${fence}text`,
        "V6.5.0-rc.1", fence, "### Agent version", "6.4.5"].join("\n")), "6.5.0-rc.1");
      assert.equal(extractPulseVersion("Upgrade from v6.4.5", [field, "unknown", `${fence}text`,
        "Agent version: 6.5.0", fence].join("\n")), null);
      assert.equal(extractPulseVersion("Upgrade from v6.4.5", [field, "6.4", `${fence}text`,
        "6.5.0", fence].join("\n")), null);
    }
  }
});

test("structured product-named versions remain declared evidence", () => {
  assert.equal(triage.internals.extractPulseVersion("Bug",
    "### Pulse version\nPulse v6.4.5-beta.1\n### Agent version\n6.4.5"), "6.4.5-beta.1");
  assert.equal(triage.internals.extractPulseVersion("Bug",
    "### Pulse version\nConfirmed on a modified main build around v6.4.5-rc.2\n"), null);
});

test("fenced headings do not truncate genuine secondary topics", () => {
  const body = ["### Additional actionable topics", "The generated command also needs review:",
    "```markdown", "### Pulse version", "A heading in the example, not a new field.", "```",
    "The command must preserve both mounts.", "### Pulse version", "6.5.0"].join("\n");
  assert.equal(triage.internals.classifyAdditionalActionableTopics(body), true);
});

test("feedback keeps its selected value separate from fenced examples", () => {
  const body = ["### Feedback type", "Documentation issue", "```text", "Regression example",
    "### Pulse version", "6.4.1", "```", "### Pulse version", "6.5.0"].join("\n");
  assert.equal(triage.internals.classifyV6FeedbackType(body), "documentation");
  assert.equal(triage.internals.extractPulseVersion("Bug", body), "6.5.0");
});

test("syncLabels cannot classify copied issue templates or disturb Community state", async () => {
  const { github, calls } = createGithub();
  const issue = { number: 2806, title: "Documentation wording",
    body: "```markdown\n### Feedback type\nBug / regression\n### Pulse version\n6.4.1\n" +
      "### Additional actionable topics\nCopied example\n```\n",
    labels: ["enhancement", "needs-retest-on-latest", "needs-human", "needs-decomposition"]
      .map(name => ({ name })) };
  await syncLabels({ github, core: createCore(), context: createContext({ issue }) });
  assert.deepEqual(calls.addLabels, []);
  assert.deepEqual(calls.removeLabel, []);
  assert.deepEqual(calls.createLabel, []);
  assert.deepEqual(calls.createComment, []);
  assert.deepEqual(calls.getLatestRelease, []);
  assert.deepEqual(calls.paginate, []);
  assert.deepEqual(calls.getIssue.map(call => call.issue_number), [2806]);
});

test("syncLabels keeps structured unknown separate from a quoted agent version", async () => {
  const { github, calls } = createGithub();
  const issue = { number: 2807, title: "Upgrade from v6.4.5",
    body: "### Pulse version\nunknown\n```text\nAgent version: 6.5.0\n```\n",
    labels: ["bug", "needs-retest-on-latest", "needs-decomposition", "operator-reviewed"]
      .map(name => ({ name })) };
  await syncLabels({ github, core: createCore(), context: createContext({ issue }) });
  assert.deepEqual(calls.addLabels.map(call => call.labels), [["needs-version-info"]]);
  assert.deepEqual(calls.createLabel.map(call => call.name), ["needs-version-info"]);
  assert.deepEqual(calls.removeLabel, []);
  assert.deepEqual(calls.createComment, []);
  assert.deepEqual(calls.getLatestRelease, []);
});

test("every actionable issue form exposes the decomposition signal", () => {
  const templateDir = path.resolve(__dirname, "../ISSUE_TEMPLATE");
  for (const name of [
    "bug_report.yml",
    "feature_request.yml",
    "v6_rc_feedback.yml",
  ]) {
    const form = fs.readFileSync(path.join(templateDir, name), "utf8");
    assert.match(form, /id: additional_topics/);
    assert.match(form, /label: Additional actionable topics/);
    assert.match(
      form,
      /id: additional_topics[\s\S]*?validations:\s*\n\s+required: true/
    );
  }
});

test("bug and pre-release forms accept truthful evidence for installs that never started", () => {
  const templateDir = path.resolve(__dirname, "../ISSUE_TEMPLATE");
  for (const name of ["bug_report.yml", "v6_rc_feedback.yml"]) {
    const form = fs.readFileSync(path.join(templateDir, name), "utf8");
    assert.match(form, /id: pulse_version[\s\S]*?failed install[\s\S]*?"unknown"/);
    assert.match(form, /id: installer_source[\s\S]*?Installer or helper source[\s\S]*?required: false/);
    assert.match(form, /Do not paste a command containing a token or other secret/);
    assert.match(form, /for an install that never started/);
  }
});

test("both bug forms keep an optional failed-upgrade timeline without unsafe collection", () => {
  for (const name of ["bug_report.yml", "v6_rc_feedback.yml"]) {
    const form = fs.readFileSync(path.resolve(__dirname, "../ISSUE_TEMPLATE", name), "utf8");
    const timeline = form.split("    id: upgrade_context\n")[1]?.split("  - type: ")[0];
    assert.ok(timeline, name);
    assert.match(timeline, /label: Upgrade attempt and recovery/);
    assert.match(timeline, /required: false/);
    for (const distinction of [
      "starting version, intended version or asset", "original time and timezone",
      "last recorded updater/installer result or exit code",
      "whole LXC, VM or host stopping", "recovery you already performed",
      "A verified download signature is not a completed upgrade",
      "a later boot does not explain the stop",
      "Today's running version does not establish the version at failure",
      'blank or "unknown" is valid', "not full journals",
      "Do not rerun the update, reboot, restore, run Diagnostics or delete rollback backups",
      "Keep backups private and preserve the original evidence",
    ]) {
      assert.ok(timeline.includes(distinction), `${name}: ${distinction}`);
    }
    const version = form.split("    id: pulse_version\n")[1].split("  - type: ")[0];
    assert.match(version, /Version running when the problem occurred, or "unknown"/);
    assert.match(version, /target and later recovered version separate/);
  }
});

test("bug and pre-release forms accept unsafe one-off failures without a second run", () => {
  const templateDir = path.resolve(__dirname, "../ISSUE_TEMPLATE");
  for (const name of ["bug_report.yml", "v6_rc_feedback.yml"]) {
    const form = fs.readFileSync(path.join(templateDir, name), "utf8");
    assert.match(form, /do not repeat/i);
    assert.match(form, /original sequence/i);
    assert.match(form, /second reproduction is not required/);
    assert.match(form, /data loss, an outage, duplicate changes, or excessive notifications/);
  }
});

test("pre-release evidence keeps screenshots visible and asks only running containers for image identity", () => {
  const form = fs.readFileSync(
    path.resolve(__dirname, "../ISSUE_TEMPLATE/v6_rc_feedback.yml"),
    "utf8"
  );
  assert.match(form, /id: image_ref[\s\S]*?For a running Docker, Compose, or Kubernetes container/);
  assert.match(form, /id: image_ref[\s\S]*?Leave blank for LXC, bare metal, and failed installs/);
  assert.match(form, /for a running container I also gave its Pulse image tag or digest/);
  const evidenceField = form.split("    id: evidence\n")[1].split("  - type: checkboxes\n")[0];
  assert.match(evidenceField, /Attach screenshots or paste relevant redacted logs/);
  assert.doesNotMatch(evidenceField, /render:/);
});

for (const name of ["bug_report.yml", "v6_rc_feedback.yml"]) {
  test(`report intake collection safety: ${name}`, () => {
    const form = fs.readFileSync(path.resolve(__dirname, "../ISSUE_TEMPLATE", name), "utf8");
    const introduction = form.split("  - type: markdown\n")[1].split("  - type: ")[0];
    assert.match(introduction, /Run Diagnostics.*live API and guest-agent requests/);
    assert.match(introduction, /do not run it during backups, freeze\/thaw or an unresponsive-host incident/);
    assert.match(introduction, /Prefer existing observations/);
    assert.match(introduction, /downloads that result without running checks again/);
    assert.doesNotMatch(introduction, /Export for GitHub \(sanitized\)/);
  });
}

for (const name of ["bug_report.yml", "v6_rc_feedback.yml"]) {
  test(`report intake excludes unreleased branch-tip guidance: ${name}`, () => {
    const form = fs.readFileSync(path.resolve(__dirname, "../ISSUE_TEMPLATE", name), "utf8");
    // Safety instructions must remain self-contained for installed versions.
    // A main/master link can describe runtime behaviour they do not ship.
    assert.doesNotMatch(form, /https:\/\/github\.com\/[^/\s)]+\/[^/\s)]+\/(?:blob|tree)\/(?:main|master)\//);
    assert.match(form, /only collect diagnostics if Pulse is running and collection is safe/);
    assert.match(form, /downloads that result without running checks again/);
    assert.match(form, /Review files and screenshots locally before posting/);
  });
}

for (const [name, field] of [
  ["bug_report.yml", "logs"],
  ["v6_rc_feedback.yml", "evidence"],
  ["feature_request.yml", "additional_context"],
]) {
  test(`report intake privacy at attachment point: ${name}`, () => {
    const form = fs.readFileSync(path.resolve(__dirname, "../ISSUE_TEMPLATE", name), "utf8");
    const introduction = form.split("  - type: markdown\n")[1].split("  - type: ")[0];
    const attachment = form.split(`    id: ${field}\n`)[1].split("  - type: ")[0];
    assert.match(introduction, /Anything you attach here is public/);
    assert.match(introduction, /even when an export is labelled "sanitized"/);
    for (const sensitive of ["session cookies", "secret URLs", "private host", "personal details", "errors", ".env", "private keys", "Copy as cURL", "full network exports"]) {
      assert.ok(introduction.includes(sensitive), `${name} must warn about ${sensitive}`);
    }
    assert.match(attachment, /Review files and screenshots locally/);
    assert.match(attachment, /identifying details.*errors/);
    assert.doesNotMatch(attachment, /render:/);
    assert.doesNotMatch(attachment, /required: true/);
  });
}

for (const [name, field] of [
  ["bug_report.yml", "logs"],
  ["v6_rc_feedback.yml", "evidence"],
]) {
  const evidenceField = () => {
    const form = fs.readFileSync(path.resolve(__dirname, "../ISSUE_TEMPLATE", name), "utf8");
    return form.split(`    id: ${field}\n`)[1].split("  - type: ")[0];
  };

  test(`performance report measurement context stays optional: ${name}`, () => {
    const attachment = evidenceField();
    assert.match(attachment, /For CPU, memory or disk-write reports/);
    assert.match(attachment, /existing readings or safe, passive observations/);
    assert.match(attachment, /Pulse process, its container or the whole host/);
    for (const context of ["units", "measurement window", "uptime", "allocated CPUs", "memory limit", "fleet size", "polling interval", "open dashboards"]) {
      assert.ok(attachment.includes(context), `${name} must distinguish ${context}`);
    }
    assert.match(attachment, /where relevant and known/);
    assert.match(attachment, /process-start CPU average is not a recent sampling window/);
    assert.match(attachment, /database size is not a write rate/);
    assert.match(attachment, /Existing screenshots are useful/);
    assert.match(attachment, /write "unavailable" when safe collection is not possible/);
    assert.doesNotMatch(attachment, /required: true|render:/);
  });

  test(`performance report attachment safety: ${name}`, () => {
    const attachment = evidenceField();
    assert.match(attachment, /Do not restart, create load or change polling or retention just to measure/);
    assert.match(attachment, /Do not attach raw profiles, heap dumps, databases or full process command lines/);
    assert.match(attachment, /summarise only relevant counters after local review/);
  });

  test(`guest memory comparisons retain optional target and sample context: ${name}`, () => {
    const attachment = evidenceField();
    assert.match(attachment, /For CPU, memory or disk-write reports about Pulse itself/);
    const guest = attachment.split("For VM or LXC memory disagreements,")[1];
    assert.ok(guest, "guest readings must not inherit Pulse process-only guidance");
    assert.match(guest, /overview, History, alert, Patrol or another tool/);
    assert.match(guest, /inside the affected guest or on its hypervisor node/);
    assert.match(guest, /original times, units and memory source or sample age only if already known or displayed/);
    assert.match(guest, /consistent private aliases.*"unknown" for missing details/);
    assert.match(guest, /Available memory, buff\/cache and a process's RSS measure different things/);
    assert.match(guest, /RSS alone is not total guest usage/);
    assert.match(guest, /cache-inclusive hypervisor footprint does not establish guest pressure/);
    assert.match(guest, /A Proxmox VM API connection may still obtain QEMU guest-agent \(QGA\) readings without a Pulse agent/);
    assert.doesNotMatch(attachment, /required: true|render:/);
  });

  test(`guest memory context does not ask for new collection or workload changes: ${name}`, () => {
    const guest = evidenceField().split("For VM or LXC memory disagreements,")[1];
    assert.ok(guest, "a guest comparison needs its own collection boundary");
    assert.match(guest, /Use existing observations only/);
    assert.match(guest, /Do not run commands, Diagnostics or guest-agent probes, install an agent, change memory settings or workloads, or repeat a backup just to answer/);
  });
}

test("guest memory triage preserves original evidence rather than inferring past provenance", () => {
  const guide = fs.readFileSync(path.resolve(__dirname, "../../docs/ISSUE_TRIAGE.md"), "utf8").replace(/\s+/g, " ");
  const guest = guide.split("For VM or LXC memory disagreements,")[1].split("For notification reports,")[0];
  assert.match(guest, /same target and episode/);
  assert.match(guest, /inside the affected guest or on its hypervisor node/);
  assert.match(guest, /original times, units and selected memory source or sample age if already known/);
  assert.match(guest, /unknown is valid, and existing reports need no refile/);
  assert.match(guest, /RSS alone is not total guest usage/);
  assert.match(guest, /A Proxmox VM API connection may still obtain QEMU guest-agent \(QGA\) readings without a Pulse agent/);
  assert.match(guest, /not measured zero or verified recovery/);
  assert.match(guest, /live memory source cannot establish the source of a historical sample/);
  assert.match(guest, /Reconcile the full thread before asking only for a consequential remaining distinction/);
  assert.match(guest, /Do not request commands, Diagnostics or guest-agent probes, an agent install, memory or workload changes, or another backup/);
});

test("performance report triage retains measurement boundaries without demanding unsafe evidence", () => {
  const guide = fs.readFileSync(path.resolve(__dirname, "../../docs/ISSUE_TRIAGE.md"), "utf8");
  assert.match(guide, /Pulse process, container or whole host/);
  assert.match(guide, /process-start CPU average is not a recent window/);
  assert.match(guide, /database\nsize is not a write rate/);
  assert.match(guide, /without making new collection a condition of reporting/);
  assert.match(guide, /Do not request raw\nprofiles, heap dumps, databases or full process command lines/);
});

test("older-version reports cannot trigger event or scheduled retest posting", async () => {
  const issue = {
    number: 1200,
    title: "Upgrade regression",
    body: "## Feedback type\nRegression\n\n## Pulse version\n6.3.1\n",
    labels: [{ name: "bug" }],
    author_association: "NONE",
    created_at: "2026-09-08T08:55:00Z",
  };
  const { github, calls } = createGithub({ latestVersion: "6.4.1", issues: [issue] });
  const args = { github, context: createContext({ issue }), core: createCore() };
  await triage.postRetestComment(args);
  assert.deepEqual(await triage.postEligibleRetestComments({
    ...args, nowMs: Date.parse("2026-09-08T09:01:00Z"),
  }), { eligibleCount: 0, postedCount: 0 });
  // Retirement must not merely shift the public write to a different API.
  for (const values of Object.values(calls)) assert.equal(values.length, 0);
});

test("label sync preserves community-owned retest state regardless of version", async () => {
  for (const version of ["6.3.1", "6.4.1", "_No response_"]) {
    const { github, calls } = createGithub({ latestVersion: "6.4.1" });
    await syncLabels({
      github, core: createCore(),
      context: createContext({ issue: {
        number: 1200, title: "Bug", body: `## Pulse version\n${version}\n`,
        labels: [{ name: "bug" }, { name: "needs-retest-on-latest" }],
      } }),
    });
    assert.ok(!calls.addLabels[0].labels.includes("needs-retest-on-latest"));
    assert.ok(!calls.removeLabel.some(call => call.name === "needs-retest-on-latest"));
    assert.equal(calls.createComment.length, 0);
  }
});

test("postRetestComment skips reopened issues", async () => {
  const { github, calls } = createGithub({ latestVersion: "6.0.1" });
  const issue = {
    number: 1471,
    title: "Disk temperature at 0°C",
    body: "## Feedback type\nBug / regression\n\n## Pulse version\n5.1.31\n",
    labels: [],
    author_association: "NONE",
  };

  await triage.postRetestComment({
    github,
    context: createContext({ action: "reopened", issue }),
    core: createCore(),
  });

  assert.equal(calls.createComment.length, 0);
});

test("postRetestComment skips maintainer-authored issues", async () => {
  const { github, calls } = createGithub({ latestVersion: "6.0.1" });
  const issue = {
    number: 1300,
    title: "Maintainer split issue on 5.1.9",
    body: "## Feedback type\nBug / regression\n\n## Pulse version\n5.1.9\n",
    labels: [],
    author_association: "OWNER",
  };

  await triage.postRetestComment({
    github,
    context: createContext({ action: "opened", issue }),
    core: createCore(),
  });

  assert.equal(calls.createComment.length, 0);
});

test("normalizeVersion accepts a capitalised V prefix", () => {
  const { normalizeVersion } = triage.internals;

  // Regression: issue #1538 reported "V6.0.4" under the "Pulse version"
  // heading and was wrongly labelled needs-version-info. The heading regexes
  // are case-insensitive and captured "V6.0.4" correctly, but handed it to a
  // case-sensitive normalizeVersion, so every extraction path returned null.
  assert.equal(normalizeVersion("V6.0.4"), "6.0.4");
  assert.equal(normalizeVersion("v6.0.4"), "6.0.4");
  assert.equal(normalizeVersion("6.0.4"), "6.0.4");
  assert.equal(normalizeVersion("V6.1.0-rc.4"), "6.1.0-rc.4");

  assert.equal(normalizeVersion("V6"), null);
  assert.equal(normalizeVersion("### Pulse version"), null);
});

test("extractPulseVersion reads a capitalised version under its heading", () => {
  const { extractPulseVersion } = triage.internals;

  assert.equal(
    extractPulseVersion("[Bug]: something broke", "### Pulse version\nV6.0.4\n"),
    "6.0.4"
  );
});

test("structured Pulse version never falls back to an upgrade source or agent version", () => {
  const { extractPulseVersion } = triage.internals;
  for (const version of ["6.4", "6", "_No response_", ""]) {
    assert.equal(extractPulseVersion(
      "[Bug]: upgrade from rcourtman/pulse:v5.1.35 to rcourtman/pulse:6 failed",
      `### Pulse version\n\n${version}\n\n### Agent version\n5.1.30\n`,
    ), null);
  }
  assert.equal(extractPulseVersion("upgrade from v5.1.35", "### Pulse version\r\n\r\nV6.4.1\r\n\r\n### Agent version\r\n5.1.30"), "6.4.1");
  assert.equal(extractPulseVersion("bug on v6.4.1", "legacy free-form report"), "6.4.1");
  assert.equal(extractPulseVersion("bug on v5.1.35", "### Pulse version"), null);
});

test("legacy version fields keep unknown and incomplete values authoritative", () => {
  const { extractPulseVersion } = triage.internals;
  for (const field of ["Pulse version:", "**Pulse Version:**", "Pulse | Version", "## Pulse - Version"]) {
    for (const value of ["unknown", "_No response_", "6.4", "6"]) {
      for (const neighbour of ["### Agent version\n6.5.0", "Operating system: Debian 13.0.0"]) {
        const body = `${field}\n${value}\n\n${neighbour}`;
        assert.equal(extractPulseVersion("upgrade from v6.4.5", body), null, body);
      }
    }
  }
  for (const value of ["unknown", "6.4", "_No response_"]) {
    assert.equal(extractPulseVersion("upgrade from v6.4.5", `Pulse version: ${value}; Agent version 6.5.0`), null);
  }
});

test("legacy version fields stop at their first visible value", () => {
  const { extractPulseVersion } = triage.internals;
  for (const value of [
    "### Agent version\n6.5.0",
    "Agent version: 6.5.0",
    "**OS / environment:** Debian 13.0.0",
    "Agent version\n6.5.0",
    "Not available; the agent runs 6.5.0",
    "```text\nunknown\n```\n### Agent version\n6.5.0",
    "",
  ]) {
    assert.equal(extractPulseVersion("upgrade from v6.4.5", `Pulse | Version\n\n${value}`), null, value);
  }
  // Prose mentioning a field is not itself a version declaration.
  assert.equal(extractPulseVersion("Bug", "I cannot find the Pulse version.\nAgent version: 6.5.0"), null);
});

test("legacy version extraction ignores hidden template versions and headings", () => {
  const { extractPulseVersion } = triage.internals;
  assert.equal(extractPulseVersion("install", "Pulse version: unknown <!-- example: v6.4.5 -->"), null);
  assert.equal(extractPulseVersion("upgrade from v6.4.5", "<!-- ### Pulse version\n6.4.5 -->\nPulse version: unknown"), null);
  assert.equal(extractPulseVersion("Bug", "Pulse | Version\n<!-- example\n6.4.5\n-->\nV6.5.0-rc.1"), "6.5.0-rc.1");
  assert.equal(extractPulseVersion("Bug", "<!-- ### Pulse version\n6.4.5 -->\n**Pulse Version:** 6.5.0"), "6.5.0");
});

test("legacy version formats still classify actual version values", () => {
  const { extractPulseVersion } = triage.internals;
  for (const body of [
    "Pulse version: V6.5.0-rc.1",
    "**Pulse Version:** `v6.5.0-rc.1`",
    "- **Pulse version**: 6.5.0-rc.1",
    "Pulse | Version\n\nV6.5.0-rc.1\nAgent version: 6.4.5",
    "## Pulse - Version\r\n\r\n```text\r\n6.5.0-rc.1\r\n```\r\n",
    "Pulse version: rcourtman/pulse:6.5.0-rc.1",
    "Pulse | Version\n`rcourtman/pulse:6.5.0-rc.1`",
  ]) {
    assert.equal(extractPulseVersion("upgrade from v6.4.5", body), "6.5.0-rc.1", body);
  }
});

test("legacy joint server-agent versions remain reporter evidence", () => {
  const { extractPulseVersion } = triage.internals;
  const body = "**Pulse version:** server + pulse-agent v6.3.2 (Docker, `rcourtman/pulse:v6.3.2`)\n**Host OS:** Unraid 7.3.2\n**Agent:** Unified Agent v6.3.2, x86_64";
  assert.equal(extractPulseVersion("[Bug]: Unraid missing disks", body), "6.3.2");
  assert.equal(extractPulseVersion("Bug", "Pulse version: server V6.5.0"), "6.5.0");
  assert.equal(extractPulseVersion("Bug", "Pulse version: unknown; server + pulse-agent v6.3.2"), null);
});

test("legacy unknown-version sync cannot fabricate release evidence or contact", async () => {
  for (const body of [
    "Pulse version: unknown\n### Agent version\n6.5.0",
    "Pulse | Version\nunknown\nOperating system: Debian 13.0.0",
    "Pulse version: unknown <!-- example: v6.4.5 -->",
  ]) {
    const { github, calls } = createGithub();
    const issue = {
      number: 2805, title: "Upgrade from v6.4.5", body,
      labels: ["bug", "needs-retest-on-latest", "needs-decomposition", "operator-reviewed"].map(name => ({ name })),
    };
    await syncLabels({ github, context: createContext({ issue }), core: createCore() });
    assert.deepEqual(calls.addLabels.map(call => call.labels), [["needs-version-info"]]);
    assert.deepEqual(calls.createLabel.map(call => call.name), ["needs-version-info"]);
    assert.equal(calls.removeLabel.length, 0);
    assert.equal(calls.getLatestRelease.length, 0);
    assert.equal(calls.createComment.length, 0);
  }
});

test("legacy known-version sync updates only classification labels", async () => {
  const { github, calls } = createGithub();
  const issue = {
    number: 2805, title: "Upgrade from v6.4.5", body: "**Pulse Version:** 6.5.0",
    labels: ["bug", "affects-6.4.5", "needs-version-info", "needs-retest-on-latest", "needs-decomposition", "operator-reviewed"].map(name => ({ name })),
  };
  await syncLabels({ github, context: createContext({ issue }), core: createCore() });
  assert.deepEqual(calls.addLabels.map(call => call.labels), [["affects-6.5.0"]]);
  assert.deepEqual(calls.removeLabel.map(call => call.name), ["affects-6.4.5", "needs-version-info"]);
  assert.equal(calls.getLatestRelease.length, 0);
  assert.equal(calls.createComment.length, 0);
});

test("ambiguous upgrade version requests information without a retest comment", async () => {
  const { github, calls } = createGithub({ latestVersion: "6.4.1" });
  const issue = {
    number: 1913,
    title: "[Bug]: upgrade from rcourtman/pulse:v5.1.35 to rcourtman/pulse:6 failed",
    body: "### Pulse version\n\n6.4\n\n### Agent version\nnone\n",
    labels: [{ name: "bug" }],
    author_association: "NONE",
  };
  const args = { github, context: createContext({ issue }), core: createCore() };
  await syncLabels(args);
  await triage.postRetestComment(args);
  const labels = calls.addLabels.at(-1).labels;
  assert.ok(labels.includes("needs-version-info"));
  assert.ok(!labels.includes("affects-5.1.35"));
  assert.ok(!labels.includes("needs-retest-on-latest"));
  assert.equal(calls.createComment.length, 0);
});

test("queued label sync cannot overwrite newer Community label decisions", async () => {
  for (const communityAdded of [true, false]) {
    const { github } = createGithub();
    const live = new Set(["bug", "affects-6.0.0", "needs-version-info"]);
    if (communityAdded) live.add("needs-retest-on-latest");
    live.add("operator-reviewed");
    github.rest.issues.setLabels = async ({ labels }) => {
      live.clear();
      labels.forEach(label => live.add(label));
    };
    github.rest.issues.addLabels = async ({ labels }) => labels.forEach(label => live.add(label));
    github.rest.issues.removeLabel = async ({ name }) => live.delete(name);
    await syncLabels({ github, core: createCore(), context: createContext({ issue: {
      number: 1200, title: "Bug", body: "## Pulse version\n6.0.1\n",
      labels: ["bug", "affects-6.0.0", "needs-version-info",
        ...(!communityAdded ? ["needs-retest-on-latest"] : [])].map(name => ({ name })),
    } }) });
    assert.equal(live.has("needs-retest-on-latest"), communityAdded);
    assert.ok(live.has("operator-reviewed"));
    assert.ok(live.has("affects-6.0.1"));
    assert.ok(!live.has("affects-6.0.0"));
    assert.ok(!live.has("needs-version-info"));
  }
});

test("version-label removal tolerates an absent label but propagates access failures", async () => {
  for (const status of [404, 403]) {
    const { github } = createGithub();
    github.rest.issues.removeLabel = async () => { throw Object.assign(new Error("API failure"), { status }); };
    const run = syncLabels({ github, core: createCore(), context: createContext({ issue: {
      number: 1200, title: "Bug", body: "## Pulse version\n6.0.1\n",
      labels: [{ name: "bug" }, { name: "affects-6.0.0" }],
    } }) });
    if (status === 404) await assert.doesNotReject(run);
    else await assert.rejects(run, { status: 403 });
  }
});

// Different software versions can coexist in the same upgrade report. Only
// the affected server field may determine the affects-* label.
test("stable-to-preview intake keeps baseline, agent and platform versions out of labels", async () => {
  for (const [running, expected] of [
    ["6.5.0-rc.1", "affects-6.5.0-rc.1"],
    ["unknown", "needs-version-info"],
  ]) {
    const { github, calls } = createGithub({ latestVersion: "6.5.0" });
    const issue = {
      number: 2400,
      title: "[v6 pre-release]: upgraded from 6.4.5",
      body: [
        "### Feedback type", "Bug / regression",
        "### Pulse version", running,
        "### Last known working Pulse version", "6.4.1",
        "### Agent version", "6.4.5",
        "### Install path", "Upgrade from a stable v6 release",
        "### OS / environment", "Debian 12 / SCALE 26.0.0-BETA.3",
        "### Additional actionable topics", "None",
      ].join("\n\n"),
      labels: [],
    };
    await syncLabels({ github, context: createContext({ issue }), core: createCore() });
    assert.deepEqual(calls.addLabels.flatMap((call) => call.labels).sort(), [expected, "bug"].sort());
    assert.equal(calls.createComment.length, 0);
  }
});

test("interrupted-upgrade targets and recovered versions cannot supply the failing version", async () => {
  for (const [failing, expected] of [
    ["6.4.5", "affects-6.4.5"],
    ["unknown", "needs-version-info"],
  ]) {
    for (const formTitle of ["[Bug]:", "[v6 pre-release]:"]) {
      const { github, calls } = createGithub({ latestVersion: "6.5.0" });
      const bugForm = formTitle === "[Bug]:";
      const issue = {
        number: 2785,
        title: `${formTitle} interrupted upgrade to 6.5.0`,
        body: [
          ...(!bugForm ? ["### Feedback type", "Bug / regression"] : []),
          "### Pulse version", failing,
          "### Last known working Pulse version", "6.4.1",
          "### Agent version", "6.5.0",
          "### Upgrade attempt and recovery",
          "Attempt: 6.4.5 → 6.5.0; automatic timer; 6 October 04:20 UTC",
          "Last recorded result: signature verified, then Broken pipe; exit unknown",
          "Observed stop: whole LXC; stop time/cause unknown",
          "Recovery already performed: booted 7 October; current version 6.5.0",
          "### Additional actionable topics", "None",
        ].join("\n\n"),
        // GitHub applies the bug form's label; the preview form declares its
        // classification in Feedback type instead. Reproduce both inputs.
        labels: bugForm ? [{ name: "bug" }] : [],
      };
      await syncLabels({ github, context: createContext({ issue }), core: createCore() });
      assert.deepEqual(calls.addLabels.flatMap((call) => call.labels).sort(),
        (bugForm ? [expected] : [expected, "bug"]).sort());
      assert.equal(calls.createComment.length, 0);
      assert.equal(calls.getLatestRelease.length, 0);
    }
  }
});

test("delayed events classify the current report and labels, not an old version", async () => {
  const eventIssue = {
    number: 2800,
    title: "Bug after upgrade from 6.4.5",
    body: "### Pulse version\nunknown\n",
    labels: [{ name: "bug" }],
    updated_at: "2026-10-08T10:00:00Z",
  };
  const currentIssue = {
    ...eventIssue,
    state: "open",
    body: "### Pulse version\n6.5.0\n",
    labels: ["bug", "affects-6.4.5", "needs-version-info",
      "needs-retest-on-latest", "operator-reviewed"].map(name => ({ name })),
    updated_at: "2026-10-08T11:00:00Z",
  };
  const { github, calls } = createGithub({ currentIssue });
  await syncLabels({ github, context: createContext({ issue: eventIssue }), core: createCore() });

  assert.deepEqual(calls.getIssue, [{ owner: "rcourtman", repo: "Pulse", issue_number: 2800 }]);
  assert.deepEqual(calls.addLabels.map(call => call.labels), [["affects-6.5.0"]]);
  assert.deepEqual(calls.removeLabel.map(call => call.name), ["affects-6.4.5", "needs-version-info"]);
  assert.equal(calls.getLatestRelease.length, 0);
  assert.equal(calls.createComment.length, 0);
});

test("a delayed declaration cannot restore a cleared decomposition task", async () => {
  const eventIssue = {
    number: 2801,
    title: "Two report topics",
    body: "### Additional actionable topics\nA separate workflow problem\n",
    labels: [],
    updated_at: "2026-10-08T10:00:00Z",
  };
  for (const body of [eventIssue.body, "### Additional actionable topics\nNone\n"]) {
    const { github, calls } = createGithub({ currentIssue: {
      ...eventIssue, state: "open", body, updated_at: "2026-10-08T11:00:00Z",
    } });
    await syncLabels({ github, context: createContext({ issue: eventIssue }), core: createCore() });
    assert.equal(calls.addLabels.length, 0);
    assert.equal(calls.removeLabel.length, 0);
    assert.equal(calls.createComment.length, 0);
  }
});

test("current feedback classification supersedes a queued bug declaration", async () => {
  const eventIssue = {
    number: 2802, title: "Feedback", labels: [],
    body: "### Feedback type\nBug / regression\n### Pulse version\n6.4.5\n",
  };
  const { github, calls } = createGithub({ currentIssue: {
    ...eventIssue, state: "open",
    body: "### Feedback type\nDocumentation issue\n### Pulse version\n6.5.0\n",
  } });
  await syncLabels({ github, context: createContext({ issue: eventIssue }), core: createCore() });
  assert.deepEqual(calls.addLabels.map(call => call.labels), [["documentation"]]);
  assert.equal(calls.removeLabel.length, 0);
  assert.equal(calls.createComment.length, 0);
});

test("a failed current-issue read stops without stale classification or mutation", async () => {
  for (const status of [403, 404, 500]) {
    const { github, calls } = createGithub();
    github.rest.issues.get = async () => {
      throw Object.assign(new Error("Read failed"), { status });
    };
    await assert.rejects(syncLabels({ github, core: createCore(), context: createContext({ issue: {
      number: 2803, title: "Old bug", body: "### Pulse version\n6.4.5\n",
      labels: [{ name: "bug" }],
    } }) }), { status });
    assert.equal(calls.getLabel.length, 0);
    assert.equal(calls.createLabel.length, 0);
    assert.equal(calls.addLabels.length, 0);
    assert.equal(calls.removeLabel.length, 0);
    assert.equal(calls.getLatestRelease.length, 0);
    assert.equal(calls.createComment.length, 0);
  }
});

test("a closed report or pull request receives no issue metadata changes", async () => {
  const eventIssue = {
    number: 2804, title: "Bug", body: "### Pulse version\n6.5.0\n",
    labels: [{ name: "bug" }],
  };
  for (const current of [{ state: "closed" }, { state: "open", pull_request: {} }]) {
    const { github, calls } = createGithub({ currentIssue: { ...eventIssue, ...current } });
    await syncLabels({ github, core: createCore(), context: createContext({ issue: eventIssue }) });
    assert.equal(calls.getLabel.length, 0);
    assert.equal(calls.addLabels.length, 0);
    assert.equal(calls.removeLabel.length, 0);
    assert.equal(calls.getLatestRelease.length, 0);
    assert.equal(calls.createComment.length, 0);
  }
});

test("a mismatched current issue identity fails closed", async () => {
  const { github, calls } = createGithub({ currentIssue: {
    number: 9999, state: "open", title: "Another report",
    body: "### Pulse version\n6.5.0\n", labels: [{ name: "bug" }],
  } });
  await assert.rejects(syncLabels({ github, core: createCore(), context: createContext({ issue: {
    number: 2805, title: "Bug", labels: [{ name: "bug" }],
  } }) }), /Current issue identity did not match/);
  assert.equal(calls.getLabel.length, 0);
  assert.equal(calls.addLabels.length, 0);
  assert.equal(calls.removeLabel.length, 0);
  assert.equal(calls.createComment.length, 0);
});
