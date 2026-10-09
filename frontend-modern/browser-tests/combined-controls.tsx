// Production components and stylesheet, with synthetic local state only.
// The browser runner supplies watchdog status. No export, import, ping or save runs.
import { createSignal } from 'solid-js';
import { render } from 'solid-js/web';
import { BackupTransferDialogs } from '../src/components/Settings/BackupTransferDialogs';
import { CopyCommandBlock } from '../src/components/Settings/CopyCommandBlock';
import { Button } from '../src/components/shared/Button';
import { Card } from '../src/components/shared/Card';
import { FilterButtonGroup } from '../src/components/shared/FilterButtonGroup';
import { FormSelect } from '../src/components/shared/FormSelect';
import { AlertDeadManDestinationSection } from '../src/features/alerts/AlertDeadManDestinationSection';
import '../src/index.css';

function Fixture() {
  const [selected, setSelected] = createSignal('all');
  const [averaging, setAveraging] = createSignal('5m');
  const [compact, setCompact] = createSignal('inherit');
  const [showExport, setShowExport] = createSignal(false);
  const [showImport, setShowImport] = createSignal(false);
  const [custom, setCustom] = createSignal(false);
  const [exportPassphrase, setExportPassphrase] = createSignal('');
  const [importPassphrase, setImportPassphrase] = createSignal('');
  const [file, setFile] = createSignal<File | null>(null);
  const [pingUrl, setPingUrl] = createSignal('https://watchdog.invalid/offline-fixture');
  const [unsaved, setUnsaved] = createSignal(false);
  const [operations, setOperations] = createSignal(0);
  const unexpectedMutation = () => setOperations((count) => count + 1);

  return (
    <main class="mx-auto max-w-4xl space-y-4 p-3 text-base-content sm:p-6">
      <h1 class="text-lg font-semibold">Combined shared controls</h1>
      <p class="text-sm text-muted">Offline presentation check; no installation or notification.</p>
      <Card data-testid="controls-card" class="space-y-4">
        <FormSelect
          label="CPU averaging"
          value={averaging()}
          onChange={(event) => setAveraging(event.currentTarget.value)}
          help="Synthetic value using the production default control."
        >
          <option value="instant">Instantaneous</option>
          <option value="5m">5-minute average</option>
          <option value="15m">15-minute average</option>
        </FormSelect>
        <FormSelect
          label="Platform override"
          density="compact"
          value={compact()}
          onChange={(event) => setCompact(event.currentTarget.value)}
        >
          <option value="inherit">Inherit</option>
          <option value="5m">5-minute average</option>
        </FormSelect>
        <FilterButtonGroup
          variant="segmented"
          ariaLabel="Resource status"
          options={[
            { value: 'all', label: 'All resources' },
            { value: 'active', label: 'Active resources' },
          ]}
          value={selected()}
          onChange={setSelected}
        />
        <CopyCommandBlock command="pulse-agent --version" />
        <output aria-label="Selected controls">
          {averaging()} / {compact()} / {selected()}
        </output>
      </Card>
      <Card tone="muted" data-testid="muted-card">
        <p class="text-sm">Muted card background</p>
      </Card>
      <AlertDeadManDestinationSection
        pingUrl={pingUrl}
        setPingUrl={setPingUrl}
        setHasUnsavedChanges={setUnsaved}
      />
      <div class="flex flex-wrap gap-3">
        <Button onClick={() => setShowExport(true)}>Create backup</Button>
        <Button onClick={() => setShowImport(true)}>Restore backup</Button>
        <Button onClick={() => setPingUrl('***REDACTED***')}>Use stored watchdog URL</Button>
      </div>
      <output aria-label="Unexpected mutations">{operations()}</output>
      <output aria-label="Unsaved watchdog change">{String(unsaved())}</output>
      <BackupTransferDialogs
        securityStatus={() => ({ hasAuthentication: true, requiresAuth: true })}
        exportPassphrase={exportPassphrase}
        setExportPassphrase={setExportPassphrase}
        useCustomPassphrase={custom}
        setUseCustomPassphrase={setCustom}
        importPassphrase={importPassphrase}
        setImportPassphrase={setImportPassphrase}
        importFile={file}
        setImportFile={setFile}
        showExportDialog={showExport}
        showImportDialog={showImport}
        showApiTokenModal={() => false}
        apiTokenInput={() => ''}
        setApiTokenInput={unexpectedMutation}
        handleExport={unexpectedMutation}
        handleImport={unexpectedMutation}
        closeExportDialog={() => setShowExport(false)}
        closeImportDialog={() => setShowImport(false)}
        closeApiTokenModal={unexpectedMutation}
        handleApiTokenAuthenticate={unexpectedMutation}
      />
    </main>
  );
}

render(() => <Fixture />, document.getElementById('root')!);
