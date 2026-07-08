import { describe, expect, it } from 'vitest';
import type { SecurityStatus } from '@/types/config';
import {
  canSeeAdminReleaseAnnouncement,
  isV5ReleaseLine,
  shouldShowV6Announcement,
} from '@/constants/releaseAnnouncements';

describe('releaseAnnouncements', () => {
  it('detects the v5 release line from semver strings', () => {
    expect(isV5ReleaseLine('v5.1.27')).toBe(true);
    expect(isV5ReleaseLine('5.1.28')).toBe(true);
    expect(isV5ReleaseLine('v6.0.0-rc.1')).toBe(false);
    expect(isV5ReleaseLine('')).toBe(false);
  });

  it('allows the announcement for non-proxy-auth sessions', () => {
    const status = {
      hasProxyAuth: false,
      proxyAuthIsAdmin: false,
    } as SecurityStatus;

    expect(canSeeAdminReleaseAnnouncement(status)).toBe(true);
  });

  it('blocks the announcement for non-admin proxy-auth sessions', () => {
    const status = {
      hasProxyAuth: true,
      proxyAuthIsAdmin: false,
    } as SecurityStatus;

    expect(canSeeAdminReleaseAnnouncement(status)).toBe(false);
  });

  it('shows the announcement only on the supported v5 surfaces', () => {
    expect(
      shouldShowV6Announcement({
        version: 'v5.1.27',
        pathname: '/proxmox/overview',
      }),
    ).toBe(true);

    expect(
      shouldShowV6Announcement({
        version: 'v5.1.27',
        pathname: '/settings/updates',
      }),
    ).toBe(true);

    expect(
      shouldShowV6Announcement({
        version: 'v5.1.27',
        pathname: '/alerts/overview',
      }),
    ).toBe(false);

    expect(
      shouldShowV6Announcement({
        version: 'v6.0.0-rc.1',
        pathname: '/settings/updates',
      }),
    ).toBe(false);
  });
});
