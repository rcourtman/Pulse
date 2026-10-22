const test = require("node:test");
const assert = require("node:assert/strict");
const fs = require("node:fs");
const path = require("node:path");

const workflowDir = path.resolve(__dirname, "../workflows");
const issueMutationPatterns = [
  /github\.rest\.issues\.createComment/,
  /github\.rest\.pulls\.createReviewComment/,
  /addDiscussionComment/,
  /post(?:Eligible)?RetestComments?/,
  /syncLabels/,
];

// This is a source contract for the workflow's plain, flat input block, not
// a general YAML parser. Reject other shapes rather than guessing their scope.
function assertTriageTokenScope(workflow) {
  const steps = workflow.split(/(?=^      - )/m).slice(1);
  const issuers = steps.filter(step => /actions\/create-github-app-token@/.test(step));
  assert.equal(issuers.length, 1, "exactly one triage token issuer is required");
  assert.match(issuers[0], /^        id: triage-token$/m);
  const parts = issuers[0].split(/^        with:\s*\n/m);
  assert.equal(parts.length, 2, "the token issuer needs one explicit input block");
  const inputs = {};
  for (const line of parts[1].split("\n")) {
    if (/^\s*(?:#.*)?$/.test(line)) continue;
    const entry = /^          ([a-z][a-z0-9-]*): (.+)$/.exec(line);
    assert.ok(entry, `unsupported token input shape: ${line}`);
    assert.ok(!Object.hasOwn(inputs, entry[1]), `duplicate token input: ${entry[1]}`);
    inputs[entry[1]] = entry[2];
  }
  assert.deepEqual(inputs, {
    "app-id": "2484142",
    "private-key": "${{ secrets.PULSE_TRIAGE_APP_PRIVATE_KEY }}",
    owner: "rcourtman",
    repositories: "Pulse",
    "permission-issues": "write",
  }, "the metadata consumer needs only Pulse issues; omitted permissions inherit the App grant");
}

test("triage App token is scoped to Pulse issues only", () => {
  const workflow = fs.readFileSync(path.join(workflowDir, "issue-version-label-sync.yml"), "utf8");
  assertTriageTokenScope(workflow);
});

test("triage token scope checks reject inherited, broader and decoy grants", () => {
  const workflow = fs.readFileSync(path.join(workflowDir, "issue-version-label-sync.yml"), "utf8");
  const input = "          permission-issues: write\n";
  const mutations = [
    workflow.replace(input, ""),
    workflow.replace("          owner: rcourtman\n", ""),
    workflow.replace("          repositories: Pulse\n", ""),
    workflow.replace("repositories: Pulse", "repositories: Pulse,pulse-pro"),
    workflow.replace("owner: rcourtman", "owner: ${{ github.event.issue.user.login }}"),
    workflow.replace(input, `${input}          permission-contents: write\n`),
    workflow.replace(input, `${input}          permission-actions: write\n`),
    workflow.replace(input, `${input}          skip-token-revoke: true\n`),
    workflow.replace(input, `${input}          permission-issues: read\n`),
    // An input under a different action must not satisfy the issuer's contract.
    workflow.replace(input, "").replace("          github-token:", `${input}          github-token:`),
  ];
  for (const [index, mutated] of mutations.entries()) {
    assert.notEqual(mutated, workflow, `scope control ${index} must change its input`);
    assert.throws(() => assertTriageTokenScope(mutated), undefined, `scope control ${index}`);
  }
});

test("automated workflow issue mutations use the dedicated triage identity", () => {
  const publishers = [];

  for (const name of fs.readdirSync(workflowDir)) {
    if (!name.endsWith(".yml") && !name.endsWith(".yaml")) continue;
    const workflow = fs.readFileSync(path.join(workflowDir, name), "utf8");
    if (!issueMutationPatterns.some((pattern) => pattern.test(workflow))) continue;

    publishers.push(name);
    assert.match(
      workflow,
      /actions\/create-github-app-token@[0-9a-f]{40}/,
      `${name} must mint a pinned GitHub App token`
    );
    assert.match(
      workflow,
      /private-key: \$\{\{ secrets\.PULSE_TRIAGE_APP_PRIVATE_KEY \}\}/,
      `${name} must use the triage App private key`
    );
    assert.match(
      workflow,
      /github-token: \$\{\{ steps\.triage-token\.outputs\.token \}\}/,
      `${name} must pass the triage App token to github-script`
    );
  }

  assert.deepEqual(publishers.sort(), [
    "issue-version-label-sync.yml",
  ]);
});


test("version-only posting and timeout closure workflows are retired", () => {
  for (const name of ["issue-version-retest-comment.yml", "close-needs-retest-timeout.yml"]) {
    assert.equal(fs.existsSync(path.join(workflowDir, name)), false, name);
  }
  for (const name of fs.readdirSync(workflowDir)) {
    if (!/\.ya?ml$/.test(name)) continue;
    const source = fs.readFileSync(path.join(workflowDir, name), "utf8");
    assert.doesNotMatch(source, /post(?:Eligible)?RetestComments?|issue-version-triage:v1|buildTimeoutCloseCommentBody/, name);
  }
});
