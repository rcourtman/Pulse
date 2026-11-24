import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, screen, waitFor, within } from '@solidjs/testing-library';
import { createSignal } from 'solid-js';
import { syncSessionPresentationPolicy } from '@/stores/sessionPresentationPolicy';
import type { Resource } from '@/types/resource';
import type { IncomingOrganizationShare, OrganizationShare } from '@/api/orgs';
import { useOrganizationSharingPanelState } from '../useOrganizationSharingPanelState';
import { OrganizationSharingPanel } from '../OrganizationSharingPanel';

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
  it('shows the target membership threshold and approval requirement before creating a share', async () => {
    renderPanel();
    const role = await screen.findByRole('combobox', { name: 'Minimum member role' });
    expect(role).toHaveValue('viewer');
    expect(role).toHaveAccessibleDescription('After a target admin or owner accepts.');
    expect(within(role).getByRole('option', { name: 'All members (Viewer)' })).toHaveValue(
      'viewer',
    );
    expect(within(role).getByRole('option', { name: 'Editor or higher' })).toHaveValue('editor');
    expect(within(role).getByRole('option', { name: 'Admin or Owner' })).toHaveValue('admin');
    expect(screen.getByRole('button', { name: 'Create Share' })).toBeDisabled();
    expect(createShareMock).not.toHaveBeenCalled();
  });

  it('names the native creation form and keeps manual-entry toggles non-submitting', async () => {
    renderPanel();
    const form = await screen.findByRole('form', { name: 'Create Share' });
    const create = screen.getByRole('button', { name: 'Create Share' });
    expect(create.closest('form')).toBe(form);
    expect(create).toHaveAttribute('type', 'submit');
    expect(create).toBeDisabled();
    const toggle = screen.getByRole('button', { name: 'Enter manually' });
    expect(toggle.closest('form')).toBe(form);
    expect(toggle).toHaveAttribute('type', 'button');
    fireEvent.click(toggle);
    expect(screen.getByRole('button', { name: 'Hide manual entry' })).toHaveAttribute(
      'type',
      'button',
    );
    expect(createShareMock).not.toHaveBeenCalled();
  });

  it('links inline sharing errors without changing field names or valid submission', async () => {
    renderPanel();
    const target = await screen.findByRole('combobox', { name: 'Target Organization' });
    fireEvent.click(screen.getByRole('button', { name: 'Enter manually' }));
    const type = screen.getByRole('combobox', { name: 'Resource Type' });
    const id = screen.getByRole('textbox', { name: 'Resource ID' });
    const create = screen.getByRole('button', { name: 'Create Share' });

    fireEvent.change(target, { target: { value: 'org-b' } });
    fireEvent.change(target, { target: { value: '' } });
    fireEvent.input(id, { target: { value: '   ' } });

    expect(target).toHaveAttribute('aria-invalid', 'true');
    expect(target).toHaveAccessibleDescription('Target organization is required');
    expect(type).toHaveAccessibleName('Resource Type');
    expect(id).toHaveAttribute('aria-invalid', 'true');
    expect(id).toHaveAccessibleName('Resource ID');
    expect(id).toHaveAccessibleDescription('Resource ID is required');
    expect(create).toBeDisabled();
    expect(createShareMock).not.toHaveBeenCalled();

    fireEvent.change(target, { target: { value: 'org-b' } });
    fireEvent.change(type, { target: { value: 'vm' } });
    fireEvent.input(id, { target: { value: '  vm-123  ' } });
    expect(target).not.toHaveAttribute('aria-invalid');
    expect(target).toHaveAccessibleDescription('Organization B · ID: org-b');
    for (const input of [type, id]) {
      expect(input).not.toHaveAttribute('aria-invalid');
      expect(input).not.toHaveAttribute('aria-describedby');
    }
    expect(create).toBeEnabled();
    fireEvent.click(create);
    await waitFor(() => expect(createShareMock).toHaveBeenCalledTimes(1));
    expect(createShareMock).toHaveBeenCalledWith(
      'org-a',
      {
        targetOrgId: 'org-b',
        resourceType: 'vm',
        resourceId: 'vm-123',
        resourceName: '',
        accessRole: 'viewer',
      },
      expect.objectContaining({
        expectedOrgID: 'org-a',
        retry: false,
        signal: expect.any(AbortSignal),
      }),
    );
  });

  it('keeps error descriptions local to each mounted sharing form', async () => {
    const views = [renderPanel(), renderPanel()];
    const ids: string[] = [];
    for (const view of views) {
      const panel = within(view.container);
      const target = await panel.findByRole('combobox', { name: 'Target Organization' });
      fireEvent.click(panel.getByRole('button', { name: 'Enter manually' }));
      const id = panel.getByRole('textbox', { name: 'Resource ID' });
      fireEvent.change(target, { target: { value: '' } });
      fireEvent.input(id, { target: { value: ' ' } });
      for (const field of [target, id]) {
        const descriptionID = field.getAttribute('aria-describedby');
        expect(descriptionID).toBeTruthy();
        expect(field).toHaveAttribute('aria-invalid', 'true');
        const description = document.getElementById(descriptionID!);
        expect(description).toBeInTheDocument();
        expect(view.container).toContainElement(description);
        ids.push(field.id, descriptionID!);
      }
    }
    expect(ids.every(Boolean)).toBe(true);
    expect(new Set(ids).size).toBe(ids.length);
    expect(createShareMock).not.toHaveBeenCalled();
  });

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

  it('does not use an old restart control to begin a read for a different selection', async () => {
    const oldRead = deferred<typeof baseOrg>();
    orgGetMock.mockReturnValueOnce(oldRead.promise).mockResolvedValue(targetOrg);
    renderPanel();
    const restart = screen.getByRole('button', { name: 'Restart sharing check' });
    expect(restart).toBeEnabled();

    // Selection has changed but the switch event has not refreshed the panel.
    getOrgIDMock.mockReturnValue(targetOrg.id);
    fireEvent.click(restart);
    expect(orgGetMock).toHaveBeenCalledTimes(1);
    expect(orgGetMock.mock.calls[0][1].signal.aborted).toBe(false);

    emit('org_switched');
    expect(orgGetMock.mock.calls[0][1].signal.aborted).toBe(true);
    expect(orgGetMock).toHaveBeenLastCalledWith(
      targetOrg.id,
      expect.objectContaining({ expectedOrgID: targetOrg.id, retry: false }),
    );
    oldRead.resolve(baseOrg);
    const saved = await screen.findByRole('group', { name: 'Saved shares', busy: false });
    expect(saved).toHaveTextContent('Sharing for Organization B (org-b)');
    expect(saved).not.toHaveTextContent('Sharing for Organization A');
    expect(orgGetMock).toHaveBeenCalledTimes(2);
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
      'pve-1 (agent:host-1)',
      'Alpha VM (vm:vm-100)',
      'Zulu VM (vm:vm-200)',
    ]);
    expect(Array.from(quickPick.options, (option) => option.value)).toEqual([
      '',
      'agent::host-1',
      'vm::vm-100',
      'vm::vm-200',
    ]);
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

  it('offers only canonical resource types in the manual resource type selector', async () => {
    renderPanel();

    await waitFor(() => {
      expect(screen.getByRole('heading', { name: 'Create Share' })).toBeInTheDocument();
    });

    fireEvent.click(screen.getByRole('button', { name: 'Enter manually' }));

    const resourceTypeSelect = screen.getByLabelText('Resource Type') as HTMLSelectElement;
    const optionValues = Array.from(resourceTypeSelect.options, (option) => option.value);
    expect(optionValues).toContain('vm');
    expect(optionValues).toContain('system-container');
    expect(optionValues).toContain('app-container');
    expect(optionValues).not.toContain('host');
    expect(optionValues).not.toContain('container');
  });

  it('creates a share with the expected payload', async () => {
    renderPanel();

    await waitFor(() => {
      expect(screen.getByRole('heading', { name: 'Create Share' })).toBeInTheDocument();
    });

    fireEvent.change(screen.getByLabelText('Target Organization'), { target: { value: 'org-b' } });
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

    fireEvent.change(screen.getByLabelText('Target Organization'), { target: { value: 'org-b' } });
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

    // The consuming panel must expose each saved party, not only its mutable name.
    const partyKeys = screen.getAllByText('org-b', { exact: true });
    expect(partyKeys).toHaveLength(2);
    for (const key of partyKeys) {
      expect(key).toBeVisible();
      expect(key.tagName).toBe('BDI');
      expect(key.closest('td')).toHaveTextContent('Organization B');
      expect(key.closest('td')).toHaveTextContent('viewer');
    }

    const incomingAction = (action: 'Accept' | 'Decline') =>
      screen.getByLabelText(`${action} share of view:alerts-high-cpu from Organization B (org-b)`, {
        selector: 'button',
      });
    expect(incomingAction('Accept')).toHaveAccessibleName(
      'Accept share of view:alerts-high-cpu from Organization B (org-b)',
    );
    expect(incomingAction('Accept')).toBeVisible();
    fireEvent.click(incomingAction('Accept'));

    await waitFor(() => {
      expect(acceptIncomingShareMock).toHaveBeenCalledWith(
        'org-a',
        'share-in-1',
        expect.objectContaining({ expectedOrgID: 'org-a', retry: false }),
      );
    });

    await waitFor(() => {
      expect(incomingAction('Decline')).toBeEnabled();
    });

    expect(incomingAction('Decline')).toHaveAccessibleName(
      'Decline share of view:alerts-high-cpu from Organization B (org-b)',
    );
    expect(incomingAction('Decline')).toBeVisible();
    fireEvent.click(incomingAction('Decline'));
    const declineDialog = screen.getByRole('dialog', { name: 'Decline incoming share?' });
    fireEvent.click(within(declineDialog).getByRole('button', { name: 'Decline share' }));

    await waitFor(() => {
      expect(declineIncomingShareMock).toHaveBeenCalledWith(
        'org-a',
        'share-in-1',
        expect.objectContaining({ expectedOrgID: 'org-a', retry: false }),
      );
    });
  });

  it.each([
    { raw: undefined, scope: 'default' },
    { raw: null, scope: 'default' },
    { raw: '', scope: 'default' },
    { raw: ' \t ', scope: 'default' },
    { raw: 'default', scope: 'default' },
    { raw: ' org-a \t', scope: 'org-a' },
    { raw: 'org-a', scope: 'org-a' },
  ])(
    'binds sharing reads and a new request to normalized scope $scope from "$raw"',
    async ({ raw, scope }) => {
      getOrgIDMock.mockReturnValue(raw);
      const selectedOrg = { ...baseOrg, id: scope };
      orgGetMock.mockResolvedValue(selectedOrg);
      listOrgsMock.mockResolvedValue([selectedOrg, targetOrg]);
      renderPanel();

      const target = await screen.findByRole('combobox', { name: 'Target Organization' });
      expect(Array.from((target as HTMLSelectElement).options, (option) => option.value)).toEqual([
        '',
        targetOrg.id,
      ]);
      const binding = {
        expectedOrgID: scope,
        retry: false,
        signal: expect.any(AbortSignal),
      };
      for (const read of [orgGetMock, listMembersMock, listSharesMock, listIncomingSharesMock]) {
        expect(read).toHaveBeenCalledExactlyOnceWith(scope, binding);
      }
      expect(listOrgsMock).toHaveBeenCalledExactlyOnceWith(binding);

      fireEvent.change(target, { target: { value: targetOrg.id } });
      fireEvent.change(screen.getByRole('combobox', { name: 'Quick Pick Resource' }), {
        target: { value: 'vm::vm-100' },
      });
      fireEvent.click(screen.getByRole('button', { name: 'Create Share' }));
      await waitFor(() =>
        expect(createShareMock).toHaveBeenCalledExactlyOnceWith(
          scope,
          {
            targetOrgId: targetOrg.id,
            resourceType: 'vm',
            resourceId: 'vm-100',
            resourceName: 'Alpha VM',
            accessRole: 'viewer',
          },
          binding,
        ),
      );
      await waitFor(() => expect(orgGetMock).toHaveBeenCalledTimes(2));
      await screen.findByRole('combobox', { name: 'Target Organization' });
      expect(loggerErrorMock).not.toHaveBeenCalled();
    },
  );
});

describe('selected sharing snapshot', () => {
  it('keeps verified identity and outgoing rows visible without enabling a write from an incomplete list', async () => {
    const unread = Object.assign(new Error('unread peer'), { status: 500, code: 'list_failed' });
    listOrgsMock.mockRejectedValueOnce(unread);
    listIncomingSharesMock.mockRejectedValueOnce(unread);
    listSharesMock.mockResolvedValue([outgoing]);
    renderPanel();

    await screen.findByText('Incoming shares could not be loaded.');
    expect(screen.getByText(baseOrg.displayName)).toBeInTheDocument();
    const savedRow = () => within(screen.getByRole('row', { name: /Alpha shared workload/ }));
    expect(savedRow().getByText('org-b', { exact: true })).toBeVisible();
    expect(savedRow().queryByText('Organization B', { exact: true })).not.toBeInTheDocument();
    expect(
      screen.queryByText('No incoming shares from other organizations.'),
    ).not.toBeInTheDocument();
    const form = screen.getByRole('form', { name: 'Create Share' });
    const recipient = within(form).getByRole('combobox', { name: 'Target Organization' });
    expect(recipient).toBeDisabled();
    expect(recipient).toHaveValue('');
    expect(
      within(recipient).getByRole('option', { name: 'Organization list unavailable' }),
    ).toBeInTheDocument();
    expect(within(form).getByRole('combobox', { name: 'Minimum member role' })).toBeDisabled();
    expect(within(form).getByRole('button', { name: 'Create Share' })).toBeDisabled();
    expect(
      savedRow().getByRole('button', { name: 'Remove share of vm:vm-alpha to org-b' }),
    ).toBeDisabled();
    fireEvent.submit(form);
    expect(createShareMock).not.toHaveBeenCalled();

    fireEvent.click(screen.getByRole('button', { name: 'Reload sharing' }));
    await screen.findByText('No incoming shares from other organizations.');
    expect(savedRow().getByText('org-b', { exact: true })).toBeVisible();
    expect(savedRow().getByText('Organization B', { exact: true })).toBeVisible();
    expect(
      savedRow().getByRole('button', {
        name: 'Remove share of vm:vm-alpha to Organization B (org-b)',
      }),
    ).toBeEnabled();
    expect(await screen.findByRole('combobox', { name: 'Target Organization' })).toHaveValue('');
    expect(deleteShareMock).not.toHaveBeenCalled();
  });

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
    expect(screen.queryByRole('button', { name: /^Remove share of / })).not.toBeInTheDocument();
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

describe('deliberate sharing destination', () => {
  const peer = { ...targetOrg, id: 'org-c', displayName: 'Organization C' };

  it.each([
    { directory: [baseOrg], label: 'no other organization' },
    { directory: [baseOrg, targetOrg], label: 'one other organization' },
    { directory: [peer, baseOrg, targetOrg], label: 'multiple organizations in API order' },
  ])('does not infer consent from $label', async ({ directory }) => {
    listOrgsMock.mockResolvedValue(directory);
    const { state } = renderState();
    await accepted(state);
    expect(state.targetOrgId()).toBe('');
    expect(state.targetOrgError()).toBe('');
    state.applyResourceQuickPick('vm::vm-100');
    expect(state.canCreateShare()).toBe(false);
    await state.createShare();
    expect(createShareMock).not.toHaveBeenCalled();
    expect(state.targetOrgError()).toBe('Target organization is required');

    // Completing the manual path must not supply a destination either.
    state.updateResourceType('vm');
    state.updateResourceId('manual-vm');
    expect(state.canCreateShare()).toBe(false);
    await state.createShare();
    expect(createShareMock).not.toHaveBeenCalled();
  });

  it('keeps Create Share disabled until a recipient is chosen explicitly', async () => {
    listOrgsMock.mockResolvedValue([baseOrg, targetOrg, peer]);
    renderPanel();
    const target = await screen.findByRole('combobox', { name: 'Target Organization' });
    const create = screen.getByRole('button', { name: 'Create Share' });
    expect(target).toHaveValue('');
    fireEvent.change(screen.getByLabelText('Quick Pick Resource'), {
      target: { value: 'vm::vm-100' },
    });
    expect(create).toBeDisabled();
    fireEvent.click(create);
    expect(createShareMock).not.toHaveBeenCalled();
    fireEvent.change(target, { target: { value: peer.id } });
    expect(create).toBeEnabled();
    fireEvent.click(create);
    await waitFor(() => expect(createShareMock).toHaveBeenCalledTimes(1));
    expect(createShareMock).toHaveBeenCalledWith(
      'org-a',
      expect.objectContaining({ targetOrgId: peer.id, resourceId: 'vm-100' }),
      strictOptions(),
    );
  });

  it.each(['reload', 'directory event', 'failed read recovery', 'new destination'])(
    'keeps an untouched destination blank across %s',
    async (refresh) => {
      const { state } = renderState();
      await accepted(state);
      state.applyResourceQuickPick('vm::vm-100');
      if (refresh === 'failed read recovery') {
        listOrgsMock.mockRejectedValueOnce(new Error('503'));
        await state.loadSharingData();
        expect(state.loadError()).toBeTruthy();
      }
      listOrgsMock.mockResolvedValue(
        refresh === 'new destination' ? [peer, baseOrg, targetOrg] : [targetOrg, baseOrg],
      );
      if (refresh === 'directory event') emit('organizations_changed');
      else await state.loadSharingData();
      await accepted(state);
      expect(state.targetOrgId()).toBe('');
      expect(state.targetOrgError()).toBe('');
      expect(state.selectedQuickPick()).toBe('vm::vm-100');
      expect(state.canCreateShare()).toBe(false);
      await state.createShare();
      expect(createShareMock).not.toHaveBeenCalled();
    },
  );

  it('retains an explicit choice by ID across a reordered directory, but not after clearing it', async () => {
    listOrgsMock.mockResolvedValue([baseOrg, targetOrg, peer]);
    const { state } = renderState();
    await accepted(state);
    state.applyResourceQuickPick('vm::vm-100');
    state.updateTargetOrg(peer.id);
    listOrgsMock.mockResolvedValue([peer, targetOrg, baseOrg]);
    await state.loadSharingData();
    expect(state.targetOrgId()).toBe(peer.id);
    expect(state.canCreateShare()).toBe(true);
    state.updateTargetOrg('');
    await state.loadSharingData();
    expect(state.targetOrgId()).toBe('');
    expect(state.canCreateShare()).toBe(false);
    await state.createShare();
    expect(createShareMock).not.toHaveBeenCalled();
  });
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

  it.each([
    'wrong organization',
    'missing shares',
    'foreign incoming target',
    'foreign incoming target with unread outgoing',
  ])('does not admit %s as a valid empty snapshot', async (problem) => {
    if (problem === 'wrong organization') orgGetMock.mockResolvedValue(targetOrg);
    if (problem === 'missing shares') listSharesMock.mockResolvedValue(null);
    if (problem.startsWith('foreign incoming target')) {
      listIncomingSharesMock.mockResolvedValue([{ ...incoming, targetOrgId: 'org-foreign' }]);
    }
    if (problem === 'foreign incoming target with unread outgoing') {
      listSharesMock.mockRejectedValue(
        Object.assign(new Error('Failed to load organization'), {
          status: 500,
          code: 'org_load_failed',
        }),
      );
    }
    const { state } = renderState();
    await waitFor(() => expect(state.loadError()).toBeTruthy());
    expect(state.org()).toBeNull();
    expect(state.canManageCurrentOrg()).toBe(false);
    await state.createShare();
    expect(createShareMock).not.toHaveBeenCalled();
  });

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
    expect(screen.queryByRole('button', { name: /^Remove share of / })).not.toBeInTheDocument();
    next.resolve([]);
    await screen.findByRole('heading', { name: 'Outgoing Shares' });
    expect(screen.queryByText(outgoing.resourceName)).not.toBeInTheDocument();
  });

  it('preserves same-selection drafts across failed reloads but clears every peer draft', async () => {
    const { state } = renderState();
    await accepted(state);
    state.updateTargetOrg('org-b');
    state.updateResourceType('vm');
    state.updateResourceId('unsent-alpha');
    state.updateResourceName('Alpha draft');
    state.setAccessRole('admin');
    orgGetMock.mockRejectedValueOnce(new Error('503'));
    await state.loadSharingData();
    expect(state.resourceId()).toBe('unsent-alpha');
    expect(state.targetOrgId()).toBe('org-b');
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
    expect(state.targetOrgId()).toBe('');
  });

  it('requires a new destination when a retained target disappears instead of silently choosing a peer', async () => {
    const { state } = renderState();
    await accepted(state);
    state.updateTargetOrg('org-b');
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

  it.each(['reload', 'directory event', 'failed read recovery', 'returned destination'])(
    'keeps a removed destination invalid across %s until an explicit current choice',
    async (refresh) => {
      const replacementOrg = { ...targetOrg, id: 'org-c', displayName: 'Organization C' };
      const { state } = renderState();
      await accepted(state);
      state.updateTargetOrg('org-b');
      state.applyResourceQuickPick('vm::vm-100');
      expect(state.targetOrgId()).toBe('org-b');
      listOrgsMock.mockResolvedValue([baseOrg, replacementOrg]);
      await state.loadSharingData();
      expect(state.targetOrgId()).toBe('');
      expect(state.canCreateShare()).toBe(false);
      await state.createShare();
      expect(createShareMock).not.toHaveBeenCalled();

      if (refresh === 'directory event') {
        const calls = listOrgsMock.mock.calls.length;
        emit('organizations_changed');
        expect(listOrgsMock).toHaveBeenCalledTimes(calls + 1);
        await accepted(state);
      } else {
        if (refresh === 'failed read recovery') {
          orgGetMock.mockRejectedValueOnce(new Error('503'));
          await state.loadSharingData();
          expect(state.loadError()).toBeTruthy();
          await state.createShare();
          expect(createShareMock).not.toHaveBeenCalled();
        }
        if (refresh === 'returned destination') {
          listOrgsMock.mockResolvedValue([baseOrg, targetOrg, replacementOrg]);
        }
        await state.loadSharingData();
      }
      // Neither another accepted read nor the old destination returning is consent.
      expect(state.targetOrgId()).toBe('');
      expect(state.targetOrgError()).toBeTruthy();
      expect(state.selectedQuickPick()).toBe('vm::vm-100');
      expect(state.resourceId()).toBe('vm-100');
      expect(state.canCreateShare()).toBe(false);
      await state.createShare();
      expect(createShareMock).not.toHaveBeenCalled();
      state.updateTargetOrg('not-in-directory');
      await state.loadSharingData();
      expect(state.targetOrgId()).toBe('');
      await state.createShare();
      expect(createShareMock).not.toHaveBeenCalled();

      state.updateTargetOrg('org-c');
      await state.loadSharingData();
      expect(state.canCreateShare()).toBe(true);
      await state.createShare();
      await accepted(state);
      expect(createShareMock).toHaveBeenCalledTimes(1);
      expect(createShareMock).toHaveBeenCalledWith(
        'org-a',
        expect.objectContaining({
          targetOrgId: 'org-c',
          resourceId: 'vm-100',
          accessRole: 'viewer',
        }),
        strictOptions(),
      );
    },
  );

  it.each(['organization', 'user'])(
    'clears the invalid destination and resource draft when the selected %s changes',
    async (selection) => {
      const replacementOrg = { ...targetOrg, id: 'org-c', displayName: 'Organization C' };
      const { state, setUser } = renderState();
      await accepted(state);
      state.updateTargetOrg('org-b');
      state.applyResourceQuickPick('vm::vm-100');
      listOrgsMock.mockResolvedValue([baseOrg, replacementOrg]);
      await state.loadSharingData();
      expect(state.targetOrgId()).toBe('');
      if (selection === 'organization') {
        getOrgIDMock.mockReturnValue('org-c');
        orgGetMock.mockResolvedValue({ ...replacementOrg, ownerUserId: 'user-1' });
        emit('org_switched');
      } else {
        orgGetMock.mockResolvedValue({ ...baseOrg, ownerUserId: 'user-3' });
        setUser('user-3');
      }
      await accepted(state);
      expect(state.targetOrgId()).toBe('');
      expect(state.selectedQuickPick()).toBe('');
      expect(state.resourceType()).toBe('');
      expect(state.resourceId()).toBe('');
      expect(state.resourceName()).toBe('');
      expect(state.canCreateShare()).toBe(false);
      await state.createShare();
      expect(createShareMock).not.toHaveBeenCalled();
    },
  );

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
        state.updateTargetOrg('org-b');
        state.applyResourceQuickPick('vm::vm-100');
        return state.createShare();
      },
    },
    {
      name: 'remove outgoing',
      mock: deleteShareMock,
      run: (state: ReturnType<typeof useOrganizationSharingPanelState>) => {
        state.deleteShare(state.outgoingShares()[0]);
        const confirmation = state.shareConfirmation();
        return confirmation ? state.confirmShareAction(confirmation) : Promise.resolve();
      },
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
      run: (state: ReturnType<typeof useOrganizationSharingPanelState>) => {
        state.declineIncomingShare(state.incomingShares()[0]);
        const confirmation = state.shareConfirmation();
        return confirmation ? state.confirmShareAction(confirmation) : Promise.resolve();
      },
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
    'keeps pending $name serialized across same-reader refreshes until success',
    async ({ mock, run }) => {
      populate();
      const pending = deferred<unknown>();
      mock.mockReturnValueOnce(pending.promise);
      const { state } = renderState();
      await accepted(state);
      const change = run(state);
      const signal = mock.mock.calls[0].at(-1).signal;

      emit('organizations_changed');
      emit('organizations_changed');
      await state.loadSharingData();
      syncSessionPresentationPolicy({
        presentationPolicy: {
          demoMode: false,
          readOnly: false,
          hideCommercial: false,
          hideUpgrade: true,
        },
      });
      expect(signal.aborted).toBe(false);
      expect(state.saving()).toBe(true);
      expect(orgGetMock).toHaveBeenCalledTimes(1);
      await run(state);
      expect(mock).toHaveBeenCalledTimes(1);

      // Only a post-completion read may replace the accepted snapshot.
      listSharesMock.mockResolvedValue([]);
      listIncomingSharesMock.mockResolvedValue([]);
      pending.resolve({});
      await change;
      await accepted(state);
      expect(orgGetMock).toHaveBeenCalledTimes(2);
      expect(state.outgoingShares()).toEqual([]);
      expect(state.incomingShares()).toEqual([]);
      expect(notificationSuccessMock).toHaveBeenCalledTimes(1);
      expect(mock).toHaveBeenCalledTimes(1);
    },
  );

  it.each(actions)(
    'keeps uncertain $name blocked after an intervening refresh until explicit recovery',
    async ({ mock, run }) => {
      populate();
      const pending = deferred<unknown>();
      mock.mockReturnValueOnce(pending.promise);
      const { state } = renderState();
      await accepted(state);
      const change = run(state);
      emit('organizations_changed');
      await state.loadSharingData();
      pending.reject(new Error('response lost after the server applied the change'));
      await change;

      expect(state.loadError()).toContain('could not be confirmed');
      expect(state.canManageCurrentOrg()).toBe(false);
      expect(state.outgoingShares()).toEqual([]);
      expect(state.incomingShares()).toEqual([]);
      expect(orgGetMock).toHaveBeenCalledTimes(1);
      expect(notificationSuccessMock).not.toHaveBeenCalled();
      await run(state);
      expect(mock).toHaveBeenCalledTimes(1);

      await state.loadSharingData();
      await accepted(state);
      expect(orgGetMock).toHaveBeenCalledTimes(2);
      expect(mock).toHaveBeenCalledTimes(1);
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

  it.each(['user', 'anonymous', 'capability', 'read-only', 'hidden', 'disposal'])(
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
      emit('organizations_changed');
      const signal = createShareMock.mock.calls[0].at(-1).signal;
      if (kind === 'user') setUser('other-user');
      if (kind === 'anonymous') setUser(undefined);
      if (kind === 'hidden')
        syncSessionPresentationPolicy({
          presentationPolicy: {
            demoMode: true,
            readOnly: true,
            hideCommercial: true,
            hideUpgrade: true,
          },
        });
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
      if (kind === 'outgoing') state.deleteShare(state.outgoingShares()[0]);
      else state.declineIncomingShare(state.incomingShares()[0]);
      const confirmation = state.shareConfirmation()!;
      expect(confirmation).not.toBeNull();
      getOrgIDMock.mockReturnValue('org-b');
      await state.confirmShareAction(confirmation);
      expect(deleteShareMock).not.toHaveBeenCalled();
      expect(declineIncomingShareMock).not.toHaveBeenCalled();
    },
  );

  it('keeps opaque quick-pick IDs intact and refuses a disappeared selection', async () => {
    const [resources, setResources] = createSignal([makeResource({ id: 'opaque::vm/100' })]);
    readResources = resources;
    const { state } = renderState();
    await accepted(state);
    state.updateTargetOrg('org-b');
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

describe('keyboard continuation through sharing rereads', () => {
  const rows = (): OrganizationShare[] =>
    ['first', 'second', 'third'].map((id) => ({
      id: `share/${id}?opaque="::]`,
      targetOrgId: targetOrg.id,
      resourceType: 'vm',
      resourceId: `outgoing::${id}/101`,
      resourceName: 'Namesake VM',
      accessRole: 'viewer',
      status: 'pending',
      createdAt: '2026-11-01T00:00:00Z',
      createdBy: 'user-1',
    }));
  let outgoing: OrganizationShare[];
  let incoming: IncomingOrganizationShare[];
  const operations = [
    'remove outgoing',
    'accept incoming',
    'decline incoming',
    'remove incoming',
  ] as const;
  type Operation = (typeof operations)[number];
  const summary = () => screen.getByText(/Sharing for/, { selector: 'p' });
  const warning = () => screen.getByRole('alert', { name: 'Organization sharing unavailable' });
  const settle = () => new Promise<void>((resolve) => setTimeout(resolve, 10));
  const method = (operation: Operation) =>
    operation === 'remove outgoing'
      ? deleteShareMock
      : operation === 'accept incoming'
        ? acceptIncomingShareMock
        : declineIncomingShareMock;
  const shareFor = (operation: Operation) =>
    operation === 'remove outgoing' ? outgoing[1] : incoming[1];
  function action(operation: Operation, share = shareFor(operation)) {
    const verb =
      operation === 'accept incoming'
        ? 'Accept'
        : operation === 'decline incoming'
          ? 'Decline'
          : 'Remove';
    const party = operation === 'remove outgoing' ? 'to' : 'from';
    return screen.getByLabelText(
      `${verb} share of vm:${share.resourceId} ${party} ${targetOrg.displayName} (${targetOrg.id})`,
      { selector: 'button' },
    );
  }
  async function mount(operation: Operation = 'remove outgoing') {
    if (operation === 'remove incoming') incoming[1] = { ...incoming[1], status: 'accepted' };
    const view = renderPanel();
    await screen.findByText('Outgoing Shares', { exact: true, selector: 'h4' });
    return view;
  }
  beforeEach(() => {
    outgoing = rows();
    incoming = rows().map((share) => ({
      ...share,
      id: `incoming/${share.id}`,
      resourceId: share.resourceId.replace('outgoing', 'incoming'),
      targetOrgId: baseOrg.id,
      sourceOrgId: targetOrg.id,
      sourceOrgName: targetOrg.displayName,
    }));
    listSharesMock.mockImplementation(async () => outgoing);
    listIncomingSharesMock.mockImplementation(async () => incoming);
  });

  it.each(operations)(
    'focuses the existing unavailable warning after an uncertain %s',
    async (operation) => {
      const pending = deferred<unknown>();
      method(operation).mockReturnValueOnce(pending.promise);
      await mount(operation);
      const control = action(operation);
      control.focus();
      fireEvent.click(control);
      if (operation !== 'accept incoming') {
        const dialog = screen.getByRole('dialog');
        fireEvent.click(
          within(dialog).getByRole('button', {
            name: operation === 'decline incoming' ? 'Decline share' : 'Remove share',
          }),
        );
      }
      await waitFor(() => expect(method(operation)).toHaveBeenCalledTimes(1));
      pending.reject(new Error('Synthetic response unavailable'));
      await waitFor(() => expect(warning()).toHaveFocus());
      expect(warning()).toHaveTextContent('does not repeat a sharing change');
      expect(warning()).toHaveAttribute('tabindex', '-1');
      expect(screen.getByRole('button', { name: 'Reload sharing' })).toBeEnabled();
      expect(orgGetMock).toHaveBeenCalledTimes(1);
      expect(method(operation)).toHaveBeenCalledTimes(1);
      expect(notificationSuccessMock).not.toHaveBeenCalled();
    },
  );

  it.each(['success', 'failure'])(
    'does not take focus on an untouched initial or background %s read',
    async (outcome) => {
      if (outcome === 'failure')
        orgGetMock.mockRejectedValue(new Error('Synthetic read unavailable'));
      renderPanel();
      if (outcome === 'failure') await screen.findByRole('alert');
      else await screen.findByText('Outgoing Shares', { exact: true, selector: 'h4' });
      await settle();
      expect(document.body).toHaveFocus();
      emit('organizations_changed');
      await waitFor(() => expect(orgGetMock).toHaveBeenCalledTimes(2));
      await settle();
      expect(document.body).toHaveFocus();
    },
  );

  it.each(['focused button', 'pointer without button focus'])(
    'keeps sharing context through an explicit reload from a %s',
    async (activation) => {
      orgGetMock.mockRejectedValueOnce(new Error('Synthetic read unavailable'));
      renderPanel();
      const reload = await screen.findByRole('button', { name: 'Reload sharing' });
      if (activation === 'focused button') reload.focus();
      fireEvent.click(reload);
      await waitFor(() => expect(summary()).toBeInTheDocument());
      expect(screen.getByRole('region', { name: 'Organization sharing' })).toHaveFocus();
      expect(summary()).toHaveTextContent('Organization A (org-a)');
      expect(summary()).toHaveAttribute('tabindex', '-1');
      for (const mock of [
        createShareMock,
        deleteShareMock,
        acceptIncomingShareMock,
        declineIncomingShareMock,
      ])
        expect(mock).not.toHaveBeenCalled();
    },
  );

  it('keeps the reader focused when an explicit reload also fails', async () => {
    orgGetMock.mockRejectedValue(new Error('Synthetic read unavailable'));
    renderPanel();
    const reload = await screen.findByRole('button', { name: 'Reload sharing' });
    reload.focus();
    fireEvent.click(reload);
    await waitFor(() => expect(orgGetMock).toHaveBeenCalledTimes(2));
    await waitFor(() => expect(warning()).toBeInTheDocument());
    expect(screen.getByRole('region', { name: 'Organization sharing' })).toHaveFocus();
    expect(reload.isConnected).toBe(false);
    expect(orgGetMock).toHaveBeenCalledTimes(2);
  });

  it.each(['write', 'reload'])(
    'does not steal focus deliberately moved outside during a %s',
    async (phase) => {
      const outside = document.createElement('button');
      document.body.append(outside);
      try {
        const pending = deferred<unknown>();
        if (phase === 'write') {
          deleteShareMock.mockReturnValueOnce(pending.promise);
          await mount();
          const control = action('remove outgoing');
          control.focus();
          fireEvent.click(control);
          const dialog = screen.getByRole('dialog', { name: 'Remove outgoing share?' });
          fireEvent.click(within(dialog).getByRole('button', { name: 'Remove share' }));
        } else {
          orgGetMock
            .mockRejectedValueOnce(new Error('Synthetic read unavailable'))
            .mockReturnValueOnce(pending.promise);
          renderPanel();
          const reload = await screen.findByRole('button', { name: 'Reload sharing' });
          reload.focus();
          fireEvent.click(reload);
        }
        outside.focus();
        if (phase === 'write') pending.reject(new Error('Synthetic response unavailable'));
        else pending.resolve(baseOrg);
        if (phase === 'write') await screen.findByRole('alert');
        else await screen.findByText('Outgoing Shares', { exact: true, selector: 'h4' });
        await settle();
        expect(outside).toHaveFocus();
      } finally {
        outside.remove();
      }
    },
  );

  it('restores the exact opaque row/action after a reordered same-reader reread', async () => {
    await mount();
    const saved = outgoing[1];
    const control = action('remove outgoing', saved);
    control.focus();
    outgoing = [...outgoing].reverse().map((share) => ({ ...share, resourceName: 'Renamed VM' }));
    emit('organizations_changed');
    await waitFor(() => expect(action('remove outgoing', saved)).toHaveFocus());
    expect(action('remove outgoing', saved)).not.toBe(control);
    expect(deleteShareMock).not.toHaveBeenCalled();
  });

  it.each(['middle', 'last', 'only'])(
    'continues at the next, previous or summary after removing the %s outgoing row',
    async (position) => {
      if (position === 'only') outgoing = outgoing.slice(0, 1);
      await mount();
      const index = position === 'middle' ? 1 : outgoing.length - 1;
      const removed = outgoing[index];
      const replacement = outgoing[index + 1] ?? outgoing[index - 1];
      const pending = deferred<unknown>();
      deleteShareMock.mockReturnValueOnce(pending.promise);
      const control = action('remove outgoing', removed);
      control.focus();
      fireEvent.click(control);
      const dialog = screen.getByRole('dialog', { name: 'Remove outgoing share?' });
      fireEvent.click(within(dialog).getByRole('button', { name: 'Remove share' }));
      await waitFor(() => expect(deleteShareMock).toHaveBeenCalledTimes(1));
      outgoing = outgoing.filter((share) => share.id !== removed.id);
      pending.resolve(undefined);
      await waitFor(() =>
        expect(replacement ? action('remove outgoing', replacement) : summary()).toHaveFocus(),
      );
      expect(acceptIncomingShareMock).not.toHaveBeenCalled();
      expect(deleteShareMock).toHaveBeenCalledTimes(1);
    },
  );

  it('keeps accepted-share continuation in the Accept family, not a new destructive action', async () => {
    await mount('accept incoming');
    const control = action('accept incoming');
    control.focus();
    const pending = deferred<unknown>();
    acceptIncomingShareMock.mockReturnValueOnce(pending.promise);
    fireEvent.click(control);
    incoming = incoming.map((share, index) =>
      index === 1 ? { ...share, status: 'accepted' } : share,
    );
    pending.resolve(undefined);
    await waitFor(() => expect(action('accept incoming', incoming[2])).toHaveFocus());
    expect(declineIncomingShareMock).not.toHaveBeenCalled();
  });

  it.each(['organization', 'user', 'capability', 'read-only', 'hidden'])(
    'forgets old keyboard intent across a %s round trip',
    async (kind) => {
      const [user, setUser] = createSignal('user-1');
      const [enabled, setEnabled] = createSignal(true);
      isMultiTenantEnabledMock.mockImplementation(enabled);
      render(() => <OrganizationSharingPanel currentUser={user()} />);
      await screen.findByText('Outgoing Shares', { exact: true, selector: 'h4' });
      action('remove outgoing').focus();
      const pending = deferred<typeof baseOrg>();
      orgGetMock.mockReturnValueOnce(pending.promise);
      emit('organizations_changed');
      if (kind === 'organization') {
        getOrgIDMock.mockReturnValue('org-b');
        emit('org_switched');
        getOrgIDMock.mockReturnValue('org-a');
        emit('org_switched');
      }
      if (kind === 'user') {
        setUser('other-user');
        setUser('user-1');
      }
      if (kind === 'capability') {
        setEnabled(false);
        setEnabled(true);
      }
      if (kind === 'read-only' || kind === 'hidden') {
        syncSessionPresentationPolicy({
          presentationPolicy: {
            demoMode: kind === 'hidden',
            readOnly: true,
            hideCommercial: kind === 'hidden',
            hideUpgrade: true,
          },
        });
        syncSessionPresentationPolicy(null);
      }
      pending.resolve(baseOrg);
      await screen.findByText('Outgoing Shares', { exact: true, selector: 'h4' });
      await settle();
      expect(document.body).toHaveFocus();
      expect(deleteShareMock).not.toHaveBeenCalled();
    },
  );

  it('restores the Create Share submit control after its saved reread', async () => {
    await mount();
    fireEvent.change(screen.getByLabelText('Target Organization'), {
      target: { value: targetOrg.id },
    });
    fireEvent.change(screen.getByLabelText('Quick Pick Resource'), {
      target: { value: 'vm::vm-100' },
    });
    const create = screen.getByText('Create Share', { selector: 'button' });
    const pending = deferred<unknown>();
    createShareMock.mockReturnValueOnce(pending.promise);
    create.focus();
    fireEvent.click(create);
    await waitFor(() => expect(createShareMock).toHaveBeenCalledTimes(1));
    pending.resolve({});
    await waitFor(() =>
      expect(screen.getByText('Create Share', { selector: 'button' })).toHaveFocus(),
    );
    expect(screen.getByText('Create Share', { selector: 'button' })).not.toBe(create);
    expect(createShareMock).toHaveBeenCalledTimes(1);
  });
  it('cancels a scheduled focus handoff when the panel unmounts', async () => {
    const view = await mount();
    const control = action('remove outgoing');
    control.focus();
    emit('organizations_changed');
    view.unmount();
    await settle();
    expect(document.body).toHaveFocus();
    expect(deleteShareMock).not.toHaveBeenCalled();
  });
});
