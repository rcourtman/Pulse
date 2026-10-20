// One bounded viewport, preserving the common safety/copy/layout assertions.
require('./pro-registry-guidance.cjs')('phone').catch((error) => {
  console.error(error);
  process.exitCode = 1;
});
