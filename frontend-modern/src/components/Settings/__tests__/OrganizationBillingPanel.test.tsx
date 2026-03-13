import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { cleanup, render, screen, waitFor } from '@solidjs/testing-library';

import { OrganizationBillingPanel } from '../OrganizationBillingPanel';

const getStatusMock = vi.hoisted(() => vi.fn());
const listOrgsMock = vi.hoisted(() => vi.fn());
const listMembersMock = vi.hoisted(() => vi.fn());
const errorMock = vi.hoisted(() => vi.fn());
const eventBusOnMock = vi.hoisted(() => vi.fn(() => () => {}));

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
    on: eventBusOnMock,
  },
}));

vi.mock('@/stores/notifications', () => ({
  notificationStore: {
    error: errorMock,
  },
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
    eventBusOnMock.mockReturnValue(() => {});

    getStatusMock.mockResolvedValue({
      valid: true,
      tier: 'cloud',
      plan_version: 'cloud_power',
      is_lifetime: false,
      days_remaining: 30,
      features: [],
      max_agents: 12,
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

  it('renders organization usage from the canonical max_agents limit only', async () => {
    render(() => <OrganizationBillingPanel nodeUsage={5} guestUsage={2} />);

    await waitFor(() => {
      expect(screen.getByText('5 / 12')).toBeInTheDocument();
    });

    expect(getStatusMock).toHaveBeenCalledTimes(1);
    expect(screen.getByText('Cloud')).toBeInTheDocument();
    expect(screen.getByText('5 / 12')).toBeInTheDocument();
    expect(screen.getByText('2 / 5')).toBeInTheDocument();
    expect(errorMock).not.toHaveBeenCalled();
  });
});
