// One bounded viewport, preserving the common safety/copy/layout assertions.
require('/workspace/frontend-modern/browser-tests/pro-registry-guidance.cjs')('desktop').catch(
  (error) => {
    console.error(error);
    process.exitCode = 1;
  },
);
