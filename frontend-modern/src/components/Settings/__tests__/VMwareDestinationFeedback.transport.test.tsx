import { cleanup, fireEvent, render, renderHook, screen, waitFor } from '@solidjs/testing-library';
import { createSignal } from 'solid-js';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { clearAuth, getOrgID, setOrgID } from '@/utils/apiClient';
import { notificationStore } from '@/stores/notifications';
import { eventBus } from '@/stores/events';
import { Dialog } from '@/components/shared/Dialog';
import { VMwareCredentialSlot } from '../ConnectionEditor/CredentialSlots/VMwareCredentialSlot';
import { useVMwareSettingsPanelState } from '../useVMwareSettingsPanelState';

vi.mock('@/stores/notifications', () => ({
  notificationStore: { success: vi.fn(), error: vi.fn(), warning: vi.fn() },
}));
vi.mock('@/utils/logger', () => ({ logger: { error: vi.fn(), warn: vi.fn(), debug: vi.fn() } }));

const saved = {
  id: 'saved-vcenter',
  name: 'Production VMware',
  host: 'https://vc.example.test',
  port: 443,
  username: 'monitor',
  password: '********',
  insecureSkipVerify: false,
  enabled: true,
  monitorVms: true,
  monitorHosts: false,
  monitorDatastores: true,
};
const json = (body: unknown, status = 200) =>
  new Response(JSON.stringify(body), {
    status,
    headers: { 'Content-Type': 'application/json', 'X-CSRF-Token': 'synthetic-no-replay' },
  });
const destinationFailure = {
  code: 'destination_not_allowed',
  message: 'This destination requires an instance administrator.',
};
const dialFailure = {
  code: 'vmware_connection_failed',
  message: 'Failed to connect to VMware vCenter',
  details: {
    category: 'invalid_config',
    error: 'This VMware destination requires an instance administrator.',
  },
};
const fetchMock = vi.fn<typeof fetch>();
const ready = async (state: ReturnType<typeof useVMwareSettingsPanelState>) => {
  await waitFor(() => expect(state.connections()).toHaveLength(1));
  state.openEditDialog(state.connections()[0]);
  state.updateForm({ name: 'Retained draft' });
};

beforeEach(() => {
  vi.clearAllMocks();
  fetchMock.mockReset().mockImplementation(async () => json([saved]));
  vi.stubGlobal('fetch', fetchMock);
  clearAuth();
  sessionStorage.clear();
  localStorage.clear();
  setOrgID('north');
});
afterEach(() => {
  cleanup();
  clearAuth();
  setOrgID(null);
  vi.unstubAllGlobals();
});

describe('VMware destination refusal through real transport', () => {
  it.each([
    ['saved', 'test'],
    ['saved', 'preview'],
    ['saved', 'save'],
    ['new', 'test'],
    ['new', 'preview'],
    ['new', 'save'],
  ] as const)(
    'keeps an exact destination refusal local to %s %s without changing the draft or replaying',
    async (mode, action) => {
      const { result } = renderHook(() => useVMwareSettingsPanelState());
      await ready(result);
      if (mode === 'new') {
        result.openCreateDialog();
        result.updateForm({
          name: 'New draft',
          host: saved.host,
          username: saved.username,
          password: 'synthetic-new-secret',
          monitorHosts: false,
        });
      }
      const draft = result.form();
      fetchMock.mockResolvedValueOnce(json(destinationFailure, 403));
      if (action === 'test') await result.testCurrentForm();
      else if (action === 'preview') await result.previewCurrentForm();
      else await result.saveCurrentForm();
      expect(result.dialogOpen()).toBe(true);
      expect(result.connections()).toHaveLength(1);
      expect(result.form()).toEqual(draft);
      expect(result.loadingError()).toBeNull();
      expect(notificationStore.error).not.toHaveBeenCalled();
      expect(result.connectionFailure()).toMatchObject({
        code: 'destination_not_allowed',
        message: destinationFailure.message,
        title: 'Check the vCenter destination',
        tone: 'warning',
      });
      expect(result.connectionFailure()?.guidance).toContain('Pulse instance administrator');
      expect(fetchMock).toHaveBeenCalledTimes(2);
      const [url, options] = fetchMock.mock.calls[1];
      expect(url).toBe(
        `/api/vmware/connections${mode === 'saved' ? '/saved-vcenter' : ''}${action === 'save' ? '' : `/${action}`}`,
      );
      expect(options?.method).toBe(action === 'save' && mode === 'saved' ? 'PUT' : 'POST');
      expect(options?.signal?.aborted).toBe(false);
      expect(new Headers(options?.headers).get('X-Pulse-Org-ID')).toBe('north');
      expect(JSON.parse(String(options?.body))).toMatchObject({
        password: mode === 'saved' ? '********' : 'synthetic-new-secret',
        insecureSkipVerify: false,
        monitorHosts: false,
      });
      result.updateForm({ host: 'https://corrected.example.test' });
      expect(result.connectionFailure()).toBeNull();
      expect(fetchMock).toHaveBeenCalledTimes(2);
    },
  );

  it.each(['test', 'preview'] as const)(
    'uses endpoint guidance for a typed invalid_config from %s, not credential advice',
    async (action) => {
      const { result } = renderHook(() => useVMwareSettingsPanelState());
      await ready(result);
      fetchMock.mockResolvedValueOnce(json(dialFailure, 400));
      if (action === 'test') await result.testCurrentForm();
      else await result.previewCurrentForm();
      expect(result.connectionFailure()?.message).toBe(dialFailure.details.error);
      expect(result.connectionFailure()?.guidance).toContain('Pulse instance administrator');
      expect(result.connectionFailure()?.guidance).not.toMatch(
        /password|credentials|Skip TLS|grant/i,
      );
      expect(result.form().hasStoredPassword).toBe(true);
      expect(result.dialogOpen()).toBe(true);
    },
  );

  it.each(['test', 'preview', 'save'] as const)(
    'keeps typed invalid-configuration feedback inline after %s',
    async (action) => {
      const { result } = renderHook(() => useVMwareSettingsPanelState());
      await ready(result);
      fetchMock.mockResolvedValueOnce(
        json(
          {
            code: 'vmware_invalid_config',
            message: 'Invalid VMware vCenter connection configuration',
          },
          400,
        ),
      );
      if (action === 'test') await result.testCurrentForm();
      else if (action === 'preview') await result.previewCurrentForm();
      else await result.saveCurrentForm();
      expect(result.dialogOpen()).toBe(true);
      expect(result.connectionFailure()?.guidance).toContain('Pulse instance administrator');
      expect(result.connectionFailure()?.guidance).not.toMatch(
        /password|credentials|Skip TLS|grant/i,
      );
      expect(result.form().hasStoredPassword).toBe(true);
      expect(notificationStore.error).not.toHaveBeenCalled();
      expect(fetchMock).toHaveBeenCalledTimes(2);
    },
  );

  it.each([
    [403, 'permission_denied'],
    [403, 'Destination_Not_Allowed'],
    [403, 'request_failed'],
    [403, 'vmware_invalid_config'],
  ])('still withdraws the reader on HTTP%s / %s', async (status, code) => {
    const { result } = renderHook(() => useVMwareSettingsPanelState());
    await ready(result);
    result.updateForm({ password: 'synthetic-unsaved-secret' });
    fetchMock.mockResolvedValueOnce(json({ code, message: 'not authority' }, status as number));
    await result.testCurrentForm();
    expect(result.dialogOpen()).toBe(false);
    expect(result.form().password).toBe('');
    expect(result.connections()).toEqual([]);
    expect(result.loadingError()).toContain('access could not be verified');
    expect(result.connectionFailure()).toBeNull();
    expect(fetchMock).toHaveBeenCalledTimes(2);
  });

  it('keeps the existing HTTP401 sign-out and shell retirement, even with a destination code', async () => {
    // Real transport clears auth/org and redirects before this hook sees 401.
    // jsdom cannot navigate; retire the shell capability explicitly after
    // verifying sign-out, rather than asserting an in-hook 403 withdrawal.
    const [allowed, setAllowed] = createSignal(true);
    const { result } = renderHook(() => useVMwareSettingsPanelState({ canLoad: allowed }));
    await ready(result);
    result.updateForm({ password: 'synthetic-unsaved-secret' });
    fetchMock.mockResolvedValueOnce(json(destinationFailure, 401));
    await result.testCurrentForm();
    expect(getOrgID()).toBeNull();
    expect(localStorage.getItem('just_logged_out')).toBe('true');
    expect(result.connectionFailure()).toBeNull();
    expect(fetchMock).toHaveBeenCalledTimes(2);
    setAllowed(false);
    expect(result.dialogOpen()).toBe(false);
    expect(result.form().password).toBe('');
    expect(result.connections()).toEqual([]);
  });

  it('preserves a saved row after its explicit test gets a destination refusal', async () => {
    const { result } = renderHook(() => useVMwareSettingsPanelState());
    await ready(result);
    result.closeDialog();
    fetchMock.mockResolvedValueOnce(json(destinationFailure, 403));
    await result.testSavedConnection(result.connections()[0]);
    expect(result.connections()).toHaveLength(1);
    expect(result.loadingError()).toBeNull();
    expect(result.form().password).toBe('');
    expect(notificationStore.error).toHaveBeenCalledWith(destinationFailure.message);
    expect(fetchMock.mock.calls.map(([url, opts]) => [url, opts?.method ?? 'GET'])).toEqual([
      ['/api/vmware/connections', 'GET'],
      ['/api/vmware/connections/saved-vcenter/test', 'POST'],
      ['/api/vmware/connections', 'GET'],
    ]);
  });

  it.each(['test', 'preview'] as const)(
    'ignores a destination result for an edited %s draft',
    async (action) => {
      const { result } = renderHook(() => useVMwareSettingsPanelState());
      await ready(result);
      let release!: (response: Response) => void;
      fetchMock.mockImplementationOnce(
        () =>
          new Promise<Response>((resolve) => {
            release = resolve;
          }),
      );
      const pending = action === 'test' ? result.testCurrentForm() : result.previewCurrentForm();
      await waitFor(() => expect(fetchMock).toHaveBeenCalledTimes(2));
      result.updateForm({ name: 'Newer draft' });
      release(json(destinationFailure, 403));
      await pending;
      expect(result.form().name).toBe('Newer draft');
      expect(result.dialogOpen()).toBe(true);
      expect(result.connectionFailure()).toBeNull();
      expect(result.monitoredSystemPreviewError()).toBeNull();
      expect(result.loadingError()).toBeNull();
      expect(fetchMock).toHaveBeenCalledTimes(2);
    },
  );

  it.each(['inventory', 'delete'] as const)(
    'does not treat a destination code as authority on %s',
    async (action) => {
      const { result } = renderHook(() => useVMwareSettingsPanelState());
      await ready(result);
      if (action === 'delete') result.openDeleteDialog(result.connections()[0]);
      fetchMock.mockResolvedValueOnce(json(destinationFailure, 403));
      if (action === 'inventory') await result.loadConnections();
      else await result.deletePendingConnection();
      expect(result.connections()).toEqual([]);
      expect(result.dialogOpen() || result.deleteDialogOpen()).toBe(false);
      expect(result.loadingError()).toContain('access could not be verified');
    },
  );

  it.each(['organisation', 'capability', 'user'] as const)(
    'rejects a late destination result after %s withdrawal',
    async (change) => {
      const [allowed, setAllowed] = createSignal(true);
      const [user, setUser] = createSignal('first');
      const { result } = renderHook(() =>
        useVMwareSettingsPanelState({ canLoad: allowed, currentUser: user }),
      );
      await ready(result);
      let release!: (response: Response) => void;
      fetchMock.mockImplementationOnce(
        () =>
          new Promise<Response>((resolve) => {
            release = resolve;
          }),
      );
      const pending = result.testCurrentForm();
      await waitFor(() => expect(fetchMock).toHaveBeenCalledTimes(2));
      if (change === 'organisation') {
        setOrgID('south');
        eventBus.emit('org_switched', 'south');
      } else if (change === 'capability') setAllowed(false);
      else setUser('second');
      release(json(destinationFailure, 403));
      await pending;
      expect(result.form().password).toBe('');
      expect(result.dialogOpen()).toBe(false);
      expect(result.connectionFailure()).toBeNull();
    },
  );

  it('renders actionable policy feedback in the real credential slot and clears it on cancel/reopen', async () => {
    const { result } = renderHook(() => useVMwareSettingsPanelState());
    await ready(result);
    render(() => (
      <Dialog
        isOpen={result.dialogOpen()}
        onClose={result.closeDialog}
        ariaLabel="Edit VMware vCenter"
      >
        <VMwareCredentialSlot
          state={result}
          editingConnection={result.connections()[0]}
          onCancel={() => {}}
          onSaved={() => {}}
        />
      </Dialog>
    ));
    fetchMock.mockResolvedValueOnce(json(dialFailure, 400));
    fireEvent.click(screen.getByRole('button', { name: 'Test connection' }));
    await waitFor(() =>
      expect(screen.getByTestId('vmware-connection-test-feedback')).toHaveTextContent(
        'Pulse instance administrator',
      ),
    );
    expect(screen.getByLabelText('Host')).toHaveValue(saved.host);
    expect(screen.getByLabelText('Skip TLS verification')).not.toBeChecked();
    expect(screen.getByLabelText(/^Password/)).toHaveAttribute(
      'placeholder',
      'Saved password retained unless replaced',
    );
    fireEvent.click(screen.getByRole('button', { name: 'Cancel' }));
    result.openEditDialog(result.connections()[0]);
    expect(result.connectionFailure()).toBeNull();
  });
});
