import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, screen, waitFor } from '@solidjs/testing-library';
import { createSignal, type Accessor, type Setter } from 'solid-js';
import { ReportingPanel, type ReportingPanelProps } from '../ReportingPanel';
import { buildLegacyReportingCatalogFallback } from '../reportingCatalogModel';
import { eventBus } from '@/stores/events';

let org = 'default';
let enabled: Accessor<boolean>;
let setEnabled: Setter<boolean>;
const api = vi.fn();
vi.mock('@/utils/apiClient', async () => ({
  ...(await vi.importActual<typeof import('@/utils/apiClient')>('@/utils/apiClient')),
  getOrgID: () => org,
  apiFetch: (...args: unknown[]) => api(...args),
}));
vi.mock('@/stores/license', () => ({
  hasFeature: () => enabled(),
  runtimeCapabilitiesLoaded: () => true,
  loadRuntimeCapabilities: vi.fn(),
}));
vi.mock('@/stores/licenseCommercial', () => ({
  getUpgradeActionDestination: () => ({ href: '/pricing', external: false }),
}));
vi.mock('@/stores/sessionPresentationPolicy', () => ({
  presentationPolicyHidesUpgradePrompts: () => true,
}));
vi.mock('../ResourcePicker', () => ({
  ResourcePicker: () => <input aria-label="Fixture resource search" />,
}));

const catalog = {
  ...buildLegacyReportingCatalogFallback(),
  vmInventoryExport: {
    id: 'vm_inventory',
    title: 'VM Inventory Export',
    description: 'Current inventory',
    format: 'csv',
    exportEndpoint: '/api/admin/reports/inventory/vms/export',
    filenamePrefix: 'vm-inventory',
    filenameDateStyle: 'utc_yyyymmdd',
    columns: [{ key: 'name', label: 'Name', description: 'VM name' }],
  },
};
const schedule = (name = 'North private report') => ({
  id: 'reused-id',
  name,
  enabled: false,
  kind: 'vm_inventory',
  format: 'csv',
  cadence: { type: 'monthly', time: '09:00', timezone: 'UTC' },
  scope: { resources: [], tags: [] },
  delivery: {
    method: 'email',
    to: ['synthetic@example.invalid'],
    attach: true,
    save_to_disk: true,
  },
});
const json = (value: unknown, status = 200) => new Response(JSON.stringify(value), { status });
function deferred<T>() {
  let resolve!: (value: T) => void;
  const promise = new Promise<T>((done) => {
    resolve = done;
  });
  return { resolve, promise };
}
function mount() {
  const [props, setProps] = createSignal<ReportingPanelProps>({
    currentUser: 'operator-a',
    canRead: true,
  });
  const view = render(() => <ReportingPanel {...props()} />);
  return { ...view, setProps };
}
async function ready() {
  await screen.findByText('North private report', { exact: true });
}
async function draft() {
  await fireEvent.click(screen.getAllByRole('button', { name: 'Create schedule' })[0]);
  await screen.findByRole('textbox', { name: 'Schedule name' });
  await fireEvent.input(screen.getByRole('textbox', { name: 'Schedule name' }), {
    target: { value: 'Private draft' },
  });
  await fireEvent.input(screen.getByRole('textbox', { name: 'Report Title' }), {
    target: { value: 'Private title' },
  });
}
beforeEach(() => {
  org = 'default';
  [enabled, setEnabled] = createSignal(true);
  api.mockReset();
  api.mockImplementation((url: string) =>
    Promise.resolve(json(url.endsWith('/schedules') ? { schedules: [schedule()] } : catalog)),
  );
});
afterEach(() => {
  cleanup();
  vi.restoreAllMocks();
});

describe('Reporting reader lifetime', () => {
  it('keeps equal-reader drafts but destroys them on a principal change', async () => {
    const view = mount();
    await ready();
    await draft();
    const search = screen.getAllByLabelText('Fixture resource search')[0];
    await fireEvent.input(search, { target: { value: 'Private search' } });
    const count = api.mock.calls.length;
    view.setProps({ currentUser: 'operator-a', canRead: true });
    expect(screen.getByRole('textbox', { name: 'Schedule name' })).toHaveValue('Private draft');
    expect(search).toHaveValue('Private search');
    expect(api).toHaveBeenCalledTimes(count);
    view.setProps({ currentUser: 'operator-b', canRead: true });
    await ready();
    expect(screen.queryByRole('textbox', { name: 'Schedule name' })).not.toBeInTheDocument();
    expect(screen.getByRole('textbox', { name: 'Report Title' })).toHaveValue('');
    expect(screen.getByLabelText('Fixture resource search')).toHaveValue('');
  });

  it('withdraws a changed credential or scope context even for the same principal', async () => {
    const view = mount();
    await ready();
    await draft();
    view.setProps({ currentUser: 'operator-a', canRead: true, accessKey: 'replacement-scopes' });
    await ready();
    expect(screen.queryByRole('textbox', { name: 'Schedule name' })).not.toBeInTheDocument();
    expect(screen.getByRole('textbox', { name: 'Report Title' })).toHaveValue('');
  });

  it('retains a refused save for deliberate retry and keeps feedback inside its reader', async () => {
    api.mockImplementation((url: string, init: RequestInit) =>
      Promise.resolve(
        init?.method === 'POST'
          ? json({ error: 'Synthetic report save refused' }, 403)
          : json(url.endsWith('/schedules') ? { schedules: [schedule()] } : catalog),
      ),
    );
    const view = mount();
    await ready();
    await draft();
    await fireEvent.change(screen.getByRole('combobox', { name: 'Report type' }), {
      target: { value: 'vm_inventory' },
    });
    await fireEvent.click(screen.getByRole('button', { name: 'Save schedule' }));
    await screen.findByRole('alert');
    expect(screen.getByRole('textbox', { name: 'Schedule name' })).toHaveValue('Private draft');
    expect(api.mock.calls.filter(([, init]) => init?.method === 'POST')).toHaveLength(1);
    await fireEvent.click(screen.getByRole('button', { name: 'Dismiss reporting feedback' }));
    expect(screen.queryByRole('alert')).not.toBeInTheDocument();
    expect(screen.getByRole('textbox', { name: 'Schedule name' })).toHaveValue('Private draft');
    await fireEvent.click(screen.getByRole('button', { name: 'Save schedule' }));
    await waitFor(() =>
      expect(api.mock.calls.filter(([, init]) => init?.method === 'POST')).toHaveLength(2),
    );
    view.setProps({ currentUser: 'operator-b', canRead: true });
    await ready();
    expect(screen.queryByRole('alert')).not.toBeInTheDocument();
  });

  it('withdraws read access and entitlement without reviving the old editor', async () => {
    const view = mount();
    await ready();
    await draft();
    view.setProps({ currentUser: 'operator-a', canRead: false });
    expect(screen.queryByText('North private report')).not.toBeInTheDocument();
    expect(screen.queryByRole('textbox', { name: 'Schedule name' })).not.toBeInTheDocument();
    const count = api.mock.calls.length;
    document.dispatchEvent(new Event('visibilitychange'));
    expect(api).toHaveBeenCalledTimes(count);
    view.setProps({ currentUser: 'operator-a', canRead: true });
    await ready();
    await draft();
    setEnabled(false);
    await waitFor(() =>
      expect(screen.queryByRole('textbox', { name: 'Schedule name' })).not.toBeInTheDocument(),
    );
    expect(screen.queryByText('North private report')).not.toBeInTheDocument();
    setEnabled(true);
    await ready();
    expect(screen.getByRole('textbox', { name: 'Report Title' })).toHaveValue('');
  });

  it('rejects an old organisation read, even when an id is reused', async () => {
    const old = deferred<Response>();
    api.mockImplementation((url: string) =>
      url.endsWith('/schedules') ? old.promise : Promise.resolve(json(catalog)),
    );
    mount();
    await screen.findByText('Loading report schedules...');
    api.mockImplementation((url: string) =>
      Promise.resolve(
        json(url.endsWith('/schedules') ? { schedules: [schedule('South report')] } : catalog),
      ),
    );
    org = 'other';
    eventBus.emit('org_switched', org);
    await screen.findByText('South report');
    old.resolve(json({ schedules: [schedule()] }));
    await Promise.resolve();
    await Promise.resolve();
    expect(screen.queryByText('North private report')).not.toBeInTheDocument();
    expect(screen.getByText('South report')).toBeInTheDocument();
    expect(
      api.mock.calls.every(
        ([, init]) =>
          init.retry === false && init.expectedOrgID && init.signal instanceof AbortSignal,
      ),
    ).toBe(true);
  });

  it('does not publish a late saved response or close the next reader draft', async () => {
    const pending = deferred<Response>();
    api.mockImplementation((url: string, init: RequestInit) =>
      init?.method === 'POST'
        ? pending.promise
        : Promise.resolve(json(url.endsWith('/schedules') ? { schedules: [schedule()] } : catalog)),
    );
    const view = mount();
    await ready();
    await draft();
    // Inventory reports need no explicit resource selection.
    await fireEvent.change(screen.getByRole('combobox', { name: 'Report type' }), {
      target: { value: 'vm_inventory' },
    });
    await fireEvent.click(screen.getByRole('button', { name: 'Save schedule' }));
    await waitFor(() =>
      expect(api.mock.calls.filter(([, init]) => init?.method === 'POST')).toHaveLength(1),
    );
    const options = api.mock.calls.find(([, init]) => init?.method === 'POST')![1];
    view.setProps({ currentUser: 'operator-b', canRead: true });
    await ready();
    await draft();
    expect(options.signal.aborted).toBe(true);
    pending.resolve(json(schedule('Old saved response')));
    await Promise.resolve();
    await Promise.resolve();
    expect(screen.getByRole('textbox', { name: 'Schedule name' })).toHaveValue('Private draft');
    expect(screen.queryByText('Old saved response')).not.toBeInTheDocument();
    expect(screen.queryByText('Report schedule saved')).not.toBeInTheDocument();
  });

  it('does not download a body that completes after the workspace is withdrawn', async () => {
    const pending = deferred<Blob>();
    const response = new Response('synthetic');
    vi.spyOn(response, 'blob').mockReturnValue(pending.promise);
    api.mockImplementation((url: string) =>
      Promise.resolve(
        url.includes('/export')
          ? response
          : json(url.endsWith('/schedules') ? { schedules: [schedule()] } : catalog),
      ),
    );
    const click = vi.spyOn(HTMLAnchorElement.prototype, 'click');
    const view = mount();
    await ready();
    await fireEvent.click(screen.getByRole('button', { name: 'Export VM Inventory' }));
    await waitFor(() => expect(response.blob).toHaveBeenCalledOnce());
    view.setProps({ currentUser: 'operator-b', canRead: true });
    await ready();
    pending.resolve(new Blob(['PRIVATE SYNTHETIC BODY']));
    await Promise.resolve();
    await Promise.resolve();
    expect(click).not.toHaveBeenCalled();
    expect(screen.queryByRole('alert')).not.toBeInTheDocument();
    expect(screen.queryByText('VM inventory exported')).not.toBeInTheDocument();
  });

  it('keeps a malformed or refused schedule read unknown and offers explicit refresh', async () => {
    api.mockImplementation((url: string) =>
      Promise.resolve(json(url.endsWith('/schedules') ? {} : catalog)),
    );
    mount();
    await screen.findByText('Report schedules are unavailable. Refresh to try again.');
    expect(screen.queryByText('No scheduled reports are configured yet.')).not.toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Create schedule' })).toBeDisabled();
    api.mockImplementation((url: string) =>
      Promise.resolve(json(url.endsWith('/schedules') ? { schedules: [] } : catalog)),
    );
    await fireEvent.click(screen.getByRole('button', { name: 'Refresh' }));
    await screen.findByText('No scheduled reports are configured yet.');
    expect(screen.getAllByRole('button', { name: 'Create schedule' })[0]).toBeEnabled();
  });

  it('revalidates the reader after a delete confirmation and never changes another organisation', async () => {
    mount();
    await ready();
    vi.spyOn(window, 'confirm').mockImplementation(() => {
      org = 'other';
      return true;
    });
    await fireEvent.click(screen.getByLabelText('Details for North private report'));
    await fireEvent.click(screen.getByRole('button', { name: 'Delete North private report' }));
    expect(api.mock.calls.some(([, init]) => init?.method === 'DELETE')).toBe(false);
  });
});
