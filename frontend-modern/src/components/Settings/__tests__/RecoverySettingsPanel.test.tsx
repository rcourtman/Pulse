import { cleanup, fireEvent, render, screen } from '@solidjs/testing-library';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { createSignal } from 'solid-js';
import { RecoverySettingsPanel } from '../RecoverySettingsPanel';

function renderRecovery(hasAuthentication = false) {
  const [enabled, setEnabled] = createSignal(true);
  const [interval, setInterval] = createSignal(300);
  const [minutes, setMinutes] = createSignal(5);
  const [custom, setCustom] = createSignal(false);
  const [, setUnsaved] = createSignal(false);
  const setShowExportDialog = vi.fn();
  const setShowImportDialog = vi.fn();
  const setUseCustomPassphrase = vi.fn();
  render(() => (
    <RecoverySettingsPanel
      backupPollingEnabled={enabled}
      setBackupPollingEnabled={setEnabled}
      backupPollingInterval={interval}
      setBackupPollingInterval={setInterval}
      backupPollingCustomMinutes={minutes}
      setBackupPollingCustomMinutes={setMinutes}
      backupPollingUseCustom={custom}
      setBackupPollingUseCustom={setCustom}
      backupPollingEnvLocked={() => false}
      backupIntervalSelectValue={() => String(interval())}
      backupIntervalSummary={() => 'Every 5 minutes'}
      setHasUnsavedChanges={setUnsaved}
      showExportDialog={() => false}
      setShowExportDialog={setShowExportDialog}
      showImportDialog={() => false}
      setShowImportDialog={setShowImportDialog}
      setUseCustomPassphrase={setUseCustomPassphrase}
      securityStatus={() => ({ hasAuthentication })}
    />
  ));
  return { setShowExportDialog, setShowImportDialog, setUseCustomPassphrase };
}

describe('RecoverySettingsPanel', () => {
  afterEach(cleanup);

  it('discloses the limited archive scope before either transfer is opened', () => {
    renderRecovery();
    const text = document.body.textContent;
    expect(text).toContain('selected workspace, not a full-server backup');
    expect(text).toContain('Proxmox connections & credentials');
    expect(text).toContain(
      'History, TrueNAS/vSphere connections, local login credentials and agent enrolment state are not included',
    );
    expect(text).not.toContain('All node connections');
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
    const guide = screen.getByRole('link', { name: 'migration guide' });
    expect(guide).toHaveAttribute('href', '/docs/MIGRATION');
    expect(guide).toHaveAttribute('target', '_blank');
    expect(guide).toHaveAttribute('rel', 'noopener noreferrer');
  });

  it('warns that restoring replaces included settings rather than offering a merge', () => {
    renderRecovery(true);
    const text = document.body.textContent;
    expect(text).toContain('Back up the destination first');
    expect(text).toContain('Replaces included settings and API-token records');
    expect(text).not.toContain('Merge or replace');
    expect(screen.getByRole('button', { name: 'Restore Backup' })).toBeEnabled();
  });

  it.each([false, true])(
    'opens export with the existing passphrase default (authentication: %s)',
    (hasAuthentication) => {
      const actions = renderRecovery(hasAuthentication);
      fireEvent.click(screen.getByRole('button', { name: 'Create Backup' }));
      expect(actions.setUseCustomPassphrase).toHaveBeenCalledExactlyOnceWith(!hasAuthentication);
      expect(actions.setShowExportDialog).toHaveBeenCalledExactlyOnceWith(true);
      expect(actions.setShowImportDialog).not.toHaveBeenCalled();
    },
  );

  it('opens only the existing import dialog from Restore Backup', () => {
    const actions = renderRecovery();
    fireEvent.click(screen.getByRole('button', { name: 'Restore Backup' }));
    expect(actions.setShowImportDialog).toHaveBeenCalledExactlyOnceWith(true);
    expect(actions.setShowExportDialog).not.toHaveBeenCalled();
    expect(actions.setUseCustomPassphrase).not.toHaveBeenCalled();
  });
});
