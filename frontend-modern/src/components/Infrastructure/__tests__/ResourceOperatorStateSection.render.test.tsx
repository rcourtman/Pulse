import { describe, expect, it, vi } from 'vitest';
import { fireEvent, render, screen, waitFor } from '@solidjs/testing-library';

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

describe('operator guidance inside a workload table', () => {
  it('opts out of inherited no-wrap without changing descriptions or saved policy', async () => {
    const record = {
      canonicalId: 'table-readable-guidance',
      monitoringMode: 'normal',
      lifecycleState: 'active',
      intentionallyOffline: false,
      neverAutoRemediate: false,
      note: 'Independent thaw and filesystem checks remain required.',
      setAt: new Date().toISOString(),
      setBy: 'fixture-administrator-with-a-long-unbroken-identity@example.invalid',
    };
    apiClientMock.apiFetchJSON.mockResolvedValue({ configured: true, state: record });
    render(() => (
      <table class="table-fixed">
        <tbody>
          <tr>
            <td>
              <ResourceOperatorStateSection
                resourceId={record.canonicalId}
                resourceType="vm"
                platformType="proxmox"
              />
            </td>
          </tr>
        </tbody>
      </table>
    ));
    const section = screen.getByRole('region', { name: 'Operator overrides' });
    expect(section).toHaveClass('min-w-0', 'whitespace-normal', 'wrap-anywhere');
    expect(section.querySelector('header')).toHaveClass('flex-wrap', 'gap-2');
    await screen.findByText(/Set by fixture-administrator-with-a-long-unbroken-identity/);
    expect(screen.getByRole('combobox', { name: 'Patrol priority' })).toHaveAccessibleDescription(
      'Orders this resource among same-severity Patrol findings.',
    );
    const monitoring = screen.getByRole('combobox', { name: 'Monitoring' });
    expect(monitoring).toHaveAccessibleDescription(
      'Expected offline hides availability noise only. Mute all stops Alerts and Patrol while keeping this resource visible.',
    );
    const lifecycle = screen.getByRole('combobox', { name: 'Lifecycle' });
    expect(lifecycle).toHaveAccessibleDescription(
      'Proxmox owns this inventory record. Retiring it stops Pulse attention and automation without deleting it from Proxmox. Restore active monitoring here at any time.',
    );
    const callsBeforeEditing = apiClientMock.apiFetchJSON.mock.calls.length;
    fireEvent.change(lifecycle, { target: { value: 'retired' } });
    expect(monitoring).toBeDisabled();
    expect(
      screen.getByRole('button', { name: 'Never auto-remediate this resource' }),
    ).toBeDisabled();
    fireEvent.click(screen.getByRole('button', { name: 'Discard' }));
    expect(lifecycle).toHaveValue('active');
    expect(monitoring).toHaveValue('normal');
    expect(monitoring).toBeEnabled();
    expect(screen.getByRole('textbox', { name: 'Operator note' })).toHaveValue(record.note);
    expect(apiClientMock.apiFetchJSON.mock.calls).toHaveLength(callsBeforeEditing);
  });
});
