const PLAYWRIGHT_LIST_LINE = /^\s*\[([^\]]+)\] › (.+?\.spec\.ts):\d+:\d+ › (.+)$/;

// Playwright's transform cache can report the same test at its authored line
// on one --list process and at a transformed line on the next. Source
// coordinates are presentation metadata, not test identity; tier accounting
// is project + spec + title.
export function parsePlaywrightTestIdentity(rawLine) {
  const line = rawLine.replace(/\x1b\[[0-9;]*m/g, '');
  const match = line.match(PLAYWRIGHT_LIST_LINE);
  if (!match) return null;

  const [, project, specFile, title] = match;
  return {
    key: `[${project}] › ${specFile} › ${title}`,
    specFile,
  };
}
