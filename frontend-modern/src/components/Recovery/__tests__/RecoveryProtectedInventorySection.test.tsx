import { render, screen } from '@solidjs/testing-library';
import { describe, expect, it } from 'vitest';

import { RecoveryProtectedInventorySection } from '@/components/Recovery/RecoveryProtectedInventorySection';
import type { ProtectionRollup, VerifyIntent } from '@/types/recovery';
import type { Resource } from '@/types/resource';

const baseRollup: ProtectionRollup = {
  rollupId: 'res:vm-100',
  itemResourceId: 'vm-100',
  display: {
    subjectLabel: 'web-prod',
    subjectType: 'proxmox-vm',
  },
  lastAttemptAt: '2026-05-10T12:00:00Z',
  lastSuccessAt: '2026-05-10T12:00:00Z',
  lastOutcome: 'success',
  platforms: ['proxmox-pbs'],
};

const renderSection = (rollup: ProtectionRollup) => {
  const rollups = [rollup];
  return render(() => (
    <RecoveryProtectedInventorySection
      filteredRollups={() => rollups}
      itemTypeFilter={() => 'all'}
      itemTypeOptions={() => ['all']}
      isMobile={false}
      kioskMode
      onSelectRollup={() => undefined}
      protectedStateFilter={() => 'all'}
      platformFilter={() => 'all'}
      platformOptions={() => ['all']}
      queryFilter={() => ''}
      resourcesById={() => new Map<string, Resource>()}
      rollups={() => rollups}
      rollupsSummary={() => ({
        total: rollups.length,
        counts: {},
        stale: 0,
        neverSucceeded: 0,
      })}
      setItemTypeFilter={() => undefined}
      setProtectedStateFilter={() => undefined}
      setPlatformFilter={() => undefined}
      setQueryFilter={() => undefined}
      setVerificationFilter={() => undefined}
      loading={() => false}
      error={() => undefined}
    />
  ));
};

describe('RecoveryProtectedInventorySection — Verify due badge', () => {
  it('renders the Verify due badge when verifyIntent is stale', () => {
    renderSection({ ...baseRollup, verifyIntent: 'stale' });

    const badge = screen.getByTestId('recovery-protected-verify-due-badge');
    expect(badge).toBeInTheDocument();
    expect(badge.textContent).toMatch(/verify due/i);
  });

  it('does not render the Verify due badge when verifyIntent is verified', () => {
    renderSection({ ...baseRollup, verifyIntent: 'verified' });

    expect(screen.queryByTestId('recovery-protected-verify-due-badge')).toBeNull();
  });

  it('does not render the Verify due badge when verifyIntent is unknown', () => {
    renderSection({ ...baseRollup, verifyIntent: 'unknown' as VerifyIntent });

    expect(screen.queryByTestId('recovery-protected-verify-due-badge')).toBeNull();
  });

  it('does not render the Verify due badge when verifyIntent is omitted (legacy backend)', () => {
    renderSection({ ...baseRollup });

    expect(screen.queryByTestId('recovery-protected-verify-due-badge')).toBeNull();
  });
});
