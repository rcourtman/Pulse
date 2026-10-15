// Real settings loader, draft state, save API and Recovery panel. Offline data only.
import { createSignal, onMount, Show } from 'solid-js';
import { render } from 'solid-js/web';
import { Router, Route } from '@solidjs/router';
import { RecoverySettingsPanel } from '../src/components/Settings/RecoverySettingsPanel';
import { useSystemSettingsState } from '../src/components/Settings/useSystemSettingsState';
import { useBackupTransferFlow } from '../src/components/Settings/useBackupTransferFlow';
import Docs from '../src/pages/Docs';
import { DarkModeContext } from '../src/contexts/appRuntime';
import '../src/index.css';

function Fixture() {
  const [, setDiscoveryEnabled] = createSignal(false);
  const [ready, setReady] = createSignal(false);
  const state = useSystemSettingsState({
    activeTab: () => 'system-recovery',
    loadSecurityStatus: async () => {},
    setDiscoveryEnabled,
    applySavedDiscoverySubnet: () => {},
  });
  const flow = useBackupTransferFlow({ securityStatus: () => ({ hasAuthentication: true }) });
  onMount(async () => {
    await state.initializeSystemSettingsState();
    setReady(true);
  });
  return (
    <main class="mx-auto min-h-screen max-w-4xl space-y-3 bg-base p-3 text-base-content">
      <h1 class="text-lg font-semibold">System settings</h1>
      <Show when={ready()}>
        <RecoverySettingsPanel
          backupPollingEnabled={state.backupPollingEnabled}
          setBackupPollingEnabled={state.setBackupPollingEnabled}
          backupPollingInterval={state.backupPollingInterval}
          setBackupPollingInterval={state.setBackupPollingInterval}
          backupPollingCustomMinutes={state.backupPollingCustomMinutes}
          setBackupPollingCustomMinutes={state.setBackupPollingCustomMinutes}
          backupPollingUseCustom={state.backupPollingUseCustom}
          setBackupPollingUseCustom={state.setBackupPollingUseCustom}
          backupPollingEnvLocked={state.backupPollingEnvLocked}
          backupIntervalSelectValue={state.backupIntervalSelectValue}
          backupIntervalSummary={state.backupIntervalSummary}
          setHasUnsavedChanges={state.setHasUnsavedChanges}
          showExportDialog={flow.showExportDialog}
          setShowExportDialog={flow.setShowExportDialog}
          showImportDialog={flow.showImportDialog}
          setShowImportDialog={flow.setShowImportDialog}
          setUseCustomPassphrase={flow.setUseCustomPassphrase}
          securityStatus={() => ({ hasAuthentication: true })}
        />
        <p role="status" aria-label="Save state">
          {state.hasUnsavedChanges() ? 'Unsaved changes' : 'No unsaved changes'}
        </p>
        <button
          type="button"
          disabled={!state.hasUnsavedChanges()}
          class="min-h-11 rounded-md border border-border px-3 focus-visible:outline-2"
          onClick={() => state.saveSettings()}
        >
          Save changes
        </button>
      </Show>
    </main>
  );
}

render(
  () => (
    <DarkModeContext.Provider value={() => document.documentElement.classList.contains('dark')}>
      <Router>
        <Route path="/docs/*docPath" component={Docs} />
        <Route path="*" component={Fixture} />
      </Router>
    </DarkModeContext.Provider>
  ),
  document.getElementById('root')!,
);
