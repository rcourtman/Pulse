import { cleanup, fireEvent, render, screen, waitFor, within } from '@solidjs/testing-library';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { BackupTransferDialogs } from '../BackupTransferDialogs';
import { useBackupTransferFlow } from '../useBackupTransferFlow';
import { ToastContainer } from '@/components/Toast/Toast';
import { apiClient, clearApiToken, getOrgID, setApiToken, setOrgID } from '@/utils/apiClient';
import { notificationStore } from '@/stores/notifications';
import { SettingsDialogs } from '../SettingsDialogs';
import type { SecurityStatus } from '@/types/config';

function renderTransfer(
  kind: 'export' | 'import',
  pending = false,
  error = '',
  showApiTokenModal = false,
) {
  return render(() => (
    <BackupTransferDialogs
      exportPassphrase={() => ''}
      setExportPassphrase={vi.fn()}
      exportPassphraseConfirmation={() => ''}
      setExportPassphraseConfirmation={vi.fn()}
      importPassphrase={() => 'secure-passphrase'}
      setImportPassphrase={vi.fn()}
      importFile={() => null}
      setImportFile={vi.fn()}
      showExportDialog={() => kind === 'export'}
      showImportDialog={() => kind === 'import'}
      showApiTokenModal={() => showApiTokenModal}
      transferOrgID={() => 'tenant-fixture'}
      isTransferring={() => pending}
      transferError={() => error}
      transferNotice={() => ''}
      apiTokenInput={() => ''}
      apiTokenError={() => ''}
      setApiTokenInput={vi.fn()}
      handleExport={vi.fn()}
      handleImport={vi.fn()}
      closeExportDialog={vi.fn()}
      closeImportDialog={vi.fn()}
      closeApiTokenModal={vi.fn()}
      handleApiTokenAuthenticate={vi.fn()}
    />
  ));
}

describe('BackupTransferDialogs', () => {
  afterEach(() => {
    cleanup();
    vi.restoreAllMocks();
    vi.unstubAllGlobals();
    clearApiToken();
    setOrgID(null);
  });

  it('discloses the actual archive scope before exporting', () => {
    renderTransfer('export');
    expect(screen.getByText('Configuration only:')).toBeInTheDocument();
    const text = screen.getByRole('dialog', { name: 'Export configuration' }).textContent;
    expect(text).toContain('SSO settings');
    expect(text).toContain('API-token records');
    expect(text).toContain('not history, TrueNAS/vSphere connections');
    expect(text).toContain('Local login credentials and sessions are not included');
    expect(text).not.toContain('NOT authentication settings');
    expect(screen.getByRole('link', { name: 'migration guide' })).toHaveAttribute(
      'href',
      '/docs/MIGRATION',
    );
  });

  it('distinguishes token transfer from agent admission before importing', () => {
    renderTransfer('import');
    expect(screen.getByText('Agent migration:')).toBeInTheDocument();
    const text = screen.getByRole('dialog', { name: 'Import configuration' }).textContent;
    expect(text).toContain('Back up the destination first');
    expect(text).toContain('not the whole installation');
    expect(text).toContain('inventory, enrolment state, profiles and assignments are not');
    expect(text).toContain('Import cannot change the Pulse URL stored on remote agents');
    expect(text).toContain('Preserve each agent');
    expect(text).toContain('verify fresh admission');
    expect(text).not.toContain('restores server-side agent records');
    const guide = screen.getByRole('link', { name: 'migration guide' });
    expect(guide).toHaveAttribute('href', '/docs/MIGRATION');
    expect(guide).toHaveAttribute('target', '_blank');
    expect(guide).toHaveAttribute('rel', 'noopener noreferrer');
    expect(screen.getByLabelText('Configuration File')).toHaveAccessibleDescription(
      'Files up to 2 MiB are read locally. Import requests, including the password, must fit the server’s 1 MiB limit.',
    );
  });

  it.each(['export', 'import'] as const)(
    'shows the fixed workspace, disables pending %s inputs and exposes progress',
    (kind) => {
      renderTransfer(kind, true);
      expect(screen.getByText('tenant-fixture')).toBeInTheDocument();
      const dialog = screen.getByRole('dialog');
      for (const control of dialog.querySelectorAll('input, button')) {
        expect(control).toBeDisabled();
      }
      expect(screen.getByRole('status')).toHaveTextContent('while Pulse waits for a response');
    },
  );

  it('keeps uncertain outcome guidance readable in the import dialog', () => {
    renderTransfer(
      'import',
      false,
      'Import was not confirmed. Check the destination before trying again.',
    );
    expect(screen.getByRole('alert')).toHaveTextContent(
      'Check the destination before trying again',
    );
  });

  it('explains that saving a token requires a deliberate transfer submission', () => {
    renderTransfer('import', false, '', true);
    const text = screen.getByRole('dialog', { name: 'API token required' }).textContent;
    expect(text).toContain('Saving it does not repeat the transfer.');
    expect(text).toContain('Review the workspace and submit again yourself.');
  });

  function renderFlow(
    kind: 'export' | 'import' = 'import',
    tokenOnly = false,
    security: Partial<SecurityStatus> = {},
  ) {
    const securityStatus = () =>
      ({
        hasAuthentication: !tokenOnly,
        apiTokenConfigured: tokenOnly,
        ...security,
      }) as SecurityStatus;
    let flow!: ReturnType<typeof useBackupTransferFlow>;
    setOrgID('tenant-fixture');
    document.cookie = 'pulse_csrf=synthetic-csrf; Path=/';
    render(() => {
      flow = useBackupTransferFlow({ securityStatus });
      return (
        <>
          <button
            onClick={() =>
              kind === 'import' ? flow.setShowImportDialog(true) : flow.setShowExportDialog(true)
            }
          >
            Open transfer
          </button>
          <SettingsDialogs
            {...flow}
            showUpdateConfirmation={() => false}
            closeUpdateConfirmation={vi.fn()}
            handleConfirmUpdate={vi.fn()}
            versionInfo={() => null}
            updateInfo={() => null}
            updatePlan={() => null}
            isInstallingUpdate={() => false}
            showPasswordModal={() => false}
            closePasswordModal={vi.fn()}
          />
          <ToastContainer />
        </>
      );
    });
    fireEvent.click(screen.getByRole('button', { name: 'Open transfer' }));
    const dialog = screen.getByRole('dialog');
    const submit = within(dialog).getByRole('button', {
      name: kind === 'import' ? 'Import' : 'Export',
    });
    expect(submit).toBeDisabled();
    fireEvent.input(within(dialog).getByLabelText('Backup passphrase'), {
      target: { value: 'synthetic-passphrase' },
    });
    if (kind === 'import') {
      expect(submit).toBeDisabled();
      const file = new File(['{"data":"synthetic-archive"}'], 'synthetic-backup.json', {
        type: 'application/json',
      });
      // jsdom does not implement Blob.text; the selected File still goes
      // through the real labelled input and onChange/SettingsDialogs binding.
      Object.defineProperty(file, 'text', {
        value: vi.fn().mockResolvedValue('{"data":"synthetic-archive"}'),
      });
      fireEvent.change(within(dialog).getByLabelText('Configuration File'), {
        target: { files: [file] },
      });
    } else {
      fireEvent.input(within(dialog).getByLabelText('Confirm backup passphrase'), {
        target: { value: 'synthetic-passphrase' },
      });
    }
    expect(submit).toBeEnabled();
    return flow;
  }

  it.each([
    { hasAuthentication: true, ssoEnabled: true },
    { hasAuthentication: true, ssoEnabled: false },
    { hasAuthentication: false },
  ])('offers the same separate, masked backup passphrase for %j', (security) => {
    renderFlow('export', false, security);
    const dialog = screen.getByRole('dialog', { name: 'Export configuration' });
    expect(within(dialog).queryByRole('radio')).not.toBeInTheDocument();
    expect(within(dialog).getByLabelText('Backup passphrase')).toHaveAttribute('type', 'password');
    expect(within(dialog).getByLabelText('Backup passphrase')).toHaveAttribute(
      'autocomplete',
      'new-password',
    );
    expect(within(dialog).getByLabelText('Backup passphrase')).toHaveAccessibleDescription(
      /different from your Pulse or SSO login password.*does not verify your login/s,
    );
    expect(within(dialog).getByLabelText('Confirm backup passphrase')).toHaveAttribute(
      'type',
      'password',
    );
    expect(within(dialog).getByLabelText('Confirm backup passphrase')).toHaveAccessibleDescription(
      'Enter it again to catch typing mistakes before creating the file.',
    );
    expect(dialog).not.toHaveTextContent('Use your login password');
  });

  it('keeps an export typo unsent and lets the operator correct it explicitly', async () => {
    const fetchMock = vi
      .fn()
      .mockImplementation(async () => new Response('synthetic refusal', { status: 403 }));
    vi.stubGlobal('fetch', fetchMock);
    const flow = renderFlow('export');
    const dialog = screen.getByRole('dialog');
    const confirm = within(dialog).getByLabelText('Confirm backup passphrase');
    fireEvent.input(confirm, { target: { value: '' } });
    expect(within(dialog).getByRole('button', { name: 'Export' })).toBeDisabled();
    fireEvent.input(confirm, { target: { value: 'synthetic-passphrase-typo' } });
    fireEvent.click(within(dialog).getByRole('button', { name: 'Export' }));
    expect(within(dialog).getByRole('alert')).toHaveTextContent(
      'Passphrases do not match. No export was sent.',
    );
    expect(fetchMock).not.toHaveBeenCalled();
    fireEvent.input(confirm, { target: { value: 'synthetic-passphrase' } });
    fireEvent.click(within(dialog).getByRole('button', { name: 'Export' }));
    await waitFor(() => expect(flow.isTransferring()).toBe(false));
    expect(fetchMock).toHaveBeenCalledTimes(1);
    expect(JSON.parse(fetchMock.mock.calls[0][1].body)).toEqual({
      passphrase: 'synthetic-passphrase',
    });
    expect(within(dialog).getByRole('alert')).toHaveTextContent('synthetic refusal');
    expect(flow.exportPassphraseConfirmation()).toBe('synthetic-passphrase');
    fireEvent.click(within(dialog).getByRole('button', { name: 'Cancel' }));
    expect(flow.exportPassphraseConfirmation()).toBe('');
    expect(flow.exportPassphrase()).toBe('');
    fireEvent.click(screen.getByRole('button', { name: 'Open transfer' }));
    expect(screen.getByLabelText('Confirm backup passphrase')).toHaveValue('');
  });

  it('asks for the original archive passphrase, including older password-based exports', () => {
    renderTransfer('import');
    expect(screen.getByLabelText('Backup passphrase')).toHaveAccessibleDescription(
      'Use the original passphrase for this file. For older backups, this may be the login password you chose when exporting, not your current login password.',
    );
    expect(screen.queryByLabelText('Confirm backup passphrase')).not.toBeInTheDocument();
  });

  it.each(['export', 'import'] as const)(
    'keeps repeated %s failures in one persistent dialog alert without toast stacking',
    async (kind) => {
      const refusal =
        'A configuration import is still running for this workspace. This request has not been applied; wait for it to finish and check the configuration before retrying.';
      const fetchMock = vi
        .fn()
        .mockImplementation(async () => new Response(refusal, { status: 409 }));
      vi.stubGlobal('fetch', fetchMock);
      const flow = renderFlow(kind);
      const dialog = screen.getByRole('dialog');
      for (let attempt = 1; attempt <= 2; attempt++) {
        fireEvent.click(
          within(dialog).getByRole('button', {
            name: kind === 'import' ? 'Import' : 'Export',
          }),
        );
        await waitFor(() => expect(flow.isTransferring()).toBe(false));
        expect(fetchMock).toHaveBeenCalledTimes(attempt);
        expect(within(dialog).getAllByRole('alert')).toHaveLength(1);
        expect(within(dialog).getByRole('alert')).toHaveTextContent(refusal);
        expect(
          screen.queryByRole('button', { name: 'Dismiss notification', hidden: true }),
        ).not.toBeInTheDocument();
        expect(
          screen.getByRole('region', { name: 'Notifications', hidden: true }),
        ).toBeEmptyDOMElement();
      }
      if (kind === 'import') {
        expect(within(dialog).getByRole('alert')).toHaveTextContent('Import was not confirmed');
        expect(flow.importPassphrase()).toBe('synthetic-passphrase');
        expect(flow.importFile()).not.toBeNull();
      }
      fireEvent.click(within(dialog).getByRole('button', { name: 'Cancel' }));
      expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
      expect(flow.importPassphrase()).toBe('');
      expect(flow.exportPassphrase()).toBe('');
      fireEvent.click(screen.getByRole('button', { name: 'Open transfer' }));
      expect(screen.queryByRole('alert')).not.toBeInTheDocument();
      expect(fetchMock).toHaveBeenCalledTimes(2);
    },
  );

  it('keeps token validation and saved status in the active dialog without replaying import', async () => {
    const fetchMock = vi.fn().mockResolvedValue(new Response('{}'));
    vi.stubGlobal('fetch', fetchMock);
    const flow = renderFlow('import', true);
    fireEvent.click(screen.getByRole('button', { name: 'Import' }));
    const prompt = await screen.findByRole('dialog', { name: 'API token required' });
    fireEvent.input(within(prompt).getByLabelText('API Token'), { target: { value: '   ' } });
    fireEvent.click(within(prompt).getByRole('button', { name: 'Authenticate' }));
    expect(within(prompt).getByRole('alert')).toHaveTextContent('Please enter the API token');
    expect(document.querySelectorAll('[role=alert]')).toHaveLength(1);
    fireEvent.input(within(prompt).getByLabelText('API Token'), {
      target: { value: 'synthetic-token' },
    });
    fireEvent.click(within(prompt).getByRole('button', { name: 'Authenticate' }));
    const dialog = screen.getByRole('dialog', { name: 'Import configuration' });
    expect(within(dialog).getByRole('status')).toHaveTextContent(
      'API token saved. Review the workspace',
    );
    expect(within(dialog).queryByRole('alert')).not.toBeInTheDocument();
    expect(flow.apiTokenInput()).toBe('');
    expect(fetchMock).not.toHaveBeenCalled();
    expect(
      screen.getByRole('region', { name: 'Notifications', hidden: true }),
    ).toBeEmptyDOMElement();
    fireEvent.click(within(dialog).getByRole('button', { name: 'Cancel' }));
    fireEvent.click(screen.getByRole('button', { name: 'Open transfer' }));
    expect(screen.queryByRole('status')).not.toBeInTheDocument();
  });

  it('returns the uncertain-import warning from a refused token prompt to the original dialog', async () => {
    const fetchMock = vi
      .fn()
      .mockImplementation(
        async () => new Response(JSON.stringify({ error: 'API token required' }), { status: 403 }),
      );
    vi.stubGlobal('fetch', fetchMock);
    renderFlow();
    fireEvent.click(screen.getByRole('button', { name: 'Import' }));
    const prompt = await screen.findByRole('dialog', { name: 'API token required' });
    expect(within(prompt).getByRole('alert')).toHaveTextContent('Import was not confirmed');
    expect(document.querySelectorAll('[role=alert]')).toHaveLength(1);
    fireEvent.input(within(prompt).getByLabelText('API Token'), {
      target: { value: 'synthetic-token' },
    });
    fireEvent.click(within(prompt).getByRole('button', { name: 'Authenticate' }));
    const dialog = screen.getByRole('dialog', { name: 'Import configuration' });
    expect(within(dialog).getByRole('alert')).toHaveTextContent(
      'Check the destination before trying again',
    );
    expect(within(dialog).getByRole('status')).toHaveTextContent('submit the transfer again');
    expect(fetchMock).toHaveBeenCalledTimes(1);
    expect(
      screen.getByRole('region', { name: 'Notifications', hidden: true }),
    ).toBeEmptyDOMElement();
  });

  describe('rendered transfers keep their destination until response decoding finishes', () => {
    const kinds = ['export', 'import'] as const;
    const fetchMock = vi.fn<typeof fetch>();
    const deferred = () => {
      let resolve!: (value: string) => void;
      const promise = new Promise<string>((yes) => {
        resolve = yes;
      });
      return { promise, resolve };
    };

    beforeEach(() => {
      fetchMock.mockReset();
      vi.stubGlobal('fetch', fetchMock);
      setApiToken('synthetic-transfer-token');
      vi.stubGlobal(
        'URL',
        class extends URL {
          static createObjectURL = vi.fn(() => 'blob:synthetic-transfer');
          static revokeObjectURL = vi.fn();
        },
      );
      vi.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(() => {});
      vi.spyOn(notificationStore, 'success');
    });

    const start = async (kind: (typeof kinds)[number]) => {
      const body = deferred();
      const response = new Response('', {
        headers: { 'Content-Type': 'application/json' },
      });
      const bodyRead = vi.spyOn(response, 'text').mockReturnValue(body.promise);
      fetchMock.mockResolvedValue(response);
      // Observe the real client promise so teardown tests await actual decode
      // completion, rather than passing before the late continuation runs.
      const decoded = vi.spyOn(apiClient, 'fetchJSON');
      const timers = vi.spyOn(globalThis, 'setTimeout');
      const flow = renderFlow(kind);
      const dialog = screen.getByRole('dialog');
      const button = within(dialog).getByRole('button', {
        name: kind === 'import' ? 'Import' : 'Export',
      });
      fireEvent.click(button);
      await waitFor(() => expect(bodyRead).toHaveBeenCalledTimes(1));
      expect(fetchMock).toHaveBeenCalledTimes(1);
      expect(decoded).toHaveBeenCalledTimes(1);
      expect(flow.isTransferring()).toBe(true);
      expect(fetchMock.mock.calls[0]).toEqual([
        `/api/config/${kind}`,
        expect.objectContaining({
          method: 'POST',
          credentials: 'include',
          signal: expect.any(AbortSignal),
          headers: expect.objectContaining({
            'X-Pulse-Org-ID': 'tenant-fixture',
            'X-CSRF-Token': 'synthetic-csrf',
            'X-API-Token': 'synthetic-transfer-token',
          }),
          body: JSON.stringify(
            kind === 'import'
              ? { passphrase: 'synthetic-passphrase', data: 'synthetic-archive' }
              : { passphrase: 'synthetic-passphrase' },
          ),
        }),
      ]);
      for (const control of dialog.querySelectorAll('input, button')) {
        expect(control).toBeDisabled();
      }
      const reloads = () => timers.mock.calls.filter(([, delay]) => delay === 2000);
      const finish = async (text = '{"status":"success","data":"synthetic-archive"}') => {
        body.resolve(text);
        await decoded.mock.results[0].value;
      };
      return { flow, dialog, finish, reloads };
    };

    const expectNoSideEffects = (reloads: () => unknown[]) => {
      expect(URL.createObjectURL).not.toHaveBeenCalled();
      expect(HTMLAnchorElement.prototype.click).not.toHaveBeenCalled();
      expect(notificationStore.success).not.toHaveBeenCalled();
      expect(reloads()).toHaveLength(0);
    };

    it('completes an import from the labelled fields once the body is decoded', async () => {
      const { flow, dialog, finish, reloads } = await start('import');
      expect(within(dialog).getByRole('status')).toHaveTextContent('waits for a response');
      expectNoSideEffects(reloads);
      await finish();
      await waitFor(() => expect(flow.isTransferring()).toBe(false));
      expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
      expect(notificationStore.success).toHaveBeenCalledExactlyOnceWith(
        'Configuration imported successfully. Reloading...',
      );
      expect(flow.importPassphrase()).toBe('');
      expect(flow.importFile()).toBeNull();
      expect(URL.createObjectURL).not.toHaveBeenCalled();
      expect(reloads()).toHaveLength(1);
      expect(fetchMock).toHaveBeenCalledTimes(1);
    });

    it('keeps an import completion bound to the workspace selected before decoding', async () => {
      const { flow, dialog, finish, reloads } = await start('import');
      setOrgID('tenant-other'); // No event or abort can substitute for the completion check.
      await finish();
      await waitFor(() => expect(flow.isTransferring()).toBe(false));
      expect(dialog).toBeInTheDocument();
      expect(flow.transferOrgID()).toBe('tenant-fixture');
      expect(within(dialog).getByText('tenant-fixture')).toBeInTheDocument();
      expect(getOrgID()).toBe('tenant-other');
      expectNoSideEffects(reloads);
      expect(within(dialog).getByRole('alert')).toHaveTextContent(
        'Import completed for workspace tenant-fixture. The selected workspace changed. No reload was performed.',
      );
      expect(within(dialog).getByLabelText('Backup passphrase')).toHaveValue('');
      expect(flow.importFile()).toBeNull();
      expect(within(dialog).getByLabelText('Configuration File')).toBeEnabled();
      expect(within(dialog).getByRole('button', { name: 'Import' })).toBeDisabled();
      fireEvent.click(within(dialog).getByRole('button', { name: 'Cancel' }));
      expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
      expect(fetchMock).toHaveBeenCalledTimes(1);
    });

    it('withholds a late export download and permits a newly confirmed workspace export', async () => {
      const { flow, dialog, finish, reloads } = await start('export');
      setOrgID('tenant-other');
      await finish();
      await waitFor(() => expect(flow.isTransferring()).toBe(false));
      expect(getOrgID()).toBe('tenant-other');
      expect(within(dialog).getByText('tenant-fixture')).toBeInTheDocument();
      expect(within(dialog).getByRole('alert')).toHaveTextContent(
        'Organisation changed. Close this dialog and reopen it in the intended workspace.',
      );
      expect(within(dialog).getByLabelText('Backup passphrase')).toHaveValue(
        'synthetic-passphrase',
      );
      expect(within(dialog).getByLabelText('Confirm backup passphrase')).toHaveValue(
        'synthetic-passphrase',
      );
      expectNoSideEffects(reloads);
      fireEvent.click(within(dialog).getByRole('button', { name: 'Export' }));
      expect(fetchMock).toHaveBeenCalledTimes(1);

      fireEvent.click(within(dialog).getByRole('button', { name: 'Cancel' }));
      fireEvent.click(screen.getByRole('button', { name: 'Open transfer' }));
      const reopened = screen.getByRole('dialog');
      expect(within(reopened).getByText('tenant-other')).toBeInTheDocument();
      expect(within(reopened).queryByRole('alert')).not.toBeInTheDocument();
      expect(within(reopened).getByLabelText('Backup passphrase')).toHaveValue('');
      expect(within(reopened).getByLabelText('Confirm backup passphrase')).toHaveValue('');
      expect(within(reopened).getByRole('button', { name: 'Export' })).toBeDisabled();
      for (const label of ['Backup passphrase', 'Confirm backup passphrase']) {
        fireEvent.input(within(reopened).getByLabelText(label), {
          target: { value: 'new-workspace-passphrase' },
        });
      }
      fetchMock.mockResolvedValueOnce(new Response('{"data":"new-synthetic-archive"}'));
      fireEvent.click(within(reopened).getByRole('button', { name: 'Export' }));
      await waitFor(() => expect(flow.isTransferring()).toBe(false));
      expect(fetchMock).toHaveBeenCalledTimes(2);
      expect(fetchMock.mock.calls[1]).toEqual([
        '/api/config/export',
        expect.objectContaining({
          body: JSON.stringify({ passphrase: 'new-workspace-passphrase' }),
          headers: expect.objectContaining({ 'X-Pulse-Org-ID': 'tenant-other' }),
        }),
      ]);
      expect(notificationStore.success).toHaveBeenCalledExactlyOnceWith(
        'Configuration exported successfully',
      );
      expect(URL.createObjectURL).toHaveBeenCalledTimes(1);
      expect(HTMLAnchorElement.prototype.click).toHaveBeenCalledTimes(1);
      expect(URL.revokeObjectURL).toHaveBeenCalledExactlyOnceWith('blob:synthetic-transfer');
      expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
      expect(flow.exportPassphrase()).toBe('');
      expect(flow.exportPassphraseConfirmation()).toBe('');
      expect(reloads()).toHaveLength(0);
    });

    it('ignores the decoded import result after the dialog owner unmounts', async () => {
      const { finish, reloads } = await start('import');
      const signal = fetchMock.mock.calls[0][1]?.signal;
      expect(signal?.aborted).toBe(false);
      cleanup();
      expect(signal?.aborted).toBe(true);
      await finish();
      expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
      expectNoSideEffects(reloads);
      expect(fetchMock).toHaveBeenCalledTimes(1);
    });
  });
});
