import { cleanup, fireEvent, render, screen, waitFor } from '@solidjs/testing-library';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { AuditWebhookPanel } from '../AuditWebhookPanel';

const apiFetchJSON = vi.fn();
const showSuccess = vi.fn();
const showWarning = vi.fn();
const existingUrl = 'https://existing.example.test/events';
const addedUrl = 'https://new.example.test/events';

vi.mock('@/utils/apiClient', () => ({
  apiFetchJSON: (...args: unknown[]) => apiFetchJSON(...args),
}));
vi.mock('@/utils/toast', () => ({
  showSuccess: (...args: unknown[]) => showSuccess(...args),
  showWarning: (...args: unknown[]) => showWarning(...args),
}));
vi.mock('@/utils/logger', () => ({ logger: { error: vi.fn() } }));
vi.mock('@/stores/license', () => ({
  getRuntimeCapabilityBlock: () => undefined,
  hasFeature: () => true,
  loadRuntimeCapabilities: vi.fn().mockResolvedValue(undefined),
}));
vi.mock('@/stores/licenseCommercial', () => ({
  getUpgradeActionDestination: () => ({ href: '/upgrade', external: false }),
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
  apiFetchJSON.mockReset().mockResolvedValue({ urls: [existingUrl] });
  showSuccess.mockReset();
  showWarning.mockReset();
});
afterEach(cleanup);

describe('audit webhook entry recovery', () => {
  it('preserves a refused entry and old destinations, then clears only after a successful retry', async () => {
    const input = await openPanel();
    apiFetchJSON.mockRejectedValueOnce(new Error('save refused')).mockResolvedValueOnce({});
    fireEvent.input(input, { target: { value: `  ${addedUrl}  ` } });
    fireEvent.keyDown(input, { key: 'Enter' });
    await waitFor(() =>
      expect(showWarning).toHaveBeenCalledWith('Failed to save webhook configuration'),
    );
    expect(input).toHaveValue(`  ${addedUrl}  `);
    expect(screen.getByText(existingUrl)).toBeInTheDocument();
    expect(screen.queryByText(addedUrl)).not.toBeInTheDocument();
    expect(showSuccess).not.toHaveBeenCalled();

    fireEvent.click(screen.getByRole('button', { name: 'Add Endpoint' }));
    await screen.findByText(addedUrl);
    expect(input).toHaveValue('');
    expect(showSuccess).toHaveBeenCalledOnce();
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
    await waitFor(() => expect(showWarning).toHaveBeenCalledOnce());
    expect(screen.getByText(existingUrl)).toBeInTheDocument();
    expect(input).toHaveValue(addedUrl);
    fireEvent.click(screen.getByRole('button', { name: 'Remove webhook endpoint' }));
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
      expect(showWarning).toHaveBeenCalledOnce();
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
