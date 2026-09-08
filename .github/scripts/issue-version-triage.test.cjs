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
} = {}) {
  const calls = {
    createComment: [],
    createLabel: [],
    getLabel: [],
    getLatestRelease: [],
    paginate: [],
    addLabels: [],
    removeLabel: [],
  };

  const github = {
    rest: {
      issues: {
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

function createContext({ action = "opened", issue }) {
  return {
    payload: {
      action,
      issue,
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

  await triage.syncLabels({
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

  await triage.syncLabels({
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

  await triage.syncLabels({
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

test("syncLabels clears decomposition after every declared topic has a disposition", async () => {
  const { github, calls } = createGithub({ latestVersion: "6.4.1" });
  const issue = {
    number: 1796,
    title: "Availability workflow feedback",
    body: "## Additional actionable topics\nNone.\n",
    labels: [{ name: "enhancement" }, { name: "needs-decomposition" }],
  };

  await triage.syncLabels({
    github,
    context: createContext({ action: "edited", issue }),
    core: createCore(),
  });

  assert.equal(calls.createLabel.length, 0);
  assert.equal(calls.addLabels.length, 0);
  assert.deepEqual(calls.removeLabel.map(call => call.name), ["needs-decomposition"]);
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
    await triage.syncLabels({
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
  await triage.syncLabels(args);
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
    await triage.syncLabels({ github, core: createCore(), context: createContext({ issue: {
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

test("classification removal tolerates an absent label but propagates access failures", async () => {
  for (const status of [404, 403]) {
    const { github } = createGithub();
    github.rest.issues.removeLabel = async () => { throw Object.assign(new Error("API failure"), { status }); };
    const run = triage.syncLabels({ github, core: createCore(), context: createContext({ issue: {
      number: 1200, title: "Feedback", body: "## Additional actionable topics\nNone.\n",
      labels: [{ name: "enhancement" }, { name: "needs-decomposition" }],
    } }) });
    if (status === 404) await assert.doesNotReject(run);
    else await assert.rejects(run, { status: 403 });
  }
});
