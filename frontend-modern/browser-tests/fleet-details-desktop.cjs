// One bounded profile; common assertions remain in fleet-details.cjs.
require('/workspace/frontend-modern/browser-tests/fleet-details.cjs')('desktop').catch((error) => {
  console.error(error);
  process.exitCode = 1;
});
