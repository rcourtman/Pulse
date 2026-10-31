import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, screen, waitFor } from '@solidjs/testing-library';
import { createSignal } from 'solid-js';
import { syncSessionPresentationPolicy } from '@/stores/sessionPresentationPolicy';
import type { Resource } from '@/types/resource';
import { useOrganizationSharingPanelState } from '../useOrganizationSharingPanelState';
import { OrganizationSharingPanel } from '../OrganizationSharingPanel';
import organizationSharingStateSource from '../useOrganizationSharingPanelState.ts?raw';

let mockResources: Resource[] = [];
let readResources = () => mockResources;

const orgGetMock = vi.fn();
const listMembersMock = vi.fn();
const listOrgsMock = vi.fn();
const listSharesMock = vi.fn();
const listIncomingSharesMock = vi.fn();
const acceptIncomingShareMock = vi.fn();
const declineIncomingShareMock = vi.fn();
const createShareMock = vi.fn();
const deleteShareMock = vi.fn();
const isMultiTenantEnabledMock = vi.fn();
const getOrgIDMock = vi.fn();
const notificationSuccessMock = vi.fn();
const notificationErrorMock = vi.fn();
const eventBusOnMock = vi.fn();
const eventBusEmitMock = vi.fn();
const listeners = new Map<string, Set<() => void>>();
const emit = (event: string) => listeners.get(event)?.forEach((listener) => listener());
const loggerErrorMock = vi.fn();

vi.mock('@/api/orgs', () => ({
  OrgsAPI: {
    get: (...args: unknown[]) => orgGetMock(...args),
    listMembers: (...args: unknown[]) => listMembersMock(...args),
    list: (...args: unknown[]) => listOrgsMock(...args),
    listShares: (...args: unknown[]) => listSharesMock(...args),
    listIncomingShares: (...args: unknown[]) => listIncomingSharesMock(...args),
    acceptIncomingShare: (...args: unknown[]) => acceptIncomingShareMock(...args),
    declineIncomingShare: (...args: unknown[]) => declineIncomingShareMock(...args),
    createShare: (...args: unknown[]) => createShareMock(...args),
    deleteShare: (...args: unknown[]) => deleteShareMock(...args),
  },
}));

vi.mock('@/hooks/useResources', () => ({
  useResources: () => ({
    resources: () => readResources(),
  }),
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
    emit: (...args: unknown[]) => eventBusEmitMock(...args),
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

const renderPanel = () => render(() => <OrganizationSharingPanel currentUser="user-1" />);

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
  acceptIncomingShareMock.mockReset();
  declineIncomingShareMock.mockReset();
  createShareMock.mockReset();
  deleteShareMock.mockReset();
  isMultiTenantEnabledMock.mockReset();
  getOrgIDMock.mockReset();
  notificationSuccessMock.mockReset();
  notificationErrorMock.mockReset();
  eventBusOnMock.mockReset();
  eventBusEmitMock.mockReset();
  listeners.clear();
  syncSessionPresentationPolicy(null);
  loggerErrorMock.mockReset();

  readResources = () => mockResources;
  mockResources = [
    makeResource({ id: 'vm-200', name: 'Zulu VM', displayName: 'Zulu VM' }),
    makeResource({ id: 'vm-100', name: 'Alpha VM', displayName: '' }),
    makeResource({ id: 'host-1', type: 'agent', name: '', displayName: '' }),
    makeResource({ id: '', type: 'vm', name: 'Hidden', displayName: 'Hidden' }),
  ];

  isMultiTenantEnabledMock.mockReturnValue(true);
  getOrgIDMock.mockReturnValue('org-a');
  eventBusOnMock.mockImplementation((event: string, callback: () => void) => {
    if (!listeners.has(event)) listeners.set(event, new Set());
    listeners.get(event)!.add(callback);
    return () => listeners.get(event)?.delete(callback);
  });
  eventBusEmitMock.mockImplementation(emit);
  vi.stubGlobal(
    'confirm',
    vi.fn(() => true),
  );

  orgGetMock.mockResolvedValue(baseOrg);
  listMembersMock.mockResolvedValue([]);
  listOrgsMock.mockResolvedValue([baseOrg, targetOrg]);
  listSharesMock.mockResolvedValue([]);
  listIncomingSharesMock.mockResolvedValue([]);
  acceptIncomingShareMock.mockResolvedValue({
    id: 'incoming-share-1',
    status: 'accepted',
  });
  declineIncomingShareMock.mockResolvedValue(undefined);
  createShareMock.mockResolvedValue({
    id: 'share-1',
    targetOrgId: 'org-b',
    resourceType: 'vm',
    resourceId: 'vm-100',
    resourceName: 'Alpha VM',
    accessRole: 'viewer',
    status: 'pending',
    createdAt: new Date().toISOString(),
    createdBy: 'user-1',
  });
  deleteShareMock.mockResolvedValue(undefined);
});

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
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

    expect(labels).toEqual(['Select resource', 'Alpha VM (vm)', 'pve-1 (agent)', 'Zulu VM (vm)']);
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

    const invalidTypeError = screen.getByText(/Invalid resource type\. Valid types:/);
    expect(invalidTypeError).toBeInTheDocument();
    expect(invalidTypeError.textContent || '').toContain('system-container');
    expect(invalidTypeError.textContent || '').toContain('app-container');
    expect(invalidTypeError.textContent || '').not.toContain(', container,');

    fireEvent.input(resourceTypeInput, { target: { value: 'vm' } });

    await waitFor(() => {
      expect(screen.queryByText(/Invalid resource type\. Valid types:/)).not.toBeInTheDocument();
    });
  });

  it('rejects legacy host alias when creating shares manually', async () => {
    renderPanel();

    await waitFor(() => {
      expect(screen.getByRole('heading', { name: 'Create Share' })).toBeInTheDocument();
    });

    fireEvent.click(screen.getByRole('button', { name: 'Enter manually' }));

    fireEvent.input(screen.getByLabelText('Resource Type'), { target: { value: 'host' } });
    fireEvent.input(screen.getByLabelText('Resource ID'), { target: { value: 'agent-123' } });

    fireEvent.click(screen.getByRole('button', { name: 'Create Share' }));

    await waitFor(() => {
      expect(screen.getByText(/Invalid resource type\. Valid types:/)).toBeInTheDocument();
    });
    expect(createShareMock).not.toHaveBeenCalled();
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
      expect(createShareMock).toHaveBeenCalledWith(
        'org-a',
        {
          targetOrgId: 'org-b',
          resourceType: 'vm',
          resourceId: 'vm-100',
          resourceName: 'Alpha VM',
          accessRole: 'viewer',
        },
        expect.objectContaining({
          expectedOrgID: 'org-a',
          retry: false,
          signal: expect.any(AbortSignal),
        }),
      );
    });
  });

  it('uses the governed resource label in quick-pick share payloads', async () => {
    mockResources = [
      makeResource({
        id: 'vm-200',
        name: 'secret-vm-200',
        displayName: 'secret-vm-200',
        policy: {
          sensitivity: 'restricted',
          routing: { scope: 'local-only', redact: ['hostname'] },
        },
        aiSafeSummary: 'Production VM',
      }),
    ];

    renderPanel();

    await waitFor(() => {
      expect(screen.getByRole('heading', { name: 'Create Share' })).toBeInTheDocument();
    });

    fireEvent.change(screen.getByLabelText('Quick Pick Resource'), {
      target: { value: 'vm::vm-200' },
    });

    fireEvent.click(screen.getByRole('button', { name: 'Create Share' }));

    await waitFor(() => {
      expect(createShareMock).toHaveBeenCalledWith(
        'org-a',
        {
          targetOrgId: 'org-b',
          resourceType: 'vm',
          resourceId: 'vm-200',
          resourceName: 'Production VM',
          accessRole: 'viewer',
        },
        expect.objectContaining({
          expectedOrgID: 'org-a',
          retry: false,
          signal: expect.any(AbortSignal),
        }),
      );
    });
  });

  it('renders share status and lets target managers accept or decline incoming shares', async () => {
    listSharesMock.mockResolvedValue([
      {
        id: 'share-out-1',
        targetOrgId: 'org-b',
        resourceType: 'vm',
        resourceId: 'vm-100',
        resourceName: 'Alpha VM',
        accessRole: 'viewer',
        status: 'pending',
        createdAt: '2026-04-20T10:00:00Z',
        createdBy: 'user-1',
      },
    ]);
    listIncomingSharesMock.mockResolvedValue([
      {
        id: 'share-in-1',
        sourceOrgId: 'org-b',
        sourceOrgName: 'Organization B',
        targetOrgId: 'org-a',
        resourceType: 'view',
        resourceId: 'alerts-high-cpu',
        resourceName: 'High CPU',
        accessRole: 'viewer',
        status: 'pending',
        createdAt: '2026-04-20T10:00:00Z',
        createdBy: 'user-2',
      },
    ]);

    renderPanel();

    await waitFor(() => {
      expect(screen.getAllByText('Pending approval').length).toBeGreaterThanOrEqual(2);
    });

    fireEvent.click(screen.getByRole('button', { name: 'Accept' }));

    await waitFor(() => {
      expect(acceptIncomingShareMock).toHaveBeenCalledWith(
        'org-a',
        'share-in-1',
        expect.objectContaining({ expectedOrgID: 'org-a', retry: false }),
      );
    });

    await waitFor(() => {
      expect((screen.getByRole('button', { name: 'Decline' }) as HTMLButtonElement).disabled).toBe(
        false,
      );
    });

    fireEvent.click(screen.getByRole('button', { name: 'Decline' }));

    await waitFor(() => {
      expect(declineIncomingShareMock).toHaveBeenCalledWith(
        'org-a',
        'share-in-1',
        expect.objectContaining({ expectedOrgID: 'org-a', retry: false }),
      );
    });
  });

  it('normalizes org scope through the shared helper', () => {
    expect(organizationSharingStateSource).toContain('normalizeOrgScope(getOrgID())');
    expect(organizationSharingStateSource).not.toContain("getOrgID() || 'default'");
  });
});

describe('selected sharing snapshot', () => {
  it('does not expose previous-org shares or controls after a failed switch', async () => {
    listSharesMock.mockResolvedValue([
      {
        id: 'alpha-share',
        targetOrgId: 'org-b',
        resourceType: 'vm',
        resourceId: 'vm-alpha',
        resourceName: 'Alpha private workload',
        accessRole: 'viewer',
        status: 'pending',
        createdAt: '2026-10-01T00:00:00Z',
        createdBy: 'user-1',
      },
    ]);
    renderPanel();
    await screen.findByText('Alpha private workload');
    getOrgIDMock.mockReturnValue('org-b');
    orgGetMock.mockRejectedValue(new Error('unavailable'));
    emit('org_switched');
    await waitFor(() =>
      expect(screen.queryByText('Alpha private workload')).not.toBeInTheDocument(),
    );
    expect(screen.queryByRole('button', { name: 'Remove', exact: true })).not.toBeInTheDocument();
    expect(screen.queryByRole('heading', { name: 'Create Share' })).not.toBeInTheDocument();
    expect(await screen.findByRole('button', { name: 'Reload sharing' })).toBeEnabled();
  });

  it('does not let late prior-org shares replace the selected snapshot', async () => {
    const alpha = {
      id: 'alpha-share',
      targetOrgId: 'org-b',
      resourceType: 'vm',
      resourceId: 'vm-alpha',
      resourceName: 'Alpha private workload',
      accessRole: 'viewer',
      status: 'pending',
      createdAt: '2026-10-01T00:00:00Z',
      createdBy: 'user-1',
    };
    const first = deferred<(typeof alpha)[]>();
    listSharesMock.mockReturnValueOnce(first.promise);
    renderPanel();
    await waitFor(() => expect(listSharesMock).toHaveBeenCalledTimes(1));
    getOrgIDMock.mockReturnValue('org-b');
    orgGetMock.mockResolvedValue({ ...targetOrg, ownerUserId: 'user-1' });
    listSharesMock.mockResolvedValue([
      { ...alpha, id: 'beta-share', targetOrgId: 'org-a', resourceName: 'Beta workload' },
    ]);
    emit('org_switched');
    await screen.findByText('Beta workload');
    first.resolve([alpha]);
    await Promise.resolve();
    await Promise.resolve();
    expect(screen.queryByText('Alpha private workload')).not.toBeInTheDocument();
    expect(screen.getByText('Beta workload')).toBeInTheDocument();
  });
});

const outgoing = {
  id: 'outgoing-1',
  targetOrgId: 'org-b',
  resourceType: 'vm',
  resourceId: 'vm-alpha',
  resourceName: 'Alpha shared workload',
  accessRole: 'viewer' as const,
  status: 'pending' as const,
  createdAt: '2026-10-01T00:00:00Z',
  createdBy: 'user-1',
};
const incoming = {
  ...outgoing,
  id: 'incoming-1',
  targetOrgId: 'org-a',
  sourceOrgId: 'org-b',
  sourceOrgName: 'Organization B',
  resourceName: 'Incoming workload',
};
function renderState() {
  let state!: ReturnType<typeof useOrganizationSharingPanelState>;
  const [user, setUser] = createSignal<string | undefined>('user-1');
  const view = render(() => {
    state = useOrganizationSharingPanelState({
      get currentUser() {
        return user();
      },
    });
    return <div />;
  });
  return { state, setUser, ...view };
}
const accepted = async (state: ReturnType<typeof useOrganizationSharingPanelState>) =>
  waitFor(() => {
    expect(state.loading()).toBe(false);
    expect(state.loadError()).toBeNull();
    expect(state.org()).not.toBeNull();
  });

const strictOptions = (org = 'org-a') =>
  expect.objectContaining({
    expectedOrgID: org,
    retry: false,
    signal: expect.any(AbortSignal),
  });

describe('sharing read admission and recovery', () => {
  it.each(['organization', 'members', 'organizations', 'outgoing', 'incoming'])(
    'keeps a failed %s read distinct from empty sharing and permits a deliberate read-only retry',
    async (part) => {
      const mocks = {
        organization: orgGetMock,
        members: listMembersMock,
        organizations: listOrgsMock,
        outgoing: listSharesMock,
        incoming: listIncomingSharesMock,
      };
      mocks[part as keyof typeof mocks].mockRejectedValueOnce(new Error('503'));
      renderPanel();
      expect(await screen.findByRole('button', { name: 'Reload sharing' })).toBeEnabled();
      expect(screen.queryByRole('heading', { name: 'Create Share' })).not.toBeInTheDocument();
      expect(screen.queryByText(/No (incoming|outgoing) shares/)).not.toBeInTheDocument();
      const calls = orgGetMock.mock.calls.length;
      fireEvent.click(screen.getByRole('button', { name: 'Reload sharing' }));
      await screen.findByRole('heading', { name: 'Outgoing Shares' });
      expect(orgGetMock.mock.calls.length).toBe(calls + 1);
      expect(createShareMock).not.toHaveBeenCalled();
      expect(deleteShareMock).not.toHaveBeenCalled();
    },
  );

  it.each(['wrong organization', 'missing shares', 'foreign incoming target'])(
    'does not admit %s as a valid empty snapshot',
    async (problem) => {
      if (problem === 'wrong organization') orgGetMock.mockResolvedValue(targetOrg);
      if (problem === 'missing shares') listSharesMock.mockResolvedValue(null);
      if (problem === 'foreign incoming target')
        listIncomingSharesMock.mockResolvedValue([{ ...incoming, targetOrgId: 'org-foreign' }]);
      const { state } = renderState();
      await waitFor(() => expect(state.loadError()).toBeTruthy());
      expect(state.org()).toBeNull();
      expect(state.canManageCurrentOrg()).toBe(false);
      await state.createShare();
      expect(createShareMock).not.toHaveBeenCalled();
    },
  );

  it('binds every read to the selected scope and invalidates all siblings after a partial failure', async () => {
    listSharesMock.mockRejectedValue(new Error('403'));
    const { state } = renderState();
    await waitFor(() => expect(state.loadError()).toBeTruthy());
    for (const method of [orgGetMock, listMembersMock, listSharesMock, listIncomingSharesMock]) {
      expect(method).toHaveBeenCalledWith('org-a', strictOptions());
      expect(method.mock.calls[0][1].signal.aborted).toBe(true);
    }
    expect(listOrgsMock).toHaveBeenCalledWith(strictOptions());
  });

  it('hides old rows and actions while a same-selection reload is pending', async () => {
    listSharesMock.mockResolvedValue([outgoing]);
    renderPanel();
    await screen.findByText(outgoing.resourceName);
    const next = deferred<(typeof outgoing)[]>();
    listSharesMock.mockReturnValueOnce(next.promise);
    emit('organizations_changed');
    expect(screen.queryByText(outgoing.resourceName)).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Remove', exact: true })).not.toBeInTheDocument();
    next.resolve([]);
    await screen.findByRole('heading', { name: 'Outgoing Shares' });
    expect(screen.queryByText(outgoing.resourceName)).not.toBeInTheDocument();
  });

  it('preserves same-selection drafts across failed reloads but clears every peer draft', async () => {
    const { state } = renderState();
    await accepted(state);
    state.updateResourceType('vm');
    state.updateResourceId('unsent-alpha');
    state.updateResourceName('Alpha draft');
    state.setAccessRole('admin');
    orgGetMock.mockRejectedValueOnce(new Error('503'));
    await state.loadSharingData();
    expect(state.resourceId()).toBe('unsent-alpha');
    await state.loadSharingData();
    expect(state.resourceId()).toBe('unsent-alpha');
    getOrgIDMock.mockReturnValue('org-b');
    orgGetMock.mockResolvedValue({ ...targetOrg, ownerUserId: 'user-1' });
    emit('org_switched');
    await accepted(state);
    expect(state.resourceId()).toBe('');
    expect(state.resourceType()).toBe('');
    expect(state.resourceName()).toBe('');
    expect(state.selectedQuickPick()).toBe('');
    expect(state.accessRole()).toBe('viewer');
    expect(state.targetOrgId()).toBe('org-a');
  });

  it('requires a new destination when a retained target disappears instead of silently choosing a peer', async () => {
    const { state } = renderState();
    await accepted(state);
    state.applyResourceQuickPick('vm::vm-100');
    expect(state.targetOrgId()).toBe('org-b');
    listOrgsMock.mockResolvedValue([
      baseOrg,
      { ...targetOrg, id: 'org-c', displayName: 'Organization C' },
    ]);
    await state.loadSharingData();
    expect(state.targetOrgId()).toBe('');
    expect(state.canCreateShare()).toBe(false);
    state.updateTargetOrg('org-b');
    await state.createShare();
    expect(createShareMock).not.toHaveBeenCalled();
    state.updateTargetOrg('org-c');
    await state.createShare();
    expect(createShareMock).toHaveBeenCalledTimes(1);
    expect(createShareMock.mock.calls[0][1].targetOrgId).toBe('org-c');
  });

  it('ignores a late failure without suppressing current actions or emitting a stale error', async () => {
    const old = deferred<typeof baseOrg>();
    orgGetMock.mockReturnValueOnce(old.promise);
    const { state } = renderState();
    getOrgIDMock.mockReturnValue('org-b');
    orgGetMock.mockResolvedValue({ ...targetOrg, ownerUserId: 'user-1' });
    emit('org_switched');
    await accepted(state);
    old.reject(new Error('old read failure'));
    await Promise.resolve();
    await Promise.resolve();
    expect(state.org()?.id).toBe('org-b');
    expect(state.canManageCurrentOrg()).toBe(true);
    expect(state.loadError()).toBeNull();
    expect(notificationErrorMock).not.toHaveBeenCalled();
  });

  it('does not apply an older same-selection reload and cancels subscriptions on disposal', async () => {
    const { state, unmount } = renderState();
    await accepted(state);
    const old = deferred<(typeof outgoing)[]>();
    listSharesMock.mockReturnValueOnce(old.promise);
    const pending = state.loadSharingData();
    await state.loadSharingData();
    old.resolve([outgoing]);
    await pending;
    expect(state.outgoingShares()).toEqual([]);
    const count = orgGetMock.mock.calls.length;
    unmount();
    emit('organizations_changed');
    emit('org_switched');
    expect(orgGetMock.mock.calls.length).toBe(count);
  });

  it('uses the explicit default selection without changing global context', async () => {
    getOrgIDMock.mockReturnValue(null);
    orgGetMock.mockResolvedValue({ ...baseOrg, id: 'default' });
    const { state } = renderState();
    await accepted(state);
    expect(orgGetMock).toHaveBeenCalledWith('default', strictOptions('default'));
    expect(state.selectedOrgID()).toBe('default');
  });
});

describe('sharing changes require an admitted current snapshot', () => {
  const actions = [
    {
      name: 'create',
      mock: createShareMock,
      run: (state: ReturnType<typeof useOrganizationSharingPanelState>) => {
        state.applyResourceQuickPick('vm::vm-100');
        return state.createShare();
      },
    },
    {
      name: 'remove outgoing',
      mock: deleteShareMock,
      run: (state: ReturnType<typeof useOrganizationSharingPanelState>) =>
        state.deleteShare(state.outgoingShares()[0]),
    },
    {
      name: 'accept incoming',
      mock: acceptIncomingShareMock,
      run: (state: ReturnType<typeof useOrganizationSharingPanelState>) =>
        state.acceptIncomingShare(state.incomingShares()[0]),
    },
    {
      name: 'decline incoming',
      mock: declineIncomingShareMock,
      run: (state: ReturnType<typeof useOrganizationSharingPanelState>) =>
        state.declineIncomingShare(state.incomingShares()[0]),
    },
  ];
  const populate = () => {
    listSharesMock.mockResolvedValue([outgoing]);
    listIncomingSharesMock.mockResolvedValue([incoming]);
  };

  it.each(actions)(
    'binds $name once and requires a read after an uncertain response',
    async ({ mock, run }) => {
      populate();
      mock.mockRejectedValueOnce(new Error('403'));
      const { state } = renderState();
      await accepted(state);
      state.updateResourceId('saved-draft');
      await run(state);
      expect(mock).toHaveBeenCalledTimes(1);
      expect(mock.mock.calls[0].at(-1)).toEqual(strictOptions());
      expect(state.loadError()).toContain('could not be confirmed');
      expect(state.canManageCurrentOrg()).toBe(false);
      await state.createShare();
      await state.deleteShare(outgoing);
      await state.acceptIncomingShare(incoming);
      await state.declineIncomingShare(incoming);
      expect(mock).toHaveBeenCalledTimes(1);
      await state.loadSharingData();
      await accepted(state);
      await run(state);
      expect(mock).toHaveBeenCalledTimes(2);
      expect(eventBusEmitMock).toHaveBeenCalledWith('organizations_changed');
      await accepted(state);
    },
  );

  it.each(actions)(
    'does not replay $name or use a late result after switching organizations',
    async ({ mock, run }) => {
      populate();
      const pending = deferred<unknown>();
      mock.mockReturnValueOnce(pending.promise);
      const { state } = renderState();
      await accepted(state);
      const change = run(state);
      await run(state);
      expect(mock).toHaveBeenCalledTimes(1);
      const signal = mock.mock.calls[0].at(-1).signal;
      getOrgIDMock.mockReturnValue('org-b');
      orgGetMock.mockResolvedValue({ ...targetOrg, ownerUserId: 'user-1' });
      listIncomingSharesMock.mockResolvedValue([]);
      emit('org_switched');
      await accepted(state);
      state.updateResourceId('beta-draft');
      pending.resolve({});
      await change;
      expect(signal.aborted).toBe(true);
      expect(state.org()?.id).toBe('org-b');
      expect(state.resourceId()).toBe('beta-draft');
      expect(notificationSuccessMock).not.toHaveBeenCalled();
      expect(notificationErrorMock).not.toHaveBeenCalled();
      expect(mock).toHaveBeenCalledTimes(1);
    },
  );

  it.each(['viewer', 'anonymous', 'read-only'])(
    'blocks all direct changes for a %s session',
    async (kind) => {
      populate();
      if (kind === 'viewer') {
        orgGetMock.mockResolvedValue({ ...baseOrg, ownerUserId: 'other' });
      }
      if (kind === 'read-only')
        syncSessionPresentationPolicy({
          presentationPolicy: {
            demoMode: false,
            readOnly: true,
            hideCommercial: false,
            hideUpgrade: true,
          },
        });
      const { state, setUser } = renderState();
      if (kind === 'anonymous') setUser(undefined);
      await accepted(state);
      expect(state.canManageCurrentOrg()).toBe(false);
      for (const { run, mock } of actions) {
        await run(state);
        expect(mock).not.toHaveBeenCalled();
      }
    },
  );

  it.each(['user', 'capability', 'read-only', 'disposal'])(
    'invalidates an in-flight write on %s change',
    async (kind) => {
      populate();
      const [enabled, setEnabled] = createSignal(true);
      isMultiTenantEnabledMock.mockImplementation(enabled);
      const pending = deferred<unknown>();
      createShareMock.mockReturnValueOnce(pending.promise);
      const { state, setUser, unmount } = renderState();
      await accepted(state);
      const change = actions[0].run(state);
      const signal = createShareMock.mock.calls[0].at(-1).signal;
      if (kind === 'user') setUser('other-user');
      if (kind === 'capability') setEnabled(false);
      if (kind === 'read-only')
        syncSessionPresentationPolicy({
          presentationPolicy: {
            demoMode: false,
            readOnly: true,
            hideCommercial: false,
            hideUpgrade: true,
          },
        });
      if (kind === 'disposal') unmount();
      expect(signal.aborted).toBe(true);
      pending.resolve({});
      await change;
      expect(notificationSuccessMock).not.toHaveBeenCalled();
      expect(createShareMock).toHaveBeenCalledTimes(1);
      if (kind === 'capability') {
        setEnabled(true);
        await accepted(state);
        expect(createShareMock).toHaveBeenCalledTimes(1);
      }
    },
  );

  it('rejects foreign or obsolete rows and does not reaccept an accepted incoming share', async () => {
    populate();
    const { state } = renderState();
    await accepted(state);
    const priorOutgoing = state.outgoingShares()[0];
    const priorIncoming = state.incomingShares()[0];
    listSharesMock.mockResolvedValue([{ ...outgoing }]);
    listIncomingSharesMock.mockResolvedValue([{ ...incoming, status: 'accepted' }]);
    await state.loadSharingData();
    await state.deleteShare(priorOutgoing);
    await state.declineIncomingShare(priorIncoming);
    await state.acceptIncomingShare(state.incomingShares()[0]);
    expect(deleteShareMock).not.toHaveBeenCalled();
    expect(declineIncomingShareMock).not.toHaveBeenCalled();
    expect(acceptIncomingShareMock).not.toHaveBeenCalled();
  });

  it.each(['outgoing', 'incoming'])(
    'rechecks the %s row after confirmation changes scope',
    async (kind) => {
      populate();
      const { state } = renderState();
      await accepted(state);
      vi.mocked(confirm).mockImplementation(() => {
        getOrgIDMock.mockReturnValue('org-b');
        return true;
      });
      if (kind === 'outgoing') await state.deleteShare(state.outgoingShares()[0]);
      else await state.declineIncomingShare(state.incomingShares()[0]);
      expect(deleteShareMock).not.toHaveBeenCalled();
      expect(declineIncomingShareMock).not.toHaveBeenCalled();
    },
  );

  it('keeps opaque quick-pick IDs intact and refuses a disappeared selection', async () => {
    const [resources, setResources] = createSignal([makeResource({ id: 'opaque::vm/100' })]);
    readResources = resources;
    const { state } = renderState();
    await accepted(state);
    state.applyResourceQuickPick('vm::opaque::vm/100');
    await state.createShare();
    await accepted(state);
    expect(createShareMock.mock.calls[0][1].resourceId).toBe('opaque::vm/100');
    setResources([]);
    expect(state.canCreateShare()).toBe(false);
    await state.createShare();
    expect(createShareMock).toHaveBeenCalledTimes(1);
  });
});
