// One admitted process tree; each driver closes its browsers/server before
// the next begins. Their own exact-source/hash checks and results stay intact.
const fs = require('node:fs');
const path = require('node:path');
const crypto = require('node:crypto');
const assert = require('node:assert/strict');
(async () => {
  const binding = JSON.parse(fs.readFileSync('tmp/resource-agent-proof/static/binding.json'));
  assert.equal(
    binding.sources['browser-tests/composed-observations.cjs'],
    crypto.createHash('sha256').update(fs.readFileSync(__filename)).digest('hex'),
  );
  for (const name of ['root-disk-observation.cjs', 'resource-composition-observation.cjs']) {
    await require(path.join(__dirname, name));
    if (process.exitCode) break;
  }
})().catch((error) => {
  console.error(error);
  process.exitCode = 1;
});
