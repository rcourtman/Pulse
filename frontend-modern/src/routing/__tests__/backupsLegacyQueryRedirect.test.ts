import { describe, expect, it } from 'vitest';
import { getBackupsLegacyQueryRedirectTarget } from '../backupsLegacyQueryRedirect';

function asURL(path: string): URL {
  return new URL(path, 'http://localhost');
}

describe('backups legacy query redirect', () => {
  it('rewrites legacy backups params into the canonical v6 contract', () => {
    const target = getBackupsLegacyQueryRedirectTarget(
      '?view=artifacts&backupType=remote&group=guest&search=vm-101&source=pbs&status=verified&foo=bar',
    );
    expect(target).not.toBeNull();
    const url = asURL(target!);
    expect(url.pathname).toBe('/backups');
    expect(url.searchParams.get('view')).toBe('events');
    expect(url.searchParams.get('mode')).toBe('remote');
    expect(url.searchParams.get('scope')).toBe('workload');
    expect(url.searchParams.get('provider')).toBe('proxmox-pbs');
    expect(url.searchParams.get('q')).toBe('vm-101');
    expect(url.searchParams.get('verification')).toBe('verified');
    expect(url.searchParams.get('status')).toBeNull();
    expect(url.searchParams.get('source')).toBeNull();
    expect(url.searchParams.get('backupType')).toBeNull();
    expect(url.searchParams.get('group')).toBeNull();
    expect(url.searchParams.get('search')).toBeNull();
    expect(url.searchParams.get('foo')).toBe('bar');
  });

  it('canonicalizes shorthand provider values', () => {
    const target = getBackupsLegacyQueryRedirectTarget('?provider=pbs&view=events');
    expect(target).not.toBeNull();
    const url = asURL(target!);
    expect(url.pathname).toBe('/backups');
    expect(url.searchParams.get('view')).toBe('events');
    expect(url.searchParams.get('provider')).toBe('proxmox-pbs');
  });

  it('returns null when no legacy params are present', () => {
    expect(getBackupsLegacyQueryRedirectTarget('?view=events&mode=remote&scope=workload&q=vm-101')).toBeNull();
    expect(getBackupsLegacyQueryRedirectTarget('')).toBeNull();
  });
});
