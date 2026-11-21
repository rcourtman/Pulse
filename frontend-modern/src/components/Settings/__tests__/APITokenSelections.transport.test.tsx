import { createSignal } from 'solid-js';
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

function expectUnchangedSiblings(selected: APITokenRecord) {
  for (const sibling of originals.filter((token) => token.id !== selected.id)) {
    for (const layout of ['desktop row', 'compact card'] as const) {
      const controls = controlsFor(layout, sibling);
      expect(controls.getByText(sibling.name)).toBeInTheDocument();
      expect(
        controls.getByText(sibling.scopes!.includes('*') ? 'Full' : 'Agent reporting'),
      ).toBeInTheDocument();
    }
  }
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
      return json({ tokens: [...saved.slice(-1), ...saved.slice(0, -1)] });
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
    it('names each repeated action and dialog with the saved namesake and masked hint', async () => {
      await openPanel();
      for (const selected of originals) {
        const subject = `${selected.name} (${hint(selected)})`;
        const controls = controlsFor(layout, selected);
        if (layout === 'compact card') {
          expect(screen.getByRole('article', { name: `API token ${subject}` })).toBeInTheDocument();
        }
        for (const [action, dialogLabel] of [
          ['Edit scopes', 'Edit API token scopes'],
          ['Rename', 'Rename API token'],
          ['Revoke', 'Revoke API token'],
        ]) {
          const name = `${action}${action === 'Edit scopes' ? ' for' : ''} ${subject}`;
          const button = controls.getByRole('button', { name });
          expect(button).toHaveTextContent(action);
          fireEvent.click(button);
          const dialog = await screen.findByRole('dialog', {
            name: `${dialogLabel} for ${subject}`,
          });
          expect(within(dialog).getByText(hint(selected), { exact: true })).toBeInTheDocument();
          for (const sibling of originals.filter((record) => record.id !== selected.id)) {
            expect(
              within(dialog).queryByText(hint(sibling), { exact: true }),
            ).not.toBeInTheDocument();
          }
          if (action === 'Rename') {
            fireEvent.input(within(dialog).getByRole('textbox', { name: 'Token name' }), {
              target: { value: 'Unsent replacement name' },
            });
            expect(dialog).toHaveAccessibleName(`${dialogLabel} for ${subject}`);
          }
          fireEvent.click(within(dialog).getByRole('button', { name: 'Cancel' }));
          await waitFor(() => expect(button).toHaveFocus());
        }
      }
      expect(writes()).toHaveLength(0);
      expect(saved).toEqual(originals);
      expect(reads()).toHaveLength(1);
      expectBoundRequests();
    });

    it('edits and renames the selected namesake through reconciled controls, preserving saved metadata and siblings', async () => {
      const view = await openPanel();
      const selected = originals[1];
      fireEvent.click(controlsFor(layout, selected).getByRole('button', { name: /^Edit scopes / }));
      const editor = within(await screen.findByRole('dialog', { name: /^Edit API token scopes / }));
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
      expectUnchangedSiblings(selected);
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
      fireEvent.click(controlsFor(layout, selected).getByRole('button', { name: /^Rename / }));
      const rename = within(await screen.findByRole('dialog', { name: /^Rename API token / }));
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
      for (const surface of ['desktop row', 'compact card'] as const) {
        expect(
          controlsFor(surface, renamed).getByRole('button', {
            name: `Rename North collector (${hint(selected)})`,
          }),
        ).toBeInTheDocument();
        expect(
          controlsFor(surface, renamed).queryByRole('button', {
            name: `Rename ${selected.name} (${hint(selected)})`,
          }),
        ).not.toBeInTheDocument();
      }
      expect(saved).toEqual([originals[0], renamed, originals[2]]);
      expectUnchangedSiblings(selected);
      expect(changed).toHaveBeenCalledTimes(2);
      expect(runtime.markAgents).not.toHaveBeenCalled();
      expect(runtime.markRuntimes).not.toHaveBeenCalled();
      expect(reads()).toHaveLength(1);

      // A fresh inventory must reproduce the saved result; optimistic display
      // updates alone are not evidence that either write reached the right ID.
      view.unmount();
      await openPanel();
      expect(reads()).toHaveLength(2);
      fireEvent.click(controlsFor(layout, renamed).getByRole('button', { name: /^Edit scopes / }));
      const reopened = within(
        await screen.findByRole('dialog', { name: /^Edit API token scopes / }),
      );
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
      const permanentWarning =
        'Revocation is permanent. Agents, scripts, and integrations using it need a replacement token to authenticate.';
      fireEvent.click(controlsFor(layout, cancelled).getByRole('button', { name: /^Revoke / }));
      const cancelledDialog = await screen.findByRole('dialog', {
        name: `Revoke API token for ${cancelled.name} (${hint(cancelled)})`,
      });
      expect(cancelledDialog).toHaveAccessibleDescription(
        `${permanentWarning} Previously seen using this token agent: agent ${cancelled.suffix} Other clients may also use it.`,
      );
      expect(writes()).toHaveLength(0);
      fireEvent.click(within(cancelledDialog).getByRole('button', { name: 'Cancel' }));
      expect(saved).toEqual(originals);
      fireEvent.click(controlsFor(layout, selected).getByRole('button', { name: /^Revoke / }));
      const confirmation = await screen.findByRole('dialog', {
        name: `Revoke API token for ${selected.name} (${hint(selected)})`,
      });
      expect(confirmation).toHaveAccessibleDescription(
        `${permanentWarning} Previously seen using this token Docker / Podman runtime: docker-host ${selected.suffix} agent: agent ${selected.suffix} Other clients may also use it.`,
      );
      expect(writes()).toHaveLength(0);
      fireEvent.click(within(confirmation).getByRole('button', { name: 'Revoke token' }));
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
            expect(
              controlsFor(surface, sibling).getByRole('button', {
                name: `${name}${name === 'Edit scopes' ? ' for' : ''} ${sibling.name} (${hint(sibling)})`,
              }),
            ).toBeEnabled();
      }
      expect(reads()).toHaveLength(1);
      expectBoundRequests();
    });

    it('discards scope and name drafts without mutating any saved namesake', async () => {
      await openPanel();
      fireEvent.click(
        controlsFor(layout, originals[0]).getByRole('button', { name: /^Edit scopes / }),
      );
      const editor = within(await screen.findByRole('dialog', { name: /^Edit API token scopes / }));
      expect(editor.getByRole('checkbox', { name: /^Full access/ })).toBeChecked();
      fireEvent.click(editor.getByRole('checkbox', { name: /^Monitoring & alerts \(read\)/ }));
      expect(editor.getByRole('checkbox', { name: /^Full access/ })).not.toBeChecked();
      fireEvent.click(editor.getByRole('button', { name: 'Cancel' }));
      fireEvent.click(controlsFor(layout, originals[1]).getByRole('button', { name: /^Rename / }));
      const rename = within(await screen.findByRole('dialog', { name: /^Rename API token / }));
      fireEvent.input(rename.getByRole('textbox', { name: 'Token name' }), {
        target: { value: 'Unsent name' },
      });
      fireEvent.click(rename.getByRole('button', { name: 'Cancel' }));
      expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
      expect(screen.queryByText('Unsent name')).not.toBeInTheDocument();
      fireEvent.click(
        controlsFor(layout, originals[0]).getByRole('button', { name: /^Edit scopes / }),
      );
      const reopened = within(
        await screen.findByRole('dialog', { name: /^Edit API token scopes / }),
      );
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
    describe('revocation focus after the request settles', () => {
      beforeEach(() => {
        // JSDOM does not lay out either responsive inventory. Model exactly
        // one visible surface here; real table/card visibility is browser-proved.
        const visibleParent = function (this: HTMLElement) {
          return this.closest(layout === 'desktop row' ? 'article' : 'tr')
            ? null
            : this.parentElement;
        };
        vi.spyOn(HTMLElement.prototype, 'offsetParent', 'get').mockImplementation(visibleParent);
      });
      afterEach(() => vi.restoreAllMocks());

      const buttonFor = (token: APITokenRecord) =>
        controlsFor(layout, token).getByRole('button', { name: /^Revoke / });
      const confirmRemoval = async (selected: APITokenRecord) => {
        const button = buttonFor(selected);
        button.focus();
        fireEvent.click(button);
        const dialog = await screen.findByRole('dialog', { name: /^Revoke API token / });
        fireEvent.click(within(dialog).getByRole('button', { name: 'Revoke token' }));
        return button;
      };
      const delayDeletion = () => {
        const handler = fetchMock.getMockImplementation()!;
        let release!: () => void;
        fetchMock.mockImplementation((url, init) => {
          if (init?.method !== 'DELETE') return handler(url, init);
          return new Promise<Response>((resolve) => {
            release = () => resolve(handler(url, init));
          });
        });
        return () => release();
      };

      it.each([0, 1, 2])(
        'continues at the surviving token after revoking sorted position %s',
        async (index) => {
          await openPanel();
          const selected = originals[index];
          const next = originals[index === 2 ? 1 : index + 1];
          await confirmRemoval(selected);
          await waitFor(() => expect(buttonFor(next)).toHaveFocus());
          expect(containerFor(layout, selected)).toBeUndefined();
          expect(writes()).toHaveLength(1);
          expect(writes()[0][0]).toBe(tokenPath(selected.id));
          expect(reads()).toHaveLength(1);
          expectBoundRequests();
        },
      );

      it('returns to New token only when no saved credentials remain', async () => {
        saved = [originals[0]];
        await openPanel();
        await confirmRemoval(originals[0]);
        await waitFor(() =>
          expect(screen.getByRole('button', { name: 'New token' })).toHaveFocus(),
        );
        expect(saved).toEqual([]);
        expect(writes()).toHaveLength(1);
        expect(screen.getByRole('button', { name: 'Generate' })).toBeDisabled();
      });

      it('returns a delayed explicit refusal to the retained credential without another request', async () => {
        await openPanel();
        let release!: (response: Response) => void;
        fetchMock.mockImplementationOnce(
          () =>
            new Promise<Response>((resolve) => {
              release = resolve;
            }),
        );
        const selected = originals[1];
        const button = await confirmRemoval(selected);
        await new Promise((resolve) => setTimeout(resolve, 10));
        release(new Response(JSON.stringify({ error: 'synthetic refusal' }), { status: 403 }));
        await waitFor(() => expect(button).toHaveFocus());
        expect(saved).toEqual(originals);
        expect(writes()).toHaveLength(1);
        expect(changed).not.toHaveBeenCalled();
        expect(runtime.markAgents).not.toHaveBeenCalled();
      });

      it('keeps an unknown outcome on its explicit read-only recovery notice', async () => {
        await openPanel();
        fetchMock.mockResolvedValueOnce(
          new Response(JSON.stringify({ error: 'synthetic failure' }), { status: 503 }),
        );
        await confirmRemoval(originals[1]);
        const notice = await screen.findByRole('alert');
        await waitFor(() => expect(notice).toHaveFocus());
        expect(within(notice).getByRole('button', { name: 'Check token inventory' })).toBeEnabled();
        expect(screen.getAllByText('Unknown')).toHaveLength(3);
        expect(writes()).toHaveLength(1);
        expect(reads()).toHaveLength(1);
        expect(changed).not.toHaveBeenCalled();
      });

      it('does not steal a deliberate focus move while revocation is pending', async () => {
        await openPanel();
        const release = delayDeletion();
        await confirmRemoval(originals[1]);
        const draft = screen.getByRole('textbox', { name: 'Token name' });
        draft.focus();
        release();
        await waitFor(() => expect(containerFor(layout, originals[1])).toBeUndefined());
        await new Promise((resolve) => setTimeout(resolve, 10));
        expect(draft).toHaveFocus();
        expect(writes()).toHaveLength(1);
      });

      it('does not refocus withdrawn management when a delayed request finishes', async () => {
        const [allowed, setAllowed] = createSignal(true);
        render(() => (
          <APITokenManager
            currentUser="token-maintainer"
            canManage={allowed()}
            onTokensChanged={changed}
          />
        ));
        await screen.findAllByText(hint(originals[0]));
        const release = delayDeletion();
        await confirmRemoval(originals[1]);
        const signal = writes()[0][1]!.signal;
        setAllowed(false);
        expect(signal?.aborted).toBe(true);
        release();
        await waitFor(() => expect(reads()).toHaveLength(2));
        await new Promise((resolve) => setTimeout(resolve, 10));
        for (const token of originals) expect(buttonFor(token)).toBeDisabled();
        expect(document.activeElement?.getAttribute('data-api-token-action')).toBeNull();
        expect(changed).not.toHaveBeenCalled();
        expect(runtime.markAgents).not.toHaveBeenCalled();
        expect(writes()).toHaveLength(1);
      });

      it('does not refocus another manager after unmount', async () => {
        const view = await openPanel();
        const release = delayDeletion();
        await confirmRemoval(originals[1]);
        view.unmount();
        const anchor = document.createElement('button');
        document.body.append(anchor);
        anchor.focus();
        release();
        await new Promise((resolve) => setTimeout(resolve, 10));
        expect(anchor).toHaveFocus();
        expect(changed).not.toHaveBeenCalled();
        anchor.remove();
      });
    });
  },
);
