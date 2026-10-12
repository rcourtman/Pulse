import { describe, expect, it, vi } from 'vitest';
import { render, screen, waitFor } from '@solidjs/testing-library';

const apiClientMock = vi.hoisted(() => ({
  apiFetchJSON: vi.fn(),
}));

vi.mock('@/utils/apiClient', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@/utils/apiClient')>();
  return {
    ...actual,
    apiFetchJSON: apiClientMock.apiFetchJSON,
  };
});

import { ResourceOperatorStateSection } from '@/components/Infrastructure/ResourceOperatorStateSection';

// Regression for issue #1621: rows saved with only "intentionally offline"
// carried "autoRemediationPolicy": {"capabilityNames": null} on the wire, and
// the section's dirty-state memo spread that value during render. The
// per-resource allowlist is retired and newer servers no longer send it, but
// an older server can, so the section must render that payload as a clean
// record.
const OPERATOR_STATE_WITH_NULL_CAPABILITIES = JSON.parse(
  `{
    "canonicalId": "docker:host-1/ct-nginx",
    "intentionallyOffline": true,
    "neverAutoRemediate": false,
    "autoRemediationPolicy": {"enabled": false, "capabilityNames": null},
    "setAt": "2026-07-20T10:00:00Z",
    "setBy": "admin"
  }`,
) as unknown;

describe('ResourceOperatorStateSection render with a retired allowlist payload', () => {
  it('renders the persisted record without crashing and reports a clean (not dirty) state', async () => {
    apiClientMock.apiFetchJSON.mockResolvedValue(OPERATOR_STATE_WITH_NULL_CAPABILITIES);

    render(() => <ResourceOperatorStateSection resourceId="docker:host-1/ct-nginx" />);

    // The section itself must be up (pre-fix, the throw escaped render
    // and nothing below the header survived).
    expect(screen.getByText('Operator overrides')).toBeTruthy();
    expect(screen.getByText('Monitoring')).toBeTruthy();
    expect(screen.getByText('Never auto-remediate')).toBeTruthy();

    // "Clear all overrides" only renders for persisted() && !isDirty() —
    // reaching it proves the retired allowlist payload left the record
    // clean.
    await waitFor(() => {
      expect(screen.getByText('Clear all overrides')).toBeTruthy();
    });
    expect(screen.getByText(/Set by admin/)).toBeTruthy();
    expect(screen.queryByText('Save overrides')).toBeNull();
  });
});

describe('maintenance schedule timestamps', () => {
  it.each(['active', 'scheduled'] as const)(
    'shows absolute future times for a %s window',
    async (kind) => {
      const start = new Date(Date.now() + (kind === 'active' ? -60_000 : 3_600_000)).toISOString();
      const end = new Date(Date.now() + 7_200_000).toISOString();
      apiClientMock.apiFetchJSON.mockResolvedValue({
        canonicalId: `maintenance-${kind}`,
        maintenanceStartAt: start,
        maintenanceEndAt: end,
      });
      render(() => <ResourceOperatorStateSection resourceId={`maintenance-${kind}`} />);
      const badge = await screen.findByText(
        kind === 'active' ? 'Maintenance window active.' : 'Maintenance window scheduled.',
      );
      expect(badge.parentElement).toHaveTextContent(new Date(end).toLocaleString());
      if (kind === 'scheduled')
        expect(badge.parentElement).toHaveTextContent(new Date(start).toLocaleString());
      expect(badge.parentElement).not.toHaveTextContent('just now');
      expect(badge.parentElement).toHaveTextContent(/attention.*paused/i);
    },
  );
});
