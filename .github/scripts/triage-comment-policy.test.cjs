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
    "close-needs-retest-timeout.yml",
    "issue-version-label-sync.yml",
    "issue-version-retest-comment.yml",
  ]);
});
