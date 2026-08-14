#!/usr/bin/env node

import { fileURLToPath } from 'node:url';
import path from 'node:path';

import { validateE2ETiering } from './check-e2e-tiering-lib.mjs';

const integrationRoot = path.resolve(
  fileURLToPath(new URL('..', import.meta.url)),
);
const result = await validateE2ETiering({ integrationRoot });
if (result.errors.length) {
  console.error('E2E tier selection validation failed:');
  for (const error of result.errors) console.error(`- ${error}`);
  process.exit(1);
}

console.log('E2E tier selection validation passed');
console.log(`  spec_files: ${result.allFiles.size}`);
console.log(`  stable_spec_files: ${result.listedStableFiles.size}`);
console.log(`  probation_spec_files: ${result.listedProbationFiles.size}`);
console.log(`  quarantined_spec_files: ${result.quarantineFiles.size}`);
console.log(`  test_project_instances: ${result.allTests.size}`);
console.log(`  stable_test_project_instances: ${result.stableTests.size}`);
console.log(
  `  probation_test_project_instances: ${result.probationTests.size}`,
);
