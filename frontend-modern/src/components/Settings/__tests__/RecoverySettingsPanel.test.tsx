import { cleanup, fireEvent, render, screen } from '@solidjs/testing-library';
import { createSignal } from 'solid-js';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { RecoverySettingsPanel } from '../RecoverySettingsPanel';

function mountPolling(enabled = true, locked = false) {
  const [backupPollingEnabled, setBackupPollingEnabled] = createSignal(enabled);
  const [backupPollingInterval, setBackupPollingInterval] = createSignal(0);
  const [backupPollingCustomMinutes, setBackupPollingCustomMinutes] = createSignal(60);
  const [backupPollingUseCustom, setBackupPollingUseCustom] = createSignal(false);
  const [showExportDialog, setShowExportDialog] = createSignal(false);
  const [showImportDialog, setShowImportDialog] = createSignal(false);
  const setHasUnsavedChanges = vi.fn();
  render(() => (
    <RecoverySettingsPanel
      backupPollingEnabled={backupPollingEnabled}
      setBackupPollingEnabled={setBackupPollingEnabled}
      backupPollingInterval={backupPollingInterval}
      setBackupPollingInterval={setBackupPollingInterval}
      backupPollingCustomMinutes={backupPollingCustomMinutes}
      setBackupPollingCustomMinutes={setBackupPollingCustomMinutes}
      backupPollingUseCustom={backupPollingUseCustom}
      setBackupPollingUseCustom={setBackupPollingUseCustom}
      backupPollingEnvLocked={() => locked}
      backupIntervalSelectValue={() =>
        backupPollingUseCustom() ? 'custom' : String(backupPollingInterval())
      }
      backupIntervalSummary={() => 'Auto'}
      setHasUnsavedChanges={setHasUnsavedChanges}
      showExportDialog={showExportDialog}
      setShowExportDialog={setShowExportDialog}
      showImportDialog={showImportDialog}
      setShowImportDialog={setShowImportDialog}
      setUseCustomPassphrase={vi.fn()}
      securityStatus={() => ({ hasAuthentication: true })}
    />
  ));
  return { backupPollingEnabled, backupPollingInterval, setHasUnsavedChanges };
}

describe('Recovery backup polling scope', () => {
  afterEach(cleanup);

  it('describes the draft, loss of freshness and safety boundary from the labelled switch', () => {
    mountPolling();
    const toggle = screen.getByRole('checkbox', { name: 'Enable backup polling' });
    expect(toggle).toBeChecked();
    expect(toggle).toHaveAccessibleDescription(/Save changes to apply/);
    expect(toggle).toHaveAccessibleDescription(
      /does not stop PVE monitoring or QEMU Guest Agent reads/,
    );
    expect(screen.getByRole('status', { name: 'Backup polling selection' })).toHaveTextContent(
      'Selected: polling on',
    );
    expect(screen.getByRole('combobox', { name: 'Polling interval' })).toHaveValue('0');
  });

  it('keeps the warning and unrefreshed-evidence consequence visible when polling is switched off', () => {
    const state = mountPolling();
    fireEvent.click(screen.getByRole('checkbox', { name: 'Enable backup polling' }));
    expect(state.backupPollingEnabled()).toBe(false);
    expect(state.setHasUnsavedChanges).toHaveBeenCalledExactlyOnceWith(true);
    expect(screen.queryByRole('combobox', { name: 'Polling interval' })).not.toBeInTheDocument();
    expect(screen.getByRole('status', { name: 'Backup polling selection' })).toHaveTextContent(
      'not be refreshed',
    );
    expect(screen.getByRole('status', { name: 'Backup polling selection' })).toHaveTextContent(
      'not current observations',
    );
    expect(screen.getByRole('note', { name: 'Backup polling scope' })).toHaveTextContent(
      'not a freeze/thaw safety precaution',
    );
    expect(
      screen.getByRole('checkbox', { name: 'Enable backup polling' }),
    ).toHaveAccessibleDescription(/Selected: polling off/);
  });

  it.each([true, false])(
    'retains caution and describes an environment-locked %s selection without offering a save',
    (enabled) => {
      const state = mountPolling(enabled, true);
      const toggle = screen.getByRole('checkbox', { name: 'Enable backup polling' });
      expect(toggle).toBeDisabled();
      expect(toggle).toHaveAccessibleDescription(/Environment override detected/);
      expect(toggle).not.toHaveAccessibleDescription(/Save changes to apply/);
      expect(screen.getByRole('note', { name: 'Backup polling scope' })).toBeVisible();
      expect(state.backupPollingEnabled()).toBe(enabled);
      expect(state.setHasUnsavedChanges).not.toHaveBeenCalled();
    },
  );

  it('can re-enable an off draft without treating automatic cadence as disabled', () => {
    const state = mountPolling(false);
    fireEvent.click(screen.getByRole('checkbox', { name: 'Enable backup polling' }));
    expect(state.backupPollingEnabled()).toBe(true);
    expect(state.backupPollingInterval()).toBe(0);
    expect(screen.getByRole('combobox', { name: 'Polling interval' })).toHaveValue('0');
    expect(screen.getByRole('status', { name: 'Backup polling selection' })).toHaveTextContent(
      'Selected: polling on',
    );
  });

  it('links the existing deployment-specific safety procedure rather than prescribing an incident restart', () => {
    mountPolling(false, true);
    expect(screen.getByRole('link', { name: 'Backup safety precaution' })).toHaveAttribute(
      'href',
      '/docs/VM_DISK_MONITORING#backup-safety',
    );
    expect(
      screen.getByText(/Schedule environment changes and any required restart/),
    ).toHaveTextContent('outside backup or freeze/thaw windows');
    expect(screen.queryByRole('button', { name: /dismiss/i })).not.toBeInTheDocument();
  });
});
