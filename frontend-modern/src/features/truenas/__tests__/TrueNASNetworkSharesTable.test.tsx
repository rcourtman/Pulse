import { cleanup, render, within } from '@solidjs/testing-library';
import { afterEach, describe, expect, it } from 'vitest';

import { TrueNASNetworkSharesTable } from '@/features/truenas/TrueNASNetworkSharesTable';
import type { Resource } from '@/types/resource';

const makeShare = (
  id: string,
  overrides: Partial<Resource> = {},
  shareOverrides: Partial<NonNullable<NonNullable<Resource['truenas']>['share']>> = {},
): Resource =>
  ({
    id,
    type: 'network-share',
    name: id,
    displayName: id,
    parentName: 'truenas-main',
    status: 'online',
    platformType: 'truenas',
    platformScopes: ['truenas'],
    truenas: {
      hostname: 'truenas-main',
      share: {
        id,
        name: id,
        protocol: 'smb',
        path: `/mnt/tank/${id}`,
        enabled: true,
        ...shareOverrides,
      },
    },
    ...overrides,
  }) as Resource;

afterEach(() => {
  cleanup();
});

describe('TrueNASNetworkSharesTable', () => {
  it('distinguishes Locked shares from stale Active shares and clears staleness on live recovery', () => {
    const staleActive = makeShare('media', {
      sourceStatus: { truenas: { status: 'stale' } },
    });
    const freshLocked = makeShare('vault', {}, { locked: true });
    const staleLocked = makeShare(
      'archive',
      { sourceStatus: { truenas: { status: 'error', error: 'API timeout' } } },
      { locked: true },
    );
    const recoveredActive = makeShare('backups', {
      sourceStatus: { truenas: { status: 'online' } },
      platformData: { sourceStatus: { truenas: { status: 'stale' } } },
    });

    const shares = [staleActive, freshLocked, staleLocked, recoveredActive];
    const { container } = render(() => (
      <TrueNASNetworkSharesTable
        shares={shares}
        scope={shares}
        emptyIcon={<span />}
        emptyTitle="No shares"
        emptyDescription="No shares"
        showToolbar={false}
      />
    ));

    const desktopState = (id: string) =>
      container.querySelector(
        `[data-truenas-share-row="${id}"] [data-truenas-share-state]`,
      ) as HTMLElement;
    const phoneState = (id: string) =>
      container.querySelector(
        `[data-truenas-share-row="${id}"] [data-truenas-share-state-phone]`,
      ) as HTMLElement;

    expect(desktopState('media')).toHaveTextContent('Active');
    expect(desktopState('media')).toHaveTextContent('not updated');
    expect(desktopState('media')).toHaveAttribute('title', 'TrueNAS has not updated this recently');
    expect(phoneState('media')).toHaveTextContent('Stale data');
    expect(phoneState('media')).not.toHaveClass('sr-only');

    expect(desktopState('vault')).toHaveTextContent('Locked');
    expect(desktopState('vault')).not.toHaveTextContent('not updated');
    expect(desktopState('vault')).toHaveAttribute('title', 'Encrypted dataset is locked');
    expect(phoneState('vault')).toHaveTextContent('Locked');
    expect(phoneState('vault')).not.toHaveClass('sr-only');

    expect(desktopState('archive')).toHaveTextContent('Locked');
    expect(desktopState('archive')).toHaveTextContent('not updated');
    expect(desktopState('archive')).toHaveAttribute(
      'title',
      'TrueNAS collection error: API timeout',
    );
    expect(phoneState('archive')).toHaveTextContent('Locked · Stale data');

    expect(desktopState('backups')).toHaveTextContent('Active');
    expect(desktopState('backups')).not.toHaveTextContent('not updated');
    expect(within(desktopState('backups')).queryByText('not updated')).toBeNull();
    expect(phoneState('backups')).toHaveClass('sr-only');
  });
});
