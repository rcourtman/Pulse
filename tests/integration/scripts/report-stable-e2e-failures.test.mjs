import assert from 'node:assert/strict';
import test from 'node:test';

import {
  githubAnnotations,
  parseFailedTestcases,
} from './report-stable-e2e-failures.mjs';

test('extracts failed and errored Playwright testcases without reporting passes', () => {
  const xml = `<?xml version="1.0"?>
    <testsuites>
      <testcase name="passes" classname="chromium › 63-pbs-active-tasks.spec.ts › PBS active tasks" />
      <testcase name="shows a running task" classname="chromium › 63-pbs-active-tasks.spec.ts › PBS active tasks">
        <failure message="63-pbs-active-tasks.spec.ts:83:3 shows a running task"><![CDATA[[chromium] › stable test
          Error: Expected "running" but received "stopped"]]></failure>
      </testcase>
      <testcase name="loads history" classname="chromium › journeys/07-audit-log-resilience.spec.ts › audit log">
        <error><![CDATA[[31mTimeout 10000ms exceeded[39m
          at tests/journeys/07-audit-log-resilience.spec.ts:30:1]]></error>
      </testcase>
    </testsuites>`;

  assert.deepEqual(parseFailedTestcases(xml), [
    {
      file: 'tests/integration/tests/63-pbs-active-tasks.spec.ts',
      message: 'Error: Expected "running" but received "stopped"',
      name: 'shows a running task',
    },
    {
      file: 'tests/integration/tests/journeys/07-audit-log-resilience.spec.ts',
      message: 'Timeout 10000ms exceeded',
      name: 'loads history',
    },
  ]);
});

test('emits bounded GitHub annotations with command escaping', () => {
  assert.deepEqual(
    githubAnnotations([
      {
        file: 'tests/integration/tests/example.spec.ts',
        message: 'expected 100%, got 0%\nsecond line',
        name: 'renders: cards, tables',
      },
    ]),
    [
      '::error file=tests/integration/tests/example.spec.ts,title=Stable E2E failure%3A renders%3A cards%2C tables::expected 100%25, got 0%25%0Asecond line',
    ],
  );
});

test('keeps a failed stable step observable when JUnit has no testcase failure', () => {
  assert.match(
    githubAnnotations([])[0],
    /^::error title=Stable E2E failure details unavailable::/,
  );
});
