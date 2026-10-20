// One bounded profile; common assertions remain in fleet-details.cjs.
require('/workspace/frontend-modern/browser-tests/fleet-details.cjs')('phone-dark').catch(
  (error) => {
    console.error(error);
    process.exitCode = 1;
  },
);
