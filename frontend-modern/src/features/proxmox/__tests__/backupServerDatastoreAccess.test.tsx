import { cleanup, fireEvent, render, screen, waitFor, within } from '@solidjs/testing-library';
import { Route, Router } from '@solidjs/router';
import { createSignal, type JSX } from 'solid-js';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { ProxmoxBackupServersTable } from '../ProxmoxBackupServersTable';
import { ProxmoxBackupsTable } from '../ProxmoxBackupsTable';
import type { Resource, ResourcePBSDatastore } from '@/types/resource';
import { resetCreateNonSuspendingQueryCacheForTest } from '@/hooks/createNonSuspendingQuery';
import { protectionPolicy } from '@/utils/__fixtures__/protectionPosture';

const api = vi.hoisted(() => ({ fetch: vi.fn(), json: vi.fn() }));
vi.mock('@/utils/apiClient', () => ({ apiFetch: api.fetch, apiFetchJSON: api.json }));

const reader = 'backup-reader-with-a-long-shared-prefix-east';
const store = 'production-archives-with-a-long-shared-prefix-east';
const otherStore = 'production-archives-with-a-long-shared-prefix-west';
const datastore: ResourcePBSDatastore = {
  name: store,
  total: 1000,
  used: 980,
  available: 20,
  status: 'available',
};
const server = (datastores: ResourcePBSDatastore[] = [datastore]): Resource => ({
  id: 'pbs-east',
  name: reader,
  displayName: reader,
  type: 'pbs',
  status: 'online',
  sourceType: 'api',
  platformType: 'proxmox-pbs',
  platformId: 'pbs-east',
  lastSeen: Date.parse('2026-10-30T19:00:00Z'),
  pbs: { instanceId: 'service-pbs-east', connectionHealth: 'healthy', datastores },
});
const mount = (component: () => JSX.Element) =>
  render(() => (
    <Router>
      <Route path="/*" component={component} />
    </Router>
  ));
const selected = () => screen.getByRole('region', { name: 'Selected PBS datastore' });
const field = (name: string) => within(selected()).getByText(name).closest('tr')!;
const disclosure = (name = store) =>
  screen.getByRole('button', {
    name: `Expand details for ${reader} · datastore ${name}`,
  });
const json = (data: unknown, status = 200) =>
  new Response(JSON.stringify(data), {
    status,
    headers: { 'Content-Type': 'application/json' },
  });

beforeEach(() => {
  resetCreateNonSuspendingQueryCacheForTest();
  window.history.replaceState({}, '', '/proxmox/backups/date');
  api.json.mockResolvedValue({ data: [], policy: protectionPolicy });
  api.fetch.mockImplementation(async () => json({}));
});
afterEach(() => {
  cleanup();
  resetCreateNonSuspendingQueryCacheForTest();
  vi.useRealTimers();
  vi.resetAllMocks();
});

describe('selected PBS datastore inspection', () => {
  it.each([320, 600, 1440])(
    'exposes the selected datastore without relying on hover at %ipx',
    async (width) => {
      mount(() => (
        <ProxmoxBackupServersTable
          servers={[server([datastore, { ...datastore, name: otherStore, used: 0 }])]}
          layoutWidth={() => width}
        />
      ));
      const east = await screen.findByRole('button', {
        name: `Expand details for ${reader} · datastore ${store}`,
      });
      expect(east).not.toBe(disclosure(otherStore));
      fireEvent.click(east);
      expect(field('Backup reader')).toHaveTextContent(reader);
      expect(field('Datastore')).toHaveTextContent(store);
      expect(field('Datastore')).not.toHaveTextContent(otherStore);
      expect(field('Capacity')).toHaveTextContent('98.0% used · 980 B / 1000 B');
      expect(field('Backup entries')).toHaveTextContent('0');
      expect(selected()).toHaveTextContent('They do not establish backup protection.');
      expect(document.getElementById(east.getAttribute('aria-controls')!)).toContainElement(
        selected(),
      );
      fireEvent.click(disclosure(otherStore));
      expect(screen.getAllByRole('region', { name: 'Selected PBS datastore' })).toHaveLength(1);
      expect(field('Datastore')).toHaveTextContent(otherStore);
      expect(field('Capacity')).toHaveTextContent('0.0% used · 0 B / 1000 B');
    },
  );

  it.each([
    ['unavailable', { ...datastore, used: 0, total: 0, status: 'unavailable' }, 'Unavailable'],
    ['unrecognised', { ...datastore, used: 0, status: 'unknown' }, 'Unknown'],
    ['invalid', { ...datastore, total: 0 }, 'Unknown'],
  ] as const)(
    'keeps %s capacity independent from a healthy connection and a genuine zero count',
    async (_name, data, label) => {
      mount(() => <ProxmoxBackupServersTable servers={[server([data])]} layoutWidth={() => 320} />);
      fireEvent.click(await screen.findByRole('button', { name: /Expand details for/ }));
      expect(field('Capacity')).toHaveTextContent(
        `${label}. This does not mean the datastore is empty.`,
      );
      expect(field('Capacity')).not.toHaveTextContent('0.0%');
      expect(field('Capacity')).not.toHaveTextContent('0 B');
      expect(field('Backup entries')).toHaveTextContent('0');
      expect(screen.getAllByText('Healthy').length).toBeGreaterThan(0);
    },
  );

  it('does not invent a selected store or count when no datastore was reported', async () => {
    mount(() => <ProxmoxBackupServersTable servers={[server([])]} layoutWidth={() => 320} />);
    fireEvent.click(await screen.findByRole('button', { name: `Expand details for ${reader}` }));
    expect(field('Datastore')).toHaveTextContent('Not reported');
    expect(field('Capacity')).toHaveTextContent('No datastore data');
    expect(field('Backup entries')).toHaveTextContent('Not reported');
  });

  it('keeps the open datastore and its DOM while live stores reorder and its capacity becomes unavailable', async () => {
    const [servers, setServers] = createSignal([
      server([datastore, { ...datastore, name: otherStore }]),
    ]);
    mount(() => <ProxmoxBackupServersTable servers={servers()} layoutWidth={() => 320} />);
    const toggle = await screen.findByRole('button', { name: /Expand details.*datastore.*east$/ });
    fireEvent.click(toggle);
    toggle.focus();
    const context = selected();
    setServers([
      server([
        { ...datastore, name: 'a-new-store' },
        { ...datastore, name: otherStore },
        { ...datastore, used: 0, total: 0, status: 'unavailable' },
      ]),
    ]);
    await waitFor(() => expect(field('Capacity')).toHaveTextContent('Unavailable.'));
    expect(selected()).toBe(context);
    expect(field('Datastore')).toHaveTextContent(store);
    expect(toggle).toHaveFocus();
    const detail = document.getElementById(toggle.getAttribute('aria-controls')!)!;
    const close = within(detail).getByRole('button', { name: /Collapse .* details/ });
    close.focus();
    fireEvent.click(close);
    await waitFor(() => expect(toggle).toHaveFocus());
    expect(screen.queryByRole('region', { name: 'Selected PBS datastore' })).toBeNull();
  });

  it('consumes real inventory read states, retries and poll counts inside the same open datastore', async () => {
    vi.useFakeTimers({ toFake: ['setInterval', 'clearInterval'] });
    let resolvePBS!: (response: Response) => void;
    let pbs = new Promise<Response>((resolve) => {
      resolvePBS = resolve;
    });
    api.fetch.mockImplementation(async (url: string) =>
      url.endsWith('/pbs')
        ? pbs
        : json({ data: { backupTasks: [], storageBackups: [], guestSnapshots: [] } }),
    );
    const mounted = mount(() => <ProxmoxBackupsTable emptyIcon={<span />} servers={[server()]} />);
    fireEvent.click(await screen.findByRole('button', { name: /Expand details.*datastore/ }));
    const context = selected();
    expect(field('Backup entries')).toHaveTextContent('Loading');
    expect(field('Capacity')).toHaveTextContent('98.0%');
    resolvePBS(json({}, 403));
    await waitFor(() => expect(field('Backup entries')).toHaveTextContent('Unavailable'));
    pbs = Promise.resolve(json({ data: { backups: [] } }));
    fireEvent.click(screen.getByRole('button', { name: 'Retry PBS inventory' }));
    await waitFor(() => expect(field('Backup entries')).toHaveTextContent('0'));
    const entry = {
      id: 'east-1',
      instance: reader,
      datastore: store,
      vmid: '101',
      backupType: 'vm',
      backupTime: '2026-10-30T18:00:00Z',
      files: ['index.json.blob'],
      verified: false,
      inProgress: true,
    };
    pbs = Promise.resolve(
      json({
        data: {
          backups: [
            entry,
            { ...entry, id: 'east-2', inProgress: false },
            { ...entry, id: 'west-1', datastore: otherStore },
          ],
        },
      }),
    );
    await vi.advanceTimersByTimeAsync(30_000);
    await waitFor(() => expect(field('Backup entries')).toHaveTextContent('2'));
    expect(selected()).toBe(context);
    expect(field('Datastore')).toHaveTextContent(store);
    expect(api.fetch.mock.calls.filter(([url]) => url === '/api/backups/pbs')).toHaveLength(3);
    const calls = api.fetch.mock.calls.length;
    mounted.unmount();
    await vi.advanceTimersByTimeAsync(60_000);
    expect(api.fetch).toHaveBeenCalledTimes(calls);
  });
});
