import { cleanup, render, screen } from '@solidjs/testing-library';
import { createSignal } from 'solid-js';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { RecoverySettingsPanel } from '../RecoverySettingsPanel';
import { UpdatesSettingsPanel } from '../UpdatesSettingsPanel';

describe('settings switch labels', () => {
  afterEach(() => {
    cleanup();
  });

  it('associates the auto-update switch with its visible label', () => {
    const [updateChannel, setUpdateChannel] = createSignal<'stable' | 'rc'>('stable');
    const [autoUpdateEnabled, setAutoUpdateEnabled] = createSignal(false);

    render(() => (
      <UpdatesSettingsPanel
        versionInfo={() => null}
        updateInfo={() => null}
        checkingForUpdates={() => false}
        updateChannel={updateChannel}
        setUpdateChannel={setUpdateChannel}
        autoUpdateEnabled={autoUpdateEnabled}
        setAutoUpdateEnabled={setAutoUpdateEnabled}
        checkForUpdates={vi.fn().mockResolvedValue(undefined)}
        setHasUnsavedChanges={vi.fn()}
        updatePlan={() => null}
        onInstallUpdate={vi.fn()}
        isInstalling={() => false}
      />
    ));

    expect(screen.getByRole('checkbox', { name: 'Automatic Stable Updates' })).toBeInTheDocument();
  });

  it('associates the backup polling switch with its visible label', () => {
    const [backupPollingEnabled, setBackupPollingEnabled] = createSignal(true);
    const [backupPollingInterval, setBackupPollingInterval] = createSignal(3600);
    const [backupPollingCustomMinutes, setBackupPollingCustomMinutes] = createSignal(60);
    const [backupPollingUseCustom, setBackupPollingUseCustom] = createSignal(false);
    const [showExportDialog, setShowExportDialog] = createSignal(false);
    const [showImportDialog, setShowImportDialog] = createSignal(false);
    const [useCustomPassphrase, setUseCustomPassphrase] = createSignal(false);

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
        backupPollingEnvLocked={() => false}
        backupIntervalSelectValue={() => '3600'}
        backupIntervalSummary={() => 'Every hour'}
        setHasUnsavedChanges={vi.fn()}
        showExportDialog={showExportDialog}
        setShowExportDialog={setShowExportDialog}
        showImportDialog={showImportDialog}
        setShowImportDialog={setShowImportDialog}
        setUseCustomPassphrase={setUseCustomPassphrase}
        securityStatus={() => ({ hasAuthentication: true })}
      />
    ));

    expect(screen.getByRole('checkbox', { name: 'Enable backup polling' })).toBeChecked();
    expect(useCustomPassphrase()).toBe(false);
  });
});
