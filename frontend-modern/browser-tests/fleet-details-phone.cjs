// One bounded profile; common assertions remain in fleet-details.cjs.
require('/workspace/frontend-modern/browser-tests/fleet-details.cjs')('phone').catch((error) => {
  console.error(error);
  process.exitCode = 1;
});
