import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, screen, waitFor, within } from '@solidjs/testing-library';
import { createSignal, type Accessor, type Setter } from 'solid-js';
import type { Organization, OrganizationShare } from '@/api/orgs';
import type { Resource } from '@/types/resource';
import { eventBus } from '@/stores/events';
import { loadRuntimeCapabilities, runtimeCapabilitiesLoadError } from '@/stores/license';
import { syncSessionPresentationPolicy } from '@/stores/sessionPresentationPolicy';
import { clearAuth, getApiToken, getOrgID, setApiToken, setOrgID } from '@/utils/apiClient';
import { OrganizationSharingPanel } from '../OrganizationSharingPanel';

// Keep the rendered form, state owner, OrgsAPI, event bus and session client
// connected. Supply only synthetic resource inventory and HTTP/notice edges;
// hook-only or API-mocked tests cannot detect a miswired picker or role input.
const notices = vi.hoisted(() => ({ success: vi.fn(), error: vi.fn() }));
vi.mock('@/stores/notifications', () => ({ notificationStore: notices }));
vi.mock('@/hooks/useResources', () => ({
  useResources: () => ({ resources: () => resources() }),
}));
vi.mock('@/utils/logger', () => ({
  logger: { error: vi.fn(), warn: vi.fn(), debug: vi.fn() },
}));

const owner = 'owner@example.test';
const orgID = 'source/east?realm=lab#saved';
const orgPath = `/api/orgs/${encodeURIComponent(orgID)}`;
const storedToken = 'synthetic-unrelated-api-token';
const csrf = 'synthetic-share-create-csrf';
const peers: Organization[] = ['third', 'first', 'second'].map((part) => ({
  id: `peer/${part}?realm=lab#saved`,
  displayName: 'Namesake organization',
}));
const target = peers[2];
const originalOrg: Organization = { id: orgID, displayName: 'East estate', ownerUserId: owner };
const selectedResourceID = 'pve::east/node/101?disk=one#saved%雪';
const resourceRows = (): Resource[] =>
  [
    { id: 'pve::third/node/101', type: 'vm' as const },
    // The same opaque ID in a different type is not the selected resource.
    { id: selectedResourceID, type: 'agent' as const },
    { id: 'pve::first/node/101', type: 'vm' as const },
    { id: selectedResourceID, type: 'vm' as const },
  ].map((identity) => ({
    ...identity,
    name: 'Namesake workload',
    displayName: 'Namesake workload',
    platformId: 'synthetic-pve',
    platformType: 'proxmox-pve',
    sourceType: 'api',
    status: 'running',
    lastSeen: 1,
  }));
type Entry = 'quick pick' | 'manual entry';
type Role = 'viewer' | 'editor' | 'admin';
const entries: Entry[] = ['quick pick', 'manual entry'];
const roles: Role[] = ['viewer', 'editor', 'admin'];
const manualResourceID = 'manual::host/202?volume=one#saved%雪';
const quickPickValue = `vm::${selectedResourceID}`;
const payloadFor = (entry: Entry, accessRole: Role = 'viewer') => ({
  targetOrgId: target.id,
  resourceType: 'vm',
  resourceId: entry === 'quick pick' ? selectedResourceID : manualResourceID,
  resourceName: entry === 'quick pick' ? 'Namesake workload' : 'Manual workload',
  accessRole,
});
const json = (value: unknown, status = 200, headers = {}) =>
  new Response(JSON.stringify(value), {
    status,
    headers: { 'Content-Type': 'application/json', ...headers },
  });
function deferred<T>() {
  let resolve!: (value: T) => void;
  const promise = new Promise<T>((yes) => {
    resolve = yes;
  });
  return { promise, resolve };
}

let resources: Accessor<Resource[]>;
let setResources: Setter<Resource[]>;
let directory: Organization[];
let outgoing: OrganizationShare[];
let applyWrite: boolean;
let healthReply: () => Promise<Response>;
let writeReply: () => Promise<Response>;
let unexpected: string[];
let capabilityReply: () => Promise<Response>;
let releases: (() => void)[];
let loads: Promise<void>[];
const capabilityBody = (enabled: boolean) => ({
  capabilities: enabled ? ['multi_tenant'] : [],
  limits: [],
});
type Capability = 'enabled' | 'withdrawn' | 'failed' | 'malformed';
const turn = () => new Promise<void>((resolve) => setTimeout(resolve, 0));
function hold<T>(cleanupValue: T) {
  const reply = deferred<T>();
  releases.push(() => reply.resolve(cleanupValue));
  return reply;
}
function refresh(reply: () => Promise<Response>) {
  capabilityReply = reply;
  const pending = loadRuntimeCapabilities(true);
  loads.push(pending);
  return pending;
}
function capabilities(kind: Capability) {
  return refresh(async () => {
    if (kind === 'failed') throw new TypeError('synthetic-private-sharing-capability-unavailable');
    return json(
      kind === 'malformed'
        ? { capabilities: 'multi_tenant', limits: [] }
        : capabilityBody(kind === 'enabled'),
    );
  });
}
function expectWithdrawn() {
  expect(screen.queryByRole('heading', { name: 'Create Share' })).not.toBeInTheDocument();
  expect(screen.queryByRole('heading', { name: 'Outgoing Shares' })).not.toBeInTheDocument();
  expect(screen.queryByRole('button', { name: 'Reload sharing' })).not.toBeInTheDocument();
  expect(screen.queryByText('No outgoing shares configured.')).not.toBeInTheDocument();
  expect(
    screen.queryByText('No incoming shares from other organizations.'),
  ).not.toBeInTheDocument();
  expect(notices.success).not.toHaveBeenCalled();
  expect(notices.error).not.toHaveBeenCalled();
  expect(getOrgID()).toBe(orgID);
  expect(getApiToken()).toBe(storedToken);
}

const fetchMock = vi.fn<typeof fetch>();
const writes = () => fetchMock.mock.calls.filter(([, init]) => (init?.method ?? 'GET') !== 'GET');
const inventoryReads = () =>
  fetchMock.mock.calls.filter(
    ([url, init]) =>
      (init?.method ?? 'GET') === 'GET' && (url === '/api/orgs' || String(url).startsWith(orgPath)),
  );
const healthReads = () => fetchMock.mock.calls.filter(([url]) => url === '/api/health');
const createButton = () => screen.getByRole('button', { name: 'Create Share' });
const targetInput = () => screen.getByRole('combobox', { name: 'Target Organization' });
const resourceInput = () => screen.getByRole('combobox', { name: 'Quick Pick Resource' });

function expectBoundRequests(id = orgID) {
  for (const [, init] of [...inventoryReads(), ...writes()]) {
    expect(init).toMatchObject({ credentials: 'include', signal: expect.any(AbortSignal) });
    const headers = new Headers(init?.headers);
    expect(headers.get('X-Pulse-Org-ID')).toBe(id);
    expect(headers.has('X-API-Token')).toBe(false);
    for (const internal of ['expectedOrgID', 'retry', 'preferSessionAuth', 'skipOrgContext'])
      expect(init).not.toHaveProperty(internal);
  }
  for (const [, init] of writes())
    expect(new Headers(init?.headers).get('X-CSRF-Token')).toBe(csrf);
  expect(getApiToken()).toBe(storedToken);
}
function expectWrite(entry: Entry, accessRole: Role = 'viewer') {
  expect(writes()).toHaveLength(1);
  const [url, init] = writes()[0];
  expect(url).toBe(`${orgPath}/shares`);
  expect(init?.method).toBe('POST');
  expect(JSON.parse(String(init?.body))).toEqual(payloadFor(entry, accessRole));
  expectBoundRequests();
}
async function openPanel() {
  const view = render(() => <OrganizationSharingPanel currentUser={owner} />);
  await screen.findByRole('heading', { name: 'Create Share' });
  expect(inventoryReads()).toHaveLength(5);
  expect(writes()).toHaveLength(0);
  expect(targetInput()).toHaveValue('');
  expect(resourceInput()).toHaveValue('');
  expect(createButton()).toBeDisabled();
  expect(
    Array.from((targetInput() as HTMLSelectElement).options, (option) => option.value),
  ).not.toContain(originalOrg.id);
  expect(targetInput()).not.toHaveAttribute('aria-describedby');
  expect(resourceInput()).not.toHaveAttribute('aria-describedby');
  expectBoundRequests();
  return view;
}
function fillForm(entry: Entry, role: Role = 'viewer') {
  const destination = within(targetInput()).getByRole('option', {
    name: `${target.displayName} (${target.id})`,
  }) as HTMLOptionElement;
  fireEvent.change(targetInput(), { target: { value: destination.value } });
  expect(targetInput()).toHaveAccessibleDescription(`${target.displayName} · ID: ${target.id}`);
  fireEvent.change(screen.getByRole('combobox', { name: 'Access Role' }), {
    target: { value: role },
  });
  if (entry === 'quick pick') {
    const resource = within(resourceInput()).getByRole('option', {
      name: `Namesake workload (vm:${selectedResourceID})`,
    }) as HTMLOptionElement;
    fireEvent.change(resourceInput(), { target: { value: resource.value } });
    expect(resourceInput()).toHaveValue(quickPickValue);
    expect(resourceInput()).toHaveAccessibleDescription(
      `Resource: Namesake workload · vm:${selectedResourceID}`,
    );
  } else {
    // A manual override must replace a previously selected, different resource.
    fireEvent.change(resourceInput(), { target: { value: quickPickValue } });
    fireEvent.click(screen.getByRole('button', { name: 'Enter manually' }));
    expect(resourceInput()).toHaveValue('');
    expect(resourceInput()).not.toHaveAttribute('aria-describedby');
    fireEvent.input(screen.getByLabelText('Resource Type'), { target: { value: ' VM ' } });
    fireEvent.input(screen.getByLabelText('Resource ID'), {
      target: { value: ` ${manualResourceID} ` },
    });
    fireEvent.input(screen.getByLabelText('Resource Name'), {
      target: { value: ' Manual workload ' },
    });
  }
  expect(targetInput()).toHaveValue(target.id);
  expect(createButton()).toBeEnabled();
}
function expectSavedGrant(entry: Entry, role: Role = 'viewer') {
  const payload = payloadFor(entry, role);
  const row = screen
    .getAllByRole('row')
    .find((candidate) => within(candidate).queryByText(`vm:${payload.resourceId}`));
  expect(row).toBeDefined();
  expect(row).toHaveTextContent(payload.resourceName);
  expect(row).toHaveTextContent('Namesake organization');
  // jsdom includes both responsive presentations; each must show the saved role.
  expect(within(row!).getByRole('cell', { name: role })).toHaveTextContent(role);
  const mobileAccess = within(row!).getByText('Access:').parentElement!;
  expect(within(mobileAccess).getByText(role, { exact: true })).toBeInTheDocument();
  expect(row).toHaveTextContent('Pending approval');
  expect(within(row!).getByRole('button', { name: /^Remove share of / })).toBeEnabled();
  // This saved state comes from the actual dispatched body, not the intended
  // fixture choice. A wrong callback must produce a wrong saved grant.
  expect(outgoing).toHaveLength(1);
  expect(outgoing[0]).toMatchObject(payload);
}

beforeEach(async () => {
  vi.clearAllMocks();
  fetchMock.mockReset();
  vi.stubGlobal('fetch', fetchMock);
  vi.stubEnv('MODE', 'production'); // Exercise cold CSRF preparation, not its test-mode skip.
  sessionStorage.clear();
  clearAuth();
  setOrgID(orgID);
  setApiToken(storedToken);
  syncSessionPresentationPolicy(null);
  document.cookie = `pulse_csrf=${csrf}; Path=/`;
  [resources, setResources] = createSignal(resourceRows());
  directory = [originalOrg, ...peers];
  outgoing = [];
  applyWrite = true;
  unexpected = [];
  releases = [];
  loads = [];
  capabilityReply = async () => json(capabilityBody(true));
  healthReply = async () => {
    document.cookie = `pulse_csrf=${csrf}; Path=/`;
    return json({});
  };
  writeReply = async () => new Response(null, { status: 204 });
  fetchMock.mockImplementation(async (input, init = {}) => {
    const url = String(input);
    const method = init.method ?? 'GET';
    if (method === 'GET') {
      if (url === '/api/license/runtime-capabilities') return capabilityReply();
      if (url === '/api/health') return healthReply();
      if (url === orgPath) return json(originalOrg);
      if (url === '/api/orgs') return json(directory);
      if (url === `${orgPath}/members`)
        return json([{ userId: owner, role: 'owner', addedAt: '2026-11-01T00:00:00Z' }]);
      if (url === `${orgPath}/shares`) return json(outgoing);
      if (url === `${orgPath}/shares/incoming`) return json([]);
    }
    if (url === `${orgPath}/shares` && method === 'POST') {
      if (applyWrite)
        outgoing.push({
          ...JSON.parse(String(init.body)),
          id: `saved/${writes().length}`,
          status: 'pending',
          createdAt: '2026-11-08T00:00:00Z',
          createdBy: owner,
        });
      return writeReply();
    }
    unexpected.push(`${method} ${url}`);
    throw new Error(`Unexpected synthetic request: ${method} ${url}`);
  });
  await refresh(capabilityReply);
});
afterEach(async () => {
  cleanup();
  for (const release of releases) release();
  if (vi.isFakeTimers()) await vi.runOnlyPendingTimersAsync();
  vi.useRealTimers();
  await Promise.allSettled(loads);
  await turn();
  clearAuth();
  syncSessionPresentationPolicy(null);
  document.cookie = 'pulse_csrf=; Max-Age=0; Path=/';
  vi.unstubAllGlobals();
  vi.unstubAllEnvs();
  expect(unexpected).toEqual([]);
});

describe('rendered Create Share controls through the real transport', () => {
  it('identifies every namesake option by its full saved key without choosing for the reader', async () => {
    await openPanel();
    const destinations = Array.from((targetInput() as HTMLSelectElement).options);
    expect(destinations.map((option) => option.textContent)).toEqual([
      'Select organization',
      ...peers.map((peer) => `${peer.displayName} (${peer.id})`),
    ]);
    const choices = Array.from((resourceInput() as HTMLSelectElement).options);
    expect(choices.map((option) => option.textContent)).toEqual([
      'Select resource',
      ...resourceRows().map((resource) => `Namesake workload (${resource.type}:${resource.id})`),
    ]);
    for (const options of [destinations, choices]) {
      expect(new Set(options.map((option) => option.textContent)).size).toBe(options.length);
    }
    fillForm('quick pick');
    const description = document.getElementById(targetInput().getAttribute('aria-describedby')!)!;
    expect(description.querySelector('bdi.font-mono')).toHaveTextContent(target.id);
    expect(writes()).toHaveLength(0);
  });

  it.each(['ID-looking name (peer/forged)', '\u202eNamesake organization'])(
    'keeps real IDs separate from the mutable name %s',
    async (name) => {
      directory = directory.map((peer) =>
        peer.id === target.id ? { ...peer, displayName: name } : peer,
      );
      setResources((rows) => rows.map((resource) => ({ ...resource, name, displayName: name })));
      await openPanel();
      const option = within(targetInput()).getByRole('option', {
        name: `${name} (${target.id})`,
      }) as HTMLOptionElement;
      fireEvent.change(targetInput(), { target: { value: option.value } });
      const resource = within(resourceInput()).getByRole('option', {
        name: `${name} (vm:${selectedResourceID})`,
      }) as HTMLOptionElement;
      fireEvent.change(resourceInput(), { target: { value: resource.value } });
      expect(targetInput()).toHaveAccessibleDescription(`${name} · ID: ${target.id}`);
      expect(resourceInput()).toHaveAccessibleDescription(
        `Resource: ${name} · vm:${selectedResourceID}`,
      );
      for (const [input, id] of [
        [targetInput(), target.id],
        [resourceInput(), `vm:${selectedResourceID}`],
      ] as const) {
        const help = document.getElementById(input.getAttribute('aria-describedby')!)!;
        expect(help.querySelector('bdi.font-mono')!.textContent).toBe(id);
        expect(help.querySelector('bdi')!.textContent).toBe(name);
      }
      fireEvent.click(createButton());
      await waitFor(() => expect(inventoryReads()).toHaveLength(10));
      expect(writes()).toHaveLength(1);
      expect(JSON.parse(String(writes()[0][1]?.body))).toEqual({
        ...payloadFor('quick pick'),
        resourceName: name,
      });
      expectBoundRequests();
    },
  );

  it.each(['', 'same as ID'])(
    'does not duplicate an ID when the saved name is %s',
    async (name) => {
      directory = [originalOrg, { ...target, displayName: name ? target.id : '' }];
      setResources((rows) =>
        rows.map((resource) => ({
          ...resource,
          name: name ? resource.id : '',
          displayName: name ? resource.id : '',
          platformId: '',
        })),
      );
      await openPanel();
      const destination = within(targetInput()).getByRole('option', {
        name: target.id,
      }) as HTMLOptionElement;
      fireEvent.change(targetInput(), { target: { value: destination.value } });
      const resource = within(resourceInput()).getByRole('option', {
        name: `vm:${selectedResourceID}`,
      }) as HTMLOptionElement;
      fireEvent.change(resourceInput(), { target: { value: resource.value } });
      expect(targetInput()).toHaveAccessibleDescription(`Organization ID: ${target.id}`);
      expect(resourceInput()).toHaveAccessibleDescription(`Resource: vm:${selectedResourceID}`);
      expect(writes()).toHaveLength(0);
    },
  );

  it('refreshes names without changing selected keys or sending old names in the grant', async () => {
    await openPanel();
    fillForm('quick pick');
    directory = [...directory]
      .reverse()
      .map((peer) => ({ ...peer, displayName: 'Renamed client' }));
    setResources((rows) =>
      [...rows].reverse().map((resource) => ({
        ...resource,
        name: 'Renamed workload',
        displayName: 'Renamed workload',
      })),
    );
    eventBus.emit('organizations_changed');
    await waitFor(() => expect(inventoryReads()).toHaveLength(10));
    await screen.findByRole('heading', { name: 'Create Share' });
    expect(targetInput()).toHaveValue(target.id);
    expect(resourceInput()).toHaveValue(quickPickValue);
    expect(targetInput()).toHaveAccessibleDescription(`Renamed client · ID: ${target.id}`);
    expect(resourceInput()).toHaveAccessibleDescription(
      `Resource: Renamed workload · vm:${selectedResourceID}`,
    );
    fireEvent.click(createButton());
    await waitFor(() => expect(inventoryReads()).toHaveLength(15));
    expect(writes()).toHaveLength(1);
    expect(JSON.parse(String(writes()[0][1]?.body))).toEqual({
      ...payloadFor('quick pick'),
      resourceName: 'Renamed workload',
    });
    expectBoundRequests();
  });

  it.each(entries.flatMap((entry) => roles.map((role) => ({ entry, role }))))(
    'saves only the selected namesake destination, full $entry identity and $role role',
    async ({ entry, role }) => {
      await openPanel();
      fillForm(entry, role);
      const reply = deferred<Response>();
      writeReply = () => reply.promise;
      fireEvent.click(createButton());
      await waitFor(() => expect(writes()).toHaveLength(1));
      expectWrite(entry, role);
      const saving = screen.getByRole('button', { name: 'Saving...' });
      expect(saving).toBeDisabled();
      expect(targetInput()).toHaveAccessibleDescription(`${target.displayName} · ID: ${target.id}`);
      if (entry === 'quick pick')
        expect(resourceInput()).toHaveAccessibleDescription(
          `Resource: Namesake workload · vm:${selectedResourceID}`,
        );
      for (const input of screen.getAllByRole('combobox')) expect(input).toBeDisabled();
      fireEvent.click(saving);
      expect(writes()).toHaveLength(1);
      expect(notices.success).not.toHaveBeenCalled();
      reply.resolve(new Response(null, { status: 204 }));
      await waitFor(() => expect(inventoryReads()).toHaveLength(10));
      await screen.findByRole('heading', { name: 'Outgoing Shares' });
      expectSavedGrant(entry, role);
      expect(notices.success).toHaveBeenCalledExactlyOnceWith(
        'Share request sent. A target organization admin must accept it before access becomes active.',
      );
      expect(notices.error).not.toHaveBeenCalled();
      expect(healthReads()).toHaveLength(0);
      expectWrite(entry, role);
      expect(getOrgID()).toBe(orgID);
    },
  );

  it('never infers the recipient even when the directory has only one peer', async () => {
    directory = [originalOrg, target];
    await openPanel();
    fireEvent.change(resourceInput(), { target: { value: quickPickValue } });
    expect(targetInput()).toHaveValue('');
    expect(createButton()).toBeDisabled();
    fireEvent.click(createButton());
    expect(writes()).toHaveLength(0);
    fireEvent.change(targetInput(), { target: { value: target.id } });
    expect(createButton()).toBeEnabled();
  });

  it('requires a current resource selection instead of silently using a removed quick pick', async () => {
    await openPanel();
    fillForm('quick pick');
    setResources((current) =>
      current.filter((resource) => resource.type !== 'vm' || resource.id !== selectedResourceID),
    );
    await screen.findByRole('status');
    expect(screen.getByText(/The selected resource is no longer available/)).toBeInTheDocument();
    expect(resourceInput()).not.toHaveAttribute('aria-describedby');
    expect(createButton()).toBeDisabled();
    fireEvent.click(createButton());
    expect(writes()).toHaveLength(0);
    // A namesake with the same ID but a different type is still present.
    const replacement = `agent::${selectedResourceID}`;
    fireEvent.change(resourceInput(), { target: { value: replacement } });
    expect(createButton()).toBeEnabled();
    fireEvent.click(createButton());
    await waitFor(() => expect(inventoryReads()).toHaveLength(10));
    await waitFor(() =>
      expect(
        screen
          .getAllByRole('row')
          .some((row) => within(row).queryByText(`agent:${selectedResourceID}`)),
      ).toBe(true),
    );
    expect(writes()).toHaveLength(1);
    expect(JSON.parse(String(writes()[0][1]?.body))).toEqual({
      ...payloadFor('quick pick'),
      resourceType: 'agent',
    });
    expectBoundRequests();
  });

  it('requires explicit recipient reselection after directory withdrawal and reappearance', async () => {
    await openPanel();
    fillForm('quick pick');
    directory = directory.filter((organization) => organization.id !== target.id);
    eventBus.emit('organizations_changed');
    await waitFor(() => expect(inventoryReads()).toHaveLength(10));
    await screen.findByRole('heading', { name: 'Create Share' });
    expect(targetInput()).toHaveValue('');
    expect(targetInput()).not.toHaveAttribute('aria-describedby');
    expect(resourceInput()).toHaveValue(quickPickValue);
    expect(createButton()).toBeDisabled();
    directory = [originalOrg, ...peers];
    eventBus.emit('organizations_changed');
    await waitFor(() => expect(inventoryReads()).toHaveLength(15));
    await screen.findByRole('heading', { name: 'Create Share' });
    expect(targetInput()).toHaveValue('');
    expect(createButton()).toBeDisabled();
    fireEvent.click(createButton());
    expect(writes()).toHaveLength(0);
    fireEvent.change(targetInput(), { target: { value: target.id } });
    expect(createButton()).toBeEnabled();
    fireEvent.click(createButton());
    await waitFor(() => expect(inventoryReads()).toHaveLength(20));
    await screen.findByRole('heading', { name: 'Outgoing Shares' });
    expectWrite('quick pick');
    expectSavedGrant('quick pick');
  });

  it.each(
    [false, true].flatMap((applied) => [403, 429, 503].map((status) => ({ applied, status }))),
  )(
    'rereads uncertain creation (applied: $applied, HTTP $status) without retrying the grant',
    async ({ applied, status }) => {
      applyWrite = applied;
      writeReply = async () =>
        json({ error: 'Synthetic acknowledgement unavailable' }, status, {
          'X-CSRF-Token': 'synthetic-refresh-must-not-replay',
          'Retry-After': '0.001',
        });
      await openPanel();
      fillForm('quick pick', 'editor');
      fireEvent.click(createButton());
      const reload = await screen.findByRole('button', { name: 'Reload sharing' });
      expect(screen.getByRole('alert')).toHaveTextContent(/could not be confirmed/i);
      expect(screen.queryByRole('button', { name: 'Create Share' })).not.toBeInTheDocument();
      expect(inventoryReads()).toHaveLength(5);
      expect(notices.success).not.toHaveBeenCalled();
      expect(notices.error).toHaveBeenCalledTimes(1);
      expectWrite('quick pick', 'editor');
      fireEvent.click(reload);
      await waitFor(() => expect(inventoryReads()).toHaveLength(10));
      await screen.findByRole('heading', { name: 'Outgoing Shares' });
      if (applied) expectSavedGrant('quick pick', 'editor');
      else {
        expect(outgoing).toEqual([]);
        expect(
          screen
            .getAllByRole('row')
            .some((row) => within(row).queryByText(`vm:${selectedResourceID}`)),
        ).toBe(false);
      }
      expect(targetInput()).toHaveAccessibleDescription(`${target.displayName} · ID: ${target.id}`);
      expect(resourceInput()).toHaveAccessibleDescription(
        `Resource: Namesake workload · vm:${selectedResourceID}`,
      );
      // Explicit recovery is a GET snapshot, not a second POST or fabricated success.
      expectWrite('quick pick', 'editor');
      expect(notices.success).not.toHaveBeenCalled();
      expect(getOrgID()).toBe(orgID);
    },
  );

  it('disables the whole form through cold CSRF preparation and dispatches the confirmed body once', async () => {
    const health = deferred<Response>();
    const reply = deferred<Response>();
    healthReply = () => health.promise;
    writeReply = () => reply.promise;
    document.cookie = 'pulse_csrf=; Max-Age=0; Path=/';
    await openPanel();
    fillForm('manual entry', 'admin');
    fireEvent.click(createButton());
    await waitFor(() => expect(healthReads()).toHaveLength(1));
    expect(writes()).toHaveLength(0);
    for (const input of [...screen.getAllByRole('combobox'), ...screen.getAllByRole('textbox')])
      expect(input).toBeDisabled();
    expect(screen.getByRole('button', { name: 'Saving...' })).toBeDisabled();
    document.cookie = `pulse_csrf=${csrf}; Path=/`;
    health.resolve(json({}));
    await waitFor(() => expect(writes()).toHaveLength(1));
    expectWrite('manual entry', 'admin');
    reply.resolve(new Response(null, { status: 204 }));
    await waitFor(() => expect(inventoryReads()).toHaveLength(10));
    await screen.findByRole('heading', { name: 'Outgoing Shares' });
    expectSavedGrant('manual entry', 'admin');
    expect(healthReads()).toHaveLength(1);
    expectWrite('manual entry', 'admin');
  });
});

// Keep the real capability producer, normalization and store connected to both
// input paths, including work still waiting on the client's CSRF preparation.
describe('Create Share through real capability withdrawal', () => {
  it.each(
    entries.flatMap((entry) =>
      (['withdrawn', 'failed', 'malformed'] as const).map((kind) => ({ entry, kind })),
    ),
  )('retires a cold $entry grant on $kind capability before dispatch', async ({ entry, kind }) => {
    const health = hold(json({}));
    healthReply = () => health.promise;
    await openPanel();
    fillForm(entry, 'admin');
    document.cookie = 'pulse_csrf=; Max-Age=0; Path=/';
    fireEvent.click(createButton());
    await waitFor(() => expect(healthReads()).toHaveLength(1));
    expect(writes()).toHaveLength(0);
    expect(screen.getByRole('button', { name: 'Saving...' })).toBeDisabled();
    vi.useFakeTimers();
    await capabilities(kind);
    expectWithdrawn();
    expect(Boolean(runtimeCapabilitiesLoadError())).toBe(kind === 'failed');
    document.cookie = `pulse_csrf=${csrf}; Path=/`;
    health.resolve(json({}));
    await vi.advanceTimersByTimeAsync(25);
    vi.useRealTimers();
    expect(writes()).toHaveLength(0);
    expect(outgoing).toEqual([]);
    expect(inventoryReads()).toHaveLength(5);
    expectWithdrawn();
    await capabilities('enabled');
    await waitFor(() => expect(inventoryReads()).toHaveLength(10));
    await screen.findByRole('heading', { name: 'Create Share' });
    expect(writes()).toHaveLength(0);
    expect(outgoing).toEqual([]);
    expect(targetInput()).toHaveValue(target.id);
    expect(screen.getByRole('combobox', { name: 'Access Role' })).toHaveValue('admin');
    if (entry === 'quick pick') expect(resourceInput()).toHaveValue(quickPickValue);
    else expect(screen.getByLabelText('Resource ID')).toHaveValue(` ${manualResourceID} `);
    expect(createButton()).toBeEnabled();
    expect(notices.success).not.toHaveBeenCalled();
    expect(notices.error).not.toHaveBeenCalled();
    expectBoundRequests();
  });

  it('ignores a superseded grant while a cold manual share stays retired', async () => {
    const health = hold(json({}));
    healthReply = () => health.promise;
    await openPanel();
    fillForm('manual entry', 'admin');
    document.cookie = 'pulse_csrf=; Max-Age=0; Path=/';
    fireEvent.click(createButton());
    await waitFor(() => expect(healthReads()).toHaveLength(1));
    const oldReply = hold(json(capabilityBody(false)));
    const oldLoad = refresh(() => oldReply.promise);
    await capabilities('withdrawn');
    expectWithdrawn();
    oldReply.resolve(json(capabilityBody(true)));
    await oldLoad;
    expectWithdrawn();
    vi.useFakeTimers();
    document.cookie = `pulse_csrf=${csrf}; Path=/`;
    health.resolve(json({}));
    await vi.advanceTimersByTimeAsync(25);
    vi.useRealTimers();
    expect(writes()).toHaveLength(0);
    expect(outgoing).toEqual([]);
    expect(inventoryReads()).toHaveLength(5);
    expectWithdrawn();
  });

  it.each(['success', 'unreadable'] as const)(
    'reads an applied grant without importing its late %s body into the new draft',
    async (outcome) => {
      const body = hold('');
      const response = json({});
      const read = vi.spyOn(response, 'text').mockImplementation(() => body.promise);
      writeReply = async () => response;
      await openPanel();
      fillForm('quick pick', 'editor');
      fireEvent.click(createButton());
      await waitFor(() => expect(read).toHaveBeenCalledOnce());
      expectWrite('quick pick', 'editor');
      const signal = writes()[0][1]?.signal;
      expect(signal?.aborted).toBe(false);
      expect(outgoing[0]).toMatchObject(payloadFor('quick pick', 'editor'));
      await capabilities('withdrawn');
      expectWithdrawn();
      expect(signal?.aborted).toBe(true);
      await capabilities('enabled');
      await waitFor(() => expect(inventoryReads()).toHaveLength(10));
      await screen.findByRole('heading', { name: 'Outgoing Shares' });
      expectSavedGrant('quick pick', 'editor');
      fillForm('manual entry', 'admin');
      fireEvent.input(screen.getByLabelText('Resource ID'), {
        target: { value: 'current::draft/independent' },
      });
      body.resolve(outcome === 'success' ? '{}' : '{"synthetic-private-partial":');
      await turn();
      expect(targetInput()).toHaveValue(target.id);
      expect(screen.getByLabelText('Resource ID')).toHaveValue('current::draft/independent');
      expect(screen.getByRole('combobox', { name: 'Access Role' })).toHaveValue('admin');
      expect(createButton()).toBeEnabled();
      expectSavedGrant('quick pick', 'editor');
      expect(inventoryReads()).toHaveLength(10);
      expectWrite('quick pick', 'editor');
      expect(notices.success).not.toHaveBeenCalled();
      expect(notices.error).not.toHaveBeenCalled();
    },
  );
});
