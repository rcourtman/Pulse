import assert from 'node:assert/strict';
import test from 'node:test';

import { parsePlaywrightTestIdentity } from './e2e-tier-identity.mjs';

test('tier identity ignores unstable Playwright source coordinates', () => {
  const authored = parsePlaywrightTestIdentity(
    '  [chromium] › 59-workloads-column-layout.spec.ts:148:3 › Workloads column layout › keeps Type readable',
  );
  const transformed = parsePlaywrightTestIdentity(
    '  [chromium] › 59-workloads-column-layout.spec.ts:97:3 › Workloads column layout › keeps Type readable',
  );

  assert.deepEqual(authored, transformed);
  assert.deepEqual(authored, {
    key: '[chromium] › 59-workloads-column-layout.spec.ts › Workloads column layout › keeps Type readable',
    specFile: '59-workloads-column-layout.spec.ts',
  });
});

test('tier identity retains project, spec, and complete title path', () => {
  assert.equal(parsePlaywrightTestIdentity('unrelated output'), null);
  assert.deepEqual(
    parsePlaywrightTestIdentity(
      '\u001b[2m  [mobile-safari] › nested/example.spec.ts:10:7 › suite › test\u001b[22m',
    ),
    {
      key: '[mobile-safari] › nested/example.spec.ts › suite › test',
      specFile: 'nested/example.spec.ts',
    },
  );
});
