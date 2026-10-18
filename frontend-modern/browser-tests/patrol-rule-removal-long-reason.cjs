// Measure a long existing reason without creating or deleting any rule.
const fs = require('node:fs');
const path = require('node:path');
const {
  journey,
  report,
  output,
} = require('/workspace/frontend-modern/browser-tests/patrol-rule-removal.cjs');
(async () => {
  try {
    await journey('/workspace/frontend-modern', 'webkit', 320, false, true, 'long');
    report.result = 'passed';
  } catch (error) {
    report.result = 'failed';
    report.failure = error.message;
    throw error;
  } finally {
    report.cleanup = {
      all_cases_closed: report.cases.every(
        (c) => c.cleanup.browser_closed && c.cleanup.server_closed,
      ),
    };
    fs.writeFileSync(path.join(output, 'long-result.json'), JSON.stringify(report, null, 2));
  }
  console.log(JSON.stringify({ result: report.result, cleanup: report.cleanup }));
})().catch((error) => {
  console.error(error.stack);
  process.exitCode = 1;
});
