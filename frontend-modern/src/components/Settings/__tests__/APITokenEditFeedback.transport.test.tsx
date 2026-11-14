import { createSignal } from 'solid-js';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, screen, waitFor, within } from '@solidjs/testing-library';
import type { APITokenRecord } from '@/api/security';
import { eventBus } from '@/stores/events';
import { notificationStore } from '@/stores/notifications';
import { clearAuth, setApiToken, setOrgID } from '@/utils/apiClient';
import { APITokenManager } from '../APITokenManager';

// Exercise the actual editor, state, SecurityAPI and apiClient. Only unrelated
// inventory/capability providers and notifications are fixed for this fixture.
vi.mock('@/api/agentCapabilities', () => ({
  fetchAgentCapabilitiesManifest: () => Promise.resolve({ requiredScopes: [] }),
}));
vi.mock('@/contexts/appRuntime', () => ({
  useWebSocket: () => ({
    markAgentsTokenRevoked: vi.fn(),
    markDockerRuntimesTokenRevoked: vi.fn(),
  }),
}));
vi.mock('@/hooks/useResources', () => ({
  useResources: () => ({ resources: () => [], byType: () => [] }),
}));
vi.mock('@/stores/notifications', () => ({
  notificationStore: { success: vi.fn(), error: vi.fn() },
}));
vi.mock('@/utils/logger', () => ({
  logger: { error: vi.fn(), debug: vi.fn(), warn: vi.fn() },
}));

const token: APITokenRecord = {
  id: 'token/one',
  name: 'Saved collector',
  prefix: 'test',
  suffix: '1234',
  createdAt: '2026-11-01T00:00:00Z',
  scopes: ['monitoring:read'],
  ownerUserId: 'fixture-admin',
  expiresAt: '2027-01-01T00:00:00Z',
};
const json = (body: unknown, status = 200) =>
  new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json' } });
const fetchMock = vi.fn<typeof fetch>();
const scopeMessage = 'Cannot grant scope settings:read: not held by caller';
type Operation = 'scope' | 'rename';
const messageFor = (operation: Operation, status = 403) =>
  operation === 'scope'
    ? status === 403
      ? scopeMessage
      : 'Unable to update API token scopes.'
    : 'Unable to rename the API token.';
const writes = () => fetchMock.mock.calls.filter(([, options]) => options?.method === 'PATCH');
const ready = async () => {
  const [reader, setReader] = createSignal('fixture-admin');
  const [canManage, setCanManage] = createSignal(true);
  render(() => <APITokenManager canManage={canManage()} currentUser={reader()} />);
  await screen.findByRole('article', { name: 'API token Saved collector (test…1234)' });
  return { setReader, setCanManage };
};
const open = async (operation: Operation) => {
  const card = screen.getByRole('article', { name: 'API token Saved collector (test…1234)' });
  fireEvent.click(
    within(card).getByRole('button', {
      name: new RegExp(`^${operation === 'scope' ? 'Edit scopes' : 'Rename'} `),
    }),
  );
  const dialog = await screen.findByRole('dialog');
  if (operation === 'scope') {
    fireEvent.click(within(dialog).getByRole('checkbox', { name: /Settings \(read\)/ }));
  } else {
    fireEvent.input(within(dialog).getByLabelText('Token name'), {
      target: { value: 'Current collector' },
    });
  }
  return dialog;
};
const save = (dialog: HTMLElement, operation: Operation) =>
  fireEvent.click(
    within(dialog).getByRole('button', {
      name: operation === 'scope' ? 'Save scopes' : 'Save',
    }),
  );
const refuse = (operation: Operation, status = 403) =>
  json({ error: operation === 'scope' ? scopeMessage : 'SYNTHETIC_PRIVATE_DETAIL' }, status);

beforeEach(() => {
  vi.clearAllMocks();
  fetchMock.mockReset();
  vi.stubGlobal('fetch', fetchMock);
  clearAuth();
  sessionStorage.clear();
  setOrgID('org-a');
  setApiToken('synthetic-only-auth');
  document.cookie = 'pulse_csrf=synthetic-only-csrf; Path=/';
  fetchMock.mockImplementation(async (_url, options) =>
    options?.method === 'PATCH'
      ? json({ error: 'Unconfigured write' }, 500)
      : json({ tokens: [token] }),
  );
});
afterEach(() => {
  cleanup();
  clearAuth();
  document.cookie = 'pulse_csrf=; Max-Age=0; Path=/';
  vi.unstubAllGlobals();
});

describe.each(['scope', 'rename'] as const)('API token %s refusal feedback', (operation) => {
  it.each([400, 403, 404, 409, 422, 429])(
    'keeps the bounded HTTP %s refusal inside the editor without changing its draft or replaying',
    async (status) => {
      await ready();
      const dialog = await open(operation);
      fetchMock.mockResolvedValueOnce(refuse(operation, status));
      save(dialog, operation);
      expect(await within(dialog).findByRole('alert')).toHaveTextContent(
        messageFor(operation, status),
      );
      expect(notificationStore.error).not.toHaveBeenCalled();
      expect(screen.queryByText('SYNTHETIC_PRIVATE_DETAIL')).not.toBeInTheDocument();
      expect(
        screen.queryByRole('button', { name: 'Check token inventory' }),
      ).not.toBeInTheDocument();
      if (operation === 'scope') {
        expect(within(dialog).getByRole('checkbox', { name: /Settings \(read\)/ })).toBeChecked();
        expect(dialog).toHaveTextContent('The token value and expiry do not change.');
      } else {
        expect(within(dialog).getByLabelText('Token name')).toHaveValue('Current collector');
        expect(dialog).toHaveTextContent('The token value and connected agents stay unchanged.');
      }
      expect(dialog).toHaveTextContent('test…1234');
      expect(writes()).toHaveLength(1);
      expect(writes()[0][0]).toBe('/api/security/tokens/token%2Fone');
      expect(writes()[0][1]).toMatchObject({
        credentials: 'include',
        headers: { 'X-Pulse-Org-ID': 'org-a', 'X-CSRF-Token': 'synthetic-only-csrf' },
        body: JSON.stringify(
          operation === 'scope'
            ? { scopes: ['monitoring:read', 'settings:read'] }
            : { name: 'Current collector' },
        ),
      });
      expect(notificationStore.success).not.toHaveBeenCalled();
    },
  );

  it('clears old feedback on an explicit retry, locks pending actions and closes on success', async () => {
    await ready();
    const dialog = await open(operation);
    fetchMock.mockResolvedValueOnce(refuse(operation));
    save(dialog, operation);
    await within(dialog).findByRole('alert');
    let finish!: (response: Response) => void;
    fetchMock.mockReturnValueOnce(new Promise<Response>((resolve) => (finish = resolve)));
    save(dialog, operation);
    await waitFor(() => expect(writes()).toHaveLength(2));
    expect(within(dialog).queryByRole('alert')).not.toBeInTheDocument();
    expect(within(dialog).getByRole('button', { name: 'Cancel' })).toBeDisabled();
    expect(within(dialog).getByRole('button', { name: /Saving/ })).toBeDisabled();
    fireEvent.keyDown(dialog, { key: 'Escape' });
    expect(screen.getByRole('dialog')).toBeInTheDocument();
    finish(
      json({
        record: {
          ...token,
          ...(operation === 'scope'
            ? { scopes: ['monitoring:read', 'settings:read'] }
            : { name: 'Current collector' }),
        },
      }),
    );
    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument());
    expect(notificationStore.success).toHaveBeenCalledTimes(1);
    expect(writes()).toHaveLength(2);
  });

  it('does not carry feedback across Cancel, reopen or the other editor', async () => {
    await ready();
    const dialog = await open(operation);
    fetchMock.mockResolvedValueOnce(refuse(operation));
    save(dialog, operation);
    await within(dialog).findByRole('alert');
    fireEvent.click(within(dialog).getByRole('button', { name: 'Cancel' }));
    const reopened = await open(operation);
    expect(within(reopened).queryByRole('alert')).not.toBeInTheDocument();
    fireEvent.click(within(reopened).getByRole('button', { name: 'Cancel' }));
    const other = await open(operation === 'scope' ? 'rename' : 'scope');
    expect(within(other).queryByRole('alert')).not.toBeInTheDocument();
    expect(writes()).toHaveLength(1);
  });

  it.each(['org', 'reader', 'permission'] as const)(
    'retires feedback and ignores a late refusal when the %s changes',
    async (change) => {
      const { setReader, setCanManage } = await ready();
      const dialog = await open(operation);
      fetchMock.mockResolvedValueOnce(refuse(operation));
      save(dialog, operation);
      await within(dialog).findByRole('alert');
      let finish!: (response: Response) => void;
      fetchMock.mockReturnValueOnce(new Promise<Response>((resolve) => (finish = resolve)));
      save(dialog, operation);
      await waitFor(() => expect(writes()).toHaveLength(2));
      if (change === 'org') {
        setOrgID('org-b');
        eventBus.emit('org_switched', 'org-b');
      } else if (change === 'reader') {
        setReader('fixture-admin-b');
      } else {
        setCanManage(false);
      }
      await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument());
      expect(writes()[1][1]?.signal?.aborted).toBe(true);
      finish(refuse(operation));
      await new Promise<void>((resolve) => setTimeout(resolve, 0));
      expect(notificationStore.error).not.toHaveBeenCalled();
      if (change !== 'permission') {
        const next = await open(operation);
        expect(within(next).queryByRole('alert')).not.toBeInTheDocument();
      }
      expect(writes()).toHaveLength(2);
    },
  );

  it('keeps uncertain results in inventory recovery rather than treating them as a retryable refusal', async () => {
    await ready();
    const dialog = await open(operation);
    fetchMock.mockResolvedValueOnce(refuse(operation));
    save(dialog, operation);
    await within(dialog).findByRole('alert');
    fetchMock.mockResolvedValueOnce(json({ error: 'SYNTHETIC_PRIVATE_DETAIL' }, 503));
    save(dialog, operation);
    const check = await screen.findByRole('button', { name: 'Check token inventory' });
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
    expect(screen.getByRole('alert')).toHaveTextContent('The change may already have been saved.');
    expect(screen.queryByText('SYNTHETIC_PRIVATE_DETAIL')).not.toBeInTheDocument();
    expect(writes()).toHaveLength(2);
    fireEvent.click(check);
    await screen.findByRole('article', { name: 'API token Saved collector (test…1234)' });
    const recovered = await open(operation);
    expect(within(recovered).queryByRole('alert')).not.toBeInTheDocument();
    expect(writes()).toHaveLength(2);
  });
});
