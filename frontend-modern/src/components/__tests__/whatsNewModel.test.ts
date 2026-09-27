import { describe, expect, it } from 'vitest';
import { readFileSync } from 'node:fs';
import path from 'node:path';
import { extractChangelog, extractHighlights, isReleaseVersion } from '../whatsNewModel';
import whatsNewCardSource from '../WhatsNewCard.tsx?raw';

describe('extractHighlights', () => {
  it('extracts the Highlights section from a release body', () => {
    const body = [
      'Intro paragraph.',
      '',
      '## Highlights',
      '- New Docker container update flow',
      '- Faster dashboard loading',
      '',
      '## Full changelog',
      '- Fix process leak in host agent command executor',
    ].join('\n');

    expect(extractHighlights(body)).toBe(
      '- New Docker container update flow\n- Faster dashboard loading',
    );
  });

  // Locks the contract with scripts/generate-release-notes.sh, whose LLM
  // template emits "### Highlights" as the first level-3 section.
  it('handles the generated release-notes format', () => {
    const body = [
      '## v6.0.6',
      '',
      '### Highlights',
      '- Post-update What’s New banner',
      '- Faster dashboard loading',
      '',
      '### New Features',
      '- Something for the changelog reader',
      '',
      '### Bug Fixes',
      '- Fix a thing (#1234)',
      '',
      '---',
      '',
      '## Installation',
      '...',
    ].join('\n');

    expect(extractHighlights(body)).toBe(
      '- Post-update What’s New banner\n- Faster dashboard loading',
    );
  });

  it('stops at the next heading of the same or higher level', () => {
    const body = [
      '## Highlights',
      '### Monitoring',
      '- Zappi surplus alerts',
      '## Other changes',
      '- internal refactor',
    ].join('\n');

    expect(extractHighlights(body)).toBe('### Monitoring\n- Zappi surplus alerts');
  });

  it('matches the heading case-insensitively and at any level', () => {
    const body = '### HIGHLIGHTS\n- something';
    expect(extractHighlights(body)).toBe('- something');
  });

  it('handles CRLF line endings from the GitHub editor', () => {
    const body = '## Highlights\r\n- one\r\n- two\r\n\r\n## Rest';
    expect(extractHighlights(body)).toBe('- one\n- two');
  });

  it('returns null when there is no Highlights section', () => {
    expect(extractHighlights('## Changelog\n- fix things')).toBeNull();
  });

  it("uses the customer-facing What's improved section when Highlights is absent", () => {
    const body = [
      '# Pulse v6.4.0 Release Notes',
      '',
      'Pulse is faster in larger environments.',
      '',
      "## What's improved",
      '',
      '- **Faster large environments** — Tables stay responsive as estates grow.',
      '- **Lighter realtime updates** — Pages do less work when resources change.',
      '',
      '## Fixes',
      '',
      '- Saved API keys are no longer returned to the browser.',
    ].join('\n');

    expect(extractHighlights(body)).toBe(
      [
        '- **Faster large environments** — Tables stay responsive as estates grow.',
        '- **Lighter realtime updates** — Pages do less work when resources change.',
      ].join('\n'),
    );
  });

  it('prefers historical Highlights when both preview headings exist', () => {
    const body = [
      '## Highlights',
      '- Short preview.',
      '',
      "## What's improved",
      '- **Longer detail** — Full customer-facing explanation.',
    ].join('\n');

    expect(extractHighlights(body)).toBe('- Short preview.');
  });

  it('returns null when the Highlights section is empty', () => {
    expect(extractHighlights('## Highlights\n\n## Changelog\n- fix')).toBeNull();
  });

  it('does not match headings that merely contain the word later', () => {
    expect(extractHighlights('## Not the Highlights\n- nope')).toBeNull();
  });
});

describe('extractChangelog', () => {
  it('builds a categorized changelog and excludes the highlights summary', () => {
    const body = [
      '# Pulse v6.2.1 Release Notes',
      '',
      '## Highlights',
      '- Agent improvements and fixes.',
      '',
      '## Added',
      '- Plans & Billing is now available before Pro activation.',
      '',
      '## Improved',
      '- Update results show when the last check ran.',
      '',
      '## Fixed',
      '- Agent downloads now work when the server redirects the request (#1696).',
      '',
      '## Upgrade Notes',
      'Use the normal update flow.',
    ].join('\n');

    expect(extractChangelog(body)).toBe(
      [
        '### Added',
        '',
        '- Plans & Billing is now available before Pro activation.',
        '',
        '### Improved',
        '',
        '- Update results show when the last check ran.',
        '',
        '### Fixed',
        '',
        '- Agent downloads now work when the server redirects the request (#1696).',
      ].join('\n'),
    );
  });

  it('normalizes generated section names into stable changelog labels', () => {
    const body = [
      '## v6.2.2',
      '### New Features',
      '- Add certificate expiry monitoring.',
      '### Improvements',
      '- Make storage filters easier to scan.',
      '### Bug Fixes',
      '- Keep acknowledged alerts dismissed after refresh.',
    ].join('\n');

    expect(extractChangelog(body)).toBe(
      [
        '### Added',
        '',
        '- Add certificate expiry monitoring.',
        '',
        '### Improved',
        '',
        '- Make storage filters easier to scan.',
        '',
        '### Fixed',
        '',
        '- Keep acknowledged alerts dismissed after refresh.',
      ].join('\n'),
    );
  });

  it("includes What's improved in the post-update changelog", () => {
    const body = [
      "## What's improved",
      '- **Faster large environments** — Tables stay responsive as estates grow.',
      '## Fixes',
      '- Storage rows remain visible after refresh.',
    ].join('\n');

    expect(extractChangelog(body)).toBe(
      [
        '### Improved',
        '',
        '- **Faster large environments** — Tables stay responsive as estates grow.',
        '',
        '### Fixed',
        '',
        '- Storage rows remain visible after refresh.',
      ].join('\n'),
    );
  });

  it('preserves nested details inside a recognized category', () => {
    const body = [
      '## Fixed',
      '### Proxmox',
      '- Storage rows no longer disappear after refresh.',
      '## Installation',
      '- Not part of the changelog.',
    ].join('\n');

    expect(extractChangelog(body)).toBe(
      '### Fixed\n\n### Proxmox\n- Storage rows no longer disappear after refresh.',
    );
  });

  it('returns null for summaries and release metadata without change categories', () => {
    expect(
      extractChangelog('## Highlights\n- Faster updates.\n\n## Installation\n- Pull the image.'),
    ).toBeNull();
  });
});

describe('isReleaseVersion', () => {
  it('accepts published release versions', () => {
    expect(isReleaseVersion('4.13.0')).toBe(true);
    expect(isReleaseVersion('v4.13.0')).toBe(true);
    expect(isReleaseVersion('4.13.0-rc.1')).toBe(true);
  });

  it('rejects dev and dirty builds', () => {
    expect(isReleaseVersion('4.13.0-dirty')).toBe(false);
    expect(isReleaseVersion('v4.13.0-3-g1a2b3c4')).toBe(false);
    expect(isReleaseVersion('development')).toBe(false);
    expect(isReleaseVersion('4.13')).toBe(false);
    expect(isReleaseVersion('4.13.0-preview')).toBe(false);
    expect(isReleaseVersion('')).toBe(false);
  });
});

describe('post-update telemetry disclosure', () => {
  it('does not announce telemetry payload changes from the release boundary', () => {
    // Payload changes are disclosed in release notes and the dated
    // PRIVACY.md changelog. A banner that pairs "we now collect more" with a
    // one-click disable button reads as an opt-out prompt, so it was retired.
    expect(whatsNewCardSource).not.toContain('TELEMETRY_PAYLOAD_NOTICE_VERSION');
    expect(whatsNewCardSource).not.toContain('telemetry-payload-update-notice');
    expect(whatsNewCardSource).not.toContain('openTelemetrySettings');
  });

  it('keeps release details opt-in and coordinates the automatic notice session', () => {
    expect(whatsNewCardSource).toContain("reserveLowPriorityNoticeSession('release-update')");
    expect(whatsNewCardSource).toContain('markVersionSeen(currentVersion);');
    expect(whatsNewCardSource).toContain('actionLabel="See what\'s new"');
    expect(whatsNewCardSource).toContain('setDialogVisible(true)');
    expect(whatsNewCardSource).not.toContain('setDialogVisible(true);\n      markVersionSeen');
  });
});

describe('current candidate notification packet', () => {
  it('keeps manual workload column sizing in the customer-facing release packet', () => {
    const releaseNotes = readFileSync(
      path.resolve(process.cwd(), '../docs/releases/RELEASE_NOTES_v6.4.0-rc.1.md'),
      'utf8',
    );

    expect(releaseNotes).toContain('Workload table columns can be resized');
    expect(releaseNotes).toContain('keeps every selected column reachable');
    expect(releaseNotes).toContain('shared through the page URL or reset');
  });

  it('keeps translation-ready webhook support in both release summaries', () => {
    const releaseNotes = readFileSync(
      path.resolve(process.cwd(), '../docs/releases/RELEASE_NOTES_v6.4.0-rc.3.md'),
      'utf8',
    );
    const changelog = readFileSync(
      path.resolve(process.cwd(), '../docs/releases/V6_CHANGELOG_v6.4.0-rc.3.md'),
      'utf8',
    );

    expect(releaseNotes).toContain('Translation-ready webhooks');
    expect(changelog).toContain('stable language-neutral message key');
  });
});
