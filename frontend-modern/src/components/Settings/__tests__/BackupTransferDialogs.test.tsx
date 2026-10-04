import { cleanup, render, screen } from '@solidjs/testing-library';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { BackupTransferDialogs } from '../BackupTransferDialogs';

function renderTransfer(kind: 'export' | 'import') {
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
});
