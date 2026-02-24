import { describe, expect, it } from 'vitest';
import {
  buildDockerImageTag,
  buildLinuxAmd64DownloadCommand,
  buildLinuxAmd64TarballName,
  buildReleaseNotesUrl,
  formatReleaseTag,
  normalizeReleaseVersion,
} from '@/components/updateVersion';

describe('updateVersion helpers', () => {
  it('normalizes optional leading v prefixes', () => {
    expect(normalizeReleaseVersion('v5.1.0')).toBe('5.1.0');
    expect(normalizeReleaseVersion('V5.1.0')).toBe('5.1.0');
    expect(normalizeReleaseVersion('  5.1.0  ')).toBe('5.1.0');
    expect(normalizeReleaseVersion('')).toBe('');
  });

  it('formats GitHub release tags safely', () => {
    expect(formatReleaseTag('v5.1.0')).toBe('v5.1.0');
    expect(formatReleaseTag('5.1.0')).toBe('v5.1.0');
    expect(formatReleaseTag(undefined)).toBe('');
  });

  it('builds resilient release links and artifact commands', () => {
    expect(buildReleaseNotesUrl('v5.1.0')).toBe(
      'https://github.com/rcourtman/Pulse/releases/tag/v5.1.0',
    );
    expect(buildLinuxAmd64TarballName('v5.1.0')).toBe('pulse-v5.1.0-linux-amd64.tar.gz');
    expect(buildDockerImageTag('v5.1.0')).toBe('5.1.0');

    const command = buildLinuxAmd64DownloadCommand('v5.1.0');
    expect(command).toContain(
      'curl -fL --retry 3 --retry-delay 2 -o pulse-v5.1.0-linux-amd64.tar.gz',
    );
    expect(command).toContain(
      'https://github.com/rcourtman/Pulse/releases/download/v5.1.0/pulse-v5.1.0-linux-amd64.tar.gz',
    );
    expect(command).toContain(
      'sudo tar -xzf pulse-v5.1.0-linux-amd64.tar.gz -C /usr/local/bin pulse',
    );
  });
});
