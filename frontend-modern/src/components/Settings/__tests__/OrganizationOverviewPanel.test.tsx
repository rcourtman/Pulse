import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { cleanup, render, screen, waitFor } from '@solidjs/testing-library';
import { OrganizationOverviewPanel } from '../OrganizationOverviewPanel';
import organizationOverviewStateSource from '../useOrganizationOverviewPanelState.ts?raw';

const orgGetMock = vi.fn();
const listMembersMock = vi.fn();
const updateOrgMock = vi.fn();
const isMultiTenantEnabledMock = vi.fn();
const getOrgIDMock = vi.fn();
const notificationSuccessMock = vi.fn();
const notificationErrorMock = vi.fn();
const eventBusOnMock = vi.fn();
const loggerErrorMock = vi.fn();

vi.mock('@/api/orgs', () => ({
  OrgsAPI: {
    get: (...args: unknown[]) => orgGetMock(...args),
    listMembers: (...args: unknown[]) => listMembersMock(...args),
    update: (...args: unknown[]) => updateOrgMock(...args),
  },
}));

vi.mock('@/stores/license', () => ({
  isMultiTenantEnabled: (...args: unknown[]) => isMultiTenantEnabledMock(...args),
}));

vi.mock('@/utils/apiClient', () => ({
  getOrgID: (...args: unknown[]) => getOrgIDMock(...args),
}));

vi.mock('@/stores/notifications', () => ({
  notificationStore: {
    success: (...args: unknown[]) => notificationSuccessMock(...args),
    error: (...args: unknown[]) => notificationErrorMock(...args),
  },
}));

vi.mock('@/stores/events', () => ({
  eventBus: {
    on: (...args: unknown[]) => eventBusOnMock(...args),
  },
}));

vi.mock('@/utils/logger', () => ({
  logger: {
    error: (...args: unknown[]) => loggerErrorMock(...args),
  },
}));

const baseOrg = {
  id: 'org-a',
  displayName: 'Organization A',
  ownerUserId: 'user-1',
  createdAt: '2024-01-10T00:00:00Z',
};

const baseMembers = [
  {
    userId: 'user-1',
    role: 'owner',
    addedAt: '2024-01-10T00:00:00Z',
  },
  {
    userId: 'user-2',
    role: 'viewer',
    addedAt: '2024-01-11T00:00:00Z',
  },
];

const renderPanel = () => render(() => <OrganizationOverviewPanel currentUser="user-1" />);

const deferred = <T,>() => {
  let resolve!: (value: T) => void;
  let reject!: (reason?: unknown) => void;
  const promise = new Promise<T>((res, rej) => {
    resolve = res;
    reject = rej;
  });
  return { promise, resolve, reject };
};

beforeEach(() => {
  orgGetMock.mockReset();
  listMembersMock.mockReset();
  updateOrgMock.mockReset();
  isMultiTenantEnabledMock.mockReset();
  getOrgIDMock.mockReset();
  notificationSuccessMock.mockReset();
  notificationErrorMock.mockReset();
  eventBusOnMock.mockReset();
  loggerErrorMock.mockReset();

  isMultiTenantEnabledMock.mockReturnValue(true);
  getOrgIDMock.mockReturnValue('org-a');
  eventBusOnMock.mockReturnValue(() => undefined);
  orgGetMock.mockResolvedValue(baseOrg);
  listMembersMock.mockResolvedValue(baseMembers);
  updateOrgMock.mockResolvedValue({ ...baseOrg, displayName: 'Updated Org' });
});

afterEach(() => {
  cleanup();
});

describe('OrganizationOverviewPanel', () => {
  it('renders loading skeleton first, then loaded content', async () => {
    const orgDeferred = deferred<typeof baseOrg>();
    orgGetMock.mockReturnValueOnce(orgDeferred.promise);

    const { container } = renderPanel();

    expect(container.querySelector('.animate-pulse')).toBeTruthy();
    expect(screen.queryByLabelText('Display Name')).not.toBeInTheDocument();

    orgDeferred.resolve(baseOrg);

    await waitFor(() => {
      expect(screen.getByLabelText('Display Name')).toBeInTheDocument();
    });
  });

  it('normalizes org scope through the shared helper', () => {
    expect(organizationOverviewStateSource).toContain('normalizeOrgScope(getOrgID())');
    expect(organizationOverviewStateSource).not.toContain("getOrgID() || 'default'");
  });
});
