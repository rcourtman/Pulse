const test = require("node:test");
const assert = require("node:assert/strict");
const fs = require("node:fs");
const path = require("node:path");

const triage = require("./issue-version-triage.cjs");

// A named source in a title is still a transition, not the failing release.
// Exercise fallback separately from the authoritative form/legacy body fields.
const ambiguousVersionTitles = [
  "Pulse 6.4.5 → 6.5.0 fails",
  "Pulse 6.4.5 -> v6.5.0 fails",
  "Pulse 6.4.5 => latest fails",
  "Pulse 6.4.5 ↔ 6.5.0",
  "Pulse 6.4.5 to 6.5.0 fails",
  "Pulse 6.4.5 through 6.5.0",
  "Pulse 6.4.5 / 6.5.0",
  "Pulse 6.4.5 or 6.5.0",
  "Pulse 6.4.5 and 6.5.0",
  "Pulse 6.4.5 – 6.5.0",
  "Pulse 6.4.5 — v6.5.0",
  "Pulse 6.4.5 - 6.5.0",
  "Pulse Pro server v6.4.5 → v6.5.0",
  "Pulse version 6.4.5 → unknown",
  "Pulse server: 6.4.5 to nightly",
  "Pulse 6.4.5 update to 6.5.0 fails",
  "rcourtman/pulse:6.4.5 → latest fails",
  "license.pulserelay.pro/pulse-pro:6.4.5 / 6.5.0",
  "[Bug]: 6.4.5 → latest fails",
  "Bug on v6.4.5 -> unknown",
  "6.4.5 to nightly fails",
  "[v6 pre-release]: 6.6.0-beta.2 / latest",
];

test("title fallback never selects a transition's named source or alternative", () => {
  for (const title of ambiguousVersionTitles) {
    assert.equal(triage.internals.extractPulseVersion(title, "Existing observations only."), null, title);
  }
});

test("ambiguous titles do not override an explicit current or unknown body version", () => {
  for (const title of ambiguousVersionTitles) {
    for (const body of ["### Pulse version\n6.5.0", "Pulse version: 6.5.0"]) {
      assert.equal(triage.internals.extractPulseVersion(title, body), "6.5.0", title);
    }
    assert.equal(triage.internals.extractPulseVersion(title, "### Pulse version\nunknown"), null, title);
  }
});

const currentVersionTitles = [
  ["Pulse 6.5.0 with PVE 9.2.0: readings missing", "6.5.0"],
  ["Pulse Pro server v6.5.0 with agent 6.4.5: readings missing", "6.5.0"],
  ["Pulse 6.5.0 and agent 6.4.5 disagree", "6.5.0"],
  ["Pulse 6.5.0 / TrueNAS CORE 13.0.0: readings missing", "6.5.0"],
  ["Pulse 6.6.0-rc.4 with Kubernetes 1.33.0", "6.6.0-rc.4"],
  ["Pulse 6.5.0: moving a node to another cluster loses readings", "6.5.0"],
  ["rcourtman/pulse:6.5.0 on PVE 9.2.0", "6.5.0"],
  ["license.pulserelay.pro/pulse-pro:6.6.0-beta.3 with agent 6.5.0", "6.6.0-beta.3"],
  ["Bug on v6.5.0: missing readings", "6.5.0"],
  ["[Bug]: 6.6.0-rc.4: sign-in fails", "6.6.0-rc.4"],
];

test("title ambiguity checks preserve explicit server versions beside other products", () => {
  for (const [title, version] of currentVersionTitles) {
    assert.equal(triage.internals.extractPulseVersion(title, "Existing observations only."), version, title);
  }
});

test("ambiguous titles cannot create an affected-version label or a public question", async () => {
  for (const title of ambiguousVersionTitles) for (const action of ["opened", "edited", "reopened"]) {
    const { github, calls } = createGithub();
    const issue = { number: 3071, title, body: "Existing observations only.",
      labels: ["bug", "needs-human", "needs-retest-on-latest"].map(name => ({ name })) };
    await syncLabels({ github, context: createContext({ action, issue }), core: createCore() });
    assert.deepEqual(calls.addLabels.map(call => call.labels), [["needs-version-info"]], `${action}: ${title}`);
    assert.deepEqual(calls.createLabel.map(call => call.name), ["needs-version-info"], title);
    assert.deepEqual(calls.removeLabel, [], title);
    assert.deepEqual(calls.createComment, [], title);
    assert.deepEqual(calls.getLatestRelease, [], title);
  }
});

test("title transitions preserve versions established from the whole conversation", async () => {
  for (const title of ambiguousVersionTitles) {
    const { github, calls } = createGithub();
    const issue = { number: 3071, title, body: "Existing observations only.",
      labels: ["bug", "affects-6.5.0", "needs-human", "needs-retest-on-latest"].map(name => ({ name })) };
    await syncLabels({ github, context: createContext({ action: "reopened", issue }), core: createCore() });
    assert.deepEqual(calls.addLabels, [], title);
    assert.deepEqual(calls.createLabel, [], title);
    assert.deepEqual(calls.removeLabel, [], title);
    assert.deepEqual(calls.createComment, [], title);
    assert.deepEqual(calls.getLatestRelease, [], title);
  }
});

test("the title consumer still classifies a single explicit server release silently", async () => {
  for (const [title, version] of currentVersionTitles) {
    const { github, calls } = createGithub();
    const issue = { number: 3071, title, body: "Existing observations only.", labels: [{ name: "bug" }] };
    await syncLabels({ github, context: createContext({ issue }), core: createCore() });
    assert.deepEqual(calls.addLabels.map(call => call.labels), [[`affects-${version}`]], title);
    assert.deepEqual(calls.createLabel.map(call => call.name), [`affects-${version}`], title);
    assert.deepEqual(calls.removeLabel, [], title);
    assert.deepEqual(calls.createComment, [], title);
    assert.deepEqual(calls.getLatestRelease, [], title);
  }
});

// GitHub issue forms do not submit markdown-only guidance as report fields.
// Detailed, optional guidance belongs immediately before the input it helps;
// test that location rather than requiring every reader to see it expanded.
function fieldGuidance(form, id) {
  const blocks = form.split(/(?=^  - type: )/m).slice(1);
  const index = blocks.findIndex(block => block.includes(`    id: ${id}\n`));
  assert.ok(index >= 0, `missing input ${id}`);
  const preceding = blocks[index - 1] || "";
  return (preceding.startsWith("  - type: markdown\n") ? preceding : "") + blocks[index];
}

function assertOptionalIntakeGuidance(form, summary, evidence) {
  const blocks = form.split(/(?=^  - type: )/m).slice(1);
  const disclosures = [];
  for (const block of blocks) {
    if (!block.includes("<details")) continue;
    assert.ok(block.startsWith("  - type: markdown\n"), "details must not be input descriptions");
    assert.doesNotMatch(block, /<details\s+[^>]*\bopen\b/);
    const groups = [...block.matchAll(/<details>\s*<summary>([^<]+)<\/summary>([\s\S]*?)<\/details>/g)];
    assert.equal(groups.length, (block.match(/<details/g) || []).length, "each disclosure must close");
    for (const [, title, content] of groups) {
      assert.ok(title.trim(), "keyboard focus needs a meaningful summary");
      assert.ok(content.trim(), "a disclosure must retain its guidance");
      disclosures.push(title);
    }
  }
  assert.deepEqual(disclosures.sort(), [
    "Failed upgrade: which attempt, stop and recovery?",
    "Several hosts or clusters: preserve which names and IDs repeat",
    "Sign-in or permissions: keep working access",
    "Notification delivery: Test versus ordinary alerts",
    "Pulse CPU, memory or disk writes: comparable measurements",
    "VM or LXC memory: guest versus hypervisor readings",
  ].sort());
  for (const [id, titles] of [
    ["upgrade_context", ["Failed upgrade:"]],
    [summary, ["Several hosts or clusters:"]],
    [evidence, ["Sign-in or permissions:", "Notification delivery:", "Pulse CPU, memory or disk writes:", "VM or LXC memory:"]],
  ]) {
    const guidance = fieldGuidance(form, id);
    for (const title of titles) assert.ok(guidance.includes(`<summary>${title}`), `${id}: ${title}`);
  }
  // Collection and public-disclosure warnings are never hidden in a disclosure.
  const introduction = blocks[0];
  assert.doesNotMatch(introduction, /<details/);
  assert.match(introduction, /do not run it during backups, freeze\/thaw or an unresponsive-host incident/);
  assert.match(introduction, /Anything you attach here is public/);
  const attachment = blocks.find(block => block.includes(`    id: ${evidence}\n`));
  assert.match(attachment, /Review files and screenshots locally/);
  assert.doesNotMatch(attachment, /<details|For notification reports|For CPU, memory or disk-write reports/);
}

for (const [name, summary, evidence] of [
  ["bug_report.yml", "bug_description", "logs"],
  ["v6_rc_feedback.yml", "summary", "evidence"],
]) {
  test(`specialist intake guidance stays optional and beside its input: ${name}`, () => {
    const form = fs.readFileSync(path.resolve(__dirname, "../ISSUE_TEMPLATE", name), "utf8");
    assertOptionalIntakeGuidance(form, summary, evidence);
  });

  test(`sign-in guidance guard rejects absent, expanded and misplaced advice: ${name}`, () => {
    const form = fs.readFileSync(path.resolve(__dirname, "../ISSUE_TEMPLATE", name), "utf8");
    const signIn = form.match(/<details>\s*<summary>Sign-in or permissions: keep working access<\/summary>[\s\S]*?<\/details>/)?.[0];
    assert.ok(signIn, "the actual sign-in disclosure is needed for these controls");
    assert.throws(() => assertOptionalIntakeGuidance(form.replace(signIn, ""), summary, evidence));
    assert.throws(() => assertOptionalIntakeGuidance(form.replace(signIn,
      signIn.replace("<details>", "<details open>")), summary, evidence));

    // Keep every disclosure, but move sign-in advice to the unrelated primary
    // description. The inventory check alone must not accept that placement.
    const blocks = form.replace(signIn, "").split(/(?=^  - type: )/m);
    const primary = blocks.findIndex(block => block.includes(`    id: ${summary}\n`));
    blocks[primary - 1] += `        ${signIn}\n\n`;
    assert.throws(() => assertOptionalIntakeGuidance(blocks.join(""), summary, evidence),
      new RegExp(`${evidence}: Sign-in or permissions:`));
  });
}

function createGithub({
  latestVersion = "6.0.1",
  existingLabels = new Set(),
  existingComments = [],
  issueEvents = [],
  issues = [],
  currentIssue = null,
} = {}) {
  const calls = {
    createComment: [],
    createLabel: [],
    getLabel: [],
    getLatestRelease: [],
    paginate: [],
    eventHistory: [],
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
        listEvents: Symbol("listEvents"),
      },
      repos: {
        async getLatestRelease() {
          calls.getLatestRelease.push(true);
          return { data: { tag_name: `v${latestVersion}` } };
        },
      },
    },
    async paginate(endpoint, parameters) {
      calls.paginate.push(endpoint);
      if (endpoint === github.rest.issues.listEvents) {
        calls.eventHistory.push(parameters);
        return issueEvents;
      }
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

test("label sync needs only same-repository issue methods, including delayed topic history", async () => {
  for (const delayed of [false, true]) {
    const issue = {
      number: 3071,
      title: "Wrong readings",
      body: "### Pulse version\n6.5.0\n\n### Additional actionable topics\nAlso cannot save the interval.",
      labels: [{ name: "bug" }, { name: "needs-version-info" }],
      updated_at: "2026-10-08T10:00:00Z",
    };
    const { github, calls } = createGithub({
      currentIssue: { ...issue, state: "open", ...(delayed ? { updated_at: "2026-10-08T10:01:00Z" } : {}) },
      issueEvents: [],
    });
    const issueMethods = new Set(["get", "getLabel", "createLabel", "addLabels", "removeLabel", "listEvents"]);
    const scopedIssues = new Proxy(github.rest.issues, {
      get(target, method) {
        assert.ok(issueMethods.has(method), `unexpected issue API: ${String(method)}`);
        if (method === "listEvents") return target[method];
        return async args => {
          assert.equal(args.owner, "rcourtman");
          assert.equal(args.repo, "Pulse");
          return target[method](args);
        };
      },
    });
    github.rest = new Proxy({ issues: scopedIssues }, {
      get(target, resource) {
        assert.equal(resource, "issues", "label sync must not require source, Actions or other repository permissions");
        return target.issues;
      },
    });
    await syncLabels({ github, context: createContext({ issue }), core: createCore() });
    assert.deepEqual(calls.createLabel.map(call => call.name), ["needs-decomposition", "affects-6.5.0"]);
    assert.deepEqual(calls.addLabels.map(call => call.labels), [["affects-6.5.0", "needs-decomposition"]]);
    assert.deepEqual(calls.removeLabel.map(call => call.name), ["needs-version-info"]);
    assert.equal(calls.eventHistory.length, delayed ? 1 : 0);
    if (delayed) {
      assert.deepEqual(calls.eventHistory[0], {
        owner: "rcourtman", repo: "Pulse", issue_number: 3071, per_page: 100,
      });
    }
  }
});

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

test("material edits to an already populated topic field restore its review task", async () => {
  const previousBody = "### Additional actionable topics\nHistory is also empty.\n\n" +
    "### Additional context\nExisting observations.\n";
  for (const value of [
    "History is also empty.\n\nBackups also identify the wrong host.",
    "Backups also identify the wrong host.",
    "History is also empty, and backups identify the wrong host.",
  ]) {
    const issue = { number: 2926, title: "Mixed report", labels:
      ["enhancement", "needs-human", "needs-retest-on-latest", "operator-reviewed"].map(name => ({ name })),
    body: previousBody.replace("History is also empty.", value) };
    const { github, calls } = createGithub();
    await syncLabels({ github, core: createCore(),
      context: createContext({ action: "edited", issue, previousBody }) });
    assert.deepEqual(calls.addLabels.map(call => call.labels), [["needs-decomposition"]], value);
    assert.deepEqual(calls.removeLabel, []);
    assert.deepEqual(calls.createComment, []);
    assert.deepEqual(calls.getLatestRelease, []);
    assert.deepEqual(calls.paginate, []);
    assert.equal(calls.getIssue.length, 2);
  }
});

test("unchanged visible topics cannot restore a completed task on unrelated or presentation edits", async () => {
  const previousBody = "### Additional actionable topics\nHistory is also empty.\n\n" +
    "### Additional context\nExisting observations.\n";
  for (const body of [
    previousBody,
    previousBody.replace("Existing observations.", "New logs outside the topic field."),
    previousBody.replace("History is also empty.", "HISTORY  is\nalso empty."),
    previousBody.replace("History is also empty.", "History is also empty.\n\n<!-- Backups identify the wrong host. -->"),
    previousBody.replace("### Additional actionable topics", "  ## Additional actionable topics ##"),
    previousBody.replaceAll("\n", "\r\n"),
  ]) {
    const issue = { number: 2927, title: "Mixed report", body, labels: [{ name: "enhancement" }] };
    const { github, calls } = createGithub();
    await syncLabels({ github, core: createCore(),
      context: createContext({ action: "edited", issue, previousBody }) });
    assert.deepEqual(calls.addLabels, [], body);
    assert.deepEqual(calls.removeLabel, [], body);
    assert.deepEqual(calls.getLabel, [], body);
    assert.deepEqual(calls.createComment, [], body);
    assert.equal(calls.getIssue.length, 1);
  }
});

test("a populated-field edit respects explicit review and a fresh pre-write read", async () => {
  const previousBody = "### Additional actionable topics\nHistory is also empty.\n";
  const issue = { number: 2928, title: "Mixed report", state: "open", labels: [{ name: "enhancement" }],
    updated_at: "2026-10-15T20:30:00Z",
    body: previousBody + "\nBackups also identify the wrong host.\n" };
  for (const scenario of ["reopened", "missing previous body", "superseded event", "closed during preparation", "already queued"]) {
    const current = structuredClone(issue);
    if (scenario === "superseded event") current.updated_at = "2026-10-15T20:31:00Z";
    if (scenario === "already queued") current.labels.push({ name: "needs-decomposition" });
    const { github, calls } = createGithub({ currentIssue: current, issueEvents: [
      { event: "unlabeled", label: { name: "needs-decomposition" }, created_at: current.updated_at },
    ] });
    if (scenario === "closed during preparation") {
      github.rest.issues.getLabel = async () => {
        github.currentIssue = { ...current, state: "closed" };
        return { data: {} };
      };
    }
    await syncLabels({ github, core: createCore(), context: createContext({
      action: scenario === "reopened" ? "reopened" : "edited", issue: current,
      previousBody: scenario === "missing previous body" ? undefined : previousBody,
      // A delayed event is distinct from the live read even if its body matches.
      ...(scenario === "superseded event" ? { issue } : {}),
    }) });
    assert.deepEqual(calls.addLabels, [], scenario);
    assert.deepEqual(calls.removeLabel, [], scenario);
    assert.deepEqual(calls.createComment, [], scenario);
    assert.deepEqual(calls.getLatestRelease, [], scenario);
    assert.deepEqual(calls.paginate, scenario === "superseded event" ? [github.rest.issues.listEvents] : [], scenario);
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

const formattedEmptyTopics = ["None", "None known.", "N/A", "NA", "No", "Not applicable!"]
  .flatMap(value => ["*", "_", "**", "__", "***", "`"].map(mark => `${mark}${value}${mark}`));

test("formatting an empty topic answer does not declare another problem", () => {
  for (const value of formattedEmptyTopics) {
    const body = `### Additional actionable topics\n${value}\n\n### Pulse version\n6.5.0`;
    assert.equal(triage.internals.classifyAdditionalActionableTopics(body), false, value);
  }
});

test("formatted empty answers cannot hide consequential topic content", () => {
  for (const value of [
    "**None**\n\nThe second host loses Docker inventory.",
    "**None**\n### Separate backup problem\nBackups belong to the wrong host.",
    "**None** except the second host loses Docker inventory.",
    "**None", "~~None~~", "[None](https://example.invalid/issue)",
    "<span>None</span>", "- **None**\n- The second host loses Docker inventory.",
    "`<!-- None -->`", "No response",
    "**The second host loses Docker inventory.**",
  ]) {
    const body = `### Additional actionable topics\n${value}\n\n### Pulse version\n6.5.0`;
    assert.equal(triage.internals.classifyAdditionalActionableTopics(body), true, value);
  }
});

for (const action of ["opened", "edited", "reopened"]) {
  test(`empty-topic formatting never invents a decomposition task on ${action}`, async () => {
    for (const value of formattedEmptyTopics) {
      const { github, calls } = createGithub();
      const issue = { number: 3100, title: "One monitoring problem",
        body: `### Additional actionable topics\n${value}\n\n### Pulse version\n6.5.0`,
        labels: ["bug", "affects-6.5.0", "needs-human", "needs-retest-on-latest", "operator-reviewed"]
          .map(name => ({ name })) };
      await syncLabels({ github, core: createCore(), context: createContext({ action, issue,
        previousBody: "### Additional actionable topics\nNone\n\n### Pulse version\n6.5.0" }) });
      assert.deepEqual(calls.addLabels, [], value);
      assert.deepEqual(calls.removeLabel, [], value);
      assert.deepEqual(calls.createLabel, [], value);
      assert.deepEqual(calls.createComment, [], value);
      assert.deepEqual(calls.getLatestRelease, [], value);
      assert.deepEqual(calls.paginate, [], value);
    }
  });
}

test("formatted empty answers preserve an existing decomposition task", async () => {
  const { github, calls } = createGithub();
  const issue = { number: 3101, title: "One monitoring problem",
    body: "### Additional actionable topics\n**None**\n\n### Pulse version\n6.5.0",
    labels: ["bug", "affects-6.5.0", "needs-decomposition", "needs-human", "needs-retest-on-latest"]
      .map(name => ({ name })) };
  await syncLabels({ github, core: createCore(), context: createContext({ action: "edited", issue,
    previousBody: "### Additional actionable topics\nA second symptom.\n\n### Pulse version\n6.5.0" }) });
  assert.deepEqual(calls.addLabels, []);
  assert.deepEqual(calls.removeLabel, []);
  assert.deepEqual(calls.createComment, []);
});

test("a real topic after a formatted empty answer still reaches review", async () => {
  const { github, calls } = createGithub();
  const issue = { number: 3102, title: "One monitoring problem",
    body: "### Additional actionable topics\n**None**\n\nThe second host loses Docker inventory.\n\n### Pulse version\n6.5.0",
    labels: ["bug", "affects-6.5.0", "needs-human", "needs-retest-on-latest"].map(name => ({ name })) };
  await syncLabels({ github, core: createCore(), context: createContext({ action: "edited", issue,
    previousBody: "### Additional actionable topics\n**None**\n\n### Pulse version\n6.5.0" }) });
  assert.deepEqual(calls.addLabels.map(call => call.labels), [["needs-decomposition"]]);
  assert.deepEqual(calls.removeLabel, []);
  assert.deepEqual(calls.createComment, []);
});

test("edited form headings accept Markdown indentation and closing hashes", () => {
  const { extractPulseVersion, classifyAdditionalActionableTopics, classifyV6FeedbackType } = triage.internals;
  for (const newline of ["\n", "\r\n"]) {
    for (let level = 1; level <= 6; level += 1) {
      for (let indent = 0; indent <= 3; indent += 1) {
        for (const closing of ["", " #", " ######## \t"]) {
          const heading = (name) => " ".repeat(indent) + "#".repeat(level) + "\t" + name + closing;
          const body = [heading("Pulse version"), "6.5.0", "",
            heading("Feedback type"), "Documentation issue", "",
            heading("Additional actionable topics"), "The second host loses its Docker inventory."]
            .join(newline);
          assert.equal(extractPulseVersion("Upgrade from v6.4.5", body), "6.5.0", body);
          assert.equal(classifyV6FeedbackType(body), "documentation", body);
          assert.equal(classifyAdditionalActionableTopics(body), true, body);
        }
      }
    }
  }
});

test("closing hashes and indentation do not let neighbouring fields become topics", () => {
  for (const followingField of ["Pulse version", "Additional context", "Logs, screenshots, or diagnostics", "Confirmations"]) {
    const body = ["  ### Additional actionable topics ###", "None", "",
      `   ## ${followingField} ####`, "An error outside the declared topic field."].join("\n");
    assert.equal(triage.internals.classifyAdditionalActionableTopics(body), false, body);
  }
  const body = [" ### Additional actionable topics #", "None", "",
    "  #### Separate backup problem ###", "Backups belong to the wrong host.", "",
    "   ### Pulse version ####", "6.5.0"].join("\n");
  assert.equal(triage.internals.classifyAdditionalActionableTopics(body), true);
});

test("plain hash-prefixed text cannot declare issue-form metadata", () => {
  const { extractPulseVersion, classifyAdditionalActionableTopics, classifyV6FeedbackType } = triage.internals;
  for (const prefix of ["####### ", "###", "    ### ", "\t### ", "> ### "]) {
    const body = [prefix + "Pulse version", "6.5.0", "",
      prefix + "Feedback type", "Bug / regression", "",
      prefix + "Additional actionable topics", "Copied template text."].join("\n");
    assert.equal(extractPulseVersion("No version in title", body), null, body);
    assert.equal(classifyV6FeedbackType(body), null, body);
    assert.equal(classifyAdditionalActionableTopics(body), null, body);
  }
  // A seven-hash line inside a topic is ordinary field content, not a boundary.
  assert.equal(classifyAdditionalActionableTopics(
    "### Additional actionable topics\nNone\n####### Pulse version\nA second symptom.\n"
  ), true);
});

test("only a separated final hash run closes an issue-form heading", () => {
  for (const suffix of ["#", " ### example", " ## #"]) {
    const body = `### Additional actionable topics${suffix}\nA second bug.\n`;
    assert.equal(triage.internals.classifyAdditionalActionableTopics(body), null, body);
  }
});

test("edited headings stay hidden inside comments and pasted code", () => {
  const { extractPulseVersion, classifyAdditionalActionableTopics, classifyV6FeedbackType } = triage.internals;
  const copied = [" ### Pulse version ###", "6.4.1", "  ### Feedback type ###",
    "Bug / regression", "   ### Additional actionable topics ###", "Copied example."].join("\n");
  for (const [open, close] of [["<!--", "-->"], ["```markdown", "```"], ["~~~markdown", "~~~"]]) {
    const body = [open, copied, close, "  ### Pulse version ###", "unknown",
      "  ### Feedback type ###", "Documentation issue", "  ### Additional actionable topics ###", "None"]
      .join("\n");
    assert.equal(extractPulseVersion("Upgrade from v6.4.5", body), null, body);
    assert.equal(classifyV6FeedbackType(body), "documentation", body);
    assert.equal(classifyAdditionalActionableTopics(body), false, body);
  }
});

test("edited version headings retain fenced values and authoritative unknowns", () => {
  for (const value of ["6.5.0-rc.1", "unknown", "6.5"]) {
    const body = [" ### Pulse version ###", "```text", value, "```", "",
      "  ### Agent version ###", "6.4.5"].join("\n");
    assert.equal(triage.internals.extractPulseVersion("Upgrade from v6.4.5", body),
      value === "6.5.0-rc.1" ? value : null, body);
  }
});

test("label consumer uses edited headings without changing Community-owned state", async () => {
  const { github, calls } = createGithub();
  const issue = { number: 2900, title: "Upgrade from v6.4.5",
    body: " ### Feedback type ###\nBug / regression\n  ### Pulse version ###\n6.5.0\n" +
      "   ### Additional actionable topics ###\nThe second host loses Docker inventory.\n",
    labels: ["enhancement", "needs-retest-on-latest", "needs-human", "operator-reviewed", "affects-6.4.5"]
      .map(name => ({ name })) };
  await syncLabels({ github, core: createCore(), context: createContext({ action: "edited", issue, previousBody: "### Pulse version\n6.4.5\n### Additional actionable topics\nNone" }) });
  assert.deepEqual(calls.addLabels.map(call => call.labels),
    [["affects-6.5.0", "bug", "needs-decomposition"]]);
  assert.deepEqual(calls.removeLabel.map(call => call.name), ["affects-6.4.5"]);
  assert.deepEqual(calls.createComment, []);
  assert.deepEqual(calls.getLatestRelease, []);
  assert.deepEqual(calls.paginate, []);
  assert.deepEqual(calls.getIssue.map(call => call.issue_number), [2900, 2900]);
});

test("invalid headings cannot create classification or decomposition mutations", async () => {
  const { github, calls } = createGithub();
  const issue = { number: 2901, title: "An example in a documentation report",
    body: "####### Feedback type\nBug / regression\n####### Pulse version\n6.5.0\n" +
      "####### Additional actionable topics\nCopied template text.\n",
    labels: ["documentation", "needs-retest-on-latest", "needs-human", "operator-reviewed"]
      .map(name => ({ name })) };
  await syncLabels({ github, core: createCore(), context: createContext({ issue }) });
  assert.deepEqual(calls.addLabels, []);
  assert.deepEqual(calls.removeLabel, []);
  assert.deepEqual(calls.createLabel, []);
  assert.deepEqual(calls.createComment, []);
  assert.deepEqual(calls.getLatestRelease, []);
  assert.deepEqual(calls.paginate, []);
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

test("literal HTML comments in pasted code cannot erase later visible form fields", () => {
  const { extractPulseVersion, classifyAdditionalActionableTopics, classifyV6FeedbackType } = triage.internals;
  for (const newline of ["\n", "\r\n"]) {
    for (const [open, close] of [["```html", "```"], ["~~~~html", "~~~~"],
      ["   ````text", "   ````"]]) {
      for (const marker of ["<!-- unfinished response", "<!-- closed response -->", "<!<!-- -->-->"]) {
        const body = [open, marker, "### Pulse version", "6.4.1", close, "",
          "### Pulse version", "6.5.0", "### Feedback type", "Documentation issue",
          "### Additional actionable topics", "Missing disk reading"].join(newline);
        assert.equal(extractPulseVersion("Upgrade from v6.4.5", body), "6.5.0", body);
        assert.equal(classifyV6FeedbackType(body), "documentation", body);
        assert.equal(classifyAdditionalActionableTopics(body), true, body);
      }
    }
    for (const indent of ["    ", "\t"]) {
      const body = [`${indent}<!-- copied log delimiter`, `${indent}Pulse version: 6.4.1`, "",
        "### Pulse version", "6.5.0", "### Feedback type", "Documentation issue",
        "### Additional actionable topics", "Missing disk reading"].join(newline);
      assert.equal(extractPulseVersion("Upgrade from v6.4.5", body), "6.5.0", body);
      assert.equal(classifyV6FeedbackType(body), "documentation", body);
      assert.equal(classifyAdditionalActionableTopics(body), true, body);
    }
  }
});

test("hidden comments still hide their contents even when they contain code fences", () => {
  const { extractPulseVersion, classifyAdditionalActionableTopics, classifyV6FeedbackType } = triage.internals;
  const hidden = ["<!-- hidden template", "```markdown", "### Pulse version", "6.4.1",
    "### Feedback type", "Bug / regression", "### Additional actionable topics", "Copied topic", "```"];
  const visible = ["### Pulse version", "6.5.0", "### Feedback type", "Documentation issue",
    "### Additional actionable topics", "None"];
  for (const newline of ["\n", "\r\n"]) {
    const closed = [...hidden, "    -->", ...visible].join(newline);
    assert.equal(extractPulseVersion("Upgrade from v6.4.5", closed), "6.5.0");
    assert.equal(classifyV6FeedbackType(closed), "documentation");
    assert.equal(classifyAdditionalActionableTopics(closed), false);
    const unfinished = [...hidden, ...visible].join(newline);
    assert.equal(extractPulseVersion("Bug", unfinished), null);
    assert.equal(classifyV6FeedbackType(unfinished), null);
    assert.equal(classifyAdditionalActionableTopics(unfinished), null);
  }
});

test("pasted HTML keeps genuine topic values without turning copied headings into fields", () => {
  const body = ["### Additional actionable topics", "```html", "<!-- unmatched response marker",
    "### Pulse version", "6.4.1", "```", "### Pulse version", "6.5.0"].join("\n");
  assert.equal(triage.internals.classifyAdditionalActionableTopics(body), true);
  assert.equal(triage.internals.extractPulseVersion("Upgrade from v6.4.5", body), "6.5.0");
});

test("inline and quoted HTML literals cannot hide subsequent form declarations", () => {
  const { extractPulseVersion, classifyV6FeedbackType, classifyAdditionalActionableTopics } = triage.internals;
  for (const evidence of [
    "The error includes `<!-- unfinished`.",
    "The error includes ``a ` tick and <!-- unfinished``.",
    "The error includes `<!-- first` and ``<!-- second``.",
    "> ```html\n> <!-- unfinished\n> ### Pulse version\n> 6.4.1\n> ```",
    "   > The response includes <!-- unfinished",
    "The literal is \\<!-- unfinished.",
  ]) {
    for (const newline of ["\n", "\r\n"]) {
      const body = ["### Existing evidence", evidence, "", "### Pulse version", "6.5.0",
        "### Feedback type", "Documentation issue", "### Additional actionable topics",
        "Storage History remains empty."].join("\n").replaceAll("\n", newline);
      assert.equal(extractPulseVersion("Upgrade from v6.4.5", body), "6.5.0", body);
      assert.equal(classifyV6FeedbackType(body), "documentation", body);
      assert.equal(classifyAdditionalActionableTopics(body), true, body);
    }
  }
});

test("an interrupted inline comment cannot turn an unclosed code fence into visible fields", () => {
  const { extractPulseVersion, classifyV6FeedbackType, classifyAdditionalActionableTopics } = triage.internals;
  for (const evidence of [
    "A literal `<!-- unfinished` followed by <!--",
    "A mismatched `<!-- opener`` remains text before a block quote",
    "An escaped \\` backtick does not protect <!-- unfinished`",
    "An even escape \\\\<!-- remains text before the quote",
    "<!-- a hidden ` backtick is not an opener --> <!-- second hidden`",
  ]) {
    const body = [evidence, "> quoted text", "```", "### Pulse version", "6.4.1",
      "### Feedback type", "Bug / regression", "### Additional actionable topics", "Copied topic",
      "-->", "### Pulse version", "unknown", "### Feedback type", "Documentation issue",
      "### Additional actionable topics", "None"].join("\n");
    // GFM's block lexer puts every later line inside the unclosed fence.
    // The previous scanner mistakenly treated an inline opener as a comment
    // across those blocks, then accepted invisible headings after its close.
    assert.equal(extractPulseVersion("Upgrade from v6.4.5", body), null, body);
    assert.equal(classifyV6FeedbackType(body), null, body);
    assert.equal(classifyAdditionalActionableTopics(body), null, body);
    const visible = body + "\n```\n### Pulse version\n6.5.0\n" +
      "### Feedback type\nDocumentation issue\n### Additional actionable topics\nNone";
    assert.equal(extractPulseVersion("Upgrade from v6.4.5", visible), "6.5.0", visible);
    assert.equal(classifyV6FeedbackType(visible), "documentation", visible);
    assert.equal(classifyAdditionalActionableTopics(visible), false, visible);
  }
});

test("syncLabels preserves visible fields after inline HTML without new contact", async () => {
  for (const evidence of ["Error: `<!-- unfinished`", "> <!-- unfinished response"]) {
    const { github, calls } = createGithub();
    const issue = { number: 2808, title: "Upgrade from v6.4.5",
      body: ["### Existing evidence", evidence, "", "### Pulse version", "6.5.0",
        "### Feedback type", "Bug / regression", "### Additional actionable topics", "Missing History"].join("\n"),
      labels: ["needs-human", "needs-retest-on-latest", "operator-reviewed", "needs-version-info"]
        .map(name => ({ name })) };
    await syncLabels({ github, core: createCore(), context: createContext({ issue }) });
    assert.deepEqual(calls.addLabels.flatMap(call => call.labels), ["affects-6.5.0", "bug", "needs-decomposition"]);
    assert.deepEqual(calls.removeLabel.map(call => call.name), ["needs-version-info"]);
    assert.deepEqual(calls.createComment, []);
    assert.deepEqual(calls.getLatestRelease, []);
    assert.deepEqual(calls.paginate, []);
    assert.deepEqual(calls.getIssue.map(call => call.issue_number), [2808, 2808]);
  }
});

test("syncLabels consumes real fields after pasted HTML without contact or state changes", async () => {
  const { github, calls } = createGithub();
  const issue = { number: 2801, title: "Upgrade from v6.4.5",
    body: ["```html", "<!-- incomplete response", "### Pulse version", "6.4.1", "```",
      "### Pulse version", "6.5.0", "### Feedback type", "Bug / regression",
      "### Additional actionable topics", "Missing disk reading"].join("\n"),
    labels: ["needs-human", "needs-retest-on-latest", "operator-reviewed", "needs-version-info"]
      .map(name => ({ name })) };
  await syncLabels({ github, core: createCore(), context: createContext({ issue }) });
  assert.deepEqual(calls.addLabels.flatMap(call => call.labels), ["affects-6.5.0", "bug", "needs-decomposition"]);
  assert.deepEqual(calls.removeLabel.map(call => call.name), ["needs-version-info"]);
  assert.deepEqual(calls.createComment, []);
  assert.deepEqual(calls.getLatestRelease, []);
  assert.deepEqual(calls.paginate, []);
  assert.deepEqual(calls.getIssue.map(call => call.issue_number), [2801, 2801]);
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
    assert.deepEqual(calls.getIssue.map(call => call.issue_number), action === "reopened" ? [2800] : [2800, 2800]);
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

test("every actionable issue form offers secondary topics without requiring a second problem", () => {
  const templateDir = path.resolve(__dirname, "../ISSUE_TEMPLATE");
  for (const [name, workaroundField] of [
    ["bug_report.yml", "Describe the bug"],
    ["feature_request.yml", "Alternatives considered"],
    ["v6_rc_feedback.yml", "What happened?"],
  ]) {
    const form = fs.readFileSync(path.join(templateDir, name), "utf8");
    const field = form.split(/(?=^  - type: )/m)
      .find(block => block.includes("    id: additional_topics\n"));
    assert.ok(field, name);
    assert.match(field, /label: Additional actionable topics/);
    assert.match(field, /required: false/);
    assert.doesNotMatch(field, /required: true|Enter "None"/);
    assert.match(field, /Leave blank for a single (?:problem|request)/);
    assert.match(field, /Only list separate bugs, requests or workflows/);
    assert.ok(field.includes(`in ${workaroundField} instead.`), name);
    const target = form.split(/(?=^  - type: )/m)
      .find(block => block.includes(`      label: ${workaroundField}\n`));
    assert.ok(target, `${name}: named workaround field exists`);
  }
});

for (const response of ["", "_No response_", "None"]) {
  test(`a single problem's workaround does not declare a secondary topic: ${JSON.stringify(response)}`, async () => {
    const { github, calls } = createGithub();
    const issue = {
      number: 42,
      title: "[Bug]: Login stops working after saving settings",
      body: `### Describe the bug\nSaving settings breaks login. Re-entering the value restores access.\n\n` +
        `### Additional actionable topics\n${response}\n\n### Pulse version\n6.5.0\n`,
      labels: [{ name: "bug" }, { name: "affects-6.5.0" }],
    };
    await syncLabels({ github, core: createCore(), context: createContext({ issue }) });
    assert.deepEqual(calls.addLabels, []);
    assert.deepEqual(calls.removeLabel, []);
    assert.deepEqual(calls.createComment, []);
  });
}

test("a genuine secondary problem still surfaces when the primary has a workaround", async () => {
  const { github, calls } = createGithub();
  const issue = {
    number: 42,
    title: "[Bug]: Login stops working after saving settings",
    body: "### Describe the bug\nSaving settings breaks login. Re-entering the value restores access.\n\n" +
      "### Additional actionable topics\nThe audit screen is also unreadable on a phone.\n\n### Pulse version\n6.5.0\n",
    labels: [{ name: "bug" }, { name: "affects-6.5.0" }],
  };
  await syncLabels({ github, core: createCore(), context: createContext({ issue }) });
  assert.deepEqual(calls.addLabels.map(call => call.labels), [["needs-decomposition"]]);
  assert.deepEqual(calls.removeLabel, []);
  assert.deepEqual(calls.createComment, []);
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
    const timeline = fieldGuidance(form, "upgrade_context");
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
    return fieldGuidance(form, field);
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

  test(`baseline memory regressions need no continuing growth or restart: ${name}`, () => {
    const attachment = evidenceField();
    assert.match(attachment, /higher steady baseline from growth during one uninterrupted run/);
    assert.match(attachment, /worth reporting even if usage stays flat/);
    assert.match(attachment, /already available versions, uptime, workload and enabled collectors\/features/);
    assert.match(attachment, /not proof of recovery compared with a days-old run/);
    assert.match(attachment, /Unknown context is valid/);
    assert.match(attachment, /do not restart, downgrade or change features to create a comparison/);
    assert.doesNotMatch(attachment, /required: true|render:/);
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

const ambiguousVersionDeclarations = [
  "6.4.5 -> 6.5.0",
  "v6.4.5 → v6.5.0",
  "6.4.5 to 6.5.0",
  "6.4.5 upgraded to 6.5.0",
  "6.4.5 / 6.5.0",
  "6.4.5 or 6.5.0",
  "6.4.5 and 6.5.0",
  "6.4.5 – 6.5.0",
  "6.4.5 - 6.5.0",
  "6.4.5 through 6.5.0",
  "v6.4.5 upgrading to v6.5.0-rc.1",
  "6.4.5 migrated to Pulse 6.5.0",
  "6.4.5 → unknown",
  "6.4.5 to latest",
  "6.4.5 -> 6.5",
  "**6.4.5** => **6.5.0**",
  "Pulse 6.4.5 → Pulse 6.5.0",
  "rcourtman/pulse:6.4.5 -> rcourtman/pulse:6.5.0",
  "license.pulserelay.pro/pulse-pro:6.4.5 → license.pulserelay.pro/pulse-pro:6.5.0",
  "pulse-v6.4.5-linux-amd64.tar.gz -> pulse-v6.5.0-linux-amd64.tar.gz",
];

test("version fields do not select an upgrade source or guess between releases", () => {
  for (const value of ambiguousVersionDeclarations) {
    for (const body of [
      `### Pulse version\n${value}\n### Agent version\n6.6.0`,
      `Pulse version: ${value}`,
      `### Pulse version\n\`\`\`text\n${value}\n\`\`\``,
    ]) {
      assert.equal(triage.internals.extractPulseVersion("Bug on v6.6.0", body), null, body);
    }
  }
});

test("unambiguous current versions retain upgrade history and companion context", () => {
  for (const value of [
    "6.5.0 (upgraded from 6.4.5)",
    "6.5.0 (previous attempt: 6.4.5 -> 6.4.6)",
    "server 6.5.0; agent 6.4.5",
    "6.5.0 / agent 6.4.5",
    "server + pulse-agent v6.5.0 (Docker, rcourtman/pulse:6.5.0)",
    "rcourtman/pulse:6.5.0 (upgraded from 6.4.5)",
    "pulse-v6.5.0-linux-amd64.tar.gz (previously 6.4.5)",
  ]) {
    assert.equal(triage.internals.extractPulseVersion("Upgrade failure",
      `### Pulse version\n${value}`), "6.5.0", value);
  }
});

test("ambiguous version intake stays silent and preserves whole-thread ownership", async () => {
  for (const value of ambiguousVersionDeclarations) {
    for (const established of [false, true]) {
      const { github, calls } = createGithub();
      const labels = ["bug", "needs-human", "needs-retest-on-latest",
        ...(established ? ["affects-6.6.0"] : [])];
      const issue = { number: 3072, title: "Upgrade stopped", body: `### Pulse version\n${value}`,
        labels: labels.map(name => ({ name })) };
      await syncLabels({ github, core: createCore(), context: createContext({ issue }) });
      assert.deepEqual(calls.addLabels.map(call => call.labels),
        established ? [] : [["needs-version-info"]], value);
      assert.deepEqual(calls.removeLabel, [], value);
      assert.deepEqual(calls.createComment, [], value);
      assert.deepEqual(calls.getLatestRelease, [], value);
      assert.deepEqual(calls.paginate, [], value);
    }
  }
});

test("free-form titles cannot turn platform, agent or upgrade versions into Pulse evidence", () => {
  const { extractPulseVersion } = triage.internals;
  for (const title of [
    "[Bug]: Unraid 7.3.2 reports missing disks",
    "[Bug]: TrueNAS SCALE 25.04.2.6 has no CPU history",
    "[Bug]: vCenter 8.0.3 collection times out",
    "[Bug]: Docker 27.0.0 loses readings",
    "[Bug]: Pulse Agent v6.5.0 does not report",
    "[Bug]: pulse-agent-v6.5.0-linux-amd64.tar.gz fails",
    "[Bug]: helm-chart-6.5.0 fails",
    "[Bug]: Upgrade 6.4.5 → 6.5.0 stopped the LXC",
    "[Bug]: Upgrading Pulse 6.4.5 to 6.5.0 failed",
    "[Bug]: Migration to Pulse 6.5.0 failed",
    "[Bug]: Migrate to Pulse 6.5.0 failed",
    "[Bug]: 6.4.5 → 6.5.0 stops",
    "[Bug]: Pulse 6.4.5 or Pulse 6.5.0",
    "[Bug]: Pulse 6.5.0.4 readings",
  ]) {
    assert.equal(extractPulseVersion(title, "Legacy free-form report."), null, title);
    // The explicit running-instance field is still authoritative.
    assert.equal(extractPulseVersion(title, "### Pulse version\n6.5.0\n"), "6.5.0", title);
  }
});

test("free-form titles retain explicit server and concise legacy release evidence", () => {
  const { extractPulseVersion } = triage.internals;
  for (const [title, version] of [
    ["Pulse 6.5.0 on Unraid 7.3.2 reports missing disks", "6.5.0"],
    ["[Bug]: Pulse server version: V6.5.0 on Docker 27.0.0", "6.5.0"],
    ["[Bug]: Pulse Pro v6.6.0-rc.3 login", "6.6.0-rc.3"],
    ["[Bug]: rcourtman/pulse:6.5.0 on Unraid 7.3.2", "6.5.0"],
    ["[Bug]: license.pulserelay.pro/pulse-pro:6.6.0-rc.3 login", "6.6.0-rc.3"],
    ["[Bug]: v6.5.0 readings are missing", "6.5.0"],
    ["bug on v6.4.1", "6.4.1"],
    ["6.5.0", "6.5.0"],
  ]) {
    assert.equal(extractPulseVersion(title, "Legacy free-form report."), version, title);
    assert.equal(extractPulseVersion(title, "### Pulse version\nunknown\n"), null, title);
  }
});

test("title-only platform evidence stays silent and preserves whole-thread metadata", async () => {
  for (const established of [false, true]) {
    const { github, calls } = createGithub();
    const labels = ["bug", "needs-retest-on-latest", "operator-reviewed",
      ...(established ? ["affects-6.5.0"] : [])].map(name => ({ name }));
    const issue = { number: 2805, title: "[Bug]: vCenter 8.0.3 collection times out",
      body: "Legacy free-form report.", labels };
    await syncLabels({ github, context: createContext({ issue }), core: createCore() });
    assert.deepEqual(calls.addLabels.map(call => call.labels), established ? [] : [["needs-version-info"]]);
    assert.deepEqual(calls.createLabel.map(call => call.name), established ? [] : ["needs-version-info"]);
    assert.equal(calls.removeLabel.length, 0);
    assert.equal(calls.getLatestRelease.length, 0);
    assert.equal(calls.createComment.length, 0);
  }
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
  await syncLabels({ github, context: createContext({ action: "edited", issue, previousBody: "Pulse version: 6.4.5" }), core: createCore() });
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
    await syncLabels({ github, core: createCore(), context: createContext({ action: "edited", previousBody: "## Pulse version\n6.0.0", issue: {
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
    const run = syncLabels({ github, core: createCore(), context: createContext({ action: "edited", previousBody: "## Pulse version\n6.0.0", issue: {
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
    labels: ["bug", "needs-version-info",
      "needs-retest-on-latest", "operator-reviewed"].map(name => ({ name })),
    updated_at: "2026-10-08T11:00:00Z",
  };
  const { github, calls } = createGithub({ currentIssue });
  await syncLabels({ github, context: createContext({ issue: eventIssue }), core: createCore() });

  assert.deepEqual(calls.getIssue, Array(2).fill({ owner: "rcourtman", repo: "Pulse", issue_number: 2800 }));
  assert.deepEqual(calls.addLabels.map(call => call.labels), [["affects-6.5.0"]]);
  assert.deepEqual(calls.removeLabel.map(call => call.name), ["needs-version-info"]);
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
    }, issueEvents: [
      { event: "unlabeled", label: { name: "needs-decomposition" }, created_at: "2026-10-08T10:30:00Z" },
    ] });
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

const feedbackChoices = [
  ["Bug / regression", "bug"],
  ["Documentation issue", "documentation"],
  ["UX / workflow friction", "enhancement"],
];

for (const [originalChoice, originalClass] of feedbackChoices) {
  for (const [, establishedClass] of feedbackChoices) {
    if (originalClass === establishedClass) continue;
    for (const event of ["unrelated edit", "reopen", "delayed open", "delayed type edit"]) {
      test(`established feedback classification survives ${event}: ${originalClass} to ${establishedClass}`, async () => {
        const body = `### Feedback type\n${originalChoice}\n### Pulse version\n6.5.0\n`;
        const labels = [establishedClass, "affects-6.5.0", "needs-human",
          "needs-retest-on-latest", "needs-decomposition", "operator-reviewed"];
        const issue = { number: 2806, title: "Reviewed mixed report", body,
          state: "open", updated_at: "2026-10-17T01:00:00Z",
          labels: labels.map(name => ({ name })) };
        const delayed = event.startsWith("delayed");
        const { github, calls } = createGithub({ currentIssue: issue });
        const context = createContext({
          issue: { ...issue, ...(delayed ? { updated_at: "2026-10-16T01:00:00Z" } : {}) },
          action: event === "reopen" ? "reopened" : event === "delayed open" ? "opened" : "edited",
          previousBody: event === "reopen" || event === "delayed open" ? undefined :
            event === "delayed type edit" ? "### Feedback type\nOther actionable feedback\n" :
              `${body}\nAn unrelated spelling correction.\n`,
        });
        await syncLabels({ github, context, core: createCore() });
        assert.equal(calls.getIssue.length, 1, "no write should be prepared");
        assert.deepEqual(calls.addLabels, []);
        assert.deepEqual(calls.removeLabel, []);
        assert.deepEqual(calls.createLabel, []);
        assert.deepEqual(calls.createComment, []);
        assert.deepEqual(calls.getLatestRelease, []);
        assert.deepEqual(calls.paginate, []);
      });
    }
  }
}

for (const [choice, expectedClass] of feedbackChoices) {
  test(`a current feedback-type correction can add ${expectedClass} without discarding other topics`, async () => {
    const oldChoice = expectedClass === "bug" ? "Documentation issue" : "Bug / regression";
    const oldClass = expectedClass === "bug" ? "documentation" : "bug";
    const previousBody = `### Feedback type\n${oldChoice}\n### Pulse version\n6.5.0\n`;
    const issue = { number: 2807, title: "Changed feedback", state: "open",
      body: `### Feedback type\n${choice}\n### Pulse version\n6.5.0\n`,
      labels: [oldClass, "affects-6.5.0", "needs-human", "needs-decomposition"].map(name => ({ name })) };
    const { github, calls } = createGithub();
    await syncLabels({ github, core: createCore(),
      context: createContext({ action: "edited", issue, previousBody }) });
    assert.deepEqual(calls.addLabels.map(call => call.labels), [[expectedClass]]);
    assert.deepEqual(calls.removeLabel, []);
    assert.deepEqual(calls.createComment, []);
  });
}

test("an equivalent feedback-type edit preserves established classification", async () => {
  const issue = { number: 2808, title: "Reviewed feedback", state: "open",
    body: "### Feedback type\n  BUG / regression <!-- original choice -->\n### Pulse version\n6.5.0\n",
    labels: [{ name: "documentation" }, { name: "affects-6.5.0" }] };
  const { github, calls } = createGithub();
  await syncLabels({ github, core: createCore(), context: createContext({ action: "edited", issue,
    previousBody: "### Feedback type\nBug / regression\n### Pulse version\n6.5.0\n" }) });
  assert.deepEqual(calls.addLabels, []);
  assert.deepEqual(calls.removeLabel, []);
  assert.deepEqual(calls.createComment, []);
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


test("quoted and multiline inline-code versions cannot replace a legacy declaration", () => {
  const { extractPulseVersion } = triage.internals;
  for (const newline of ["\n", "\r\n"]) {
    for (const copied of [
      "> Copied old log\nPulse version: 6.4.5",
      "Pasted text: `\nPulse version: 6.4.5\n`",
      "Pasted text: **`\nPulse version: 6.4.5\n`**",
      "- Copied old log\n  Pulse version: 6.4.5",
      "<pre>\nPulse version: 6.4.5\n</pre>",
    ]) {
      const body = (copied + "\n\nPulse version: **`6.5.0`**").replaceAll("\n", newline);
      assert.equal(extractPulseVersion("Upgrade from v6.4.5", body), "6.5.0", body);
      assert.equal(extractPulseVersion("Bug", copied.replaceAll("\n", newline)), null, copied);
    }
  }
});

test("raw HTML and nested headings cannot declare form metadata", () => {
  const { extractPulseVersion, classifyV6FeedbackType, classifyAdditionalActionableTopics } = triage.internals;
  const fields = "### Pulse version\n6.4.5\n### Feedback type\nBug / regression\n" +
    "### Additional actionable topics\nCopied template text.";
  for (const copied of [
    "<div>\n" + fields + "\n</div>",
    "<pre>\n" + fields + "\n</pre>",
    "<script>\n" + fields + "\n</script>",
    fields.split("\n").map(line => "> " + line).join("\n"),
    "- Copied template\n\n" + fields.split("\n").map(line => "  " + line).join("\n"),
  ]) {
    assert.equal(extractPulseVersion("No declared version", copied), null, copied);
    assert.equal(classifyV6FeedbackType(copied), null, copied);
    assert.equal(classifyAdditionalActionableTopics(copied), null, copied);
    const body = copied + "\n\n### Pulse version\nunknown\n### Feedback type\nDocumentation issue\n" +
      "### Additional actionable topics\nNone";
    assert.equal(extractPulseVersion("Upgrade from v6.4.5", body), null, body);
    assert.equal(classifyV6FeedbackType(body), "documentation", body);
    assert.equal(classifyAdditionalActionableTopics(body), false, body);
  }
});

test("the actual label consumer keeps copied declarations from changing report or Community state", async () => {
  const { github, calls } = createGithub();
  const issue = { number: 2902, title: "Pasted old installer output",
    body: "> Copied log\nPulse version: 6.4.5\n\n" +
      "<div>\n### Feedback type\nBug / regression\n### Additional actionable topics\nExample\n</div>\n\n" +
      "Pulse version: `6.5.0`",
    labels: ["bug", "affects-6.4.5", "needs-version-info", "needs-retest-on-latest", "needs-human", "operator-reviewed"]
      .map(name => ({ name })) };
  await syncLabels({ github, core: createCore(), context: createContext({ action: "edited", issue, previousBody: "Pulse version: 6.4.5" }) });
  assert.deepEqual(calls.addLabels.map(call => call.labels), [["affects-6.5.0"]]);
  assert.deepEqual(calls.removeLabel.map(call => call.name), ["affects-6.4.5", "needs-version-info"]);
  assert.deepEqual(calls.createComment, []);
  assert.deepEqual(calls.getLatestRelease, []);
  assert.deepEqual(calls.paginate, []);
  assert.deepEqual(calls.getIssue.map(call => call.issue_number), [2902, 2902]);
});


test("triage workflow prepares and checks the exact parser before minting its write token", () => {
  const workflow = fs.readFileSync(path.resolve(__dirname, "../workflows/issue-version-label-sync.yml"), "utf8");
  const install = workflow.indexOf("run: npm ci --ignore-scripts --no-audit --no-fund --prefix .github/scripts");
  const check = workflow.indexOf("run: npm run test:triage --prefix .github/scripts");
  const credential = workflow.indexOf("- name: Mint triage bot token");
  assert.ok(install >= 0 && check > install && credential > check);
  assert.match(workflow, /node-version: '24'/);
  assert.match(workflow, /actions\/setup-node@[0-9a-f]{40}/);
  assert.match(workflow, /persist-credentials: false/);
  for (const file of ["package.json", "package-lock.json", "issue-version-triage.test.cjs", "triage-comment-policy.test.cjs"]) {
    assert.ok(workflow.includes(`.github/scripts/${file}`), file);
  }
  assert.ok(workflow.includes("docs/ISSUE_TRIAGE.md"));
  assert.ok(workflow.includes(".github/ISSUE_TEMPLATE"));
  assert.ok(workflow.includes(".github/workflows"));
  assert.doesNotMatch(workflow, /npm install|--ignore-scripts=false|\$\{\{ github\.event\.issue\.body/);
});

test("the standalone triage graph reuses the existing exact parser without transitive or lifecycle additions", () => {
  const manifest = JSON.parse(fs.readFileSync(path.join(__dirname, "package.json"), "utf8"));
  const lock = JSON.parse(fs.readFileSync(path.join(__dirname, "package-lock.json"), "utf8"));
  assert.equal(manifest.private, true);
  assert.deepEqual(manifest.dependencies, { marked: "17.0.6" });
  assert.deepEqual(lock.packages[""].dependencies, manifest.dependencies);
  assert.deepEqual(Object.keys(lock.packages).sort(), ["", "node_modules/marked"]);
  const parser = lock.packages["node_modules/marked"];
  assert.equal(parser.version, manifest.dependencies.marked);
  assert.equal(parser.resolved, "https://registry.npmjs.org/marked/-/marked-17.0.6.tgz");
  assert.match(parser.integrity, /^sha512-/);
  assert.equal(parser.hasInstallScript, undefined);
  assert.equal(parser.dependencies, undefined);
  assert.equal(parser.optionalDependencies, undefined);
});


// Exercise the real mutation consumer, not just its Markdown classifier. A
// successful first read cannot authorize a stale write after label preparation.
for (const [scenario, changes] of [
  ["closure", { state: "closed" }],
  ["body correction", { body: "### Pulse version\n6.6.0\n" }],
  ["title correction", { title: "Corrected report" }],
  ["review activity", { updated_at: "2026-10-14T10:01:00Z" }],
  ["classification removal", { labels: [] }],
  ["human ownership", { labels: [{ name: "bug" }, { name: "needs-human" }] }],
  ["pull request", { pull_request: {} }],
]) {
  test(`label preparation cannot outlive a report's ${scenario}`, async () => {
    const issue = {
      number: 2910, state: "open", title: "Current bug",
      body: "### Pulse version\n6.5.0\n### Additional actionable topics\nA separate History problem\n",
      updated_at: "2026-10-14T10:00:00Z",
      labels: ["bug", "affects-6.4.5", "needs-version-info"].map(name => ({ name })),
    };
    const { github, calls } = createGithub({ currentIssue: structuredClone(issue) });
    const read = github.rest.issues.get;
    github.rest.issues.get = async payload => ({ data: structuredClone((await read(payload)).data) });
    github.rest.issues.getLabel = async ({ name }) => {
      calls.getLabel.push(name);
      github.currentIssue = { ...github.currentIssue, ...structuredClone(changes) };
      return { data: { name } };
    };
    const messages = [];
    await syncLabels({ github, context: createContext({ issue }), core: { info: text => messages.push(text) } });
    assert.equal(calls.getIssue.length, 2);
    assert.equal(calls.addLabels.length, 0);
    assert.equal(calls.removeLabel.length, 0);
    assert.equal(calls.createComment.length, 0);
    assert.equal(calls.getLatestRelease.length, 0);
    assert.equal(calls.paginate.length, 0);
    assert.ok(messages.includes("Issue changed during metadata preparation. Skipping outdated label delta."));
  });
}

test("a non-bug classification also requires a current report before its write", async () => {
  const issue = { number: 2911, state: "open", title: "Feedback", labels: [],
    body: "### Feedback type\nDocumentation issue\n" };
  const { github, calls } = createGithub({ currentIssue: issue });
  const read = github.rest.issues.get;
  github.rest.issues.get = async payload => {
    const result = await read(payload);
    return { data: calls.getIssue.length === 1 ? result.data : { ...issue, state: "closed" } };
  };
  await syncLabels({ github, context: createContext({ issue }), core: createCore() });
  assert.equal(calls.getIssue.length, 2);
  assert.equal(calls.addLabels.length, 0);
  assert.equal(calls.removeLabel.length, 0);
});

for (const status of [403, 404, 500]) {
  test(`a failed pre-write read (${status}) cannot use the initial snapshot`, async () => {
    const issue = { number: 2912, state: "open", title: "Bug", labels: [{ name: "bug" }],
      body: "### Pulse version\n6.5.0\n" };
    const { github, calls } = createGithub({ currentIssue: issue });
    const read = github.rest.issues.get;
    github.rest.issues.get = async payload => {
      const result = await read(payload);
      if (calls.getIssue.length === 2) throw Object.assign(new Error("Read failed"), { status });
      return result;
    };
    await assert.rejects(syncLabels({ github, context: createContext({ issue }), core: createCore() }), { status });
    assert.equal(calls.getIssue.length, 2);
    assert.equal(calls.addLabels.length, 0);
    assert.equal(calls.removeLabel.length, 0);
    assert.equal(calls.createComment.length, 0);
  });
}

test("a mismatched pre-write issue identity fails closed", async () => {
  const issue = { number: 2913, state: "open", title: "Bug", labels: [{ name: "bug" }],
    body: "### Pulse version\n6.5.0\n" };
  const { github, calls } = createGithub({ currentIssue: issue });
  const read = github.rest.issues.get;
  github.rest.issues.get = async payload => {
    const result = await read(payload);
    return { data: calls.getIssue.length === 1 ? result.data : { ...issue, number: 9999 } };
  };
  await assert.rejects(syncLabels({ github, context: createContext({ issue }), core: createCore() }),
    /Current issue identity did not match/);
  assert.equal(calls.addLabels.length, 0);
  assert.equal(calls.removeLabel.length, 0);
});

test("the pre-write comparison treats label order as irrelevant", async () => {
  const issue = { number: 2914, state: "open", title: "Bug", updated_at: "2026-10-08T10:00:00Z",
    labels: ["bug", "needs-human", "needs-retest-on-latest", "affects-6.4.5"].map(name => ({ name })),
    body: "### Pulse version\n6.5.0\n" };
  const { github, calls } = createGithub({ currentIssue: issue });
  const read = github.rest.issues.get;
  github.rest.issues.get = async payload => {
    const result = await read(payload);
    return { data: calls.getIssue.length === 1 ? result.data : { ...issue, labels: [...issue.labels].reverse() } };
  };
  await syncLabels({ github, context: createContext({ action: "edited", issue, previousBody: "### Pulse version\n6.4.5" }), core: createCore() });
  assert.deepEqual(calls.addLabels.map(call => call.labels), [["affects-6.5.0"]]);
  assert.deepEqual(calls.removeLabel.map(call => call.name), ["affects-6.4.5"]);
});

test("a report with no label delta needs no additional pre-write read", async () => {
  const issue = { number: 2915, state: "open", title: "Bug",
    labels: ["bug", "affects-6.5.0", "needs-human"].map(name => ({ name })),
    body: "### Pulse version\n6.5.0\n" };
  const { github, calls } = createGithub({ currentIssue: issue });
  await syncLabels({ github, context: createContext({ issue }), core: createCore() });
  assert.equal(calls.getIssue.length, 1);
  assert.equal(calls.addLabels.length, 0);
  assert.equal(calls.removeLabel.length, 0);
});

test("explicit unknown current versions retire only managed exact-version labels", async () => {
  const preserved = ["bug", "needs-retest-on-latest", "needs-decomposition",
    "needs-human", "operator-reviewed", "affects-main", "affects-storage"];
  for (const laterVersionKnown of [false, true]) for (const body of [
    "### Pulse version\nunknown\n",
    "Pulse version: unknown; agent version 6.5.0",
    "### Pulse version\n6.5\n### Agent version\n6.5.0",
    "### Pulse version\n_No response_\n",
    "### Pulse version\n\n### Agent version\n6.5.0",
  ]) {
    const { github, calls } = createGithub();
    const issue = { number: 2916, title: "Upgrade from v6.4.5", body,
      labels: [...preserved, "affects-6.4.5", ...(laterVersionKnown ? ["affects-6.6.0-beta.3"] : [])].map(name => ({ name })) };
    await syncLabels({ github, context: createContext({ action: "edited", issue, previousBody: "### Pulse version\n6.4.5" }), core: createCore() });
    assert.deepEqual(calls.addLabels.map(call => call.labels), laterVersionKnown ? [] : [["needs-version-info"]], body);
    assert.deepEqual(calls.removeLabel.map(call => call.name), ["affects-6.4.5"], body);
    assert.equal(calls.getIssue.length, 2);
    assert.equal(calls.getLatestRelease.length, 0);
    assert.equal(calls.createComment.length, 0);
  }
});

test("free-form reports preserve established version metadata without a form declaration", async () => {
  for (const [title, body, versions] of [
    ["Windows guest reads stop after restart", "**Environment**\n- Pulse Pro 6.5.0 (Docker), upgraded from 6.4.5\n- Guest agent 110.0.2", ["6.5.0"]],
    ["Telegram rejects ordinary alerts", "## Environment\n\n- Pulse server v6.5.0-rc.1 (LXC), upgraded from v6.4.5", ["6.4.5", "6.5.0-rc.1"]],
    ["ARM temperature is missing", "## Environment\n\n* Pulse v6.4.1 (Docker)\n* Unified agent v6.4.1", ["6.4.1"]],
    ["Upgrade from v6.4.5", "The running version is unavailable.", ["6.4.5"]],
    ["TrueNAS CORE 13.0.6 readings", "Version evidence is in an earlier comment or screenshot.", ["6.5.0"]],
    ["Bug on v6.6.0-beta.3", "An unrelated edit to a free-form report.", ["6.5.0"]],
  ]) {
    const { github, calls } = createGithub();
    const issue = { number: 2922, title, body,
      labels: ["bug", ...versions.map(version => `affects-${version}`),
        "needs-retest-on-latest", "needs-human", "operator-reviewed"].map(name => ({ name })) };
    await syncLabels({ github, context: createContext({ action: "edited", issue }), core: createCore() });
    assert.deepEqual(calls.addLabels, [], title);
    assert.deepEqual(calls.removeLabel, [], title);
    assert.deepEqual(calls.getLabel, [], title);
    assert.deepEqual(calls.createLabel, [], title);
    assert.equal(calls.getIssue.length, 1, title);
    assert.deepEqual(calls.getLatestRelease, [], title);
    assert.deepEqual(calls.createComment, [], title);
    assert.deepEqual(calls.paginate, [], title);
  }
});

test("absent version declarations do not invent evidence from unrelated labels", async () => {
  const { github, calls } = createGithub();
  const issue = { number: 2923, title: "Bug", body: "Version is unavailable.",
    labels: ["bug", "affects-main", "affects-storage", "needs-human"].map(name => ({ name })) };
  await syncLabels({ github, context: createContext({ issue }), core: createCore() });
  assert.deepEqual(calls.addLabels.map(call => call.labels), [["needs-version-info"]]);
  assert.deepEqual(calls.removeLabel, []);
  assert.deepEqual(calls.createComment, []);
});

test("free-form version preservation still applies other current metadata deltas", async () => {
  const { github, calls } = createGithub();
  const issue = { number: 2924, title: "Guest reads stop after restart",
    body: "## Environment\n\n- Pulse Pro 6.5.0\n\n### Additional actionable topics\n\nHistory is also missing.\n\n### Confirmations\n\nDone.",
    labels: ["bug", "affects-6.5.0", "needs-human"].map(name => ({ name })) };
  await syncLabels({ github, context: createContext({ issue }), core: createCore() });
  assert.deepEqual(calls.addLabels.map(call => call.labels), [["needs-decomposition"]]);
  assert.deepEqual(calls.removeLabel, []);
  assert.deepEqual(calls.getLabel, ["needs-decomposition"]);
  assert.equal(calls.getIssue.length, 2);
  assert.deepEqual(calls.createComment, []);
});

test("free-form version preservation cannot apply an unrelated stale metadata delta", async () => {
  const { github, calls } = createGithub();
  const issue = { number: 2925, title: "Bug",
    body: "### Additional actionable topics\n\nHistory is missing too.\n\n### Confirmations\n\nDone.",
    labels: ["bug", "affects-6.5.0"].map(name => ({ name })) };
  github.rest.issues.getLabel = async () => {
    github.currentIssue = { ...github.currentIssue, state: "closed" };
    return { data: {} };
  };
  await syncLabels({ github, context: createContext({ issue }), core: createCore() });
  assert.deepEqual(calls.addLabels, []);
  assert.deepEqual(calls.removeLabel, []);
  assert.equal(calls.getIssue.length, 2);
});

test("a delayed known-version event preserves the current report for whole-thread review", async () => {
  const eventIssue = { number: 2917, title: "Bug", body: "### Pulse version\n6.4.5\n",
    labels: [{ name: "bug" }, { name: "affects-6.4.5" }] };
  const { github, calls } = createGithub({ currentIssue: {
    ...eventIssue, state: "open", body: "### Pulse version\nunknown\n",
    updated_at: "2026-10-14T11:00:00Z",
    labels: [...eventIssue.labels, { name: "needs-retest-on-latest" }, { name: "needs-version-info" }],
  } });
  await syncLabels({ github, context: createContext({ action: "edited", issue: eventIssue }), core: createCore() });
  assert.equal(calls.addLabels.length, 0);
  assert.deepEqual(calls.removeLabel, []);
  assert.equal(calls.getIssue.length, 1, "a superseded event has no version correction to apply");
  assert.equal(calls.createComment.length, 0);
});

test("unknown-version removal cannot overwrite a correction during preparation", async () => {
  const issue = { number: 2918, title: "Bug", body: "### Pulse version\nunknown\n",
    labels: ["bug", "affects-6.5.0", "needs-version-info"].map(name => ({ name })) };
  const { github, calls } = createGithub();
  github.rest.issues.getLabel = async () => {
    github.currentIssue = { ...github.currentIssue, body: "### Pulse version\n6.5.0\n" };
    return { data: {} };
  };
  await syncLabels({ github, context: createContext({ action: "edited", issue, previousBody: "### Pulse version\n6.5.0" }), core: createCore() });
  assert.equal(calls.getIssue.length, 2);
  assert.equal(calls.addLabels.length, 0);
  assert.equal(calls.removeLabel.length, 0);
});

test("unknown versions do not remove metadata from non-bug feedback", async () => {
  const { github, calls } = createGithub();
  const issue = { number: 2919, title: "Documentation", body: "### Pulse version\nunknown\n",
    labels: ["documentation", "affects-6.5.0", "needs-retest-on-latest"].map(name => ({ name })) };
  await syncLabels({ github, context: createContext({ issue }), core: createCore() });
  assert.equal(calls.addLabels.length, 0);
  assert.equal(calls.removeLabel.length, 0);
  assert.equal(calls.getIssue.length, 1);
});

test("already unknown-version metadata is a no-op without an extra live read", async () => {
  const { github, calls } = createGithub({ existingLabels: new Set(["needs-version-info"]) });
  const issue = { number: 2920, title: "Bug", body: "### Pulse version\nunknown\n",
    labels: ["bug", "needs-version-info", "needs-human", "affects-main"].map(name => ({ name })) };
  await syncLabels({ github, context: createContext({ issue }), core: createCore() });
  assert.equal(calls.addLabels.length, 0);
  assert.equal(calls.removeLabel.length, 0);
  assert.equal(calls.getIssue.length, 1);
});

test("attempted server archives separate release versions from packaging", () => {
  const { extractPulseVersion } = triage.internals;
  for (const version of ["6.5.0", "6.5.0-rc.1", "6.6.0-beta.3", "6.6.0-dev.branch-2"]) {
    for (const filename of [
      ...["386", "amd64", "arm64", "armv6", "armv7"].map(arch =>
        `pulse-v${version}-linux-${arch}.tar.gz`),
      `pulse-v${version}.tar.gz`,
      `pulse-${version}.tgz`,
    ]) {
      for (const body of [
        `### Pulse version\n${filename}\n### Agent version\nnone`,
        `Pulse version: **\`${filename}\`**`,
      ]) {
        assert.equal(extractPulseVersion("Failed install", body), version, filename);
      }
    }
  }
});

test("non-server artifacts cannot supply an attempted server version", () => {
  const { extractPulseVersion } = triage.internals;
  for (const filename of [
    "pulse-agent-v6.6.0-beta.3-linux-amd64.tar.gz",
    "pulse-agent-helper-v6.6.0-beta.3-linux-amd64.tar.gz",
    "pulse-agent-runner-v6.6.0-beta.3-linux-amd64.tar.gz",
    "pulse-mcp-v6.6.0-beta.3-windows-amd64.zip",
    "pulse-provider-msp-v6.6.0-beta.3.tar.gz",
    "pulse-v6.6.0-beta.3-linux-amd64.tar.gz.sha256",
    "pulse-v6.6.0-beta.3-linux-amd64.tar.gz.sig",
    "pulse-v6.6.0-beta.3-linux-amd64.tar.gz.sshsig",
    "pulse-v6.6.0-beta.3-release.sbom.spdx.json.sha256",
    "install.sh",
  ]) {
    assert.equal(extractPulseVersion("Upgrade from v6.4.5",
      `### Pulse version\n${filename}\n### Agent version\n6.5.0`), null, filename);
  }
});

test("archive recognition neither borrows a later filename nor shortens bare prereleases", () => {
  const { extractPulseVersion } = triage.internals;
  assert.equal(extractPulseVersion("Upgrade from v6.4.5",
    "### Pulse version\nunknown; downloaded pulse-v6.6.0-beta.3-linux-amd64.tar.gz"), null);
  for (const version of ["6.6.0-dev.branch-2", "6.6.0-beta.3-linux-amd64", "6.6.0-custom.tgz"]) {
    assert.equal(extractPulseVersion("Bug", `### Pulse version\n${version}`), version);
  }
});

for (const [filename, version] of [
  ["pulse-v6.6.0-beta.3-linux-amd64.tar.gz", "6.6.0-beta.3"],
  ["pulse-v6.6.0-beta.3-linux-armv7.tar.gz", "6.6.0-beta.3"],
  ["pulse-v6.6.0-beta.3.tar.gz", "6.6.0-beta.3"],
  ["pulse-6.6.0-beta.3.tgz", "6.6.0-beta.3"],
  ["pulse-v6.5.0-linux-amd64.tar.gz", "6.5.0"],
]) {
  test(`archive intake reconciles exact-version labels silently: ${filename}`, async () => {
    const { github, calls } = createGithub();
    const independentlyEstablishedLabel = "affects-6.7.0";
    const issue = { number: 2921, title: "[Bug]: Failed install",
      body: `### Pulse version\n${filename}\n### Agent version\nnone`,
      labels: ["bug", "affects-6.4.5", independentlyEstablishedLabel, "needs-version-info", "needs-human",
        "needs-retest-on-latest"].map(name => ({ name })) };
    await syncLabels({ github, context: createContext({ action: "edited", issue, previousBody: "### Pulse version\n6.4.5" }), core: createCore() });
    assert.deepEqual(calls.addLabels.map(call => call.labels), [[`affects-${version}`]]);
    assert.deepEqual(calls.removeLabel.map(call => call.name),
      ["affects-6.4.5", "needs-version-info"]);
    assert.deepEqual(calls.createLabel.map(call => call.name), [`affects-${version}`]);
    assert.deepEqual(calls.createComment, []);
    assert.deepEqual(calls.getLatestRelease, []);
    assert.deepEqual(calls.paginate, []);
    assert.equal(calls.getIssue.length, 2);
  });
}

// A later retest can establish an affected version without changing the original
// form. #2077 has both its original 6.4.1 and a later 6.6.0 report. An unrelated
// edit or reopen must not erase that independently reviewed evidence.
test("unchanged version fields preserve later whole-thread version evidence", async () => {
  for (const declaration of ["### Pulse version\n6.4.1", "Pulse version: 6.4.1",
    "### Pulse version\nunknown"]) {
    for (const action of ["edited", "reopened"]) {
      const { github, calls } = createGithub();
      const issue = { number: 2077, title: "CORE monitoring still missing",
        body: `${declaration}\n\n### Additional context\nCorrected spelling.`,
        labels: ["bug", "affects-6.4.1", "affects-6.6.0", "needs-human",
          "needs-retest-on-latest"].map(name => ({ name })) };
      await syncLabels({ github, core: createCore(), context: createContext({ action, issue,
        previousBody: `${declaration}\n\n### Additional context\nOriginal spelling.` }) });
      assert.deepEqual(calls.addLabels, [], `${action}: ${declaration}`);
      assert.deepEqual(calls.removeLabel, [], `${action}: ${declaration}`);
      assert.deepEqual(calls.createLabel, []);
      assert.deepEqual(calls.createComment, []);
      assert.deepEqual(calls.getLatestRelease, []);
      assert.deepEqual(calls.paginate, []);
    }
  }
});

test("a current version correction retires only its own previous declaration", async () => {
  for (const current of ["6.5.0", "unknown"]) {
    const { github, calls } = createGithub();
    const issue = { number: 2930, title: "Connection failed",
      body: `### Pulse version\n${current}`,
      labels: ["bug", "affects-6.4.1", "affects-6.6.0", "needs-version-info",
        "needs-human", "needs-retest-on-latest"].map(name => ({ name })) };
    await syncLabels({ github, core: createCore(), context: createContext({ action: "edited",
      issue, previousBody: "### Pulse version\n6.4.1" }) });
    assert.deepEqual(calls.addLabels.map(call => call.labels), current === "unknown" ? [] : [["affects-6.5.0"]]);
    assert.deepEqual(calls.removeLabel.map(call => call.name), ["affects-6.4.1", "needs-version-info"]);
    assert.ok(!calls.createLabel.some(call => call.name === "needs-version-info"));
    assert.deepEqual(calls.createComment, []);
    assert.deepEqual(calls.getLatestRelease, []);
    assert.deepEqual(calls.paginate, []);
  }
});

test("unattributed and superseded events cannot retire established versions", async () => {
  for (const scenario of ["opened", "missing previous body", "newer live body", "newer live activity"]) {
    const { github, calls } = createGithub();
    const issue = { number: 2931, title: "A monitoring report", body: "### Pulse version\n6.4.1",
      labels: ["bug", "affects-6.4.1", "affects-6.6.0"].map(name => ({ name })) };
    const context = createContext({ action: scenario === "opened" ? "opened" : "edited", issue,
      ...(scenario === "missing previous body" ? {} : { previousBody: "### Pulse version\n6.6.0" }) });
    if (scenario.startsWith("newer live")) {
      github.currentIssue = { ...context.payload.issue, state: "open",
        ...(scenario === "newer live body" ? { body: "### Pulse version\nunknown" } : {}),
        updated_at: "2026-10-08T11:00:00Z" };
    }
    await syncLabels({ github, core: createCore(), context });
    assert.deepEqual(calls.addLabels, [], scenario);
    assert.deepEqual(calls.removeLabel, [], scenario);
    assert.deepEqual(calls.createLabel, [], scenario);
    assert.deepEqual(calls.createComment, [], scenario);
  }
});

test("a newly declared version adds evidence without claiming other labels were wrong", async () => {
  const { github, calls } = createGithub();
  const issue = { number: 2932, title: "Bug", body: "### Pulse version\n6.5.0",
    labels: ["bug", "affects-6.4.1", "needs-version-info"].map(name => ({ name })) };
  await syncLabels({ github, core: createCore(), context: createContext({ action: "edited",
    issue, previousBody: "Version evidence was supplied in a comment." }) });
  assert.deepEqual(calls.addLabels.map(call => call.labels), [["affects-6.5.0"]]);
  assert.deepEqual(calls.removeLabel.map(call => call.name), ["needs-version-info"]);
  assert.deepEqual(calls.createComment, []);
});

test("preserved form versions do not hide a newly declared secondary symptom", async () => {
  const { github, calls } = createGithub();
  const previousBody = "### Additional actionable topics\nNone\n### Pulse version\n6.4.1";
  const issue = { number: 2933, title: "Bug",
    body: previousBody.replace("None", "History is also missing."),
    labels: ["bug", "affects-6.4.1", "affects-6.6.0"].map(name => ({ name })) };
  await syncLabels({ github, core: createCore(), context: createContext({ action: "edited",
    issue, previousBody }) });
  assert.deepEqual(calls.addLabels.map(call => call.labels), [["needs-decomposition"]]);
  assert.deepEqual(calls.removeLabel, []);
  assert.deepEqual(calls.createComment, []);
});

function delayedTopicContext(action = "edited") {
  const previousBody = "### Additional actionable topics\nNone\n\n### Additional context\nOriginal logs.";
  const issue = {
    number: 3030, title: "Mixed report", state: "open",
    body: previousBody.replace("None", "History is also missing."),
    labels: ["bug", "affects-6.6.0", "needs-human", "needs-retest-on-latest", "operator-reviewed"]
      .map(name => ({ name })),
    updated_at: "2026-10-17T02:00:00Z",
  };
  return createContext({ action, issue, previousBody });
}

for (const laterActivity of ["comment", "title edit", "unrelated body edit"]) {
  test(`delayed topic declaration survives a later ${laterActivity}`, async () => {
    const context = delayedTopicContext();
    const currentIssue = { ...context.payload.issue, updated_at: "2026-10-17T02:01:00Z" };
    if (laterActivity === "title edit") currentIssue.title = "Clarified mixed report";
    if (laterActivity === "unrelated body edit") currentIssue.body = currentIssue.body.replace("Original logs.", "Later logs.");
    const { github, calls } = createGithub({ currentIssue, issueEvents: [
      // A previous task was cleared before this new symptom was declared.
      { event: "unlabeled", label: { name: "needs-decomposition" }, created_at: "2026-10-17T01:59:59Z" },
      { event: "unlabeled", label: { name: "needs-input" }, created_at: currentIssue.updated_at },
    ] });
    await syncLabels({ github, context, core: createCore() });
    assert.deepEqual(calls.addLabels.map(call => call.labels), [["needs-decomposition"]]);
    assert.deepEqual(calls.eventHistory, [{ owner: "rcourtman", repo: "Pulse", issue_number: 3030, per_page: 100 }]);
    assert.deepEqual(calls.removeLabel, []);
    assert.deepEqual(calls.createComment, []);
    assert.deepEqual(calls.getLatestRelease, []);
    assert.equal(calls.getIssue.length, 2);

    // A subsequent unrelated edit cannot be relied on to rescue the original
    // declaration: it adds no task itself and must not reset Community state.
    github.currentIssue = { ...currentIssue, labels: [...currentIssue.labels, { name: "needs-decomposition" }] };
    await syncLabels({ github, core: createCore(), context: createContext({
      action: "edited", issue: github.currentIssue, previousBody: context.payload.issue.body,
    }) });
    assert.equal(calls.addLabels.length, 1);
    assert.equal(calls.eventHistory.length, 1);
  });
}

test("delayed topic declaration survives an opening overtaken by a comment", async () => {
  const context = delayedTopicContext("opened");
  const { github, calls } = createGithub({ currentIssue: {
    ...context.payload.issue, updated_at: "2026-10-17T02:01:00Z",
  } });
  await syncLabels({ github, context, core: createCore() });
  assert.deepEqual(calls.addLabels.map(call => call.labels), [["needs-decomposition"]]);
  assert.deepEqual(calls.removeLabel, []);
  assert.deepEqual(calls.createComment, []);
});

for (const removedAt of ["2026-10-17T02:00:00Z", "2026-10-17T02:00:30Z", undefined, "invalid"]) {
  test(`delayed topic review is not restored after an explicit removal at ${removedAt}`, async () => {
    const context = delayedTopicContext();
    const { github, calls } = createGithub({ currentIssue: {
      ...context.payload.issue, updated_at: "2026-10-17T02:01:00Z",
    }, issueEvents: [
      ...Array.from({ length: 125 }, () => ({ event: "renamed", created_at: "2026-10-17T02:00:20Z" })),
      { event: "unlabeled", label: { name: "needs-decomposition" }, created_at: removedAt },
    ] });
    await syncLabels({ github, context, core: createCore() });
    assert.equal(calls.eventHistory.length, 1, "inspect the complete paginated aggregate, including a late removal");
    assert.deepEqual(calls.addLabels, []);
    assert.deepEqual(calls.removeLabel, []);
    assert.deepEqual(calls.createLabel, []);
    assert.deepEqual(calls.createComment, []);
  });
}

for (const scenario of ["changed topic", "cleared topic", "reopen", "unrelated event", "missing previous body", "missing event time", "missing current time", "future event", "already queued"]) {
  test(`delayed topic history is not consulted for ${scenario}`, async () => {
    const context = delayedTopicContext();
    const currentIssue = structuredClone(context.payload.issue);
    currentIssue.updated_at = "2026-10-17T02:01:00Z";
    if (scenario === "changed topic") currentIssue.body = currentIssue.body.replace("History is also missing.", "A different problem.");
    if (scenario === "cleared topic") currentIssue.body = currentIssue.body.replace("History is also missing.", "None");
    if (scenario === "reopen") context.payload.action = "reopened";
    if (scenario === "unrelated event") context.payload.changes.body.from = context.payload.issue.body;
    if (scenario === "missing previous body") delete context.payload.changes;
    if (scenario === "missing event time") delete context.payload.issue.updated_at;
    if (scenario === "missing current time") delete currentIssue.updated_at;
    if (scenario === "future event") context.payload.issue.updated_at = "2026-10-17T02:02:00Z";
    if (scenario === "already queued") currentIssue.labels.push({ name: "needs-decomposition" });
    const { github, calls } = createGithub({ currentIssue });
    await syncLabels({ github, context, core: createCore() });
    assert.deepEqual(calls.eventHistory, []);
    assert.deepEqual(calls.addLabels, []);
    assert.deepEqual(calls.removeLabel, []);
    assert.deepEqual(calls.createComment, []);
  });
}

for (const failure of ["access error", "unavailable history"]) {
  test(`delayed topic declaration fails closed on ${failure}`, async () => {
    const context = delayedTopicContext();
    const { github, calls } = createGithub({ currentIssue: {
      ...context.payload.issue, updated_at: "2026-10-17T02:01:00Z",
    } });
    github.paginate = async () => {
      if (failure === "access error") throw Object.assign(new Error("Forbidden"), { status: 403 });
      return undefined;
    };
    await assert.rejects(syncLabels({ github, context, core: createCore() }));
    assert.deepEqual(calls.addLabels, []);
    assert.deepEqual(calls.removeLabel, []);
    assert.deepEqual(calls.createLabel, []);
    assert.deepEqual(calls.createComment, []);
  });
}

for (const change of ["close", "body edit", "label review"]) {
  test(`delayed topic declaration keeps the pre-write guard after a concurrent ${change}`, async () => {
    const context = delayedTopicContext();
    const currentIssue = { ...context.payload.issue, updated_at: "2026-10-17T02:01:00Z" };
    const { github, calls } = createGithub({ currentIssue, existingLabels: new Set(["needs-decomposition"]) });
    github.paginate = async () => {
      const changed = { ...currentIssue, updated_at: "2026-10-17T02:02:00Z" };
      if (change === "close") changed.state = "closed";
      if (change === "body edit") changed.body = changed.body.replace("History is also missing.", "None");
      if (change === "label review") changed.labels = [...changed.labels, { name: "needs-input" }];
      github.currentIssue = changed;
      return [];
    };
    await syncLabels({ github, context, core: createCore() });
    assert.equal(calls.getIssue.length, 2);
    assert.deepEqual(calls.addLabels, []);
    assert.deepEqual(calls.removeLabel, []);
    assert.deepEqual(calls.createLabel, []);
    assert.deepEqual(calls.createComment, []);
  });
}


function duplicateLabelError() {
  return Object.assign(new Error("Repository label already exists"), {
    status: 422,
    response: { data: { errors: [{ resource: "Label", field: "name", code: "already_exists" }] } },
  });
}

for (const [label, labels, body] of [
  ["affects-6.6.0-rc.2", ["bug"], "### Pulse version\n6.6.0-rc.2\n"],
  ["needs-version-info", ["bug"], "### Pulse version\nunknown\n"],
  ["needs-decomposition", ["enhancement"], "### Additional actionable topics\nA separate History failure\n"],
]) {
  test(`concurrent issue jobs both complete after creating the same ${label} label`, async () => {
    const definitions = new Set();
    let missingReads = 0;
    let releaseReads;
    const bothMissing = new Promise(resolve => { releaseReads = resolve; });
    const jobs = [2920, 2921].map(number => {
      const issue = { number, title: "Report", body, labels: labels.map(name => ({ name })) };
      const { github, calls } = createGithub({ existingLabels: definitions });
      const readLabel = github.rest.issues.getLabel;
      github.rest.issues.getLabel = async payload => {
        try {
          return await readLabel(payload);
        } catch (error) {
          if (error.status === 404) {
            missingReads += 1;
            if (missingReads === 2) releaseReads();
            await bothMissing;
          }
          throw error;
        }
      };
      github.rest.issues.createLabel = async payload => {
        calls.createLabel.push(payload);
        if (definitions.has(payload.name)) throw duplicateLabelError();
        definitions.add(payload.name);
        return { data: payload };
      };
      return { github, calls, context: createContext({ issue }) };
    });

    const results = await Promise.allSettled(jobs.map(job => syncLabels({ ...job, core: createCore() })));
    assert.deepEqual(results.map(result => result.status), ["fulfilled", "fulfilled"]);
    assert.equal(missingReads, 2, "both jobs must observe the initial missing definition");
    assert.deepEqual([...definitions], [label]);
    assert.deepEqual(jobs.map(job => job.calls.getLabel.length).sort(), [1, 2], "one conflict readback, no write retry");
    for (const { calls, context } of jobs) {
      assert.equal(calls.createLabel.length, 1);
      assert.deepEqual(calls.createLabel[0], {
        owner: "rcourtman", repo: "Pulse", name: label,
        color: label.startsWith("affects-") ? "0e8a16" : "fbca04",
        description: label.startsWith("affects-") ? "Bug reported against Pulse 6.6.0-rc.2"
          : label === "needs-version-info" ? "Issue is missing required Pulse version metadata"
            : "Issue declares additional actionable topics that need linked dispositions",
      });
      assert.equal(calls.getIssue.length, 2, "normal fresh pre-write check still runs");
      assert.deepEqual(calls.addLabels, [{ owner: "rcourtman", repo: "Pulse", issue_number: context.payload.issue.number, labels: [label] }]);
      assert.deepEqual(calls.removeLabel, []);
      assert.deepEqual(calls.createComment, []);
      assert.deepEqual(calls.getLatestRelease, []);
    }
  });
}

for (const [scenario, error] of [
  ["forbidden", Object.assign(new Error("Forbidden"), { status: 403 })],
  ["transport uncertainty", new Error("Connection closed")],
  ["server error", Object.assign(new Error("Server error"), { status: 500 })],
  ["unspecified validation", Object.assign(new Error("Validation failed"), { status: 422 })],
  ["malformed validation details", Object.assign(duplicateLabelError(), { response: { data: { errors: "already_exists" } } })],
  ["different validation", Object.assign(duplicateLabelError(), { response: { data: { errors: [{ resource: "Label", field: "color", code: "invalid" }] } } })],
  ["mixed duplicate and validation", Object.assign(duplicateLabelError(), { response: { data: { errors: [
    { resource: "Label", field: "name", code: "already_exists" },
    { resource: "Label", field: "color", code: "invalid" },
  ] } } })],
]) {
  test(`label creation preserves ${scenario} without another provider read or write`, async () => {
    const issue = { number: 2922, title: "Report", body: "### Pulse version\n6.6.0-rc.2\n", labels: [{ name: "bug" }] };
    const { github, calls } = createGithub();
    github.rest.issues.createLabel = async payload => { calls.createLabel.push(payload); throw error; };
    await assert.rejects(syncLabels({ github, context: createContext({ issue }), core: createCore() }), received => received === error);
    assert.equal(calls.getLabel.length, 1);
    assert.equal(calls.createLabel.length, 1);
    assert.equal(calls.getIssue.length, 1);
    assert.deepEqual(calls.addLabels, []);
    assert.deepEqual(calls.removeLabel, []);
    assert.deepEqual(calls.createComment, []);
  });
}

for (const status of [403, 404, 500]) {
  test(`a duplicate-label conflict requires successful definition readback (${status})`, async () => {
    const issue = { number: 2923, title: "Report", body: "### Pulse version\n6.6.0-rc.2\n", labels: [{ name: "bug" }] };
    const { github, calls } = createGithub();
    const readbackError = Object.assign(new Error("Readback failed"), { status });
    github.rest.issues.getLabel = async payload => {
      calls.getLabel.push(payload.name);
      throw calls.getLabel.length === 1 ? Object.assign(new Error("Missing"), { status: 404 }) : readbackError;
    };
    github.rest.issues.createLabel = async payload => { calls.createLabel.push(payload); throw duplicateLabelError(); };
    await assert.rejects(syncLabels({ github, context: createContext({ issue }), core: createCore() }), error => error === readbackError);
    assert.equal(calls.getLabel.length, 2);
    assert.equal(calls.createLabel.length, 1);
    assert.equal(calls.getIssue.length, 1);
    assert.deepEqual(calls.addLabels, []);
    assert.deepEqual(calls.removeLabel, []);
    assert.deepEqual(calls.createComment, []);
  });
}

for (const existing of [null, { name: "affects-6.5.0" }]) {
  test(`duplicate-label readback cannot substitute an unavailable or different definition: ${JSON.stringify(existing)}`, async () => {
    const issue = { number: 2924, title: "Report", body: "### Pulse version\n6.6.0-rc.2\n", labels: [{ name: "bug" }] };
    const { github, calls } = createGithub();
    const conflict = duplicateLabelError();
    github.rest.issues.getLabel = async payload => {
      calls.getLabel.push(payload.name);
      if (calls.getLabel.length === 1) throw Object.assign(new Error("Missing"), { status: 404 });
      return { data: existing };
    };
    github.rest.issues.createLabel = async payload => { calls.createLabel.push(payload); throw conflict; };
    await assert.rejects(syncLabels({ github, context: createContext({ issue }), core: createCore() }), error => error === conflict);
    assert.equal(calls.getLabel.length, 2);
    assert.equal(calls.createLabel.length, 1);
    assert.deepEqual(calls.addLabels, []);
    assert.deepEqual(calls.removeLabel, []);
    assert.deepEqual(calls.createComment, []);
  });
}

test("duplicate-label readback cannot override a newer Community disposition", async () => {
  const issue = { number: 2925, state: "open", title: "Report", updated_at: "2026-10-08T10:00:00Z",
    body: "### Pulse version\n6.6.0-rc.2\n", labels: [{ name: "bug" }] };
  const { github, calls } = createGithub({ currentIssue: structuredClone(issue) });
  github.rest.issues.getLabel = async payload => {
    calls.getLabel.push(payload.name);
    if (calls.getLabel.length === 1) throw Object.assign(new Error("Missing"), { status: 404 });
    github.currentIssue = { ...issue, state: "closed" };
    return { data: { name: payload.name } };
  };
  github.rest.issues.createLabel = async payload => { calls.createLabel.push(payload); throw duplicateLabelError(); };
  await syncLabels({ github, context: createContext({ issue }), core: createCore() });
  assert.equal(calls.getLabel.length, 2);
  assert.equal(calls.createLabel.length, 1);
  assert.equal(calls.getIssue.length, 2);
  assert.deepEqual(calls.addLabels, []);
  assert.deepEqual(calls.removeLabel, []);
  assert.deepEqual(calls.createComment, []);
});

const serverReferenceDigest = "a".repeat(64);
const serverReferenceCases = [
  ["rcourtman/pulse:6.5.0", "6.5.0"],
  ["rcourtman/pulse:v6.6.0-rc.2", "6.6.0-rc.2"],
  ["license.pulserelay.pro/pulse-pro:6.6.0-beta.3", "6.6.0-beta.3"],
  [`license.pulserelay.pro/pulse-pro:6.6.0-rc.2@sha256:${serverReferenceDigest}`, "6.6.0-rc.2"],
  [`rcourtman/pulse:6.5.0@sha256:${serverReferenceDigest}`, "6.5.0"],
  ["pulse-pro:6.5.0", "6.5.0"],
  ["pulse-v6.5.0", "6.5.0"],
  ["pulse_6.6.0-rc.2", null],
  ["pulse-agent:6.5.0", null],
  ["pulse-agent-v6.5.0-linux-amd64", null],
  ["pulse-agent-helper-v6.5.0-linux-amd64", null],
  ["pulse-agent-runner-v6.5.0-linux-amd64", null],
  ["pulse-mcp-v6.5.0", null],
  ["pulse-provider-msp-v6.5.0", null],
  ["rcourtman/pulse-agent:6.5.0", null],
  ["license.pulserelay.pro/pulse-pro:latest; agent 6.5.0", null],
  ["rcourtman/pulse:latest (agent version 6.5.0)", null],
  ["rcourtman/pulse:6 (last known working server 6.5.0)", null],
  ["rcourtman/pulse:6.5.0_custom", null],
  [`license.pulserelay.pro/pulse-pro@sha256:${serverReferenceDigest} (agent 6.5.0)`, null],
  ["another-product:6.5.0", null],
];

for (const [reference, version] of serverReferenceCases) {
  test(`server reference identity: ${reference}`, () => {
    for (const body of [
      `### Pulse version\n${reference}\n\n### Agent version\n6.4.5`,
      `Pulse version: **\`${reference}\`**`,
      `Pulse | Version\n\`${reference}\`\nAgent version: 6.4.5`,
      ...(reference.includes(":") ? [`### Pulse version\n_${reference}_`] : []),
    ]) {
      assert.equal(triage.internals.extractPulseVersion("Upgrade from v6.4.5", body), version, body);
    }
  });
}

for (const [reference, version] of serverReferenceCases.filter(([reference]) =>
  reference.includes(":") || reference.includes("mcp") || reference.includes("agent"))) {
  test(`server reference silent intake: ${reference}`, async () => {
    const { github, calls } = createGithub();
    const issue = { number: 2933, title: "[Bug]: Missing readings",
      body: `### Pulse version\n${reference}\n\n### Agent version\n6.4.5`,
      labels: ["bug", "needs-human", "needs-retest-on-latest", "operator-reviewed"].map(name => ({ name })) };
    await syncLabels({ github, context: createContext({ issue }), core: createCore() });
    const expected = version ? `affects-${version}` : "needs-version-info";
    assert.deepEqual(calls.addLabels.map(call => call.labels), [[expected]]);
    assert.deepEqual(calls.createLabel.map(call => call.name), [expected]);
    assert.deepEqual(calls.removeLabel, []);
    assert.equal(calls.getIssue.length, 2);
    assert.deepEqual(calls.createComment, []);
    assert.deepEqual(calls.getLatestRelease, []);
    assert.deepEqual(calls.paginate, []);
  });
}

test("server reference correction preserves independent whole-thread evidence", async () => {
  const { github, calls } = createGithub();
  const issue = { number: 2934, title: "[Bug]: Missing readings",
    body: "### Pulse version\nlicense.pulserelay.pro/pulse-pro:6.6.0-beta.3",
    labels: ["bug", "affects-6.4.5", "affects-6.7.0", "needs-version-info", "needs-human",
      "needs-retest-on-latest"].map(name => ({ name })) };
  await syncLabels({ github, core: createCore(), context: createContext({ action: "edited", issue,
    previousBody: "### Pulse version\nrcourtman/pulse:6.4.5" }) });
  assert.deepEqual(calls.addLabels.map(call => call.labels), [["affects-6.6.0-beta.3"]]);
  assert.deepEqual(calls.removeLabel.map(call => call.name), ["affects-6.4.5", "needs-version-info"]);
  assert.equal(calls.getIssue.length, 2);
  assert.deepEqual(calls.createComment, []);
  assert.deepEqual(calls.getLatestRelease, []);
  assert.deepEqual(calls.paginate, []);
});
