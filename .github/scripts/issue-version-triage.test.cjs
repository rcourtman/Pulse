const test = require("node:test");
const assert = require("node:assert/strict");
const fs = require("node:fs");
const path = require("node:path");

const triage = require("./issue-version-triage.cjs");

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

for (const [name, summary, evidence] of [
  ["bug_report.yml", "bug_description", "logs"],
  ["v6_rc_feedback.yml", "summary", "evidence"],
]) {
  test(`specialist intake guidance stays optional and beside its input: ${name}`, () => {
    const form = fs.readFileSync(path.resolve(__dirname, "../ISSUE_TEMPLATE", name), "utf8");
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
      "Notification delivery: Test versus ordinary alerts",
      "Pulse CPU, memory or disk writes: comparable measurements",
      "VM or LXC memory: guest versus hypervisor readings",
    ].sort());
    for (const [id, titles] of [
      ["upgrade_context", ["Failed upgrade:"]],
      [summary, ["Several hosts or clusters:"]],
      [evidence, ["Notification delivery:", "Pulse CPU, memory or disk writes:", "VM or LXC memory:"]],
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
  });
}

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


test("feedback classification follows visible selected text, not link destinations or titles", () => {
  const { classifyV6FeedbackType } = triage.internals;
  for (const newline of ["\n", "\r\n"]) {
    for (const [selection, expected] of [
      ['[Documentation issue](https://example.invalid/bugs)', "documentation"],
      ['[Documentation issue](https://example.invalid/context "Bug / regression")', "documentation"],
      ['[**Documentation issue**](https://example.invalid/bugs "Performance issue")', "documentation"],
      ['[UX / workflow friction](https://example.invalid/bugs)', "enhancement"],
      ['[Other actionable feedback](https://example.invalid/regression)', "enhancement"],
      ['[Unknown](https://example.invalid/bugs "Performance issue")', null],
      ['Documentation issue [details](https://example.invalid/bugs)', null],
    ]) {
      assert.equal(classifyV6FeedbackType([
        "### Feedback type", selection, "", "### Pulse version", "unknown",
      ].join(newline)), expected, selection);
    }
  }
});

test("feedback classification accepts each form choice and legacy bug selections", () => {
  const form = fs.readFileSync(path.resolve(__dirname, "../ISSUE_TEMPLATE/v6_rc_feedback.yml"), "utf8");
  const field = form.split("    id: feedback_type\n")[1].split("  - type: ")[0];
  const selections = [...field.matchAll(/^        - (.+)$/gm)].map(match => match[1]);
  const expected = {
    "Bug / regression": "bug",
    "Upgrade / migration issue": "bug",
    "UX / workflow friction": "enhancement",
    "Performance issue": "bug",
    "Documentation issue": "documentation",
    "Other actionable feedback": "enhancement",
  };
  assert.deepEqual(selections.sort(), Object.keys(expected).sort());
  for (const [selection, label] of Object.entries({ ...expected, Bug: "bug", Regression: "bug" })) {
    for (const value of [selection, selection.toUpperCase(), `**${selection}**`,
      `\`${selection}\``, `[${selection}](https://example.invalid/context)`]) {
      assert.equal(triage.internals.classifyV6FeedbackType(`### Feedback type\n${value}\n`), label, value);
    }
  }
});

test("examples, withdrawn selections and prose do not invent a feedback class", () => {
  for (const selection of [
    '![Bug / regression](https://example.invalid/image.png)',
    '~~Bug / regression~~', '> Bug / regression', '<span>Bug / regression</span>',
    'Not a bug; asking about debugging', 'Unknown; Performance issue elsewhere',
    '_No response_', 'unknown', '',
  ]) {
    assert.equal(triage.internals.classifyV6FeedbackType(
      `### Feedback type\n${selection}\n\n### What happened?\nBug / regression\n`), null, selection);
  }
});

test("syncLabels consumes the selected feedback class without invented version demands or contact", async () => {
  for (const action of ["opened", "edited", "reopened"]) {
    for (const [selection, classification] of [
      ['[Documentation issue](https://example.invalid/bugs "Performance issue")', "documentation"],
      ['[UX / workflow friction](https://example.invalid/bugs)', "enhancement"],
      ['[Unknown](https://example.invalid/bugs)', null],
      ['![Bug / regression](https://example.invalid/image.png)', null],
      ['~~Bug / regression~~', null],
    ]) {
      const { github, calls } = createGithub();
      const labels = ["needs-human", "needs-retest-on-latest", "needs-decomposition", "operator-reviewed"];
      const issue = { number: 2930, title: "Existing feedback", labels: labels.map(name => ({ name })),
        body: `### Feedback type\n${selection}\n\n### Pulse version\nunknown\n` };
      await syncLabels({ github, core: createCore(), context: createContext({ action, issue }) });
      assert.deepEqual(calls.addLabels.map(call => call.labels), classification ? [[classification]] : [], selection);
      assert.deepEqual(calls.createLabel, [], selection);
      assert.deepEqual(calls.removeLabel, [], selection);
      assert.deepEqual(calls.createComment, [], selection);
      assert.deepEqual(calls.getLatestRelease, [], selection);
      assert.deepEqual(calls.paginate, [], selection);
    }
  }
});

test("an unrecognised feedback selection cannot undo an existing maintainer bug classification", async () => {
  const { github, calls } = createGithub();
  const labels = ["bug", "affects-6.5.0", "needs-human", "needs-retest-on-latest", "needs-decomposition"];
  const issue = { number: 2931, title: "Existing report", labels: labels.map(name => ({ name })),
    body: '### Feedback type\n[Unknown](https://example.invalid/bugs)\n\n### Pulse version\n6.5.0\n' };
  await syncLabels({ github, core: createCore(), context: createContext({ issue }) });
  assert.deepEqual(calls.addLabels, []);
  assert.deepEqual(calls.removeLabel, []);
  assert.deepEqual(calls.createComment, []);
  assert.deepEqual(calls.getLatestRelease, []);
  assert.deepEqual(calls.paginate, []);
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

for (const [name, before, after] of [
  ["appended", "Add a storage filter.", "Add a storage filter.\nAlso retain failed notification attempts."],
  ["replaced", "Add a storage filter.", "Retain failed notification attempts."],
  ["revised", "Retain one failed attempt.", "Retain every failed attempt."],
]) {
  test(`syncLabels requeues ${name} secondary topics after an earlier disposition`, async () => {
    const body = value => `### Additional actionable topics\n${value}\n### Pulse version\n6.5.0\n`;
    const { github, calls } = createGithub();
    await syncLabels({
      github,
      context: createContext({ action: "edited", previousBody: body(before), issue: {
        number: 1796, title: "Monitoring workflow", body: body(after),
        labels: [{ name: "enhancement" }, { name: "operator-reviewed" }],
      } }),
      core: createCore(),
    });
    assert.deepEqual(calls.addLabels.map(call => call.labels), [["needs-decomposition"]]);
    assert.equal(calls.removeLabel.length, 0);
    assert.equal(calls.createComment.length, 0);
    assert.equal(calls.getLatestRelease.length, 0);
  });
}

for (const [name, before, after] of [
  ["an unrelated field", "### Summary\nOld wording\n### Additional actionable topics\nAdd a storage filter.\n",
    "### Summary\nNew wording\n### Additional actionable topics\nAdd a storage filter.\n"],
  ["whitespace only", "### Additional actionable topics\nRetain every failed attempt.\n",
    "### Additional actionable topics\n\nRetain every\nfailed attempt.  \n"],
  ["hidden comments only", "### Additional actionable topics\nRetain every failed attempt. <!-- old note -->\n",
    "### Additional actionable topics\nRetain every failed attempt. <!-- new note -->\n"],
  ["removing the topics", "### Additional actionable topics\nAdd a storage filter.\n",
    "### Additional actionable topics\nNone.\n"],
]) {
  test(`syncLabels does not requeue dispositioned topics for ${name}`, async () => {
    const { github, calls } = createGithub();
    await syncLabels({
      github,
      context: createContext({ action: "edited", previousBody: before, issue: {
        number: 1796, title: "Monitoring workflow", body: after,
        labels: [{ name: "enhancement" }, { name: "operator-reviewed" }],
      } }),
      core: createCore(),
    });
    assert.equal(calls.addLabels.length, 0);
    assert.equal(calls.removeLabel.length, 0);
    assert.equal(calls.createComment.length, 0);
  });
}

test("triage guidance preserves revised-topic review", () => {
  const guide = fs.readFileSync(path.resolve(__dirname, "../../docs/ISSUE_TRIAGE.md"), "utf8");
  assert.match(guide.replace(/\s+/g, " "), /adding or revising actionable content/);
  assert.match(guide, /after an earlier topic was dispositioned/);
});

test("a revised topic cannot override a newer Community disposition", async () => {
  const previousBody = "### Additional actionable topics\nRetain one failed attempt.\n";
  const issue = {
    number: 1796, title: "Monitoring workflow", state: "open",
    body: "### Additional actionable topics\nRetain every failed attempt.\n",
    updated_at: "2026-10-08T10:00:00Z", labels: [{ name: "enhancement" }],
  };
  for (const stage of ["before initial read", "during label preparation"]) {
    const { github, calls } = createGithub({ currentIssue: { ...issue } });
    if (stage === "before initial read") {
      github.currentIssue.updated_at = "2026-10-08T11:00:00Z";
    } else {
      const getLabel = github.rest.issues.getLabel;
      github.rest.issues.getLabel = async payload => {
        github.currentIssue = { ...issue, updated_at: "2026-10-08T11:00:00Z" };
        return getLabel(payload);
      };
    }
    await syncLabels({ github, core: createCore(),
      context: createContext({ action: "edited", previousBody, issue }) });
    assert.equal(calls.addLabels.length, 0, stage);
    assert.equal(calls.removeLabel.length, 0, stage);
    assert.equal(calls.createComment.length, 0, stage);
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
  await syncLabels({ github, core: createCore(), context: createContext({ issue }) });
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

const formattedNoTopicResponses = [
  "**None**",
  "*None.*",
  "__N/A__",
  "`none known`",
  "**_No response_**",
  '[None](https://example.invalid/context "not another topic")',
];

test("visible no-topic responses stay empty through ordinary inline formatting", () => {
  for (const value of formattedNoTopicResponses) {
    const body = `### Additional actionable topics\n${value}\n\n### Pulse version\n6.5.0`;
    assert.equal(triage.internals.classifyAdditionalActionableTopics(body), false, value);
  }
});

test("no-topic formatting cannot discard qualifiers, block evidence or images", () => {
  for (const value of [
    "**None**, except the saved target changes after switching organisation.",
    "None\n\n**The backup issue also affects another host.**",
    "None\n\n```text\n### Pulse version\n6.5.0\n```",
    "- **None**\n- The saved target also changes.",
    "![None](https://example.invalid/evidence.png)",
    "~~None~~",
    "`**None**`",
    "\\*None\\*",
  ]) {
    const body = `### Additional actionable topics\n${value}\n\n### Pulse version\n6.5.0`;
    assert.equal(triage.internals.classifyAdditionalActionableTopics(body), true, value);
  }
});

test("formatted no-topic edits preserve completed reviews without public contact", async () => {
  const labels = ["enhancement", "needs-human", "needs-retest-on-latest"];
  const body = value => `### Additional actionable topics\n${value}\n\n### Desired outcome\nKeep the current workflow.`;
  for (const value of formattedNoTopicResponses) {
    for (const reviewed of [false, true]) {
      for (const action of ["opened", "edited"]) {
        const { github, calls } = createGithub();
        const issueLabels = reviewed ? [...labels, "needs-decomposition"] : labels;
        const issue = { number: 2926, title: "Existing feature request", body: body(value),
          labels: issueLabels.map(name => ({ name })) };
        await syncLabels({ github, core: createCore(), context: createContext({
          action, issue, previousBody: body("None"),
        }) });
        assert.deepEqual(calls.addLabels, [], `${action}: ${value}`);
        assert.deepEqual(calls.removeLabel, [], value);
        assert.deepEqual(calls.createLabel, [], value);
        assert.deepEqual(calls.createComment, [], value);
        assert.deepEqual(calls.getLatestRelease, [], value);
        assert.deepEqual(calls.paginate, [], value);
      }
    }
  }
});

test("a qualified revision after a formatted empty response still requires review", async () => {
  for (const value of formattedNoTopicResponses) {
    const { github, calls } = createGithub();
    const body = topic => `### Additional actionable topics\n${topic}\n\n### Pulse version\n6.5.0`;
    const issue = { number: 2927, title: "Existing report", body: body(`${value}, except backup identity is also wrong.`),
      labels: ["enhancement", "needs-human"].map(name => ({ name })) };
    await syncLabels({ github, core: createCore(), context: createContext({
      action: "edited", issue, previousBody: body(value),
    }) });
    assert.deepEqual(calls.addLabels.map(call => call.labels), [["needs-decomposition"]], value);
    assert.deepEqual(calls.removeLabel, [], value);
    assert.deepEqual(calls.createComment, [], value);
    assert.deepEqual(calls.getLatestRelease, [], value);
    assert.deepEqual(calls.paginate, [], value);
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

function topicBoundaryFields() {
  const fields = new Set(["Additional context"]);
  const directory = path.resolve(__dirname, "../ISSUE_TEMPLATE");
  for (const name of fs.readdirSync(directory).filter(name => /\.ya?ml$/.test(name))) {
    const form = fs.readFileSync(path.join(directory, name), "utf8");
    for (const [, label] of form.matchAll(/^      label: (.+)$/gm)) {
      if (label !== "Additional actionable topics") fields.add(label);
    }
  }
  return [...fields];
}

test("every current form field bounds a reordered topic declaration", () => {
  const { classifyAdditionalActionableTopics } = triage.internals;
  for (const field of topicBoundaryFields()) {
    for (const newline of ["\n", "\r\n"]) {
      for (const level of [2, 3]) {
        for (const empty of ["", "None", "_No response_", "<!-- optional -->"]) {
          const body = ["### Additional actionable topics", empty, "",
            `${"#".repeat(level)} ${field.toUpperCase()}`, "Existing context, not another topic."].join(newline);
          assert.equal(classifyAdditionalActionableTopics(body), false, body);
        }
      }
    }
  }
});

test("a form-named topic subheading remains actionable inside the declaration", () => {
  const { classifyAdditionalActionableTopics } = triage.internals;
  for (const field of topicBoundaryFields()) {
    for (const level of [1, 2, 3, 4, 5]) {
      const body = [`${"#".repeat(level)} Additional actionable topics`, "None", "",
        `${"#".repeat(level + 1)} ${field}`, "A separate problem on this surface.", "",
        `${"#".repeat(level)} Pulse version`, "6.5.0"].join("\n");
      assert.equal(classifyAdditionalActionableTopics(body), true, body);
    }
  }
});

test("copied form-named headings cannot hide a later topic", () => {
  const { classifyAdditionalActionableTopics } = triage.internals;
  for (const copied of [
    "```text\n### Agent version\n6.5.0\n```",
    "> ### Agent version\n> 6.5.0",
    "<pre>\n### Agent version\n6.5.0\n</pre>",
    "<!--\n### Agent version\n6.5.0\n-->",
  ]) {
    const body = ["### Additional actionable topics", "None", "", copied, "",
      "A separate problem still needs a disposition.", "", "### OS / environment", "API only"].join("\n");
    assert.equal(classifyAdditionalActionableTopics(body), true, body);
  }
});

test("opening or editing neighbouring context cannot invent a decomposition task", async () => {
  for (const field of topicBoundaryFields()) {
    for (const action of ["opened", "edited"]) {
      const { github, calls } = createGithub();
      const body = (value) => `### Additional actionable topics\nNone\n\n### ${field}\n${value}\n`;
      const labels = ["enhancement", "needs-human", "release-blocker", "needs-retest-on-latest", "affects-6.5.0"];
      const issue = { number: 2923, title: "Existing report", body: body("Corrected existing context"),
        labels: labels.map(name => ({ name })) };
      await syncLabels({ github, core: createCore(),
        context: createContext({ action, issue, previousBody: body("Earlier existing context") }) });
      assert.deepEqual(calls.addLabels, [], `${action}: ${field}`);
      assert.deepEqual(calls.removeLabel, [], field);
      assert.deepEqual(calls.createLabel, [], field);
      assert.deepEqual(calls.createComment, [], field);
      assert.deepEqual(calls.getLatestRelease, [], field);
      assert.deepEqual(calls.paginate, [], field);
    }
  }
});

test("feature context edits preserve a cleared review while real topic edits restore it", async () => {
  const { github, calls } = createGithub();
  const body = (context, topic = "None") => [
    "### Additional actionable topics", topic, "",
    "### Desired outcome", context, "",
    "### Alternatives considered", "An existing workaround.",
  ].join("\n");
  const labels = ["enhancement", "needs-human", "needs-retest-on-latest"];
  const issue = { number: 2925, title: "Existing feature request", body: body("Clarified outcome"),
    labels: labels.map(name => ({ name })) };
  await syncLabels({ github, core: createCore(), context: createContext({
    action: "edited", issue, previousBody: body("Original outcome"),
  }) });
  assert.deepEqual(calls.addLabels, []);
  assert.deepEqual(calls.createLabel, []);

  const revised = { ...issue, body: body("Clarified outcome", "Also retain failed attempts in History.") };
  const context = createContext({
    action: "edited", issue: revised, previousBody: issue.body,
  });
  github.currentIssue = { ...context.payload.issue, state: "open" };
  await syncLabels({ github, core: createCore(), context });
  assert.deepEqual(calls.addLabels.map(call => call.labels), [["needs-decomposition"]]);
  assert.deepEqual(calls.removeLabel, []);
  assert.deepEqual(calls.createComment, []);
  assert.deepEqual(calls.getLatestRelease, []);
  assert.deepEqual(calls.paginate, []);
});

test("real revised topics still reopen review without disposing of existing tasks", async () => {
  for (const existingTask of [false, true]) {
    const { github, calls } = createGithub();
    const previousBody = "### Additional actionable topics\nNone\n### Agent version\n6.5.0";
    const body = "### Additional actionable topics\nNone\n#### Installation type\n" +
      "The saved installer selection is also lost.\n### Agent version\n6.5.0";
    const issue = { number: 2924, title: "Existing report", body,
      labels: ["enhancement", "needs-human", "needs-retest-on-latest",
        ...(existingTask ? ["needs-decomposition"] : [])].map(name => ({ name })) };
    await syncLabels({ github, core: createCore(),
      context: createContext({ action: "edited", issue, previousBody }) });
    assert.deepEqual(calls.addLabels.map(call => call.labels), existingTask ? [] : [["needs-decomposition"]]);
    assert.deepEqual(calls.removeLabel, []);
    assert.deepEqual(calls.createComment, []);
    assert.deepEqual(calls.getLatestRelease, []);
    assert.deepEqual(calls.paginate, []);
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

  assert.deepEqual(calls.getIssue, Array(2).fill({ owner: "rcourtman", repo: "Pulse", issue_number: 2800 }));
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
  await syncLabels({ github, core: createCore(), context: createContext({ issue }) });
  assert.deepEqual(calls.addLabels.map(call => call.labels), [["affects-6.5.0"]]);
  assert.deepEqual(calls.removeLabel.map(call => call.name), ["affects-6.4.5", "needs-version-info"]);
  assert.deepEqual(calls.createComment, []);
  assert.deepEqual(calls.getLatestRelease, []);
  assert.deepEqual(calls.paginate, []);
  assert.deepEqual(calls.getIssue.map(call => call.issue_number), [2902, 2902]);
});


test("triage workflow prepares and checks the exact parser before writing labels", () => {
  const workflow = fs.readFileSync(path.resolve(__dirname, "../workflows/issue-version-label-sync.yml"), "utf8");
  const install = workflow.indexOf("run: npm ci --ignore-scripts --no-audit --no-fund --prefix .github/scripts");
  const check = workflow.indexOf("run: npm run test:triage --prefix .github/scripts");
  const write = workflow.indexOf("- name: Sync issue version metadata");
  assert.ok(install >= 0 && check > install && write > check);
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
  const issue = { number: 2914, state: "open", title: "Bug",
    labels: ["bug", "needs-human", "needs-retest-on-latest", "affects-6.4.5"].map(name => ({ name })),
    body: "### Pulse version\n6.5.0\n" };
  const { github, calls } = createGithub({ currentIssue: issue });
  const read = github.rest.issues.get;
  github.rest.issues.get = async payload => {
    const result = await read(payload);
    return { data: calls.getIssue.length === 1 ? result.data : { ...issue, labels: [...issue.labels].reverse() } };
  };
  await syncLabels({ github, context: createContext({ issue }), core: createCore() });
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

test("unknown current versions retire only managed exact-version labels", async () => {
  const preserved = ["bug", "needs-retest-on-latest", "needs-decomposition",
    "needs-human", "operator-reviewed", "affects-main", "affects-storage"];
  for (const body of [
    "### Pulse version\nunknown\n",
    "Pulse version: unknown; agent version 6.5.0",
    "### Pulse version\n6.5\n### Agent version\n6.5.0",
    "### Pulse version\n_No response_\n",
    "### Pulse version\n\n### Agent version\n6.5.0",
    "The running version is unavailable.",
  ]) {
    const { github, calls } = createGithub();
    const issue = { number: 2916, title: "Upgrade from v6.4.5", body,
      labels: [...preserved, "affects-6.4.5", "affects-6.6.0-beta.3"].map(name => ({ name })) };
    await syncLabels({ github, context: createContext({ action: "edited", issue }), core: createCore() });
    assert.deepEqual(calls.addLabels.map(call => call.labels), [["needs-version-info"]], body);
    assert.deepEqual(calls.removeLabel.map(call => call.name), ["affects-6.4.5", "affects-6.6.0-beta.3"], body);
    assert.equal(calls.getIssue.length, 2);
    assert.equal(calls.getLatestRelease.length, 0);
    assert.equal(calls.createComment.length, 0);
  }
});

test("a delayed known-version event cleans metadata from the current unknown report", async () => {
  const eventIssue = { number: 2917, title: "Bug", body: "### Pulse version\n6.4.5\n",
    labels: [{ name: "bug" }, { name: "affects-6.4.5" }] };
  const { github, calls } = createGithub({ currentIssue: {
    ...eventIssue, state: "open", body: "### Pulse version\nunknown\n",
    updated_at: "2026-10-14T11:00:00Z",
    labels: [...eventIssue.labels, { name: "needs-retest-on-latest" }, { name: "needs-version-info" }],
  } });
  await syncLabels({ github, context: createContext({ action: "edited", issue: eventIssue }), core: createCore() });
  assert.equal(calls.addLabels.length, 0);
  assert.deepEqual(calls.removeLabel.map(call => call.name), ["affects-6.4.5"]);
  assert.equal(calls.getIssue.length, 2, "a removals-only delta still needs the live-state fence");
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
  await syncLabels({ github, context: createContext({ action: "edited", issue }), core: createCore() });
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

test("Pro server artifacts retain their declared version without borrowing agent evidence", () => {
  const { extractPulseVersion } = triage.internals;
  for (const version of ["6.5.0", "6.6.0-beta.3", "6.6.0-rc.6"]) {
    for (const artifact of [
      `pulse-pro-v${version}-linux-amd64.tar.gz`,
      `pulse-pro-v${version}-linux-arm64.tar.gz`,
      `license.pulserelay.pro/pulse-pro:${version}`,
    ]) {
      for (const value of [artifact, `**\`${artifact}\`**`, `[${artifact}](https://example.invalid/6.4.5)`]) {
        for (const field of ["### Pulse version\n", "Pulse version: ", "- **Pulse version**: "]) {
          for (const newline of ["\n", "\r\n"]) {
            const body = `${field}${value}\n\n### Agent version\n6.4.5`.replaceAll("\n", newline);
            assert.equal(extractPulseVersion("Upgrade from v6.4.5", body), version, body);
          }
        }
      }
    }
  }
  for (const value of [
    "pulse-pro-agent-v6.6.0-rc.6-linux-amd64.tar.gz",
    "pulse-pro-v6.6.0-rc.6-linux-amd64.tar.gz.sshsig",
    "pulse-pro-v6.6.0-rc.6-linux-amd64.tar.gz.sha256",
    "pulse-pro-v6.6.0-rc.6-linux-amd64.tar.gz.sig",
    "unknown; downloaded pulse-pro-v6.6.0-rc.6-linux-amd64.tar.gz",
    "[unknown](https://example.invalid/pulse-pro-v6.6.0-rc.6-linux-amd64.tar.gz)",
  ]) {
    assert.equal(extractPulseVersion("Upgrade from v6.4.5",
      `### Pulse version\n${value}\n### Agent version\n6.4.5`), null, value);
  }
});

for (const value of [
  "pulse-pro-v6.6.0-rc.6-linux-amd64.tar.gz",
  "pulse-pro-v6.6.0-rc.6-linux-arm64.tar.gz",
  "license.pulserelay.pro/pulse-pro:6.6.0-rc.6",
]) {
  test(`Pro server intake updates only silent version metadata: ${value}`, async () => {
    const { github, calls } = createGithub();
    const issue = { number: 2923, title: "Failed install", labels: [
      "bug", "affects-6.4.5", "needs-version-info", "needs-human", "release-blocker",
      "needs-decomposition", "needs-retest-on-latest",
    ].map(name => ({ name })),
    body: `### Pulse version\n${value}\n### Agent version\n6.4.5` };
    await syncLabels({ github, context: createContext({ issue }), core: createCore() });
    assert.deepEqual(calls.addLabels.map(call => call.labels), [["affects-6.6.0-rc.6"]]);
    assert.deepEqual(calls.removeLabel.map(call => call.name), ["affects-6.4.5", "needs-version-info"]);
    assert.deepEqual(calls.createLabel.map(call => call.name), ["affects-6.6.0-rc.6"]);
    assert.deepEqual(calls.createComment, []);
    assert.deepEqual(calls.getLatestRelease, []);
    assert.deepEqual(calls.paginate, []);
    assert.equal(calls.getIssue.length, 2);
  });
}

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
    const oldArtifactLabel = `affects-${filename.replace(/^pulse-v?/, "")}`;
    const issue = { number: 2921, title: "[Bug]: Failed install",
      body: `### Pulse version\n${filename}\n### Agent version\nnone`,
      labels: ["bug", "affects-6.4.5", oldArtifactLabel, "needs-version-info", "needs-human",
        "needs-retest-on-latest"].map(name => ({ name })) };
    await syncLabels({ github, context: createContext({ issue }), core: createCore() });
    assert.deepEqual(calls.addLabels.map(call => call.labels), [[`affects-${version}`]]);
    assert.deepEqual(calls.removeLabel.map(call => call.name),
      ["affects-6.4.5", oldArtifactLabel, "needs-version-info"]);
    assert.deepEqual(calls.createLabel.map(call => call.name), [`affects-${version}`]);
    assert.deepEqual(calls.createComment, []);
    assert.deepEqual(calls.getLatestRelease, []);
    assert.deepEqual(calls.paginate, []);
    assert.equal(calls.getIssue.length, 2);
  });
}

test("declared versions use visible release-link text, including nested inline formatting", () => {
  const { extractPulseVersion } = triage.internals;
  for (const [value, expected] of [
    ["[v6.5.0](https://github.com/rcourtman/Pulse/releases/tag/v6.4.5)", "6.5.0"],
    ["**[6.5.0](https://github.com/rcourtman/Pulse/releases/tag/v6.5.0)**", "6.5.0"],
    ["_[`v6.5.0`](https://github.com/rcourtman/Pulse/releases/tag/v6.5.0)_", "6.5.0"],
    ["[6\\.5\\.0](https://github.com/rcourtman/Pulse/releases/tag/v6.5.0)", "6.5.0"],
    ["[pulse-v6.6.0-beta.3-linux-amd64.tar.gz](https://github.com/rcourtman/Pulse/releases/tag/v6.6.0-beta.3)", "6.6.0-beta.3"],
  ]) {
    for (const body of [
      `### Pulse version\n${value}\n### Agent version\n6.4.5`,
      `Pulse version: ${value}`,
      `- **Pulse version**: ${value}`,
    ]) {
      for (const newline of ["\n", "\r\n"]) {
        assert.equal(extractPulseVersion("Upgrade from v6.4.5", body.replaceAll("\n", newline)), expected, body);
      }
    }
  }
});

test("link targets, titles, images and literal Markdown cannot invent a declared version", () => {
  const { extractPulseVersion } = triage.internals;
  for (const value of [
    "[unknown](https://github.com/rcourtman/Pulse/releases/tag/v6.5.0)",
    '[unknown](https://github.com/rcourtman/Pulse "6.5.0")',
    "![6.5.0](https://github.com/rcourtman/Pulse/releases/tag/v6.5.0)",
    "~~[6.5.0](https://github.com/rcourtman/Pulse/releases/tag/v6.5.0)~~",
    "`[6.5.0](https://github.com/rcourtman/Pulse/releases/tag/v6.5.0)`",
    "<span>6.5.0</span>",
    "[release](https://github.com/rcourtman/Pulse/releases/tag/v6.5.0) 6.4.5",
    "[6.5.0](unterminated",
  ]) {
    assert.equal(extractPulseVersion("Upgrade from v6.4.5",
      `### Pulse version\n${value}\n### Agent version\n6.5.0`), null, value);
  }
});

test("the label consumer accepts a linked version without fetching links or contacting the reporter", async () => {
  for (const [visible, expected] of [["**`v6.5.0`**", "6.5.0"], ["unknown", null]]) {
    const { github, calls } = createGithub();
    const issue = { number: 2922, title: "[Bug]: Upgrade from v6.4.5",
      body: `### Pulse version\n[${visible}](https://github.com/rcourtman/Pulse/releases/tag/v6.6.0-rc.5)\n### Agent version\n6.4.5`,
      labels: ["bug", "affects-6.4.5", "needs-version-info", "needs-human", "release-blocker",
        "needs-decomposition", "needs-retest-on-latest"].map(name => ({ name })) };
    await syncLabels({ github, context: createContext({ issue }), core: createCore() });
    assert.deepEqual(calls.addLabels.map(call => call.labels), expected ? [["affects-6.5.0"]] : []);
    assert.deepEqual(calls.removeLabel.map(call => call.name), expected
      ? ["affects-6.4.5", "needs-version-info"] : ["affects-6.4.5"]);
    assert.deepEqual(calls.createLabel.map(call => call.name), expected ? ["affects-6.5.0"] : ["needs-version-info"]);
    assert.deepEqual(calls.createComment, []);
    assert.deepEqual(calls.getLatestRelease, []);
    assert.deepEqual(calls.paginate, []);
    assert.equal(calls.getIssue.length, 2);
  }
});


test("title fallback cannot borrow a platform or agent release as the Pulse server version", () => {
  const { extractPulseVersion } = triage.internals;
  for (const title of [
    "[Bug]: vSphere 8.0.3 enrichment fails",
    "[Bug]: TrueNAS SCALE 25.10.3 reconnects",
    "[Bug]: Debian 13.0.0 install failure",
    "[Bug]: pulse-agent 6.5.0 loses disk data",
    "[v6 pre-release]: Pulse agent v6.6.0-rc.6 loses disk data",
    "Pulse unknown; agent 6.5.0 remains connected",
  ]) {
    assert.equal(extractPulseVersion(title, "Legacy report without a declared server version."), null, title);
  }
});

test("title fallback retains explicit Pulse and neutral version-leading declarations", () => {
  const { extractPulseVersion } = triage.internals;
  for (const [title, expected] of [
    ["[Bug]: Pulse 6.5.0 loses readings", "6.5.0"],
    ["vSphere 8.0.3 enrichment fails in Pulse v6.5.0", "6.5.0"],
    ["[Bug]: TrueNAS 25.10.3 in Pulse Pro version: V6.6.0-rc.6", "6.6.0-rc.6"],
    ["[Bug]: rcourtman/pulse:6.5.0 loses readings", "6.5.0"],
    ["[Bug]: license.pulserelay.pro/pulse-pro:6.6.0-rc.6 loses readings", "6.6.0-rc.6"],
    ["v6.4.3-rc.1 (LXC): Pulse degrades", "6.4.3-rc.1"],
    ["[Bug]: v6.6.0-rc.6 loses readings", "6.6.0-rc.6"],
    ["Bug on v6.5.0", "6.5.0"],
  ]) {
    assert.equal(extractPulseVersion(title, "Legacy report."), expected, title);
    assert.equal(extractPulseVersion(title, "### Pulse version\nunknown"), null, title);
    assert.equal(extractPulseVersion(title, "### Pulse version\n6.4.5"), "6.4.5", title);
  }
  assert.equal(extractPulseVersion("Upgrade from Pulse 6.4.5 to Pulse 6.5.0", "Legacy report."), null);
});

test("version-field transitions do not assert a failing source or target release", () => {
  const { extractPulseVersion } = triage.internals;
  for (const value of [
    "6.5.0 → 6.6.0-rc.1",
    "v6.5.0->v6.6.0-rc.1",
    "6.5.0 => unknown",
    "6.5.0 to latest",
    "6.5.0 &rarr; 6.6.0-rc.1",
    "**6.5.0** to **6.6.0-rc.1**",
    "`6.5.0 → 6.6.0-rc.1` (recovered to 6.5.0)",
    "[6.5.0](https://example.invalid/6.4.5) → [6.6.0-rc.1](https://example.invalid/6.4.5)",
    "server 6.5.0 → server 6.6.0-rc.1",
    "rcourtman/pulse:6.5.0 → rcourtman/pulse:6.6.0-rc.1",
    "pulse-pro-v6.5.0-linux-amd64.tar.gz → pulse-pro-v6.6.0-rc.1-linux-amd64.tar.gz",
  ]) {
    for (const field of ["### Pulse version\n", "Pulse version: ", "- **Pulse version**: "]) {
      for (const newline of ["\n", "\r\n"]) {
        const body = `${field}${value}\n\n### Agent version\n6.5.0\n\n` +
          "### Upgrade attempt and recovery\nRecovered to 6.5.0; failing executable unknown.";
        assert.equal(extractPulseVersion("Pulse 6.4.5 upgrade failed",
          body.replaceAll("\n", newline)), null, body);
      }
    }
  }
});

test("title transitions stay ambiguous without discarding a separately declared failing runtime", () => {
  const { extractPulseVersion } = triage.internals;
  for (const title of [
    "Pulse 6.5.0 → 6.6.0-rc.1 failed",
    "[Bug]: v6.5.0 to v6.6.0-rc.1 failed",
    "[Bug]: rcourtman/pulse:6.5.0 => rcourtman/pulse:6.6.0-rc.1",
    "6.5.0 → unknown after recovery",
  ]) {
    assert.equal(extractPulseVersion(title, "Legacy report."), null, title);
    assert.equal(extractPulseVersion(title, "### Pulse version\n6.6.0-rc.1"), "6.6.0-rc.1", title);
  }
  for (const [value, expected] of [
    ["6.6.0-rc.1 (upgraded from 6.5.0)", "6.6.0-rc.1"],
    ["6.5.0 to reproduce the backup failure", "6.5.0"],
    ["rcourtman/pulse:6.6.0-rc.1; agent 6.5.0", "6.6.0-rc.1"],
  ]) {
    assert.equal(extractPulseVersion("Upgrade from v6.4.5", `### Pulse version\n${value}`), expected);
  }
  assert.equal(extractPulseVersion("TrueNAS 25.04.0 → 25.10.0 fails in Pulse 6.5.0",
    "Legacy report."), "6.5.0");
});

test("transition metadata consumer preserves Community state without inferring a release or contacting the reporter", async () => {
  for (const [title, body] of [
    ["Upgrade failed", "### Pulse version\n6.5.0 → 6.6.0-rc.1\n### Agent version\n6.5.0"],
    ["Pulse 6.5.0 to 6.6.0-rc.1 failed", "Legacy report."],
  ]) {
    const { github, calls } = createGithub();
    const issue = { number: 2928, title, body,
      labels: ["bug", "affects-6.5.0", "needs-human", "release-blocker", "needs-decomposition",
        "needs-retest-on-latest", "operator-reviewed"].map(name => ({ name })) };
    await syncLabels({ github, context: createContext({ issue }), core: createCore() });
    assert.deepEqual(calls.addLabels.map(call => call.labels), [["needs-version-info"]]);
    assert.deepEqual(calls.removeLabel.map(call => call.name), ["affects-6.5.0"]);
    assert.deepEqual(calls.createLabel.map(call => call.name), ["needs-version-info"]);
    assert.deepEqual(calls.createComment, []);
    assert.deepEqual(calls.getLatestRelease, []);
    assert.deepEqual(calls.paginate, []);
    assert.equal(calls.getIssue.length, 2);
  }
});

test("title-version label consumer corrects only its own metadata without contact or release inference", async () => {
  for (const [title, body, version] of [
    ["[Bug]: vSphere 8.0.3 enrichment fails", "Legacy report.", null],
    ["vSphere 8.0.3 enrichment fails in Pulse 6.5.0", "Legacy report.", "6.5.0"],
    ["[Bug]: TrueNAS SCALE 25.10.3 reconnects", "### Pulse version\n6.5.0", "6.5.0"],
  ]) {
    const { github, calls } = createGithub();
    const issue = { number: 2925, title, body,
      labels: ["bug", "affects-8.0.3", "needs-human", "release-blocker", "needs-decomposition",
        "needs-retest-on-latest", "operator-reviewed"].map(name => ({ name })) };
    await syncLabels({ github, context: createContext({ issue }), core: createCore() });
    const expectedLabel = version ? `affects-${version}` : "needs-version-info";
    assert.deepEqual(calls.addLabels.map(call => call.labels), [[expectedLabel]]);
    assert.deepEqual(calls.removeLabel.map(call => call.name), ["affects-8.0.3"]);
    assert.deepEqual(calls.createLabel.map(call => call.name), [expectedLabel]);
    assert.deepEqual(calls.createComment, []);
    assert.deepEqual(calls.getLatestRelease, []);
    assert.deepEqual(calls.paginate, []);
    assert.equal(calls.getIssue.length, 2);
  }
});

test("version fields retain document-wide full, collapsed and shortcut reference links", () => {
  for (const [value, definitions, expected] of [
    ["[6.5.0][release]", '[release]: https://example.invalid/6.4.5 "6.4.5"', "6.5.0"],
    ["[6.5.0][]", "[6.5.0]: https://example.invalid/6.4.5", "6.5.0"],
    ["[6.5.0]", "[6.5.0]: https://example.invalid/6.4.5", "6.5.0"],
    ["**[`v6.5.0`][ReLeAsE]**", "[release]: https://example.invalid/6.4.5", "6.5.0"],
    ["[pulse-pro-v6.6.0-rc.6-linux-arm64.tar.gz][release]",
      "[release]: https://example.invalid/6.4.5", "6.6.0-rc.6"],
    ["[unknown][release]", '[release]: https://example.invalid/6.5.0 "6.5.0"', null],
  ]) {
    for (const field of ["### Pulse version\n", "Pulse version: ", "- **Pulse version**: "]) {
      const report = `${field}${value}\n\n### Agent version\n6.4.5`;
      for (const body of [`${definitions}\n\n${report}`, `${report}\n\n${definitions}`]) {
        for (const newline of ["\n", "\r\n"]) {
          assert.equal(triage.internals.extractPulseVersion("Upgrade from v6.4.5",
            body.replaceAll("\n", newline)), expected, body);
        }
      }
    }
  }
});

test("reference definitions cannot invent values from destinations, examples or hidden content", () => {
  for (const [value, definitions] of [
    ["[release][6.5.0]", "[6.5.0]: https://example.invalid/6.5.0"],
    ["![6.5.0][release]", "[release]: https://example.invalid/6.5.0"],
    ["~~[6.5.0][release]~~", "[release]: https://example.invalid/6.5.0"],
    ["`[6.5.0][release]`", "[release]: https://example.invalid/6.5.0"],
    ["[6.5.0][release]", "<!--\n[release]: https://example.invalid/6.5.0\n-->"],
    ["[6.5.0][release]", "```markdown\n[release]: https://example.invalid/6.5.0\n```"],
    ["[6.5.0][release]", "    [release]: https://example.invalid/6.5.0"],
    ["[6.5.0][release]", "[other]: https://example.invalid/6.5.0"],
  ]) {
    assert.equal(triage.internals.extractPulseVersion("Upgrade from v6.4.5",
      `### Pulse version\n${value}\n\n### Agent version\n6.4.5\n\n${definitions}`), null, value);
  }
});

test("reference-linked feedback follows every visible form choice, not its definition", () => {
  for (const [selection, expected] of [
    ["Bug / regression", "bug"], ["Upgrade / migration issue", "bug"],
    ["Performance issue", "bug"], ["Documentation issue", "documentation"],
    ["UX / workflow friction", "enhancement"], ["Other actionable feedback", "enhancement"],
  ]) {
    for (const [value, definitions] of [
      [`[${selection}][choice]`, '[choice]: https://example.invalid/bug "Bug / regression"'],
      [`[${selection}][]`, `[${selection}]: https://example.invalid/bug`],
      [`[${selection}]`, `[${selection}]: https://example.invalid/bug`],
    ]) {
      const report = `### Feedback type\n${value}\n\n### Pulse version\nunknown`;
      for (const body of [`${definitions}\n\n${report}`, `${report}\n\n${definitions}`]) {
        assert.equal(triage.internals.classifyV6FeedbackType(body), expected, body);
      }
    }
  }
  for (const value of ["[choice][bug]", "![Bug / regression][bug]",
    "~~[Bug / regression][bug]~~", "`[Bug / regression][bug]`"]) {
    assert.equal(triage.internals.classifyV6FeedbackType(
      `### Feedback type\n${value}\n\n### Pulse version\nunknown\n\n[bug]: https://example.invalid/bug`), null, value);
  }
});

test("linked no-topic responses stay empty without discarding qualifications or evidence", () => {
  for (const value of ["[None][topic]", "[None][]", "[None]", "**[`None`][topic]**"]) {
    const definitions = '[topic]: https://example.invalid/bug "Bug / regression"\n[None]: https://example.invalid/bug';
    for (const [topic, expected] of [[value, false],
      [`${value}, except the saved target also changes.`, true],
      [`${value}\n\nThe installer also loses its selection.`, true]]) {
      const report = `### Additional actionable topics\n${topic}\n\n### Desired outcome\nKeep the current workflow.`;
      for (const body of [`${definitions}\n\n${report}`, `${report}\n\n${definitions}`]) {
        assert.equal(triage.internals.classifyAdditionalActionableTopics(body), expected, body);
      }
    }
  }
  for (const topic of ["![None][topic]", "~~[None][topic]~~", "`[None][topic]`", "[None][undefined]"]) {
    assert.equal(triage.internals.classifyAdditionalActionableTopics(
      `### Additional actionable topics\n${topic}\n\n### Pulse version\n6.5.0\n\n[topic]: https://example.invalid/bug`), true, topic);
  }
});

test("the silent consumer uses linked declarations without invented tasks or reporter contact", async () => {
  for (const [selection, version, expectedLabels] of [
    ["Documentation issue", "unknown", ["documentation"]],
    ["Bug / regression", "6.5.0", ["affects-6.5.0", "bug"]],
  ]) {
    const { github, calls } = createGithub();
    const issue = { number: 2928, title: "Current feedback", labels: ["needs-human",
      "release-blocker", "needs-retest-on-latest", "operator-reviewed"].map(name => ({ name })),
    body: `### Feedback type\n[${selection}][choice]\n\n### Pulse version\n[${version}][release]\n\n` +
      "### Additional actionable topics\n[None][topic]\n\n### What happened?\nCurrent report.\n\n" +
      '[choice]: https://example.invalid/bug "Bug / regression"\n' +
      '[release]: https://example.invalid/6.4.5 "6.4.5"\n[topic]: https://example.invalid/bug' };
    await syncLabels({ github, context: createContext({ issue }), core: createCore() });
    assert.deepEqual(calls.addLabels.map(call => call.labels), [expectedLabels]);
    assert.deepEqual(calls.createLabel.map(call => call.name),
      version === "6.5.0" ? ["affects-6.5.0"] : []);
    assert.deepEqual(calls.removeLabel, []);
    assert.deepEqual(calls.createComment, []);
    assert.deepEqual(calls.getLatestRelease, []);
    assert.deepEqual(calls.paginate, []);
    assert.equal(calls.getIssue.length, 2);
  }
});

test("unversioned server images and companion binaries cannot borrow a later version", () => {
  const values = [
    "rcourtman/pulse:latest; agent 6.5.0",
    "license.pulserelay.pro/pulse-pro:latest; TrueNAS 25.10.3",
    "rcourtman/pulse:6.6; agent 6.5.0",
    "pulse:unknown; server 6.5.0 after recovery",
    "pulse:latest (previously 6.4.5)",
    "registry.6.4.5.invalid/pulse:latest; agent 6.5.0",
    "pulse-agent v6.5.0",
    "pulse-agent-helper 6.5.0",
    "pulse-agent-runner:6.5.0",
    "pulse-mcp 6.5.0",
    "pulse-provider-msp 6.5.0",
    "pulse-pro-agent v6.5.0",
  ];
  for (const value of values) {
    for (const field of ["### Pulse version\n", "Pulse version: ", "- **Pulse version**: "]) {
      for (const newline of ["\n", "\r\n"]) {
        const body = `${field}${value}\n\n### Agent version\n6.4.5`;
        assert.equal(triage.internals.extractPulseVersion("Pulse 6.4.5 upgrade failed",
          body.replaceAll("\n", newline)), null, body);
      }
    }
  }
});

test("a complete server token supplies its own version, not a registry or later number", () => {
  for (const [value, expected] of [
    ["v6.6.0-rc.6; agent 6.5.0", "6.6.0-rc.6"],
    ["Pulse v6.6.0-rc.6; TrueNAS 25.10.3", "6.6.0-rc.6"],
    ["server + pulse-agent v6.3.2", "6.3.2"],
    ["pulse-v6.6.0-rc.6", "6.6.0-rc.6"],
    ["pulse_v6.6.0-rc.6", "6.6.0-rc.6"],
    ["pulse-pro-v6.6.0-rc.6", "6.6.0-rc.6"],
    ["registry.6.4.5.invalid/pulse:6.6.0-rc.6; agent 6.5.0", "6.6.0-rc.6"],
    ["registry.6.4.5.invalid/pulse-pro:v6.6.0-rc.6", "6.6.0-rc.6"],
    ["**[registry.6.4.5.invalid/pulse:6.6.0-rc.6](https://example.invalid/6.5.0)**", "6.6.0-rc.6"],
    ["registry.6.4.5.invalid/pulse:6.5.0 → 6.6.0-rc.6", null],
    ["registry.6.4.5.invalid/pulse-pro:6.5.0 to latest", null],
  ]) {
    assert.equal(triage.internals.extractPulseVersion("Upgrade from v6.4.5",
      `### Pulse version\n${value}\n\n### Agent version\n6.4.5`), expected, value);
  }
});

test("silent triage retires borrowed versions without changing Community disposition", async () => {
  for (const action of ["opened", "edited", "reopened"]) {
    for (const [value, borrowedVersion] of [
      ["rcourtman/pulse:latest; agent 6.5.0", "6.5.0"],
      ["license.pulserelay.pro/pulse-pro:latest; TrueNAS 25.10.3", "25.10.3"],
      ["pulse-agent v6.5.0", "6.5.0"],
      ["rcourtman/pulse:6.6; agent 6.5.0", "6.5.0"],
    ]) {
      const { github, calls } = createGithub();
      const preserved = ["bug", "needs-human", "release-blocker", "needs-decomposition",
        "needs-retest-on-latest", "operator-reviewed"];
      const issue = { number: 2929, title: "Pulse 6.4.5 upgrade failed",
        body: `### Pulse version\n${value}\n\n### Agent version\n6.4.5`,
        labels: [...preserved, `affects-${borrowedVersion}`].map(name => ({ name })) };
      await syncLabels({ github, context: createContext({ action, issue }), core: createCore() });
      assert.deepEqual(calls.addLabels.map(call => call.labels), [["needs-version-info"]], value);
      assert.deepEqual(calls.removeLabel.map(call => call.name), [`affects-${borrowedVersion}`], value);
      assert.deepEqual(calls.createLabel.map(call => call.name), ["needs-version-info"], value);
      assert.deepEqual(calls.createComment, []);
      assert.deepEqual(calls.getLatestRelease, []);
      assert.deepEqual(calls.paginate, []);
      assert.equal(calls.getIssue.length, 2);
    }
  }
});
// Spy on the locked parser, not a production counter or elapsed-time threshold.
// The complete report should be lexed once, even when fields use definitions
// elsewhere in it. Field-sized lexing remains necessary and is not counted.
for (const action of ["opened", "edited", "reopened"]) {
  test(`one Markdown snapshot supplies the ${action} label decision`, async () => {
    const { Lexer } = require("marked");
    const originalLex = Lexer.prototype.lex;
    const previousBody = [
      "### Feedback type", "[selected]", "### Pulse version", "[server]",
      "### Additional actionable topics", "[topics]", "",
      "[selected]: https://example.invalid/type", // label still supplies text
      "[server]: https://example.invalid/old", "[topics]: https://example.invalid/old",
    ].join("\n").replace("[selected]\n", "Documentation issue\n")
      .replace("[server]\n", "6.4.5\n").replace("[topics]\n", "None\n");
    const body = [
      "### Feedback type", "[Bug / regression][selected]",
      "### Pulse version", "[6.5.0][server]",
      "### Additional actionable topics", "[Another workflow problem][topics]",
      "### Logs, screenshots, or diagnostics", "```text",
      "Pulse version: 9.9.9", "### Feedback type", "Documentation issue", "```", "",
      "[selected]: https://example.invalid/Documentation-issue",
      "[server]: https://example.invalid/6.4.5", "[topics]: https://example.invalid/None",
    ].join("\n");
    const counts = new Map([[body, 0], [previousBody, 0]]);
    Lexer.prototype.lex = function (source) {
      if (counts.has(source)) counts.set(source, counts.get(source) + 1);
      return originalLex.call(this, source);
    };
    const { github, calls } = createGithub();
    const issue = { number: 2930, title: "Upgrade from 6.4.5", body,
      labels: ["bug", "affects-6.4.5", "needs-human", "release-blocker",
        "needs-retest-on-latest"].map(name => ({ name })) };
    try {
      await syncLabels({ github, context: createContext({ action, issue,
        previousBody: action === "edited" ? previousBody : undefined }), core: createCore() });
    } finally {
      Lexer.prototype.lex = originalLex;
    }
    assert.equal(counts.get(body), 1, "full current report parses per decision");
    assert.equal(counts.get(previousBody), action === "edited" ? 1 : 0,
      "previous declaration is parsed only when the edited event needs comparison");
    assert.deepEqual(calls.addLabels.map(call => call.labels), [action === "reopened"
      ? ["affects-6.5.0"] : ["affects-6.5.0", "needs-decomposition"]]);
    assert.deepEqual(calls.removeLabel.map(call => call.name), ["affects-6.4.5"]);
    assert.deepEqual(calls.createComment, []);
    assert.deepEqual(calls.getLatestRelease, []);
    assert.deepEqual(calls.paginate, []);
    assert.equal(calls.getIssue.length, 2);
  });
}

test("Markdown snapshots never leak reference definitions between reports or revisions", async () => {
  const { Lexer } = require("marked");
  const originalLex = Lexer.prototype.lex;
  const body = (version, topic, hasReleaseDefinition = true) => [
    "### Feedback type", "[Bug / regression][type]", "### Pulse version", version,
    "### Additional actionable topics", topic, "", "[type]: https://example.invalid/type",
    ...(hasReleaseDefinition ? ["[release]: https://example.invalid/6.4.5"] : []),
    "[topic]: https://example.invalid/topic",
  ].join("\n");
  const bodies = [body("[6.5.0][release]", "[Another topic][topic]"),
    body("[6.6.0][release]", "[None][topic]", false), body("[6.6.0-rc.6][release]", "None")];
  const counts = new Map(bodies.map(value => [value, 0]));
  Lexer.prototype.lex = function (source) {
    if (counts.has(source)) counts.set(source, counts.get(source) + 1);
    return originalLex.call(this, source);
  };
  const results = bodies.map((value, i) => {
    const { github, calls } = createGithub();
    const issue = { number: i === 1 ? 2932 : 2931, title: "Bug", body: value,
      labels: [{ name: "bug" }, { name: "needs-human" }] };
    return { github, calls, issue };
  });
  try {
    await Promise.all(results.map(({ github, issue }) => syncLabels({
      github, context: createContext({ issue }), core: createCore(),
    })));
  } finally {
    Lexer.prototype.lex = originalLex;
  }
  assert.deepEqual([...counts.values()], [1, 1, 1]);
  assert.deepEqual(results.map(({ calls }) => calls.addLabels[0].labels), [
    ["affects-6.5.0", "needs-decomposition"], ["needs-version-info"], ["affects-6.6.0-rc.6"],
  ]);
  for (const { calls } of results) {
    assert.deepEqual(calls.removeLabel, []);
    assert.deepEqual(calls.createComment, []);
    assert.deepEqual(calls.getLatestRelease, []);
    assert.deepEqual(calls.paginate, []);
  }
});

test("non-rendered reference definitions cannot invent secondary topics at the end of a report", () => {
  for (const value of ["", "None", "[None][empty]", "**[None][empty]**"]) {
    for (const newline of ["\n", "\r\n"]) {
      const body = `### Additional actionable topics\n${value}\n\n` +
        "[empty]: https://example.invalid/another-bug\n";
      assert.equal(triage.internals.classifyAdditionalActionableTopics(body.replaceAll("\n", newline)), false);
    }
  }
  for (const value of ["Another bug", "None, except another workflow problem", "[Another bug][empty]"]) {
    assert.equal(triage.internals.classifyAdditionalActionableTopics(
      `### Additional actionable topics\n${value}\n\n[empty]: https://example.invalid/None\n`
    ), true, value);
  }
});

test("literal reference-definition examples remain visible evidence for topic review", () => {
  for (const example of [
    "```markdown\n[example]: https://example.invalid/topic\n```",
    "    [example]: https://example.invalid/topic",
    "`[example]: https://example.invalid/topic`",
  ]) {
    assert.equal(triage.internals.classifyAdditionalActionableTopics(
      `### Additional actionable topics\nNone\n\n${example}\n`
    ), true, example);
  }
});

function labelNameConflict() {
  return Object.assign(new Error("Validation Failed"), {
    status: 422,
    response: { data: { errors: [{ resource: "Label", field: "name", code: "already_exists" }] } },
  });
}

for (const [label, classification, body] of [
  ["affects-6.7.0", "bug", "### Pulse version\n6.7.0\n"],
  ["needs-version-info", "bug", "### Pulse version\nunknown\n"],
  ["needs-decomposition", "enhancement", "### Additional actionable topics\nAlso retain failed attempts.\n"],
]) {
  test(`label-creation race preserves both reports' metadata: ${label}`, async () => {
    const existingLabels = new Set();
    let missingReads = 0;
    let releaseReads;
    const bothMissing = new Promise(resolve => { releaseReads = resolve; });
    const preserved = [classification, "needs-human", "release-blocker",
      "needs-retest-on-latest", "operator-reviewed"];
    const workers = [2933, 2934].map(number => {
      const { github, calls } = createGithub({ existingLabels });
      const getLabel = github.rest.issues.getLabel;
      const createLabel = github.rest.issues.createLabel;
      github.rest.issues.getLabel = async payload => {
        if (!existingLabels.has(payload.name)) {
          // Both jobs must have observed absence before either can create it.
          calls.getLabel.push(payload.name);
          missingReads += 1;
          if (missingReads === 2) releaseReads();
          await bothMissing;
          throw Object.assign(new Error("Not Found"), { status: 404 });
        }
        return getLabel(payload);
      };
      github.rest.issues.createLabel = async payload => {
        if (existingLabels.has(payload.name)) {
          calls.createLabel.push(payload);
          throw labelNameConflict();
        }
        return createLabel(payload);
      };
      const issue = { number, title: "Monitoring report", body,
        labels: preserved.map(name => ({ name })) };
      return { github, calls, issue };
    });
    const results = await Promise.allSettled(workers.map(({ github, issue }) =>
      syncLabels({ github, context: createContext({ issue }), core: createCore() })));
    assert.deepEqual(results.map(result => result.status), ["fulfilled", "fulfilled"]);
    assert.equal(missingReads, 2);
    assert.deepEqual([...existingLabels], [label]);
    assert.equal(workers.reduce((sum, worker) => sum + worker.calls.getLabel.length, 0), 3);
    for (const { calls } of workers) {
      assert.deepEqual(calls.addLabels.map(call => call.labels), [[label]]);
      assert.equal(calls.createLabel.length, 1, "never replay label creation");
      assert.deepEqual(calls.removeLabel, []);
      assert.deepEqual(calls.createComment, []);
      assert.deepEqual(calls.getLatestRelease, []);
      assert.deepEqual(calls.paginate, []);
      assert.equal(calls.getIssue.length, 2, "recheck report before applying metadata");
    }
  });
}

test("label-creation reconciliation requires the exact current label", async () => {
  for (const current of [{ name: "affects-6.7.0" }, { name: "AFFECTS-6.7.0" },
    { name: "affects-6.6.0" }, {}, null]) {
    const { github, calls } = createGithub();
    const conflict = labelNameConflict();
    github.rest.issues.getLabel = async ({ name }) => {
      calls.getLabel.push(name);
      if (calls.getLabel.length === 1) throw Object.assign(new Error("Not Found"), { status: 404 });
      return { data: current };
    };
    github.rest.issues.createLabel = async payload => {
      calls.createLabel.push(payload);
      throw conflict;
    };
    const issue = { number: 2935, title: "Monitoring report", body: "### Pulse version\n6.7.0\n",
      labels: [{ name: "bug" }] };
    const result = syncLabels({ github, context: createContext({ issue }), core: createCore() });
    const matches = current?.name?.toLowerCase() === "affects-6.7.0";
    if (matches) await result;
    else await assert.rejects(result, error => error === conflict);
    assert.equal(calls.getLabel.length, 2);
    assert.equal(calls.createLabel.length, 1);
    assert.deepEqual(calls.addLabels.map(call => call.labels), matches ? [["affects-6.7.0"]] : []);
    assert.deepEqual(calls.removeLabel, []);
    assert.deepEqual(calls.createComment, []);
  }
});

test("label-creation non-conflict failures are not retried or hidden", async () => {
  for (const [status, errors] of [
    [401], [403], [429], [500], [422], [422, "unavailable"],
    [422, [{ resource: "Label", field: "name", code: "invalid" }]],
    [422, [{ resource: "Issue", field: "name", code: "already_exists" }]],
    [422, [{ resource: "Label", field: "description", code: "already_exists" }]],
  ]) {
    const { github, calls } = createGithub();
    const failure = Object.assign(new Error("Creation failed"), {
      status, response: { data: { errors } },
    });
    github.rest.issues.createLabel = async payload => {
      calls.createLabel.push(payload);
      throw failure;
    };
    const issue = { number: 2936, title: "Monitoring report", body: "### Pulse version\n6.7.0\n",
      labels: [{ name: "bug" }] };
    await assert.rejects(syncLabels({ github, context: createContext({ issue }), core: createCore() }),
      error => error === failure);
    assert.equal(calls.getLabel.length, 1);
    assert.equal(calls.createLabel.length, 1);
    assert.deepEqual(calls.addLabels, []);
    assert.deepEqual(calls.removeLabel, []);
    assert.deepEqual(calls.createComment, []);
  }
});

test("label-creation reconciliation read failures stop report mutations", async () => {
  for (const status of [403, 404, 503]) {
    const { github, calls } = createGithub();
    const failure = Object.assign(new Error("Read failed"), { status });
    github.rest.issues.getLabel = async ({ name }) => {
      calls.getLabel.push(name);
      if (calls.getLabel.length === 1) throw Object.assign(new Error("Not Found"), { status: 404 });
      throw failure;
    };
    github.rest.issues.createLabel = async payload => {
      calls.createLabel.push(payload);
      throw labelNameConflict();
    };
    const issue = { number: 2937, title: "Monitoring report", body: "### Pulse version\n6.7.0\n",
      labels: [{ name: "bug" }] };
    await assert.rejects(syncLabels({ github, context: createContext({ issue }), core: createCore() }),
      error => error === failure);
    assert.equal(calls.getLabel.length, 2);
    assert.equal(calls.createLabel.length, 1);
    assert.deepEqual(calls.addLabels, []);
    assert.deepEqual(calls.removeLabel, []);
    assert.deepEqual(calls.createComment, []);
  }
});

test("label-creation reconciliation still respects a newer Community decision", async () => {
  const issue = { number: 2938, title: "Monitoring report", body: "### Pulse version\n6.7.0\n",
    state: "open", updated_at: "2026-10-08T10:00:00Z", labels: [{ name: "bug" }] };
  const { github, calls } = createGithub({ currentIssue: issue });
  github.rest.issues.getLabel = async ({ name }) => {
    calls.getLabel.push(name);
    if (calls.getLabel.length === 1) throw Object.assign(new Error("Not Found"), { status: 404 });
    github.currentIssue = { ...issue, updated_at: "2026-10-08T11:00:00Z",
      labels: [...issue.labels, { name: "needs-human" }] };
    return { data: { name } };
  };
  github.rest.issues.createLabel = async payload => {
    calls.createLabel.push(payload);
    throw labelNameConflict();
  };
  await syncLabels({ github, context: createContext({ issue }), core: createCore() });
  assert.equal(calls.getLabel.length, 2);
  assert.equal(calls.createLabel.length, 1);
  assert.equal(calls.getIssue.length, 2);
  assert.deepEqual(calls.addLabels, []);
  assert.deepEqual(calls.removeLabel, []);
  assert.deepEqual(calls.createComment, []);
});
