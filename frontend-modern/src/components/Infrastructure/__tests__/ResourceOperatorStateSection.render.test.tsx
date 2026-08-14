import { describe, expect, it, vi } from 'vitest';
import { render, screen, waitFor } from '@solidjs/testing-library';

import type { ResourceCapability } from '@/types/resource';

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
// left the auto_remediation_policy_json column NULL, and the pre-fix read
// path skipped normalization for NULL columns, so the wire carried
// "capabilityNames": null. The section's dirty-state memo spread that value
// during render, throwing "null is not iterable" straight into the route
// error boundary. These tests feed the section the real pre-fix wire
// payload through the canonical API client to pin the null tolerance.
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

const RESTART_CAPABILITY: ResourceCapability[] = [
  {
    name: 'restart',
    type: 'docker',
    description: 'Restart the container',
    autoAuthorization: 'low_risk',
  },
];

describe('ResourceOperatorStateSection render with capabilityNames: null', () => {
  it('renders the persisted record without crashing and reports a clean (not dirty) state', async () => {
    apiClientMock.apiFetchJSON.mockResolvedValue(OPERATOR_STATE_WITH_NULL_CAPABILITIES);

    render(() => (
      <ResourceOperatorStateSection
        resourceId="docker:host-1/ct-nginx"
        capabilities={RESTART_CAPABILITY}
      />
    ));

    // The section itself must be up (pre-fix, the throw escaped render
    // and nothing below the header survived).
    expect(screen.getByText('Operator overrides')).toBeTruthy();
    expect(screen.getByText('Monitoring')).toBeTruthy();
    expect(screen.getByText('Never auto-remediate')).toBeTruthy();

    // "Clear all overrides" only renders for persisted() && !isDirty() —
    // reaching it proves the dirty-state memo evaluated the null
    // capabilityNames without throwing and treated it as [].
    await waitFor(() => {
      expect(screen.getByText('Clear all overrides')).toBeTruthy();
    });
    expect(screen.getByText(/Set by admin/)).toBeTruthy();
    expect(screen.queryByText('Save overrides')).toBeNull();
  });

  it('hydrates a null capability allowlist as an empty selection in the automatic-actions block', async () => {
    apiClientMock.apiFetchJSON.mockResolvedValue(OPERATOR_STATE_WITH_NULL_CAPABILITIES);

    render(() => (
      <ResourceOperatorStateSection
        resourceId="docker:host-1/ct-nginx-2"
        capabilities={RESTART_CAPABILITY}
      />
    ));

    await waitFor(() => {
      expect(screen.getByText('Clear all overrides')).toBeTruthy();
    });

    // The automatic-actions block renders (the resource has an eligible
    // capability) with its toggle off and no phantom selection derived
    // from the null allowlist.
    expect(screen.getByLabelText('Automatic actions')).toBeTruthy();
    expect(screen.queryByText('Allowed actions')).toBeNull();
  });
});
