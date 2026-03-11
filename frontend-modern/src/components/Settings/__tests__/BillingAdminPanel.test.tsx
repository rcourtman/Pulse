import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, screen, waitFor } from '@solidjs/testing-library';

import { BillingAdminPanel } from '../BillingAdminPanel';

const listOrganizationsMock = vi.fn();
const getBillingStateMock = vi.fn();
const putBillingStateMock = vi.fn();
const successMock = vi.fn();
const errorMock = vi.fn();

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

vi.mock('@/utils/logger', () => ({
  logger: {
    error: vi.fn(),
  },
}));

describe('BillingAdminPanel', () => {
  beforeEach(() => {
    listOrganizationsMock.mockReset();
    getBillingStateMock.mockReset();
    putBillingStateMock.mockReset();
    successMock.mockReset();
    errorMock.mockReset();

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

  it('does not fabricate plan_version from subscription state when billing state has no plan', async () => {
    const stateWithoutPlan = {
      capabilities: ['multi_tenant'],
      limits: { max_agents: 10 },
      meters_enabled: [],
      subscription_state: 'active',
      stripe_customer_id: 'cus_123',
    };
    getBillingStateMock.mockResolvedValue(stateWithoutPlan);
    putBillingStateMock.mockImplementation(async (_orgID: string, payload: Record<string, unknown>) => payload);

    render(() => <BillingAdminPanel />);

    await waitFor(() => {
      expect(screen.getByText('Org One')).toBeInTheDocument();
    });

    await fireEvent.click(screen.getByRole('button', { name: 'Suspend Org' }));

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
      limits: { max_agents: 30 },
      meters_enabled: [],
      subscription_state: 'active',
      plan_version: 'cloud_power',
    };
    getBillingStateMock.mockResolvedValue(stateWithPlan);
    putBillingStateMock.mockImplementation(async (_orgID: string, payload: Record<string, unknown>) => payload);

    render(() => <BillingAdminPanel />);

    await waitFor(() => {
      expect(screen.getByText('Org One')).toBeInTheDocument();
    });

    await fireEvent.click(screen.getByRole('button', { name: 'Suspend Org' }));

    await waitFor(() => {
      expect(putBillingStateMock).toHaveBeenCalledTimes(1);
    });

    const payload = putBillingStateMock.mock.calls[0][1] as Record<string, unknown>;
    expect(payload.subscription_state).toBe('suspended');
    expect(payload.plan_version).toBe('cloud_power');
  });
});
