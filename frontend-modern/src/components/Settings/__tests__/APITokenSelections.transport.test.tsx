import { afterEach, beforeAll, beforeEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, screen, waitFor, within } from '@solidjs/testing-library';
import type { APITokenRecord } from '@/api/security';
import type { Resource } from '@/types/resource';
import {
  AGENT_REPORT_SCOPE,
  DOCKER_REPORT_SCOPE,
  MONITORING_READ_SCOPE,
} from '@/constants/apiScopes';
import { clearAuth, getOrgID, setApiToken, setOrgID } from '@/utils/apiClient';
import { APITokenManager } from '../APITokenManager';

// Keep both rendered inventories, dialogs, state owner, SecurityAPI and the
// authenticated HTTP client connected. Hook-only or API-mocked tests cannot
// catch a control that submits a different same-name token's identity.
const runtime = vi.hoisted(() => ({ markAgents: vi.fn(), markRuntimes: vi.fn() }));
vi.mock('@/contexts/appRuntime', () => ({
  useWebSocket: () => ({
    markAgentsTokenRevoked: runtime.markAgents,
    markDockerRuntimesTokenRevoked: runtime.markRuntimes,
  }),
}));
vi.mock('@/api/agentCapabilities', () => ({
  fetchAgentCapabilitiesManifest: () => Promise.resolve({ requiredScopes: [] }),
}));
vi.mock('@/stores/notifications', () => ({
  notificationStore: { success: vi.fn(), error: vi.fn() },
}));
vi.mock('@/hooks/useResources', () => ({
  useResources: () => ({
    resources: () => resources,
    byType: (type: string) => resources.filter((resource) => resource.type === type),
  }),
}));

const orgID = 'estate/north';
const authToken = 'synthetic-only-token-selection-auth';
const csrfToken = 'synthetic-only-token-selection-csrf';
const originals: APITokenRecord[] = ['first', 'second', 'third'].map((part, index) => ({
  id: `token/${part}?client=north#saved`,
  name: 'Fleet collector',
  prefix: 'test',
  suffix: `${part}-${index}`,
  createdAt: `2026-11-0${3 - index}T00:00:00Z`,
  lastUsedAt: `2026-11-04T00:00:0${index}Z`,
  expiresAt: `2027-01-0${index + 1}T00:00:00Z`,
  ownerUserId: `saved-owner-${part}`,
  scopes:
    index === 0
      ? ['*']
      : index === 1
        ? [DOCKER_REPORT_SCOPE, AGENT_REPORT_SCOPE]
        : [AGENT_REPORT_SCOPE],
}));
const resources: Resource[] = originals.flatMap((token) =>
  (['agent', 'docker-host'] as const)
    .filter(
      (type) =>
        type === 'agent' ||
        token.scopes?.some((scope) => scope === '*' || scope === DOCKER_REPORT_SCOPE),
    )
    .map<Resource>((type) => ({
      id: `${type}-${token.suffix}`,
      type,
      name: `${type} ${token.suffix}`,
      displayName: `${type} ${token.suffix}`,
      platformId: `${type}-${token.suffix}`,
      platformType: type === 'agent' ? 'agent' : 'docker',
      sourceType: 'agent',
      status: 'online',
      lastSeen: 1,
      tags: [],
      platformData:
        type === 'agent'
          ? { agent: { agentId: `agent-${token.suffix}`, tokenId: token.id } }
          : { docker: { hostSourceId: `runtime-${token.suffix}`, tokenId: token.id } },
    })),
);
type Layout = 'desktop row' | 'compact card';
const tokenPath = (id: string) => `/api/security/tokens/${encodeURIComponent(id)}`;
const fetchMock = vi.fn<typeof fetch>();
const changed = vi.fn();
let saved: APITokenRecord[];
const json = (body: unknown) =>
  new Response(JSON.stringify(body), { headers: { 'Content-Type': 'application/json' } });
const reads = () => fetchMock.mock.calls.filter(([, init]) => !init?.method);
const writes = () => fetchMock.mock.calls.filter(([, init]) => init?.method);
const hint = (token: APITokenRecord) => `${token.prefix}…${token.suffix}`;

beforeAll(async () => {
  // Transform the actual lazy dialog module before timing control assertions.
  // Selection is under test, not the dev server's first module compilation.
  await import('../APITokenManagerDialogs');
});

function containerFor(layout: Layout, token: APITokenRecord) {
  return screen
    .queryAllByText(hint(token))
    .map((element) => element.closest(layout === 'desktop row' ? 'tr' : 'article'))
    .find((element) => element !== null);
}

function controlsFor(layout: Layout, token: APITokenRecord) {
  const container = containerFor(layout, token);
  expect(container).toBeDefined();
  return within(container! as HTMLElement);
}

async function openPanel() {
  const view = render(() => (
    <APITokenManager currentUser="token-maintainer" canManage onTokensChanged={changed} />
  ));
  await screen.findAllByText(hint(originals[0]));
  for (const token of saved) {
    for (const layout of ['desktop row', 'compact card'] as const)
      expect(controlsFor(layout, token).getByText(token.name!)).toBeInTheDocument();
  }
  return view;
}

function expectBoundRequests() {
  for (const [, init] of fetchMock.mock.calls)
    expect(init).toMatchObject({
      credentials: 'include',
      signal: expect.any(AbortSignal),
      headers: { 'X-Pulse-Org-ID': orgID, 'X-API-Token': authToken },
    });
  for (const [, init] of writes())
    expect(init?.headers).toMatchObject({ 'X-CSRF-Token': csrfToken });
  expect(getOrgID()).toBe(orgID);
}

beforeEach(() => {
  clearAuth();
  sessionStorage.clear();
  setOrgID(orgID);
  setApiToken(authToken);
  vi.stubEnv('MODE', 'production');
  document.cookie = `pulse_csrf=${csrfToken}; Path=/`;
  window.history.replaceState({}, '', '/settings/api-access');
  changed.mockClear();
  runtime.markAgents.mockClear();
  runtime.markRuntimes.mockClear();
  saved = structuredClone(originals);
  fetchMock.mockReset();
  fetchMock.mockImplementation(async (input, init = {}) => {
    const path = String(input);
    const method = init.method ?? 'GET';
    // Return an unsorted inventory so tests cannot rely on fixture array index
    // matching the visible, creation-sorted row or card position.
    if (path === '/api/security/tokens' && method === 'GET')
      return json({ tokens: [...saved].reverse() });
    const selected = saved.find((token) => tokenPath(token.id) === path);
    // Derive the fixture's saved result from the dispatched path and body, not
    // the identity or values the test intended to submit.
    if (selected && method === 'PATCH') {
      const updated = { ...selected, ...JSON.parse(String(init.body)) };
      saved = saved.map((token) => (token.id === selected.id ? updated : token));
      return json({ record: updated });
    }
    if (selected && method === 'DELETE') {
      saved = saved.filter((token) => token.id !== selected.id);
      return new Response(null, { status: 204 });
    }
    throw new Error(`Unexpected synthetic token request: ${method} ${path}`);
  });
  vi.stubGlobal('fetch', fetchMock);
});

afterEach(() => {
  cleanup();
  clearAuth();
  setOrgID(null);
  document.cookie = 'pulse_csrf=; Max-Age=0; Path=/';
  window.history.replaceState({}, '', '/');
  vi.unstubAllGlobals();
  vi.unstubAllEnvs();
});

describe.each<Layout>(['desktop row', 'compact card'])(
  'API token selection from the %s',
  (layout) => {
    it('edits and renames the selected namesake through reconciled controls, preserving saved metadata and siblings', async () => {
      const view = await openPanel();
      const selected = originals[1];
      fireEvent.click(controlsFor(layout, selected).getByRole('button', { name: 'Edit scopes' }));
      const editor = within(await screen.findByRole('dialog', { name: 'Edit API token scopes' }));
      const docker = editor.getByRole('checkbox', { name: /^Docker \/ Podman reporting/ });
      const agent = editor.getByRole('checkbox', { name: /^Agent reporting/ });
      const monitoring = editor.getByRole('checkbox', { name: /^Monitoring & alerts \(read\)/ });
      expect(docker).toBeChecked();
      expect(agent).toBeChecked();
      expect(monitoring).not.toBeChecked();
      expect(editor.getByRole('checkbox', { name: /^Full access/ })).not.toBeChecked();
      expect(editor.getByRole('button', { name: 'Save scopes' })).toBeDisabled();
      fireEvent.click(docker);
      fireEvent.click(agent);
      expect(editor.getByRole('button', { name: 'Save scopes' })).toBeDisabled();
      fireEvent.click(monitoring);
      expect(writes()).toHaveLength(0);
      expect(saved).toEqual(originals);
      fireEvent.click(editor.getByRole('button', { name: 'Save scopes' }));
      await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument());
      expect(writes()).toHaveLength(1);
      expect(writes()[0][0]).toBe(tokenPath(selected.id));
      expect(writes()[0][1]?.method).toBe('PATCH');
      expect(JSON.parse(String(writes()[0][1]?.body))).toEqual({ scopes: [MONITORING_READ_SCOPE] });
      const scoped = { ...selected, scopes: [MONITORING_READ_SCOPE] };
      expect(saved).toEqual([originals[0], scoped, originals[2]]);
      for (const surface of ['desktop row', 'compact card'] as const) {
        expect(
          controlsFor(surface, selected).getByText('Monitoring & alerts (read)'),
        ).toBeInTheDocument();
        expect(
          controlsFor(surface, selected).queryByText('Docker / Podman reporting'),
        ).not.toBeInTheDocument();
      }

      // This is the reconciled grid proxy / replacement card, not the original
      // object passed to the dialog. Its next action must retain the saved ID.
      fireEvent.click(controlsFor(layout, selected).getByRole('button', { name: 'Rename' }));
      const rename = within(await screen.findByRole('dialog', { name: 'Rename API token' }));
      expect(rename.getByRole('textbox', { name: 'Token name' })).toHaveValue(selected.name);
      fireEvent.input(rename.getByRole('textbox', { name: 'Token name' }), {
        target: { value: '  North collector  ' },
      });
      fireEvent.click(rename.getByRole('button', { name: 'Save' }));
      await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument());
      expect(writes()).toHaveLength(2);
      expect(writes()[1][0]).toBe(tokenPath(selected.id));
      expect(writes()[1][1]?.method).toBe('PATCH');
      expect(JSON.parse(String(writes()[1][1]?.body))).toEqual({ name: 'North collector' });
      const renamed = { ...scoped, name: 'North collector' };
      expect(saved).toEqual([originals[0], renamed, originals[2]]);
      expect(changed).toHaveBeenCalledTimes(2);
      expect(runtime.markAgents).not.toHaveBeenCalled();
      expect(runtime.markRuntimes).not.toHaveBeenCalled();
      expect(reads()).toHaveLength(1);

      // A fresh inventory must reproduce the saved result; optimistic display
      // updates alone are not evidence that either write reached the right ID.
      view.unmount();
      await openPanel();
      expect(reads()).toHaveLength(2);
      fireEvent.click(controlsFor(layout, renamed).getByRole('button', { name: 'Edit scopes' }));
      const reopened = within(await screen.findByRole('dialog', { name: 'Edit API token scopes' }));
      expect(
        reopened.getByRole('checkbox', { name: /^Monitoring & alerts \(read\)/ }),
      ).toBeChecked();
      expect(reopened.getByRole('checkbox', { name: /^Agent reporting/ })).not.toBeChecked();
      fireEvent.click(reopened.getByRole('button', { name: 'Cancel' }));
      expect(writes()).toHaveLength(2);
      expectBoundRequests();
    });

    it('cancels a sibling revoke, then revokes only the selected credential and its connected consumers', async () => {
      await openPanel();
      const cancelled = originals[2];
      const selected = originals[1];
      fireEvent.click(controlsFor(layout, cancelled).getByRole('button', { name: 'Revoke' }));
      const cancelledDialog = within(
        await screen.findByRole('dialog', { name: 'Revoke API token' }),
      );
      expect(cancelledDialog.getByText(/agents, scripts, or integrations/)).toBeInTheDocument();
      expect(writes()).toHaveLength(0);
      fireEvent.click(cancelledDialog.getByRole('button', { name: 'Cancel' }));
      expect(saved).toEqual(originals);
      fireEvent.click(controlsFor(layout, selected).getByRole('button', { name: 'Revoke' }));
      const confirmation = within(await screen.findByRole('dialog', { name: 'Revoke API token' }));
      expect(writes()).toHaveLength(0);
      fireEvent.click(confirmation.getByRole('button', { name: 'Revoke token' }));
      await waitFor(() => expect(containerFor(layout, selected)).toBeUndefined());
      expect(writes()).toHaveLength(1);
      expect(writes()[0][0]).toBe(tokenPath(selected.id));
      expect(writes()[0][1]).toMatchObject({ method: 'DELETE' });
      expect(writes()[0][1]?.body).toBeUndefined();
      expect(saved).toEqual([originals[0], cancelled]);
      expect(changed).toHaveBeenCalledTimes(1);
      expect(runtime.markAgents).toHaveBeenCalledWith(selected.id, [`agent-${selected.suffix}`]);
      expect(runtime.markRuntimes).toHaveBeenCalledWith(selected.id, [
        `runtime-${selected.suffix}`,
      ]);
      for (const args of runtime.markAgents.mock.calls)
        expect(args).toEqual([selected.id, [`agent-${selected.suffix}`]]);
      for (const args of runtime.markRuntimes.mock.calls)
        expect(args).toEqual([selected.id, [`runtime-${selected.suffix}`]]);
      for (const surface of ['desktop row', 'compact card'] as const) {
        expect(containerFor(surface, selected)).toBeUndefined();
        for (const sibling of saved)
          for (const name of ['Edit scopes', 'Rename', 'Revoke'])
            expect(controlsFor(surface, sibling).getByRole('button', { name })).toBeEnabled();
      }
      expect(reads()).toHaveLength(1);
      expectBoundRequests();
    });

    it('discards scope and name drafts without mutating any saved namesake', async () => {
      await openPanel();
      fireEvent.click(
        controlsFor(layout, originals[0]).getByRole('button', { name: 'Edit scopes' }),
      );
      const editor = within(await screen.findByRole('dialog', { name: 'Edit API token scopes' }));
      expect(editor.getByRole('checkbox', { name: /^Full access/ })).toBeChecked();
      fireEvent.click(editor.getByRole('checkbox', { name: /^Monitoring & alerts \(read\)/ }));
      expect(editor.getByRole('checkbox', { name: /^Full access/ })).not.toBeChecked();
      fireEvent.click(editor.getByRole('button', { name: 'Cancel' }));
      fireEvent.click(controlsFor(layout, originals[1]).getByRole('button', { name: 'Rename' }));
      const rename = within(await screen.findByRole('dialog', { name: 'Rename API token' }));
      fireEvent.input(rename.getByRole('textbox', { name: 'Token name' }), {
        target: { value: 'Unsent name' },
      });
      fireEvent.click(rename.getByRole('button', { name: 'Cancel' }));
      expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
      expect(screen.queryByText('Unsent name')).not.toBeInTheDocument();
      fireEvent.click(
        controlsFor(layout, originals[0]).getByRole('button', { name: 'Edit scopes' }),
      );
      const reopened = within(await screen.findByRole('dialog', { name: 'Edit API token scopes' }));
      expect(reopened.getByRole('checkbox', { name: /^Full access/ })).toBeChecked();
      expect(
        reopened.getByRole('checkbox', { name: /^Monitoring & alerts \(read\)/ }),
      ).not.toBeChecked();
      fireEvent.click(reopened.getByRole('button', { name: 'Cancel' }));
      expect(saved).toEqual(originals);
      expect(writes()).toHaveLength(0);
      expect(changed).not.toHaveBeenCalled();
      expect(runtime.markAgents).not.toHaveBeenCalled();
      expect(runtime.markRuntimes).not.toHaveBeenCalled();
      expect(reads()).toHaveLength(1);
      expectBoundRequests();
    });
  },
);
