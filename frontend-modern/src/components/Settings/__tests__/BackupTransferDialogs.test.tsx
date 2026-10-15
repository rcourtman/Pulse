import { cleanup, render, screen } from '@solidjs/testing-library';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { BackupTransferDialogs } from '../BackupTransferDialogs';
import { createSignal, type ComponentProps } from 'solid-js';
import type { BackupTransferStatus } from '../useBackupTransferFlow';
import type { SecurityStatus } from '@/types/config';

function renderTransfer(
  kind: 'export' | 'import',
  overrides: Partial<ComponentProps<typeof BackupTransferDialogs>> = {},
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
      showApiTokenModal={() => false}
      apiTokenInput={() => ''}
      setApiTokenInput={vi.fn()}
      transferStatus={() => null}
      transferError={() => null}
      handleExport={vi.fn()}
      handleImport={vi.fn()}
      closeExportDialog={vi.fn()}
      closeImportDialog={vi.fn()}
      closeApiTokenModal={vi.fn()}
      handleApiTokenAuthenticate={vi.fn()}
      {...overrides}
    />
  ));
}

describe('BackupTransferDialogs', () => {
  afterEach(cleanup);

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

  it('keeps import controls locked and explains that dispatch cannot be cancelled', () => {
    renderTransfer('import', { transferStatus: () => ({ kind: 'import', phase: 'request' }) });
    expect(screen.getByLabelText('Configuration File')).toBeDisabled();
    expect(screen.getByLabelText('Backup Password')).toBeDisabled();
    expect(screen.getByRole('button', { name: 'Cancel' })).toBeDisabled();
    expect(screen.getByRole('button', { name: 'Importing…' })).toBeDisabled();
    expect(screen.getByRole('status')).toHaveTextContent('cannot be cancelled or rolled back here');
  });

  it('announces read, reload and unconfirmed import feedback in the consuming dialog', () => {
    const [status, setStatus] = createSignal<BackupTransferStatus | null>({
      kind: 'import',
      phase: 'reading',
    });
    const [error, setError] = createSignal<string | null>(null);
    renderTransfer('import', { transferStatus: status, transferError: error });
    expect(screen.getByRole('status')).toHaveTextContent('Reading the selected backup');
    setStatus({ kind: 'import', phase: 'reloading' });
    expect(screen.getByRole('status')).toHaveTextContent('Configuration imported');
    setStatus(null);
    setError('Could not confirm the import. Settings may already have changed.');
    expect(screen.getByRole('alert')).toHaveTextContent('Settings may already have changed');
    expect(screen.queryByRole('status')).not.toBeInTheDocument();
    expect(screen.getByLabelText('Backup Password')).not.toBeDisabled();
  });

  it('requires the encryption minimum for login-password exports too', () => {
    renderTransfer('export', {
      securityStatus: () => ({ hasAuthentication: true }) as SecurityStatus,
      exportPassphrase: () => 'short',
    });
    expect(screen.getByRole('button', { name: 'Export' })).toBeDisabled();
  });
});
