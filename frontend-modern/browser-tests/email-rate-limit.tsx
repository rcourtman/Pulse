// Existing SMTP editor and mutation owner with fixture-only persisted responses.
// No notification, retry, dismissal or licensed/native operation is performed.
import { Show, createSignal, onMount } from 'solid-js';
import { render } from 'solid-js/web';
import { AlertEmailDestinationsSection } from '../src/features/alerts/AlertEmailDestinationsSection';
import { useAlertDestinationsState } from '../src/features/alerts/useAlertDestinationsState';
import '../src/index.css';

function Fixture() {
  const state = useAlertDestinationsState({ activeTab: () => 'destinations' });
  const [ready, setReady] = createSignal(false);
  const [saving, setSaving] = createSignal(false);
  const [status, setStatus] = createSignal('');
  onMount(async () => {
    await state.loadDestinations();
    setReady(true);
  });
  return (
    <main class="mx-auto max-w-4xl p-3 text-base-content">
      <h1 class="mb-4 text-xl font-semibold">Notifications</h1>
      <Show when={ready()}>
        <AlertEmailDestinationsSection
          config={state.emailConfig()}
          setConfig={state.setEmailConfig}
          setHasUnsavedChanges={() => setStatus('Unsaved changes')}
          onTest={() => {
            throw new Error('Sending a test notification is outside this fixture');
          }}
          testing={false}
        />
        <button
          class="mt-3 rounded-md bg-blue-600 px-4 py-2 text-white"
          disabled={saving()}
          onClick={async () => {
            setSaving(true);
            try {
              await state.saveDestinations();
              setStatus('Saved');
            } catch {
              setStatus('Save failed');
            } finally {
              setSaving(false);
            }
          }}
        >
          Save changes
        </button>
        <p role="status">{status()}</p>
      </Show>
    </main>
  );
}

render(() => <Fixture />, document.getElementById('root')!);
