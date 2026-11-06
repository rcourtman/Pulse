import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { cleanup, render, waitFor } from '@solidjs/testing-library';
import { useOrganizationAccessPanelState } from '../useOrganizationAccessPanelState';
import { useOrganizationSharingPanelState } from '../useOrganizationSharingPanelState';
import { clearApiToken, setOrgID } from '@/utils/apiClient';
import { eventBus } from '@/stores/events';
import { loadRuntimeCapabilities } from '@/stores/license';
import { syncSessionPresentationPolicy } from '@/stores/sessionPresentationPolicy';

const feedback = vi.hoisted(() => ({ success: vi.fn(), error: vi.fn() }));
vi.mock('@/stores/notifications', () => ({ notificationStore: feedback }));
vi.mock('@/utils/logger', () => ({ logger: { error: vi.fn(), debug: vi.fn() } }));
vi.mock('@/hooks/useResources', () => ({ useResources: () => ({ resources: () => [] }) }));

const orgID = 'client-operations';
const owner = 'owner@example.test';
const target = 'peer-client';
const response = (data: unknown) => new Response(JSON.stringify(data), { status: 200 });
const deferred = <T,>() => {
  let resolve!: (value: T) => void;
  let reject!: (reason: unknown) => void;
  const promise = new Promise<T>((yes, no) => {
    resolve = yes;
    reject = no;
  });
  return { promise, resolve, reject };
};
let completion: ReturnType<typeof deferred<Response>>;
let applied = false;
let failCapabilities = false;
let mutationSignal: AbortSignal | undefined;
const requests: { url: string; method: string }[] = [];
const writes = () => requests.filter((request) => request.method === 'POST');
const reads = () => requests.filter((request) => request.url === `/api/orgs/${orgID}`);
const writablePolicy = () =>
  syncSessionPresentationPolicy({
    presentationPolicy: {
      demoMode: false,
      readOnly: false,
      hideCommercial: false,
      hideUpgrade: true,
    },
  });

beforeEach(async () => {
  completion = deferred<Response>();
  applied = false;
  failCapabilities = false;
  mutationSignal = undefined;
  requests.length = 0;
  feedback.success.mockReset();
  feedback.error.mockReset();
  clearApiToken();
  setOrgID(orgID);
  document.cookie = 'pulse_csrf=synthetic-csrf; Path=/';
  writablePolicy();
  vi.stubGlobal(
    'fetch',
    vi.fn(async (url: string, init: RequestInit) => {
      const method = init.method || 'GET';
      requests.push({ url, method });
      if (url === '/api/license/runtime-capabilities') {
        if (failCapabilities) throw new Error('Synthetic capability read failure');
        return response({ capabilities: ['multi_tenant'], limits: [], max_history_days: 7 });
      }
      if (method === 'POST') {
        mutationSignal = init.signal as AbortSignal;
        return completion.promise;
      }
      if (url === `/api/orgs/${orgID}`)
        return response({ id: orgID, displayName: 'Operations', ownerUserId: owner });
      if (url === '/api/orgs')
        return response([{ id: orgID }, { id: target, displayName: 'Peer Client' }]);
      if (url.endsWith('/members')) return response([{ userId: owner, role: 'owner' }]);
      if (url.endsWith('/invitations'))
        return response(applied ? [{ userId: 'new-user', role: 'viewer' }] : []);
      if (url === '/api/org-invitations' || url.endsWith('/shares/incoming')) return response([]);
      if (url.endsWith('/shares'))
        return response(applied ? [{ id: 'share-1', targetOrgId: target }] : []);
      throw new Error(`Unexpected fixture read: ${url}`);
    }),
  );
  await loadRuntimeCapabilities(true);
});

afterEach(() => {
  cleanup();
  clearApiToken();
  setOrgID(null);
  vi.unstubAllGlobals();
});

const open = (panel: 'access' | 'sharing') => {
  let state!:
    | ReturnType<typeof useOrganizationAccessPanelState>
    | ReturnType<typeof useOrganizationSharingPanelState>;
  render(() => {
    state =
      panel === 'access'
        ? useOrganizationAccessPanelState({ currentUser: owner })
        : useOrganizationSharingPanelState({ currentUser: owner });
    return <div />;
  });
  const submit = () => {
    if ('inviteMember' in state) {
      state.setInviteUserID('new-user');
      return state.inviteMember();
    }
    state.updateTargetOrg(target);
    state.updateResourceType('vm');
    state.updateResourceId('vm-100');
    return state.createShare();
  };
  const reload = () =>
    'loadOrganizationAccess' in state ? state.loadOrganizationAccess() : state.loadSharingData();
  const ready = async () => {
    await waitFor(() => {
      expect(state.loading()).toBe(false);
      expect(state.loadError()).toBeNull();
      expect(state.org()?.id).toBe(orgID);
    });
  };
  return { state, submit, reload, ready };
};

describe('pending organization changes with the real capability store and HTTP client', () => {
  it.each(
    (['access', 'sharing'] as const).flatMap((panel) =>
      ['success', 'uncertain'].map((outcome) => ({ panel, outcome })),
    ),
  )('retains $panel $outcome across unchanged-reader refreshes', async ({ panel, outcome }) => {
    const { state, submit, reload, ready } = open(panel);
    await ready();
    const change = submit();
    await waitFor(() => expect(writes()).toHaveLength(1));
    eventBus.emit('organizations_changed');
    eventBus.emit('organizations_changed');
    writablePolicy();
    await loadRuntimeCapabilities(true);
    await reload();
    expect(mutationSignal?.aborted).toBe(false);
    expect(state.saving()).toBe(true);
    expect(reads()).toHaveLength(1);
    await submit();
    expect(writes()).toHaveLength(1);
    applied = true;
    if (outcome === 'uncertain') completion.reject(new Error('Response lost after apply'));
    else
      completion.resolve(
        response(
          panel === 'access'
            ? { kind: 'invitation', invitation: { userId: 'new-user', role: 'viewer' } }
            : { id: 'share-1' },
        ),
      );
    await change;
    if (outcome === 'uncertain') {
      expect(state.loadError()).toContain('could not be confirmed');
      expect(reads()).toHaveLength(1);
      expect(feedback.success).not.toHaveBeenCalled();
      await reload();
    } else expect(feedback.success).toHaveBeenCalledTimes(1);
    await ready();
    expect(reads()).toHaveLength(2);
    expect(writes()).toHaveLength(1);
  });

  it.each(['access', 'sharing'] as const)(
    'cancels %s when a forced capability refresh fails closed',
    async (panel) => {
      const { state, submit, ready } = open(panel);
      await ready();
      const change = submit();
      await waitFor(() => expect(writes()).toHaveLength(1));
      eventBus.emit('organizations_changed');
      expect(mutationSignal?.aborted).toBe(false);
      failCapabilities = true;
      await loadRuntimeCapabilities(true);
      expect(mutationSignal?.aborted).toBe(true);
      expect(state.saving()).toBe(false);
      expect(state.org()).toBeNull();
      completion.reject(new Error('Late response after authority became unavailable'));
      await change;
      expect(feedback.success).not.toHaveBeenCalled();
      expect(feedback.error).not.toHaveBeenCalled();
      expect(writes()).toHaveLength(1);
    },
  );

  it.each(['access', 'sharing'] as const)(
    'cancels %s on a same-ID org switch because capabilities are withdrawn',
    async (panel) => {
      const { state, submit, ready } = open(panel);
      await ready();
      const change = submit();
      await waitFor(() => expect(writes()).toHaveLength(1));
      eventBus.emit('organizations_changed');
      expect(mutationSignal?.aborted).toBe(false);
      eventBus.emit('org_switched', orgID);
      expect(mutationSignal?.aborted).toBe(true);
      expect(state.saving()).toBe(false);
      await ready();
      completion.resolve(response({ kind: 'invitation', invitation: { role: 'viewer' } }));
      await change;
      expect(feedback.success).not.toHaveBeenCalled();
      expect(feedback.error).not.toHaveBeenCalled();
      expect(writes()).toHaveLength(1);
    },
  );
});
