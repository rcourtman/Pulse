import { describe, expect, it } from 'vitest';

import type { Resource } from '@/types/resource';
import { buildServiceDetailLinks } from '@/components/Infrastructure/serviceDetailLinks';

const baseResource = (overrides: Partial<Resource>): Resource => ({
  id: 'resource-1',
  type: 'host',
  name: 'host-1',
  displayName: 'Host 1',
  platformId: 'host-1',
  platformType: 'host-agent',
  sourceType: 'api',
  status: 'online',
  lastSeen: Date.now(),
  platformData: { sources: ['agent'] },
  ...overrides,
});

describe('buildServiceDetailLinks', () => {
  it('returns PBS drill-down link to backups filtered for remote PBS backups', () => {
    const links = buildServiceDetailLinks(
      baseResource({
        id: 'pbs-main',
        type: 'pbs',
        name: 'pbs-main',
        displayName: 'PBS Main',
        platformType: 'proxmox-pbs',
      }),
    );

    expect(links).toEqual([
      {
        href: '/backups?source=pbs&backupType=remote',
        label: 'Open in Backups',
        compactLabel: 'Backups',
        ariaLabel: 'Open PBS backups for PBS Main',
      },
    ]);
  });

  it('returns PMG drill-down link to mail gateway thresholds', () => {
    const links = buildServiceDetailLinks(
      baseResource({
        id: 'pmg-main',
        type: 'pmg',
        name: 'pmg-main',
        displayName: 'PMG Main',
        platformType: 'proxmox-pmg',
      }),
    );

    expect(links).toEqual([
      {
        href: '/alerts/thresholds/mail-gateway',
        label: 'Open PMG thresholds',
        compactLabel: 'Thresholds',
        ariaLabel: 'Open PMG thresholds for PMG Main',
      },
    ]);
  });

  it('returns no service links for non-service resources', () => {
    const links = buildServiceDetailLinks(baseResource({ type: 'host' }));
    expect(links).toEqual([]);
  });
});
