import { describe, expect, it } from 'vitest';
import {
  buildDockerImageTag,
  buildLinuxAmd64DownloadCommand,
  buildLinuxAmd64TarballName,
  buildReleaseNotesUrl,
} from '@/components/updateVersion';

const GITHUB_RELEASES_BASE_URL = 'https://github.com/rcourtman/Pulse/releases';

describe('buildReleaseNotesUrl branch coverage', () => {
  it('returns the releases index URL when the version normalizes to empty (falsy-tag ternary arm)', () => {
    expect(buildReleaseNotesUrl(undefined)).toBe(GITHUB_RELEASES_BASE_URL);
    expect(buildReleaseNotesUrl(null)).toBe(GITHUB_RELEASES_BASE_URL);
    expect(buildReleaseNotesUrl('')).toBe(GITHUB_RELEASES_BASE_URL);
    expect(buildReleaseNotesUrl('   ')).toBe(GITHUB_RELEASES_BASE_URL);
    expect(buildReleaseNotesUrl('\t\n')).toBe(GITHUB_RELEASES_BASE_URL);
    // A bare leading "v" is stripped to "" by normalizeReleaseVersion, so the
    // formatted tag is empty and the falsy arm is taken here too.
    expect(buildReleaseNotesUrl('v')).toBe(GITHUB_RELEASES_BASE_URL);
    expect(buildReleaseNotesUrl('V')).toBe(GITHUB_RELEASES_BASE_URL);
  });

  it('returns the tag-scoped URL when a version is present (truthy-tag ternary arm)', () => {
    expect(buildReleaseNotesUrl('v5.1.0')).toBe(`${GITHUB_RELEASES_BASE_URL}/tag/v5.1.0`);
    // No leading "v": normalize keeps it, format re-adds the prefix.
    expect(buildReleaseNotesUrl('5.1.0')).toBe(`${GITHUB_RELEASES_BASE_URL}/tag/v5.1.0`);
    // Surrounding whitespace is trimmed before formatting.
    expect(buildReleaseNotesUrl('  v5.1.0  ')).toBe(`${GITHUB_RELEASES_BASE_URL}/tag/v5.1.0`);
    // Uppercase "V" prefix is stripped case-insensitively.
    expect(buildReleaseNotesUrl('V5.1.0')).toBe(`${GITHUB_RELEASES_BASE_URL}/tag/v5.1.0`);
  });
});

describe('buildDockerImageTag branch coverage', () => {
  it('falls back to "latest" when the version normalizes to empty (|| fallback arm)', () => {
    expect(buildDockerImageTag(undefined)).toBe('latest');
    expect(buildDockerImageTag(null)).toBe('latest');
    expect(buildDockerImageTag('')).toBe('latest');
    expect(buildDockerImageTag('   ')).toBe('latest');
    expect(buildDockerImageTag('v')).toBe('latest');
    expect(buildDockerImageTag('V')).toBe('latest');
  });

  it('returns the normalized version when present (truthy arm)', () => {
    expect(buildDockerImageTag('v5.1.0')).toBe('5.1.0');
    expect(buildDockerImageTag('5.1.0')).toBe('5.1.0');
    expect(buildDockerImageTag('  v5.1.0  ')).toBe('5.1.0');
    expect(buildDockerImageTag('V5.1.0')).toBe('5.1.0');
  });
});

describe('buildLinuxAmd64TarballName branch coverage', () => {
  it('returns the unversioned tarball name when the version normalizes to empty (falsy ternary arm)', () => {
    expect(buildLinuxAmd64TarballName(undefined)).toBe('pulse-linux-amd64.tar.gz');
    expect(buildLinuxAmd64TarballName(null)).toBe('pulse-linux-amd64.tar.gz');
    expect(buildLinuxAmd64TarballName('')).toBe('pulse-linux-amd64.tar.gz');
    expect(buildLinuxAmd64TarballName('   ')).toBe('pulse-linux-amd64.tar.gz');
    expect(buildLinuxAmd64TarballName('v')).toBe('pulse-linux-amd64.tar.gz');
    expect(buildLinuxAmd64TarballName('V')).toBe('pulse-linux-amd64.tar.gz');
  });

  it('returns the versioned tarball name when a version is present (truthy ternary arm)', () => {
    expect(buildLinuxAmd64TarballName('v5.1.0')).toBe('pulse-v5.1.0-linux-amd64.tar.gz');
    expect(buildLinuxAmd64TarballName('5.1.0')).toBe('pulse-v5.1.0-linux-amd64.tar.gz');
    expect(buildLinuxAmd64TarballName('  v5.1.0  ')).toBe('pulse-v5.1.0-linux-amd64.tar.gz');
    expect(buildLinuxAmd64TarballName('V5.1.0')).toBe('pulse-v5.1.0-linux-amd64.tar.gz');
  });
});

describe('buildLinuxAmd64DownloadCommand branch coverage', () => {
  it('returns an empty string when the version normalizes to empty (early-return guard)', () => {
    expect(buildLinuxAmd64DownloadCommand(undefined)).toBe('');
    expect(buildLinuxAmd64DownloadCommand(null)).toBe('');
    expect(buildLinuxAmd64DownloadCommand('')).toBe('');
    expect(buildLinuxAmd64DownloadCommand('   ')).toBe('');
    expect(buildLinuxAmd64DownloadCommand('v')).toBe('');
    expect(buildLinuxAmd64DownloadCommand('V')).toBe('');
  });

  it('builds the full curl + tar command for a "v"-prefixed version (truthy-tag arm)', () => {
    const expected =
      'curl -fL --retry 3 --retry-delay 2 -o pulse-v5.1.0-linux-amd64.tar.gz ' +
      `${GITHUB_RELEASES_BASE_URL}/download/v5.1.0/pulse-v5.1.0-linux-amd64.tar.gz\n` +
      'sudo tar -xzf pulse-v5.1.0-linux-amd64.tar.gz -C /usr/local/bin pulse';
    expect(buildLinuxAmd64DownloadCommand('v5.1.0')).toBe(expected);
  });

  it('uses a consistent tag and tarball name for an un-prefixed version (truthy-tag arm)', () => {
    const expected =
      'curl -fL --retry 3 --retry-delay 2 -o pulse-v5.1.0-linux-amd64.tar.gz ' +
      `${GITHUB_RELEASES_BASE_URL}/download/v5.1.0/pulse-v5.1.0-linux-amd64.tar.gz\n` +
      'sudo tar -xzf pulse-v5.1.0-linux-amd64.tar.gz -C /usr/local/bin pulse';
    expect(buildLinuxAmd64DownloadCommand('5.1.0')).toBe(expected);
  });

  it('trims surrounding whitespace before building the command (truthy-tag arm)', () => {
    const expected =
      'curl -fL --retry 3 --retry-delay 2 -o pulse-v5.1.0-linux-amd64.tar.gz ' +
      `${GITHUB_RELEASES_BASE_URL}/download/v5.1.0/pulse-v5.1.0-linux-amd64.tar.gz\n` +
      'sudo tar -xzf pulse-v5.1.0-linux-amd64.tar.gz -C /usr/local/bin pulse';
    expect(buildLinuxAmd64DownloadCommand('  v5.1.0  ')).toBe(expected);
  });
});
