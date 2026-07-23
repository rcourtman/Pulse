import { describe, expect, it } from 'vitest';
import { extractHighlights, isReleaseVersion } from '../whatsNewModel';
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

  it('returns null when the Highlights section is empty', () => {
    expect(extractHighlights('## Highlights\n\n## Changelog\n- fix')).toBeNull();
  });

  it('does not match headings that merely contain the word later', () => {
    expect(extractHighlights('## Not the Highlights\n- nope')).toBeNull();
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
  it('keeps the schema-v2 notice on the shared non-blocking release boundary', () => {
    expect(whatsNewCardSource).toContain('TELEMETRY_PAYLOAD_NOTICE_VERSION');
    expect(whatsNewCardSource).toContain('data-testid="telemetry-payload-update-notice"');
    expect(whatsNewCardSource).toContain('layout="banner"');
    expect(whatsNewCardSource).toContain("openTelemetrySettings('preview')");
    expect(whatsNewCardSource).toContain("openTelemetrySettings('disable')");
    expect(whatsNewCardSource).toContain('PRIVACY_DOC_URL');
  });
});
