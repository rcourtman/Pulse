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

// The triage App installation can administer the repository, so its key must
// not be an Actions secret any workflow on any branch could read. Workflow
// issue mutations use the run's own GITHUB_TOKEN, scoped to issues.
test("automated workflow issue mutations use an issues-scoped GITHUB_TOKEN, not a secret", () => {
  const publishers = [];

  for (const name of fs.readdirSync(workflowDir)) {
    if (!name.endsWith(".yml") && !name.endsWith(".yaml")) continue;
    const workflow = fs.readFileSync(path.join(workflowDir, name), "utf8");
    if (!issueMutationPatterns.some((pattern) => pattern.test(workflow))) continue;

    publishers.push(name);
    assert.match(
      workflow,
      /^permissions:\n  contents: read\n  issues: write\n(?! )/m,
      `${name} must grant exactly contents: read and issues: write`
    );
    assert.doesNotMatch(workflow, /^\s+permissions:/m, `${name} must not override permissions per job`);
    assert.match(
      workflow,
      /github-token: \$\{\{ github\.token \}\}/,
      `${name} must pass the run's GITHUB_TOKEN to github-script`
    );
    assert.doesNotMatch(workflow, /secrets\./, `${name} must not read repository secrets`);
    assert.doesNotMatch(workflow, /create-github-app-token/, `${name} must not mint an App token`);
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
