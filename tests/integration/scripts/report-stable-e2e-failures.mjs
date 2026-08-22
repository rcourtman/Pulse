#!/usr/bin/env node

import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const decodeXml = (value) =>
  value
    .replaceAll('&quot;', '"')
    .replaceAll('&apos;', "'")
    .replaceAll('&lt;', '<')
    .replaceAll('&gt;', '>')
    .replaceAll('&amp;', '&');

const attribute = (source, name) => {
  const match = source.match(new RegExp(`\\b${name}="([^"]*)"`));
  return match ? decodeXml(match[1]) : '';
};

const stripCdata = (value) =>
  decodeXml(value.replace(/^<!\[CDATA\[/, '').replace(/\]\]>$/, ''));

const compactFailure = (value) => {
  const lines = stripCdata(value)
    .replace(/\u001b\[[0-9;]*m/g, '')
    .split(/\r?\n/)
    .map((line) => line.trim())
    .filter(Boolean);
  const plain =
    lines.find((line) =>
      /^(?:[A-Za-z]*Error:|Error:|Timeout \d|Timed out|Expected |Received )/.test(
        line,
      ),
    ) || lines.find((line) => !line.startsWith('at '));
  return (plain || 'Playwright reported a stable-tier failure').slice(0, 1200);
};

const sourceFile = (classname) => {
  const normalized = classname.replaceAll('\\', '/');
  const match = normalized.match(
    /(?:^| › )((?:journeys\/)?[^ ›]+\.spec\.ts)(?: › |$)/,
  );
  return match ? `tests/integration/tests/${match[1]}` : '';
};

export const parseFailedTestcases = (xml) => {
  const failures = [];
  const testcasePattern = /<testcase\b([^>]*?)(?<!\/)>([\s\S]*?)<\/testcase>/g;
  for (const testcase of xml.matchAll(testcasePattern)) {
    const body = testcase[2];
    const failure = body.match(/<(?:failure|error)\b([^>]*)>([\s\S]*?)<\/(?:failure|error)>/);
    if (!failure) continue;

    const classname = attribute(testcase[1], 'classname');
    const name = attribute(testcase[1], 'name') || 'Unnamed Playwright test';
    const bodyMessage = compactFailure(failure[2]);
    const summaryMessage = attribute(failure[1], 'message');
    const message = /^(?:[A-Za-z]*Error:|Error:|Timeout \d|Timed out|Expected |Received )/.test(
      bodyMessage,
    )
      ? bodyMessage
      : summaryMessage || bodyMessage;
    failures.push({
      file: sourceFile(classname),
      message: compactFailure(message),
      name,
    });
  }
  return failures;
};

const escapeCommandData = (value) =>
  value.replaceAll('%', '%25').replaceAll('\r', '%0D').replaceAll('\n', '%0A');

const escapeCommandProperty = (value) =>
  escapeCommandData(value).replaceAll(':', '%3A').replaceAll(',', '%2C');

export const githubAnnotations = (failures) => {
  if (failures.length === 0) {
    return [
      '::error title=Stable E2E failure details unavailable::' +
        'The stable step failed without a testcase failure in JUnit XML; inspect the retained Playwright report and runtime logs.',
    ];
  }

  return failures.map(({ file, message, name }) => {
    const properties = [
      file ? `file=${escapeCommandProperty(file)}` : '',
      `title=${escapeCommandProperty(`Stable E2E failure: ${name}`)}`,
    ]
      .filter(Boolean)
      .join(',');
    return `::error ${properties}::${escapeCommandData(message)}`;
  });
};

const isMain =
  process.argv[1] &&
  path.resolve(process.argv[1]) === path.resolve(fileURLToPath(import.meta.url));

if (isMain) {
  const reportPath = path.resolve(process.argv[2] || 'test-results/junit.xml');
  let xml;
  try {
    xml = fs.readFileSync(reportPath, 'utf8');
  } catch (error) {
    console.log(
      '::error title=Stable E2E JUnit report unavailable::' +
        escapeCommandData(`Could not read ${reportPath}: ${error.message}`),
    );
    process.exit(0);
  }

  const failures = parseFailedTestcases(xml);
  console.log(`Stable E2E JUnit failures: ${failures.length}`);
  for (const annotation of githubAnnotations(failures)) {
    console.log(annotation);
  }
}
