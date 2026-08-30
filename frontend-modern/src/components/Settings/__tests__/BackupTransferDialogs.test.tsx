import { cleanup, render, screen } from '@solidjs/testing-library';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { BackupTransferDialogs } from '../BackupTransferDialogs';

describe('BackupTransferDialogs', () => {
  afterEach(() => {
    cleanup();
  });

  it('discloses that configuration import cannot retarget remote agents', () => {
    render(() => (
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
        showExportDialog={() => false}
        showImportDialog={() => true}
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

    expect(screen.getByText('Agent migration:')).toBeInTheDocument();
    expect(
      screen.getByText(/it cannot change the Pulse URL stored on remote agents/i),
    ).toBeInTheDocument();
  });
});
