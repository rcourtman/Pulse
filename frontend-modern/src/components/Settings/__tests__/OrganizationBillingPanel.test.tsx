import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { cleanup, render, screen, waitFor } from '@solidjs/testing-library';

import { OrganizationBillingPanel } from '../OrganizationBillingPanel';
import organizationBillingPanelSource from '../OrganizationBillingPanel.tsx?raw';
import organizationBillingLoadingStateSource from '../OrganizationBillingLoadingState.tsx?raw';
import organizationBillingStateSource from '../useOrganizationBillingPanelState.ts?raw';

const getStatusMock = vi.hoisted(() => vi.fn());
const listOrgsMock = vi.hoisted(() => vi.fn());
const listMembersMock = vi.hoisted(() => vi.fn());
const errorMock = vi.hoisted(() => vi.fn());
const eventBusOnMock = vi.hoisted(() => vi.fn());
const eventBusHandlers = vi.hoisted(() => [] as Array<() => void>);
const presentationPolicyHidesOrganizationSurfacesMock = vi.hoisted(() => vi.fn());

vi.mock('@/api/license', () => ({
  LicenseAPI: {
    getStatus: getStatusMock,
  },
}));

vi.mock('@/api/orgs', () => ({
  OrgsAPI: {
    list: listOrgsMock,
    listMembers: listMembersMock,
  },
}));

vi.mock('@/utils/apiClient', () => ({
  getOrgID: () => 'org-1',
}));

vi.mock('@/stores/license', () => ({
  isMultiTenantEnabled: () => true,
}));

vi.mock('@/stores/events', () => ({
  eventBus: {
    on: eventBusOnMock as unknown as (event: string, handler: () => void) => () => void,
  },
}));

vi.mock('@/stores/notifications', () => ({
  notificationStore: {
    error: errorMock,
  },
}));

vi.mock('@/stores/sessionPresentationPolicy', () => ({
  presentationPolicyHidesOrganizationSurfaces: presentationPolicyHidesOrganizationSurfacesMock,
}));

vi.mock('@/utils/logger', () => ({
  logger: {
    error: vi.fn(),
  },
}));

describe('OrganizationBillingPanel', () => {
  beforeEach(() => {
    getStatusMock.mockReset();
    listOrgsMock.mockReset();
    listMembersMock.mockReset();
    errorMock.mockReset();
    eventBusOnMock.mockReset();
    presentationPolicyHidesOrganizationSurfacesMock.mockReset();
    eventBusHandlers.length = 0;
    eventBusOnMock.mockImplementation((_event: string, handler: () => void) => {
      eventBusHandlers.push(handler);
      return () => {};
    });
    presentationPolicyHidesOrganizationSurfacesMock.mockReturnValue(false);

    getStatusMock.mockResolvedValue({
      valid: true,
      tier: 'cloud',
      plan_version: 'cloud_power',
      is_lifetime: false,
      days_remaining: 30,
      features: [],
      max_monitored_systems: 12,
      max_guests: 5,
      email: 'owner@example.com',
      expires_at: '2026-04-01T00:00:00Z',
    });
    listOrgsMock.mockResolvedValue([{ id: 'org-1' }, { id: 'org-2' }]);
    listMembersMock.mockResolvedValue([{ id: 'user-1' }, { id: 'user-2' }, { id: 'user-3' }]);
  });

  afterEach(() => {
    cleanup();
  });

  it('renders organization usage from the canonical max_monitored_systems limit only', async () => {
    render(() => <OrganizationBillingPanel nodeUsage={5} guestUsage={2} />);

    await waitFor(() => {
      expect(screen.getByText('5 / 12')).toBeInTheDocument();
    });

    expect(getStatusMock).toHaveBeenCalledTimes(1);
    expect(screen.getByText('Plan')).toBeInTheDocument();
    expect(screen.getByText('Usage')).toBeInTheDocument();
    expect(screen.getByText('Cloud')).toBeInTheDocument();
    expect(screen.getByText('5 / 12')).toBeInTheDocument();
    expect(screen.getByText('2 / 5')).toBeInTheDocument();
    expect(errorMock).not.toHaveBeenCalled();
  });

  it('reloads billing data when the active organization changes', async () => {
    render(() => <OrganizationBillingPanel nodeUsage={5} guestUsage={2} />);

    await waitFor(() => {
      expect(getStatusMock).toHaveBeenCalledTimes(1);
    });

    eventBusHandlers[0]?.();

    await waitFor(() => {
      expect(getStatusMock).toHaveBeenCalledTimes(2);
    });
    expect(listOrgsMock).toHaveBeenCalledTimes(2);
    expect(listMembersMock).toHaveBeenCalledTimes(2);
  });

  it('keeps organization billing runtime split into dedicated state and loading owners', () => {
    expect(organizationBillingPanelSource).toContain('./useOrganizationBillingPanelState');
    expect(organizationBillingPanelSource).toContain('./OrganizationBillingLoadingState');
    expect(organizationBillingPanelSource).not.toContain('createSignal(');
    expect(organizationBillingPanelSource).not.toContain('onMount(() =>');
    expect(organizationBillingStateSource).toContain('normalizeOrgScope(getOrgID())');
    expect(organizationBillingStateSource).toContain("eventBus.on('org_switched'");
    expect(organizationBillingStateSource).not.toContain("getOrgID() || 'default'");
    expect(organizationBillingLoadingStateSource).toContain('animate-pulse');
  });

  it('stays unavailable in demo mode without loading organization billing data', () => {
    presentationPolicyHidesOrganizationSurfacesMock.mockReturnValue(true);

    render(() => <OrganizationBillingPanel nodeUsage={5} guestUsage={2} />);

    expect(
      screen.getByText('Organization settings are not available on this server.'),
    ).toBeInTheDocument();
    expect(getStatusMock).not.toHaveBeenCalled();
    expect(listOrgsMock).not.toHaveBeenCalled();
    expect(listMembersMock).not.toHaveBeenCalled();
  });
});
