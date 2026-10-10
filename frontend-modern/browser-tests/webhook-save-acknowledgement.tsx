// Actual destination section, editor, API client and mutation owner. The runner
// supplies synthetic local responses; no notification, provider or saved host.
import { render } from 'solid-js/web';
import { AlertWebhookDestinationsSection } from '../src/features/alerts/AlertWebhookDestinationsSection';
import { useAlertWebhookDestinationsState } from '../src/features/alerts/useAlertWebhookDestinationsState';
import '../src/index.css';

function Fixture() {
  const state = useAlertWebhookDestinationsState();
  return (
    <main class="mx-auto min-h-screen max-w-4xl space-y-4 bg-surface p-4 text-base-content">
      <h1 class="text-lg font-semibold">Notification destinations</h1>
      <p class="text-sm text-muted">Offline save-recovery check. No notification is sent.</p>
      <AlertWebhookDestinationsSection
        webhooks={state.webhooks()}
        addWebhook={state.addWebhook}
        updateWebhook={state.updateWebhook}
        deleteWebhook={state.deleteWebhook}
        testWebhook={state.testWebhook}
        testingWebhook={state.testingWebhook()}
      />
      <output aria-label="Saved destination names" class="block text-sm">
        {state
          .webhooks()
          .map((hook) => hook.name)
          .join(', ')}
      </output>
    </main>
  );
}

render(() => <Fixture />, document.getElementById('root')!);
