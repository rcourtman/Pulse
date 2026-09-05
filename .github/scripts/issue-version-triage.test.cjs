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
    setLabels: [],
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
        async setLabels(payload) {
          calls.setLabels.push(payload);
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

test("syncLabels adds affects and retest labels for older bug reports", async () => {
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

  assert.equal(calls.setLabels.length, 1);
  assert.deepEqual(calls.setLabels[0].labels, [
    "affects-6.0.0-rc.1",
    "bug",
    "needs-retest-on-latest",
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

  assert.equal(calls.setLabels.length, 1);
  assert.deepEqual(calls.setLabels[0].labels, ["documentation"]);
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
  assert.deepEqual(calls.setLabels[0].labels, [
    "enhancement",
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
  assert.deepEqual(calls.setLabels[0].labels, ["enhancement"]);
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

test("postRetestComment comments once for older non-maintainer bug reports", async () => {
  const { github, calls } = createGithub({ latestVersion: "6.0.1" });
  const issue = {
    number: 1200,
    title: "Upgrade regression",
    body: "## Feedback type\nRegression\n\n## Pulse version\n5.1.9\n",
    labels: [],
    author_association: "NONE",
  };

  await triage.postRetestComment({
    github,
    context: createContext({ action: "opened", issue }),
    core: createCore(),
  });

  assert.equal(calls.createComment.length, 1);
  assert.match(
    calls.createComment[0].body,
    /<!-- issue-version-triage:v1 -->/
  );
  assert.ok(
    calls.createComment[0].body.endsWith(`\n\n${triage.internals.TRIAGE_FOOTER}`)
  );
  assert.equal(
    calls.createComment[0].body.split(triage.internals.TRIAGE_FOOTER).length - 1,
    1
  );
});

test("scheduled retest guidance waits five minutes and reads the current issue", async () => {
  const nowMs = Date.parse("2026-08-26T09:06:00Z");
  const issues = [
    {
      number: 1780,
      title: "Agent token scope",
      body: "## Feedback type\nBug / regression\n\n## Pulse version\n6.3.2\n",
      labels: [{ name: "bug" }],
      author_association: "NONE",
      created_at: "2026-08-26T09:01:51Z",
    },
  ];
  const { github, calls } = createGithub({ latestVersion: "6.3.2", issues });

  const result = await triage.postEligibleRetestComments({
    github,
    context: createContext({ issue: null }),
    core: createCore(),
    nowMs,
  });

  assert.deepEqual(result, { eligibleCount: 0, postedCount: 0 });
  assert.equal(calls.createComment.length, 0);

  const laterResult = await triage.postEligibleRetestComments({
    github,
    context: createContext({ issue: null }),
    core: createCore(),
    nowMs: Date.parse("2026-08-26T09:07:00Z"),
  });

  assert.deepEqual(laterResult, { eligibleCount: 1, postedCount: 0 });
  assert.equal(calls.createComment.length, 0);
});

test("scheduled retest guidance posts once after the grace window", async () => {
  const issues = [
    {
      number: 1200,
      title: "Upgrade regression",
      body: "## Feedback type\nRegression\n\n## Pulse version\n5.1.9\n",
      labels: [{ name: "bug" }],
      author_association: "NONE",
      created_at: "2026-08-26T08:55:00Z",
    },
  ];
  const { github, calls } = createGithub({ latestVersion: "6.3.2", issues });

  const result = await triage.postEligibleRetestComments({
    github,
    context: createContext({ issue: null }),
    core: createCore(),
    nowMs: Date.parse("2026-08-26T09:01:00Z"),
  });

  assert.deepEqual(result, { eligibleCount: 1, postedCount: 1 });
  assert.equal(calls.createComment.length, 1);
  assert.equal(calls.createComment[0].issue_number, 1200);
  assert.ok(
    calls.createComment[0].body.endsWith(`\n\n${triage.internals.TRIAGE_FOOTER}`)
  );
});

test("scheduled retest guidance defers to an existing maintainer response", async () => {
  const issues = [
    {
      number: 1790,
      title: "TrueNAS SMART evidence is hidden",
      body: "## Feedback type\nBug / regression\n\n## Pulse version\n6.3.1\n",
      labels: [{ name: "bug" }],
      author_association: "NONE",
      created_at: "2026-08-28T08:01:33Z",
    },
  ];
  const existingComments = [
    {
      body: "Fixed on main; wait for a release containing the fix before retesting.",
      author_association: "OWNER",
    },
  ];
  const { github, calls } = createGithub({
    latestVersion: "6.3.2",
    existingComments,
    issues,
  });

  const result = await triage.postEligibleRetestComments({
    github,
    context: createContext({ issue: null }),
    core: createCore(),
    nowMs: Date.parse("2026-08-28T08:10:00Z"),
  });

  assert.deepEqual(result, { eligibleCount: 1, postedCount: 0 });
  assert.equal(calls.createComment.length, 0);
});

test("timeout close comments use the canonical triage footer", () => {
  const { buildTimeoutCloseCommentBody, CLOSE_COMMENT_MARKER, TRIAGE_FOOTER } =
    triage.internals;
  const body = buildTimeoutCloseCommentBody(7);

  assert.ok(body.startsWith(CLOSE_COMMENT_MARKER));
  assert.ok(body.endsWith(`\n\n${TRIAGE_FOOTER}`));
  assert.equal(body.split(TRIAGE_FOOTER).length - 1, 1);
  assert.doesNotMatch(body, /automated maintainer|supervised by|AI\)/i);
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
