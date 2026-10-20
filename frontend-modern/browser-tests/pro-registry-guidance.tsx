// The real Updates panel, runtime reader and API client. Only HTTP responses
// are synthetic. This fixture never performs login, update or install actions.
import { createSignal, onMount } from 'solid-js';
import { render } from 'solid-js/web';
import { UpdatesSettingsPanel } from '../src/components/Settings/UpdatesSettingsPanel';
import { UpdatesAPI, type UpdateInfo, type VersionInfo } from '../src/api/updates';
import { loadRuntimeCapabilities } from '../src/stores/license';
import { markSystemSettingsLoadedWithDefaults } from '../src/stores/systemSettings';
import '../src/index.css';

function Fixture() {
  const [version, setVersion] = createSignal<VersionInfo | null>(null);
  const [update, setUpdate] = createSignal<UpdateInfo | null>(null);
  const [channel, setChannel] = createSignal<'stable' | 'rc'>('stable');
  const [autoUpdate, setAutoUpdate] = createSignal(false);
  const [dirty, setDirty] = createSignal(false);
  onMount(async () => {
    markSystemSettingsLoadedWithDefaults();
    await loadRuntimeCapabilities(true);
    setVersion(await UpdatesAPI.getVersion());
    if (new URLSearchParams(location.search).get('mode') === 'update') {
      setUpdate(await UpdatesAPI.checkForUpdates('stable'));
    }
    document.documentElement.dataset.ready = 'true';
  });
  return (
    <main class="mx-auto max-w-5xl p-3 sm:p-6">
      <h1 class="mb-4 text-xl font-semibold">Settings</h1>
      <UpdatesSettingsPanel
        versionInfo={version}
        updateInfo={update}
        checkingForUpdates={() => false}
        updateChannel={channel}
        setUpdateChannel={setChannel}
        autoUpdateEnabled={autoUpdate}
        setAutoUpdateEnabled={setAutoUpdate}
        checkForUpdates={async () => undefined}
        setHasUnsavedChanges={setDirty}
        updatePlan={() => null}
        onInstallUpdate={() => {
          throw new Error('Unexpected install');
        }}
        isInstalling={() => false}
      />
      <p class="sr-only" data-testid="settings-dirty">
        {dirty() ? 'unsaved' : 'saved'}
      </p>
    </main>
  );
}

render(() => <Fixture />, document.getElementById('root')!);
