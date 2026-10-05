const { spawnSync } = require('node:child_process');
for (const script of ['guest-discovery-safety.cjs', 'discovery-outcome-ownership.cjs']) {
  const result = spawnSync(
    process.execPath,
    ['/workspace/frontend-modern/browser-tests/' + script],
    { stdio: 'inherit' },
  );
  if (result.status !== 0) {
    console.error(
      script,
      'did not complete successfully',
      result.status,
      result.signal,
      result.error?.message,
    );
    process.exit(result.status || 1);
  }
}
console.log('Combined guest Discovery pause and outcome ownership browser proof passed.');
