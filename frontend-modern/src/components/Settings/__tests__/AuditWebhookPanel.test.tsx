import { cleanup, fireEvent, render, screen, waitFor } from '@solidjs/testing-library';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { createSignal } from 'solid-js';
import { eventBus } from '@/stores/events';
import { AuditWebhookPanel } from '../AuditWebhookPanel';
import { useAuditWebhookPanelState } from '../useAuditWebhookPanelState';

const apiFetchJSON = vi.fn();
const showSuccess = vi.fn();
const showWarning = vi.fn();
const existingUrl = 'https://existing.example.test/events';
const addedUrl = 'https://new.example.test/events';
let selectedOrg: string | null = 'tenant-a';
const [enabled, setEnabled] = createSignal(true);

vi.mock('@/utils/apiClient', () => ({
  apiFetchJSON: (...args: unknown[]) => apiFetchJSON(...args),
  getOrgID: () => selectedOrg,
}));
vi.mock('@/utils/toast', () => ({
  showSuccess: (...args: unknown[]) => showSuccess(...args),
  showWarning: (...args: unknown[]) => showWarning(...args),
}));
vi.mock('@/utils/logger', () => ({ logger: { error: vi.fn() } }));
vi.mock('@/stores/license', () => ({
  getRuntimeCapabilityBlock: () => undefined,
  hasFeature: () => enabled(),
  loadRuntimeCapabilities: vi.fn().mockResolvedValue(undefined),
}));
vi.mock('@/stores/licenseCommercial', () => ({
  getUpgradeActionDestination: () => ({
    href: 'https://plans.example.test/upgrade',
    external: true,
  }),
}));
vi.mock('@/stores/sessionPresentationPolicy', () => ({
  presentationPolicyHidesUpgradePrompts: () => false,
}));

const writeCalls = () =>
  apiFetchJSON.mock.calls.filter(([, options]) => options?.method === 'POST');
const openPanel = async (canManage = true) => {
  render(() => <AuditWebhookPanel canManage={canManage} />);
  await screen.findByText(existingUrl);
  return screen.getByRole('textbox', { name: 'Audit webhook URL' });
};

beforeEach(() => {
  selectedOrg = 'tenant-a';
  setEnabled(true);
  apiFetchJSON.mockReset().mockResolvedValue({ urls: [existingUrl] });
  showSuccess.mockReset();
  showWarning.mockReset();
});
afterEach(cleanup);

const defer = <T,>() => {
  let resolve!: (value: T) => void;
  let reject!: (error: Error) => void;
  const promise = new Promise<T>((yes, no) => {
    resolve = yes;
    reject = no;
  });
  return { promise, resolve, reject };
};

const emptyMessage = 'No audit webhooks configured yet.';
const readErrorMessage = 'Audit webhook configuration could not be loaded.';

describe('audit webhook read admission', () => {
  it('blocks Enter and button saves while the selected configuration has not arrived', async () => {
    const read = defer<{ urls: string[] }>();
    apiFetchJSON.mockReturnValueOnce(read.promise);
    render(() => <AuditWebhookPanel />);
    const input = screen.getByRole('textbox', { name: 'Audit webhook URL' });
    fireEvent.input(input, { target: { value: addedUrl } });
    expect(screen.getByText('Loading audit webhooks…')).toBeInTheDocument();
    expect(screen.queryByText(emptyMessage)).not.toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Add Endpoint' })).toBeDisabled();
    fireEvent.keyDown(input, { key: 'Enter' });
    expect(writeCalls()).toHaveLength(0);
    read.resolve({ urls: [existingUrl] });
    await screen.findByText(existingUrl);
    expect(input).toHaveValue(addedUrl);
    expect(screen.getByRole('button', { name: 'Add Endpoint' })).toBeEnabled();
    expect(writeCalls()).toHaveLength(0);
  });

  it('keeps failure unknown, preserves the draft and reads on explicit retry without saving', async () => {
    apiFetchJSON.mockRejectedValueOnce(new Error('private backend text must not be rendered'));
    render(() => <AuditWebhookPanel />);
    await screen.findByText(readErrorMessage);
    const input = screen.getByRole('textbox', { name: 'Audit webhook URL' });
    fireEvent.input(input, { target: { value: `  ${addedUrl}  ` } });
    fireEvent.keyDown(input, { key: 'Enter' });
    expect(screen.queryByText(emptyMessage)).not.toBeInTheDocument();
    expect(screen.getByText(/Existing destinations are unknown/)).toBeInTheDocument();
    expect(screen.queryByText(/private backend text/)).not.toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Add Endpoint' })).toBeDisabled();
    expect(apiFetchJSON).toHaveBeenCalledTimes(1);
    expect(showSuccess).not.toHaveBeenCalled();
    const retry = defer<{ urls: string[] }>();
    apiFetchJSON.mockReturnValueOnce(retry.promise);
    fireEvent.click(screen.getByRole('button', { name: 'Retry loading' }));
    eventBus.emit('org_switched', 'tenant-a');
    fireEvent.keyDown(input, { key: 'Enter' });
    expect(apiFetchJSON).toHaveBeenCalledTimes(2);
    expect(writeCalls()).toHaveLength(0);
    expect(input).toHaveValue(`  ${addedUrl}  `);
    retry.resolve({ urls: [existingUrl] });
    await screen.findByText(existingUrl);
    expect(screen.queryByText(readErrorMessage)).not.toBeInTheDocument();
    expect(input).toHaveValue(`  ${addedUrl}  `);
    expect(writeCalls()).toHaveLength(0);
    fireEvent.click(screen.getByRole('button', { name: 'Add Endpoint' }));
    await screen.findByText(addedUrl);
    expect(writeCalls()).toHaveLength(1);
    expect(JSON.parse(writeCalls()[0][1].body)).toEqual({ urls: [existingUrl, addedUrl] });
    for (const [, options] of apiFetchJSON.mock.calls) {
      expect(options).toMatchObject({ expectedOrgID: 'tenant-a', retry: false });
      expect(options.signal).toBeInstanceOf(AbortSignal);
    }
  });

  it('retains last loaded destinations and the draft across a failed same-selection refresh', async () => {
    const input = await openPanel();
    fireEvent.input(input, { target: { value: addedUrl } });
    const refresh = defer<{ urls: string[] }>();
    apiFetchJSON.mockReturnValueOnce(refresh.promise);
    eventBus.emit('org_switched', 'tenant-a');
    expect(screen.getByText(existingUrl)).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Remove webhook endpoint' })).toBeDisabled();
    fireEvent.keyDown(input, { key: 'Enter' });
    refresh.reject(new Error('read unavailable'));
    await screen.findByText(readErrorMessage);
    expect(screen.getByText(/Last loaded destinations are shown below/)).toBeInTheDocument();
    expect(screen.getByText(existingUrl)).toBeInTheDocument();
    expect(input).toHaveValue(addedUrl);
    expect(screen.getByRole('button', { name: 'Remove webhook endpoint' })).toBeDisabled();
    expect(writeCalls()).toHaveLength(0);
    fireEvent.click(screen.getByRole('button', { name: 'Retry loading' }));
    await waitFor(() => expect(screen.queryByText(readErrorMessage)).not.toBeInTheDocument());
    await waitFor(() => expect(screen.getByRole('button', { name: 'Add Endpoint' })).toBeEnabled());
    expect(writeCalls()).toHaveLength(0);
  });

  it.each([[], null])(
    'accepts explicit known-empty %j and still requires a deliberate save',
    async (urls) => {
      apiFetchJSON.mockResolvedValueOnce({ urls });
      render(() => <AuditWebhookPanel />);
      await screen.findByText(emptyMessage);
      const input = screen.getByRole('textbox', { name: 'Audit webhook URL' });
      fireEvent.input(input, { target: { value: addedUrl } });
      expect(screen.getByRole('button', { name: 'Add Endpoint' })).toBeEnabled();
      expect(writeCalls()).toHaveLength(0);
      fireEvent.keyDown(input, { key: 'Enter' });
      await screen.findByText(addedUrl);
      expect(JSON.parse(writeCalls()[0][1].body)).toEqual({ urls: [addedUrl] });
    },
  );

  it.each([undefined, null, {}, { urls: false }, { urls: '' }, { urls: {} }, { urls: [false] }])(
    'does not mistake a malformed response for an empty selection: %j',
    async (response) => {
      apiFetchJSON.mockResolvedValueOnce(response);
      render(() => <AuditWebhookPanel />);
      await screen.findByText(readErrorMessage);
      const input = screen.getByRole('textbox', { name: 'Audit webhook URL' });
      fireEvent.input(input, { target: { value: addedUrl } });
      fireEvent.keyDown(input, { key: 'Enter' });
      expect(screen.queryByText(emptyMessage)).not.toBeInTheDocument();
      expect(screen.getByRole('button', { name: 'Add Endpoint' })).toBeDisabled();
      expect(writeCalls()).toHaveLength(0);
    },
  );

  it.each(['resolve', 'reject'] as const)(
    'ignores an obsolete read %s after a tenant switch and allows deliberate default recovery',
    async (finish) => {
      const oldRead = defer<{ urls: string[] }>();
      apiFetchJSON.mockReturnValueOnce(oldRead.promise);
      render(() => <AuditWebhookPanel />);
      selectedOrg = 'tenant-b';
      apiFetchJSON.mockRejectedValueOnce(new Error('new tenant unavailable'));
      eventBus.emit('org_switched', 'tenant-b');
      await screen.findByText(readErrorMessage);
      if (finish === 'resolve') oldRead.resolve({ urls: [existingUrl] });
      else oldRead.reject(new Error('old tenant unavailable'));
      await Promise.resolve();
      expect(screen.queryByText(existingUrl)).not.toBeInTheDocument();
      expect(screen.getByText(readErrorMessage)).toBeInTheDocument();
      expect(apiFetchJSON.mock.calls[0][1].signal.aborted).toBe(true);
      expect(apiFetchJSON).toHaveBeenCalledTimes(2);
      selectedOrg = null;
      apiFetchJSON.mockResolvedValueOnce({ urls: [] });
      eventBus.emit('org_switched', 'default');
      await screen.findByText(emptyMessage);
      expect(apiFetchJSON.mock.calls[2][1]).toMatchObject({
        expectedOrgID: 'default',
        retry: false,
      });
      const input = screen.getByRole('textbox', { name: 'Audit webhook URL' });
      fireEvent.input(input, { target: { value: addedUrl } });
      fireEvent.keyDown(input, { key: 'Enter' });
      await screen.findByText(addedUrl);
      expect(writeCalls()[0][1]).toMatchObject({ expectedOrgID: 'default', retry: false });
      expect(JSON.parse(writeCalls()[0][1].body)).toEqual({ urls: [addedUrl] });
    },
  );

  it('rejects a stale removal confirmation when org storage changes without an event', async () => {
    await openPanel();
    fireEvent.click(screen.getByRole('button', { name: 'Remove webhook endpoint' }));
    selectedOrg = 'tenant-b';
    fireEvent.click(screen.getByRole('button', { name: 'Remove webhook' }));
    expect(writeCalls()).toHaveLength(0);
    expect(screen.queryByText(existingUrl)).not.toBeInTheDocument();
    await screen.findByText(readErrorMessage);
    expect(screen.getByRole('button', { name: 'Add Endpoint' })).toBeDisabled();
  });

  it('discards a pending save result after selection changes without replaying the mutation', async () => {
    const input = await openPanel();
    const save = defer<object>();
    apiFetchJSON.mockReturnValueOnce(save.promise);
    fireEvent.input(input, { target: { value: addedUrl } });
    fireEvent.keyDown(input, { key: 'Enter' });
    selectedOrg = 'tenant-b';
    apiFetchJSON.mockResolvedValueOnce({ urls: [] });
    eventBus.emit('org_switched', 'tenant-b');
    save.resolve({});
    await screen.findByText(emptyMessage);
    expect(screen.queryByText(existingUrl)).not.toBeInTheDocument();
    expect(screen.queryByText(addedUrl)).not.toBeInTheDocument();
    expect(input).toHaveValue('');
    expect(showSuccess).not.toHaveBeenCalled();
    expect(writeCalls()).toHaveLength(1);
    expect(writeCalls()[0][1].signal.aborted).toBe(true);
    expect(apiFetchJSON).toHaveBeenCalledTimes(3);
  });

  it('invalidates a read on capability loss and does not admit its late result', async () => {
    const read = defer<{ urls: string[] }>();
    apiFetchJSON.mockReturnValueOnce(read.promise);
    render(() => <AuditWebhookPanel />);
    setEnabled(false);
    read.resolve({ urls: [existingUrl] });
    await Promise.resolve();
    expect(apiFetchJSON.mock.calls[0][1].signal.aborted).toBe(true);
    expect(screen.queryByText(existingUrl)).not.toBeInTheDocument();
    expect(screen.queryByRole('textbox', { name: 'Audit webhook URL' })).not.toBeInTheDocument();
    expect(writeCalls()).toHaveLength(0);
    setEnabled(true);
    await screen.findByText(existingUrl);
    expect(apiFetchJSON).toHaveBeenCalledTimes(2);
  });

  it('withdraws same-selection data and draft on capability loss and requires a fresh read', async () => {
    const input = await openPanel();
    fireEvent.input(input, { target: { value: addedUrl } });
    setEnabled(false);
    expect(screen.queryByText(existingUrl)).not.toBeInTheDocument();
    expect(screen.queryByRole('textbox', { name: 'Audit webhook URL' })).not.toBeInTheDocument();
    apiFetchJSON.mockRejectedValueOnce(new Error('read failed after capability refresh'));
    setEnabled(true);
    await screen.findByText(readErrorMessage);
    expect(screen.queryByText(existingUrl)).not.toBeInTheDocument();
    expect(screen.getByText(/Existing destinations are unknown/)).toBeInTheDocument();
    expect(screen.getByRole('textbox', { name: 'Audit webhook URL' })).toHaveValue('');
    expect(screen.getByRole('button', { name: 'Add Endpoint' })).toBeDisabled();
    expect(writeCalls()).toHaveLength(0);
  });

  it('requires a read after a same-selection save is interrupted by capability refresh', async () => {
    const input = await openPanel();
    const save = defer<object>();
    apiFetchJSON.mockReturnValueOnce(save.promise);
    fireEvent.input(input, { target: { value: addedUrl } });
    fireEvent.keyDown(input, { key: 'Enter' });
    setEnabled(false);
    apiFetchJSON.mockRejectedValueOnce(new Error('fresh read unavailable'));
    setEnabled(true);
    save.resolve({});
    await screen.findByText(readErrorMessage);
    expect(screen.queryByText(existingUrl)).not.toBeInTheDocument();
    expect(screen.getByRole('textbox', { name: 'Audit webhook URL' })).toHaveValue('');
    expect(screen.getByRole('button', { name: 'Add Endpoint' })).toBeDisabled();
    expect(
      screen.queryByRole('button', { name: 'Remove webhook endpoint' }),
    ).not.toBeInTheDocument();
    expect(showSuccess).not.toHaveBeenCalled();
    expect(apiFetchJSON).toHaveBeenCalledTimes(3);
    apiFetchJSON.mockResolvedValueOnce({ urls: [existingUrl, addedUrl] });
    fireEvent.click(screen.getByRole('button', { name: 'Retry loading' }));
    await screen.findByText(addedUrl);
    expect(writeCalls()).toHaveLength(1);
    expect(screen.getByRole('textbox', { name: 'Audit webhook URL' })).toHaveValue('');
    expect(showSuccess).not.toHaveBeenCalled();
  });

  it.each(['resolve', 'reject'] as const)(
    'invalidates a disposed read before its late %s',
    async (finish) => {
      const read = defer<{ urls: string[] }>();
      apiFetchJSON.mockReturnValueOnce(read.promise);
      const panel = render(() => <AuditWebhookPanel />);
      panel.unmount();
      if (finish === 'resolve') read.resolve({ urls: [existingUrl] });
      else read.reject(new Error('disposed read'));
      await Promise.resolve();
      eventBus.emit('org_switched', 'tenant-a');
      expect(apiFetchJSON).toHaveBeenCalledOnce();
      expect(apiFetchJSON.mock.calls[0][1].signal.aborted).toBe(true);
      expect(showSuccess).not.toHaveBeenCalled();
    },
  );

  it('allows a read-only retry but never grants write access after recovery', async () => {
    apiFetchJSON.mockRejectedValueOnce(new Error('read refused'));
    render(() => <AuditWebhookPanel canManage={false} />);
    await screen.findByText(readErrorMessage);
    fireEvent.click(screen.getByRole('button', { name: 'Retry loading' }));
    await screen.findByText(existingUrl);
    expect(screen.getByRole('textbox', { name: 'Audit webhook URL' })).toBeDisabled();
    expect(screen.getByRole('button', { name: 'Remove webhook endpoint' })).toBeDisabled();
    expect(writeCalls()).toHaveLength(0);
  });
});

describe('audit webhook entry recovery', () => {
  it('preserves a refused entry and old destinations, then clears only after a successful retry', async () => {
    const input = await openPanel();
    apiFetchJSON.mockRejectedValueOnce(new Error('save refused')).mockResolvedValueOnce({});
    fireEvent.input(input, { target: { value: `  ${addedUrl}  ` } });
    fireEvent.keyDown(input, { key: 'Enter' });
    await screen.findByRole('alert');
    expect(screen.getByRole('alert')).toHaveTextContent('Failed to save webhook configuration');
    expect(input).toHaveValue(`  ${addedUrl}  `);
    expect(screen.getByText(existingUrl)).toBeInTheDocument();
    expect(screen.queryByText(addedUrl)).not.toBeInTheDocument();
    expect(showSuccess).not.toHaveBeenCalled();

    fireEvent.click(screen.getByRole('button', { name: 'Add Endpoint' }));
    await screen.findByText(addedUrl);
    expect(input).toHaveValue('');
    await screen.findByText('Audit webhooks updated');
    expect(showSuccess).not.toHaveBeenCalled();
    expect(writeCalls().map(([, options]) => JSON.parse(options.body))).toEqual([
      { urls: [existingUrl, addedUrl] },
      { urls: [existingUrl, addedUrl] },
    ]);
  });

  it('keeps a newer draft and rejects concurrent Enter/removal writes during a pending save', async () => {
    const input = await openPanel();
    let finishSave!: (value: object) => void;
    apiFetchJSON.mockReturnValueOnce(
      new Promise((resolve) => {
        finishSave = resolve;
      }),
    );
    fireEvent.input(input, { target: { value: addedUrl } });
    fireEvent.keyDown(input, { key: 'Enter' });
    await waitFor(() =>
      expect(screen.getByRole('button', { name: 'Add Endpoint' })).toBeDisabled(),
    );
    expect(screen.getByRole('button', { name: 'Remove webhook endpoint' })).toBeDisabled();
    fireEvent.input(input, { target: { value: 'https://next.example.test/events' } });
    fireEvent.keyDown(input, { key: 'Enter' });
    expect(writeCalls()).toHaveLength(1);
    finishSave({});
    await screen.findByText(addedUrl);
    expect(input).toHaveValue('https://next.example.test/events');
    expect(screen.getByRole('button', { name: 'Add Endpoint' })).toBeEnabled();
  });

  it('requires explicit removal and retains the endpoint and draft after a refused removal', async () => {
    const input = await openPanel();
    fireEvent.input(input, { target: { value: addedUrl } });
    fireEvent.click(screen.getByRole('button', { name: 'Remove webhook endpoint' }));
    fireEvent.click(screen.getByRole('button', { name: 'Cancel' }));
    expect(writeCalls()).toHaveLength(0);
    expect(screen.getByText(existingUrl)).toBeInTheDocument();

    apiFetchJSON.mockRejectedValueOnce(new Error('save refused')).mockResolvedValueOnce({});
    fireEvent.click(screen.getByRole('button', { name: 'Remove webhook endpoint' }));
    fireEvent.click(screen.getByRole('button', { name: 'Remove webhook' }));
    await screen.findByRole('alert');
    expect(screen.getByRole('dialog', { name: 'Remove audit webhook' })).toBeInTheDocument();
    expect(screen.getAllByText(existingUrl)).toHaveLength(2);
    expect(input).toHaveValue(addedUrl);
    fireEvent.click(screen.getByRole('button', { name: 'Remove webhook' }));
    await waitFor(() => expect(screen.queryByText(existingUrl)).not.toBeInTheDocument());
    expect(input).toHaveValue(addedUrl);
    expect(writeCalls().map(([, options]) => JSON.parse(options.body))).toEqual([
      { urls: [] },
      { urls: [] },
    ]);
  });

  it.each(['not a URL', existingUrl])(
    'does not write an invalid or duplicate entry: %s',
    async (draft) => {
      const input = await openPanel();
      fireEvent.input(input, { target: { value: draft } });
      fireEvent.keyDown(input, { key: 'Enter' });
      expect(screen.getByRole('alert')).toHaveTextContent(
        draft === existingUrl ? 'This URL is already configured' : 'Please enter a valid URL',
      );
      expect(showWarning).not.toHaveBeenCalled();
      expect(writeCalls()).toHaveLength(0);
      expect(input).toHaveValue(draft);
    },
  );

  it('does not allow a read-only account to write through Enter or removal', async () => {
    const input = await openPanel(false);
    expect(input).toBeDisabled();
    expect(screen.getByRole('button', { name: 'Add Endpoint' })).toBeDisabled();
    expect(screen.getByRole('button', { name: 'Remove webhook endpoint' })).toBeDisabled();
    fireEvent.keyDown(input, { key: 'Enter' });
    expect(writeCalls()).toHaveLength(0);
  });
});

describe('audit webhook reader lifetime', () => {
  it('never reads or exposes configuration without read admission, and recovers read-only', async () => {
    const [read, setRead] = createSignal(false);
    render(() => <AuditWebhookPanel currentUser="reader" canRead={read()} canManage={false} />);
    expect(
      screen.getByText('Audit webhook access is unavailable for this account.'),
    ).toBeInTheDocument();
    expect(screen.queryByRole('textbox')).not.toBeInTheDocument();
    expect(apiFetchJSON).not.toHaveBeenCalled();
    setRead(true);
    await screen.findByText(existingUrl);
    const input = screen.getByRole('textbox', { name: 'Audit webhook URL' });
    expect(input).toBeDisabled();
    fireEvent.keyDown(input, { key: 'Enter' });
    expect(writeCalls()).toHaveLength(0);
    setRead(false);
    expect(screen.queryByText(existingUrl)).not.toBeInTheDocument();
    expect(screen.queryByRole('textbox')).not.toBeInTheDocument();
    expect(apiFetchJSON).toHaveBeenCalledOnce();
  });

  it.each(['resolve', 'reject'] as const)(
    'rejects an old principal read %s even after that name returns',
    async (finish) => {
      const oldRead = defer<{ urls: string[] }>();
      const currentRead = defer<{ urls: string[] }>();
      const [user, setUser] = createSignal<string | undefined>('first');
      apiFetchJSON.mockReturnValueOnce(oldRead.promise);
      render(() => <AuditWebhookPanel currentUser={user()} />);
      apiFetchJSON.mockResolvedValueOnce({ urls: ['https://second.example.test/events'] });
      setUser('second');
      await screen.findByText('https://second.example.test/events');
      apiFetchJSON.mockReturnValueOnce(currentRead.promise);
      setUser('first');
      expect(screen.queryByText('https://second.example.test/events')).not.toBeInTheDocument();
      if (finish === 'resolve') oldRead.resolve({ urls: [existingUrl] });
      else oldRead.reject(new Error('private obsolete read error'));
      await Promise.resolve();
      expect(screen.queryByText(existingUrl)).not.toBeInTheDocument();
      expect(screen.getByText('Loading audit webhooks…')).toBeInTheDocument();
      expect(screen.queryByRole('alert')).not.toBeInTheDocument();
      expect(apiFetchJSON.mock.calls[0][1].signal.aborted).toBe(true);
      currentRead.resolve({ urls: [] });
      await screen.findByText(emptyMessage);
      expect(apiFetchJSON).toHaveBeenCalledTimes(3);
    },
  );

  it('withdraws a draft, confirmation and endpoint on write withdrawal before read-only recovery', async () => {
    const [manage, setManage] = createSignal(true);
    render(() => <AuditWebhookPanel currentUser="reader" canManage={manage()} />);
    await screen.findByText(existingUrl);
    fireEvent.input(screen.getByRole('textbox'), { target: { value: addedUrl } });
    fireEvent.click(screen.getByRole('button', { name: 'Remove webhook endpoint' }));
    const read = defer<{ urls: string[] }>();
    apiFetchJSON.mockReturnValueOnce(read.promise);
    setManage(false);
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
    expect(screen.queryByText(existingUrl)).not.toBeInTheDocument();
    expect(screen.getByRole('textbox')).toHaveValue('');
    expect(screen.getByRole('textbox')).toBeDisabled();
    read.resolve({ urls: [existingUrl] });
    await screen.findByText(existingUrl);
    expect(screen.getByRole('button', { name: 'Remove webhook endpoint' })).toBeDisabled();
    fireEvent.keyDown(screen.getByRole('textbox'), { key: 'Enter' });
    expect(writeCalls()).toHaveLength(0);
    setManage(true);
    await waitFor(() =>
      expect(screen.getByRole('button', { name: 'Remove webhook endpoint' })).toBeEnabled(),
    );
    expect(screen.getByRole('textbox')).toHaveValue('');
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
    expect(apiFetchJSON).toHaveBeenCalledTimes(3);
  });

  it.each(['resolve', 'reject'] as const)(
    'ignores an obsolete save %s without finishing a new reader read or erasing their draft',
    async (finish) => {
      const [user, setUser] = createSignal('first');
      render(() => <AuditWebhookPanel currentUser={user()} />);
      await screen.findByText(existingUrl);
      const save = defer<object>();
      apiFetchJSON.mockReturnValueOnce(save.promise);
      fireEvent.input(screen.getByRole('textbox'), { target: { value: addedUrl } });
      fireEvent.keyDown(screen.getByRole('textbox'), { key: 'Enter' });
      expect(writeCalls()).toHaveLength(1);
      const nextRead = defer<{ urls: string[] }>();
      apiFetchJSON.mockReturnValueOnce(nextRead.promise);
      setUser('second');
      expect(screen.queryByText(existingUrl)).not.toBeInTheDocument();
      expect(screen.getByRole('textbox')).toHaveValue('');
      fireEvent.input(screen.getByRole('textbox'), { target: { value: addedUrl } });
      if (finish === 'resolve') save.resolve({});
      else save.reject(new Error('private obsolete write refusal'));
      await Promise.resolve();
      expect(screen.getByText('Loading audit webhooks…')).toBeInTheDocument();
      expect(screen.getByRole('textbox')).toHaveValue(addedUrl);
      expect(screen.queryByRole('alert')).not.toBeInTheDocument();
      expect(screen.queryByText('Audit webhooks updated')).not.toBeInTheDocument();
      nextRead.resolve({ urls: [] });
      await screen.findByText(emptyMessage);
      expect(screen.getByRole('button', { name: 'Add Endpoint' })).toBeEnabled();
      expect(writeCalls()[0][1].signal.aborted).toBe(true);
      expect(apiFetchJSON).toHaveBeenCalledTimes(3);
      expect(showSuccess).not.toHaveBeenCalled();
      expect(showWarning).not.toHaveBeenCalled();
    },
  );

  it.each(['resolve', 'reject'] as const)(
    'withdraws an obsolete save %s when org storage drifts without an event',
    async (finish) => {
      const input = await openPanel();
      const save = defer<object>();
      apiFetchJSON.mockReturnValueOnce(save.promise);
      fireEvent.input(input, { target: { value: addedUrl } });
      fireEvent.keyDown(input, { key: 'Enter' });
      selectedOrg = 'tenant-b';
      if (finish === 'resolve') save.resolve({});
      else save.reject(new Error('private stale save failure'));
      await screen.findByText(readErrorMessage);
      expect(screen.queryByText(existingUrl)).not.toBeInTheDocument();
      expect(input).toHaveValue('');
      expect(screen.queryByText('Audit webhooks updated')).not.toBeInTheDocument();
      expect(screen.getByRole('button', { name: 'Add Endpoint' })).toBeDisabled();
      expect(apiFetchJSON).toHaveBeenCalledTimes(2);
      expect(showSuccess).not.toHaveBeenCalled();
      expect(showWarning).not.toHaveBeenCalled();
    },
  );

  it('keeps an unchanged-reader draft and confirmation through copied status props', async () => {
    const [reader, setReader] = createSignal({
      currentUser: 'reader',
      canRead: true,
      canManage: true,
    });
    render(() => <AuditWebhookPanel {...reader()} />);
    await screen.findByText(existingUrl);
    fireEvent.input(screen.getByRole('textbox'), { target: { value: addedUrl } });
    fireEvent.click(screen.getByRole('button', { name: 'Remove webhook endpoint' }));
    setReader({ ...reader() });
    expect(screen.getByRole('dialog', { name: 'Remove audit webhook' })).toBeInTheDocument();
    expect(screen.getByRole('textbox')).toHaveValue(addedUrl);
    expect(apiFetchJSON).toHaveBeenCalledOnce();
    fireEvent.click(screen.getByRole('button', { name: 'Cancel' }));
    expect(writeCalls()).toHaveLength(0);
  });

  it.each(['resolve', 'reject'] as const)(
    'erases owned state on disposal and ignores a late save %s',
    async (finish) => {
      let state!: ReturnType<typeof useAuditWebhookPanelState>;
      const panel = render(() => {
        state = useAuditWebhookPanelState({ currentUser: 'reader' });
        return null;
      });
      await waitFor(() => expect(state.webhookUrls()).toEqual([existingUrl]));
      state.setNewUrl(addedUrl);
      state.openRemoveWebhook(existingUrl);
      const save = defer<object>();
      apiFetchJSON.mockReturnValueOnce(save.promise);
      const pending = state.handleRemoveWebhook();
      panel.unmount();
      expect(state.webhookUrls()).toEqual([]);
      expect(state.newUrl()).toBe('');
      expect(state.pendingRemoval()).toBeNull();
      expect(state.actionError()).toBeNull();
      expect(state.successNotice()).toBeNull();
      expect(state.saving()).toBe(false);
      expect(state.hasLoadedConfiguration()).toBe(false);
      expect(state.canEdit()).toBe(false);
      expect(writeCalls()[0][1].signal.aborted).toBe(true);
      if (finish === 'resolve') save.resolve({});
      else save.reject(new Error('disposed write refusal'));
      await pending;
      state.setNewUrl(addedUrl);
      state.openRemoveWebhook(existingUrl);
      await state.handleAddWebhook();
      await state.fetchWebhooks();
      eventBus.emit('org_switched', 'tenant-b');
      expect(state.webhookUrls()).toEqual([]);
      expect(state.newUrl()).toBe('');
      expect(state.successNotice()).toBeNull();
      expect(state.actionError()).toBeNull();
      expect(apiFetchJSON).toHaveBeenCalledTimes(2);
      expect(showSuccess).not.toHaveBeenCalled();
      expect(showWarning).not.toHaveBeenCalled();
    },
  );

  it('withdraws local validation and success feedback with the principal', async () => {
    const [user, setUser] = createSignal('first');
    render(() => <AuditWebhookPanel currentUser={user()} />);
    await screen.findByText(existingUrl);
    fireEvent.input(screen.getByRole('textbox'), { target: { value: existingUrl } });
    fireEvent.keyDown(screen.getByRole('textbox'), { key: 'Enter' });
    expect(screen.getByRole('alert')).toHaveTextContent('This URL is already configured');
    setUser('second');
    await screen.findByText(existingUrl);
    expect(screen.queryByRole('alert')).not.toBeInTheDocument();
    fireEvent.input(screen.getByRole('textbox'), { target: { value: addedUrl } });
    fireEvent.keyDown(screen.getByRole('textbox'), { key: 'Enter' });
    await screen.findByText('Audit webhooks updated');
    setUser('third');
    await screen.findByText(existingUrl);
    expect(screen.queryByText('Audit webhooks updated')).not.toBeInTheDocument();
    expect(screen.getByRole('textbox')).toHaveValue('');
    expect(showSuccess).not.toHaveBeenCalled();
    expect(showWarning).not.toHaveBeenCalled();
  });
});
