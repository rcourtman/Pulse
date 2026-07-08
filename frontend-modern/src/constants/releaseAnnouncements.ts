import type { SecurityStatus } from '@/types/config';

export const V5_MAINTENANCE_BRANCH = 'release/5.1';

export const V6_GA_ANNOUNCEMENT = {
  id: 'v6-ga-available',
  upgradeGuideUrl: 'https://github.com/rcourtman/Pulse/blob/main/docs/UPGRADE_v6.md',
  changelogUrl: 'https://github.com/rcourtman/Pulse/blob/main/docs/releases/V6_CHANGELOG.md',
} as const;

function parseMajorVersion(version?: string | null): number | null {
  const match = String(version || '').trim().match(/^v?(\d+)/i);
  if (!match) {
    return null;
  }

  const major = Number(match[1]);
  return Number.isFinite(major) ? major : null;
}

export function isV5ReleaseLine(version?: string | null): boolean {
  return parseMajorVersion(version) === 5;
}

export function canSeeAdminReleaseAnnouncement(
  securityStatus?: SecurityStatus | null,
): boolean {
  if (!securityStatus) {
    return true;
  }

  if (!securityStatus.hasProxyAuth) {
    return true;
  }

  return securityStatus.proxyAuthIsAdmin === true;
}

export function shouldShowV6Announcement(opts: {
  version?: string | null;
  pathname?: string | null;
  securityStatus?: SecurityStatus | null;
}): boolean {
  if (!isV5ReleaseLine(opts.version)) {
    return false;
  }

  if (!canSeeAdminReleaseAnnouncement(opts.securityStatus)) {
    return false;
  }

  const path = opts.pathname || '';
  return (
    path === '/' ||
    path === '/proxmox' ||
    path === '/proxmox/overview' ||
    path === '/settings' ||
    path.startsWith('/settings/')
  );
}
