import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, screen, waitFor, within } from '@solidjs/testing-library';

import { BillingAdminPanel } from '../BillingAdminPanel';

const isMobileMock = vi.fn();
const listOrganizationsMock = vi.fn();
const getBillingStateMock = vi.fn();
const putBillingStateMock = vi.fn();
const successMock = vi.fn();
const errorMock = vi.fn();
const presentationPolicyHidesOrganizationSurfacesMock = vi.fn();

vi.mock('@/hooks/useBreakpoint', () => ({
  useBreakpoint: () => ({ isMobile: () => isMobileMock() }),
}));

vi.mock('@/api/billingAdmin', () => ({
  BillingAdminAPI: {
    listOrganizations: (...args: unknown[]) => listOrganizationsMock(...args),
    getBillingState: (...args: unknown[]) => getBillingStateMock(...args),
    putBillingState: (...args: unknown[]) => putBillingStateMock(...args),
  },
}));

vi.mock('@/stores/license', () => ({
  isHostedModeEnabled: () => true,
  isMultiTenantEnabled: () => true,
}));

vi.mock('@/stores/notifications', () => ({
  notificationStore: {
    success: (...args: unknown[]) => successMock(...args),
    error: (...args: unknown[]) => errorMock(...args),
  },
}));

vi.mock('@/stores/sessionPresentationPolicy', () => ({
  presentationPolicyIsReadOnly: () => false,
  presentationPolicyHidesOrganizationSurfaces: (...args: unknown[]) =>
    presentationPolicyHidesOrganizationSurfacesMock(...args),
}));

vi.mock('@/utils/logger', () => ({
  logger: {
    error: vi.fn(),
  },
}));

describe('BillingAdminPanel', () => {
  beforeEach(() => {
    isMobileMock.mockReset().mockReturnValue(false);
    listOrganizationsMock.mockReset();
    getBillingStateMock.mockReset();
    putBillingStateMock.mockReset();
    successMock.mockReset();
    errorMock.mockReset();
    presentationPolicyHidesOrganizationSurfacesMock.mockReset();
    presentationPolicyHidesOrganizationSurfacesMock.mockReturnValue(false);

    listOrganizationsMock.mockResolvedValue([
      {
        org_id: 'org-1',
        display_name: 'Org One',
        owner_user_id: 'user-1',
        created_at: '2024-01-01T00:00:00Z',
        suspended: false,
        soft_deleted: false,
      },
    ]);
  });

  afterEach(() => {
    cleanup();
  });

  it.each([
    { suspended: true, soft_deleted: false, badge: 'suspended' },
    { suspended: true, soft_deleted: true, badge: 'soft-deleted' },
  ])('retains the $badge inventory badge in the details name', async (status) => {
    listOrganizationsMock.mockResolvedValue([
      {
        org_id: 'org-1',
        display_name: 'Org One',
        owner_user_id: 'user-1',
        created_at: '2024-01-01T00:00:00Z',
        suspended: status.suspended,
        soft_deleted: status.soft_deleted,
      },
    ]);
    getBillingStateMock.mockResolvedValue({
      capabilities: [],
      limits: {},
      subscription_state: 'suspended',
    });
    render(() => <BillingAdminPanel />);
    const name = `Billing details for Org One (org-1), ${status.badge}`;
    const details = await screen.findByRole('button', { name });
    expect(details).toHaveTextContent(status.badge);
    await fireEvent.click(details);
    expect(screen.getByRole('region', { name })).toHaveAttribute(
      'id',
      details.getAttribute('aria-controls'),
    );
  });

  it.each(['', 'org-1'])('uses the exact ID once when the display name is %j', async (name) => {
    listOrganizationsMock.mockResolvedValue([
      {
        org_id: 'org-1',
        display_name: name,
        owner_user_id: 'user-1',
        created_at: '2024-01-01T00:00:00Z',
        suspended: false,
        soft_deleted: false,
      },
    ]);
    getBillingStateMock.mockResolvedValue({
      capabilities: [],
      limits: {},
      subscription_state: 'active',
    });
    render(() => <BillingAdminPanel />);
    const suspend = await screen.findByRole('button', { name: 'Suspend Org for org-1' });
    await waitFor(() => expect(suspend).toBeEnabled());
    expect(screen.getByRole('button', { name: 'Activate Org for org-1' })).toBeDisabled();
    await fireEvent.click(screen.getByRole('button', { name: 'Billing details for org-1' }));
    expect(screen.getByRole('region', { name: 'Billing details for org-1' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Reload billing for org-1' })).toBeEnabled();
    expect(putBillingStateMock).not.toHaveBeenCalled();
  });

  it('does not fabricate plan_version from subscription state when billing state has no plan', async () => {
    const stateWithoutPlan = {
      capabilities: ['multi_tenant'],
      limits: { max_monitored_systems: 10 },
      meters_enabled: [],
      subscription_state: 'active',
      stripe_customer_id: 'cus_123',
    };
    getBillingStateMock.mockResolvedValue(stateWithoutPlan);
    putBillingStateMock.mockImplementation(
      async (_orgID: string, payload: Record<string, unknown>) => payload,
    );

    render(() => <BillingAdminPanel />);

    await waitFor(() => {
      expect(screen.getByText('Org One')).toBeInTheDocument();
    });

    await fireEvent.click(screen.getByRole('button', { name: 'Suspend Org for Org One (org-1)' }));
    const dialog = screen.getByRole('dialog', { name: 'Suspend organization' });
    await fireEvent.click(within(dialog).getByRole('button', { name: 'Suspend organization' }));

    await waitFor(() => {
      expect(putBillingStateMock).toHaveBeenCalledTimes(1);
    });

    const payload = putBillingStateMock.mock.calls[0][1] as Record<string, unknown>;
    expect(payload.subscription_state).toBe('suspended');
    expect(payload).not.toHaveProperty('plan_version');
  });

  it('preserves the existing canonical plan_version when updating subscription state', async () => {
    const stateWithPlan = {
      capabilities: ['multi_tenant'],
      limits: { max_monitored_systems: 30 },
      meters_enabled: [],
      subscription_state: 'active',
      plan_version: 'cloud_power',
    };
    getBillingStateMock.mockResolvedValue(stateWithPlan);
    putBillingStateMock.mockImplementation(
      async (_orgID: string, payload: Record<string, unknown>) => payload,
    );

    render(() => <BillingAdminPanel />);

    await waitFor(() => {
      expect(screen.getByText('Org One')).toBeInTheDocument();
    });

    await fireEvent.click(screen.getByRole('button', { name: 'Suspend Org for Org One (org-1)' }));
    const dialog = screen.getByRole('dialog', { name: 'Suspend organization' });
    await fireEvent.click(within(dialog).getByRole('button', { name: 'Suspend organization' }));

    await waitFor(() => {
      expect(putBillingStateMock).toHaveBeenCalledTimes(1);
    });

    const payload = putBillingStateMock.mock.calls[0][1] as Record<string, unknown>;
    expect(payload.subscription_state).toBe('suspended');
    expect(payload.plan_version).toBe('cloud_power');
  });

  it('keeps organization active when suspension dialog is cancelled', async () => {
    getBillingStateMock.mockResolvedValue({
      capabilities: ['multi_tenant'],
      limits: {},
      subscription_state: 'active',
    });

    render(() => <BillingAdminPanel />);

    const suspendButton = await screen.findByRole('button', {
      name: 'Suspend Org for Org One (org-1)',
    });
    await waitFor(() => expect(suspendButton).toBeEnabled());
    await fireEvent.click(suspendButton);
    const dialog = screen.getByRole('dialog', { name: 'Suspend organization' });
    await fireEvent.click(within(dialog).getByRole('button', { name: 'Cancel' }));

    expect(screen.queryByRole('dialog', { name: 'Suspend organization' })).not.toBeInTheDocument();
    expect(putBillingStateMock).not.toHaveBeenCalled();
  });

  it('stays unavailable in demo mode without loading hosted billing admin data', () => {
    presentationPolicyHidesOrganizationSurfacesMock.mockReturnValue(true);

    render(() => <BillingAdminPanel />);

    expect(
      screen.getByText('Organization settings are not available on this server.'),
    ).toBeInTheDocument();
    expect(listOrganizationsMock).not.toHaveBeenCalled();
    expect(getBillingStateMock).not.toHaveBeenCalled();
  });
  it('keeps one set of mobile subscription actions beside their confirmed billing state', async () => {
    isMobileMock.mockReturnValue(true);
    getBillingStateMock.mockResolvedValue({
      capabilities: [],
      limits: {},
      meters_enabled: [],
      subscription_state: 'active',
    });
    putBillingStateMock.mockImplementation(async (_orgID, payload) => payload);
    render(() => <BillingAdminPanel />);
    await waitFor(() =>
      expect(screen.getByRole('button', { name: 'Suspend Org for Org One (org-1)' })).toBeEnabled(),
    );
    expect(screen.getAllByRole('button', { name: 'Suspend Org for Org One (org-1)' })).toHaveLength(
      1,
    );
    expect(
      screen.getAllByRole('button', { name: 'Activate Org for Org One (org-1)' }),
    ).toHaveLength(1);
    expect(
      screen.getByRole('button', { name: 'Suspend Org for Org One (org-1)' }).closest('td'),
    ).toHaveTextContent('Active');
    await fireEvent.click(screen.getByRole('button', { name: 'Suspend Org for Org One (org-1)' }));
    const dialog = screen.getByRole('dialog', { name: 'Suspend organization' });
    await fireEvent.click(within(dialog).getByRole('button', { name: 'Suspend organization' }));
    await waitFor(() => expect(putBillingStateMock).toHaveBeenCalledTimes(1));
    expect(putBillingStateMock.mock.calls[0][0]).toBe('org-1');
    expect(putBillingStateMock.mock.calls[0][1].subscription_state).toBe('suspended');
  });
});
