import { cleanup, fireEvent, render, screen, waitFor, within } from '@solidjs/testing-library';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { BackupTransferDialogs } from '../BackupTransferDialogs';
import { useBackupTransferFlow } from '../useBackupTransferFlow';
import { ToastContainer } from '@/components/Toast/Toast';
import { clearApiToken, setOrgID } from '@/utils/apiClient';
import type { SecurityStatus } from '@/types/config';

function renderTransfer(
  kind: 'export' | 'import',
  pending = false,
  error = '',
  showApiTokenModal = false,
) {
  return render(() => (
    <BackupTransferDialogs
      securityStatus={() => null}
      exportPassphrase={() => ''}
      setExportPassphrase={vi.fn()}
      useCustomPassphrase={() => false}
      setUseCustomPassphrase={vi.fn()}
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

  function renderFlow(kind: 'export' | 'import' = 'import', tokenOnly = false) {
    const securityStatus = () =>
      ({ hasAuthentication: !tokenOnly, apiTokenConfigured: tokenOnly }) as SecurityStatus;
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
          <BackupTransferDialogs {...flow} securityStatus={securityStatus} />
          <ToastContainer />
        </>
      );
    });
    fireEvent.click(screen.getByRole('button', { name: 'Open transfer' }));
    if (kind === 'import') {
      flow.setImportPassphrase('synthetic-passphrase');
      flow.setImportFile({ text: async () => '{"data":"synthetic-archive"}' } as File);
    } else flow.setExportPassphrase('synthetic-passphrase');
    return flow;
  }

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
});
