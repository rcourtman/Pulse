import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, screen, waitFor } from '@solidjs/testing-library';
import type { Resource } from '@/types/resource';
import { OrganizationSharingPanel } from '../OrganizationSharingPanel';

let mockResources: Resource[] = [];

const orgGetMock = vi.fn();
const listMembersMock = vi.fn();
const listOrgsMock = vi.fn();
const listSharesMock = vi.fn();
const listIncomingSharesMock = vi.fn();
const createShareMock = vi.fn();
const deleteShareMock = vi.fn();
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
    list: (...args: unknown[]) => listOrgsMock(...args),
    listShares: (...args: unknown[]) => listSharesMock(...args),
    listIncomingShares: (...args: unknown[]) => listIncomingSharesMock(...args),
    createShare: (...args: unknown[]) => createShareMock(...args),
    deleteShare: (...args: unknown[]) => deleteShareMock(...args),
  },
}));

vi.mock('@/hooks/useResources', () => ({
  useResources: () => ({
    resources: () => mockResources,
  }),
  getDisplayName: (resource: { displayName?: string; name?: string; id: string }) =>
    resource.displayName || resource.name || resource.id,
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

const makeResource = (overrides: Partial<Resource> = {}): Resource => ({
  id: 'vm-100',
  type: 'vm',
  name: 'Alpha VM',
  displayName: 'Alpha VM',
  platformId: 'pve-1',
  platformType: 'proxmox-pve',
  sourceType: 'api',
  status: 'running',
  lastSeen: Date.now(),
  ...overrides,
});

const baseOrg = {
  id: 'org-a',
  displayName: 'Organization A',
  ownerUserId: 'user-1',
};

const targetOrg = {
  id: 'org-b',
  displayName: 'Organization B',
  ownerUserId: 'user-2',
};

const renderPanel = () =>
  render(() => <OrganizationSharingPanel currentUser="user-1" />);

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
  listOrgsMock.mockReset();
  listSharesMock.mockReset();
  listIncomingSharesMock.mockReset();
  createShareMock.mockReset();
  deleteShareMock.mockReset();
  isMultiTenantEnabledMock.mockReset();
  getOrgIDMock.mockReset();
  notificationSuccessMock.mockReset();
  notificationErrorMock.mockReset();
  eventBusOnMock.mockReset();
  loggerErrorMock.mockReset();

  mockResources = [
    makeResource({ id: 'vm-200', name: 'Zulu VM', displayName: 'Zulu VM' }),
    makeResource({ id: 'vm-100', name: 'Alpha VM', displayName: '' }),
    makeResource({ id: 'host-1', type: 'host', name: '', displayName: '' }),
    makeResource({ id: '', type: 'vm', name: 'Hidden', displayName: 'Hidden' }),
  ];

  isMultiTenantEnabledMock.mockReturnValue(true);
  getOrgIDMock.mockReturnValue('org-a');
  eventBusOnMock.mockReturnValue(() => undefined);

  orgGetMock.mockResolvedValue(baseOrg);
  listMembersMock.mockResolvedValue([]);
  listOrgsMock.mockResolvedValue([baseOrg, targetOrg]);
  listSharesMock.mockResolvedValue([]);
  listIncomingSharesMock.mockResolvedValue([]);
  createShareMock.mockResolvedValue({
    id: 'share-1',
    targetOrgId: 'org-b',
    resourceType: 'vm',
    resourceId: 'vm-100',
    resourceName: 'Alpha VM',
    accessRole: 'viewer',
    createdAt: new Date().toISOString(),
    createdBy: 'user-1',
  });
  deleteShareMock.mockResolvedValue(undefined);
});

afterEach(() => {
  cleanup();
});

describe('OrganizationSharingPanel', () => {
  it('renders loading skeleton first, then loaded content', async () => {
    const orgDeferred = deferred<typeof baseOrg>();
    orgGetMock.mockReturnValueOnce(orgDeferred.promise);

    const { container } = renderPanel();

    expect(container.querySelector('.animate-pulse')).toBeTruthy();
    expect(screen.queryByRole('heading', { name: 'Create Share' })).not.toBeInTheDocument();

    orgDeferred.resolve(baseOrg);

    await waitFor(() => {
      expect(screen.getByRole('heading', { name: 'Create Share' })).toBeInTheDocument();
    });
  });

  it('derives quick-pick options from unified resources', async () => {
    renderPanel();

    await waitFor(() => {
      expect(screen.getByRole('heading', { name: 'Create Share' })).toBeInTheDocument();
    });

    const quickPick = screen.getByLabelText('Quick Pick Resource') as HTMLSelectElement;
    const labels = Array.from(quickPick.options).map((option) => option.textContent?.trim());

    expect(labels).toEqual([
      'Select resource',
      'Alpha VM (vm)',
      'host-1 (host)',
      'Zulu VM (vm)',
    ]);
    expect(labels).not.toContain('Hidden (vm)');
  });

  it('populates resource fields from quick-pick selection', async () => {
    renderPanel();

    await waitFor(() => {
      expect(screen.getByRole('heading', { name: 'Create Share' })).toBeInTheDocument();
    });

    fireEvent.click(screen.getByRole('button', { name: 'Enter manually' }));

    const quickPick = screen.getByLabelText('Quick Pick Resource');
    fireEvent.change(quickPick, { target: { value: 'vm::vm-100' } });

    expect(screen.getByLabelText('Resource Type')).toHaveValue('vm');
    expect(screen.getByLabelText('Resource ID')).toHaveValue('vm-100');
    expect(screen.getByLabelText('Resource Name')).toHaveValue('Alpha VM');
  });

  it('validates manual resource type and clears the error for valid values', async () => {
    renderPanel();

    await waitFor(() => {
      expect(screen.getByRole('heading', { name: 'Create Share' })).toBeInTheDocument();
    });

    fireEvent.click(screen.getByRole('button', { name: 'Enter manually' }));

    const resourceTypeInput = screen.getByLabelText('Resource Type');
    fireEvent.input(resourceTypeInput, { target: { value: 'invalid-type' } });

    expect(
      screen.getByText(
        'Invalid resource type. Valid types: vm, container, host, storage, pbs, pmg',
      ),
    ).toBeInTheDocument();

    fireEvent.input(resourceTypeInput, { target: { value: 'vm' } });

    await waitFor(() => {
      expect(
        screen.queryByText(
          'Invalid resource type. Valid types: vm, container, host, storage, pbs, pmg',
        ),
      ).not.toBeInTheDocument();
    });
  });

  it('creates a share with the expected payload', async () => {
    renderPanel();

    await waitFor(() => {
      expect(screen.getByRole('heading', { name: 'Create Share' })).toBeInTheDocument();
    });

    fireEvent.change(screen.getByLabelText('Quick Pick Resource'), {
      target: { value: 'vm::vm-100' },
    });

    fireEvent.click(screen.getByRole('button', { name: 'Create Share' }));

    await waitFor(() => {
      expect(createShareMock).toHaveBeenCalledWith('org-a', {
        targetOrgId: 'org-b',
        resourceType: 'vm',
        resourceId: 'vm-100',
        resourceName: 'Alpha VM',
        accessRole: 'viewer',
      });
    });
  });
});
