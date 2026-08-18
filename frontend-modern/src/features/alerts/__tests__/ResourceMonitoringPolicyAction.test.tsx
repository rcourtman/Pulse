import { cleanup, fireEvent, render, screen, waitFor } from '@solidjs/testing-library';
import { afterEach, describe, expect, it, vi } from 'vitest';

const operatorStateMock = vi.hoisted(() => ({
  get: vi.fn(),
  set: vi.fn(),
}));

vi.mock('@/api/resourceOperatorState', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@/api/resourceOperatorState')>();
  return {
    ...actual,
    getResourceOperatorState: operatorStateMock.get,
    setResourceOperatorState: operatorStateMock.set,
  };
});

vi.mock('@/stores/notifications', () => ({
  notificationStore: { success: vi.fn(), error: vi.fn() },
}));

import { ResourceMonitoringPolicyAction } from '../ResourceMonitoringPolicyAction';

describe('ResourceMonitoringPolicyAction', () => {
  afterEach(() => {
    cleanup();
    vi.clearAllMocks();
  });

  it('persists expected-offline policy through the canonical resource state API', async () => {
    operatorStateMock.get.mockResolvedValue(null);
    operatorStateMock.set.mockResolvedValue({});
    render(() => (
      <ResourceMonitoringPolicyAction
        resourceId="vm:101"
        resourceName="legacy-lxc"
        resourceType="system-container"
      />
    ));

    fireEvent.click(screen.getByText('Monitoring'));
    expect(screen.getByText(/Proxmox remains the inventory owner/)).toBeInTheDocument();
    fireEvent.click(screen.getByText('Expected offline'));

    await waitFor(() => {
      expect(operatorStateMock.set).toHaveBeenCalledWith(
        'vm:101',
        expect.objectContaining({
          monitoringMode: 'expected_offline',
          lifecycleState: 'active',
          intentionallyOffline: true,
        }),
      );
    });
  });

  it('renders the policy menu through a body portal so scroll containers cannot clip it', () => {
    operatorStateMock.get.mockResolvedValue(null);
    render(() => (
      <ResourceMonitoringPolicyAction
        resourceId="vm:101"
        resourceName="legacy-lxc"
        resourceType="system-container"
      />
    ));

    const trigger = screen.getByText('Monitoring');
    fireEvent.click(trigger);
    const menu = screen.getByRole('menu');
    expect(trigger.parentElement?.contains(menu)).toBe(false);
    expect(document.body.contains(menu)).toBe(true);
    expect(menu.className).toContain('fixed');
  });

  it('retires provider inventory without changing its previous monitoring mode', async () => {
    operatorStateMock.get.mockResolvedValue({
      canonicalId: 'vm:101',
      monitoringMode: 'expected_offline',
      lifecycleState: 'active',
      intentionallyOffline: true,
      neverAutoRemediate: false,
      autoRemediationPolicy: { enabled: false, capabilityNames: [] },
      setAt: '2026-08-11T09:00:00Z',
    });
    operatorStateMock.set.mockResolvedValue({});
    render(() => (
      <ResourceMonitoringPolicyAction
        resourceId="vm:101"
        resourceName="legacy-lxc"
        resourceType="system-container"
      />
    ));

    fireEvent.click(screen.getByText('Monitoring'));
    fireEvent.click(screen.getByText('Retire from monitoring'));

    await waitFor(() => {
      expect(operatorStateMock.set).toHaveBeenCalledWith(
        'vm:101',
        expect.objectContaining({
          monitoringMode: 'expected_offline',
          lifecycleState: 'retired',
        }),
      );
    });
  });
});
