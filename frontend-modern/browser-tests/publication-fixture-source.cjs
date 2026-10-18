// The browser's isolated source copy does not mount the worktree's Git store.
// A supplied SHA is an attribution supplied by the source owner, not a Git read
// or execution authority. Imported source and served bytes remain hash-bound.
const assert = require('node:assert/strict');

function publicationSourceIdentity(args, readHead) {
  if (args.length === 0) {
    const source_sha = readHead().trim();
    assert.match(source_sha, /^[a-f0-9]{40}$/, 'invalid Git source identity');
    return { source_sha, source_identity: 'git-head' };
  }
  assert.equal(args.length, 2, 'expected only --source-sha <full SHA>');
  assert.equal(args[0], '--source-sha', 'unknown fixture preparation argument');
  assert.match(args[1], /^[a-f0-9]{40}$/, 'expected a full lowercase source SHA');
  return { source_sha: args[1], source_identity: 'supplied-for-isolated-copy' };
}

module.exports = { publicationSourceIdentity };
