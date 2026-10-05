// Production form and state, source-exact template text supplied by the runner.
// No real notification receiver, credentials, save or Test operation.
import { createSignal } from 'solid-js';
import { render } from 'solid-js/web';
import { WebhookConfig } from '../src/components/Alerts/WebhookConfig';
import '../src/index.css';

function Fixture() {
  const [operations, setOperations] = createSignal(0);
  const unexpectedOperation = () => setOperations((count) => count + 1);
  return (
    <main class="mx-auto min-h-screen max-w-4xl space-y-4 bg-surface p-4 text-base-content">
      <h1 class="text-lg font-semibold">Webhook setup</h1>
      <p class="text-sm text-muted">Offline presentation check. No notification is sent.</p>
      <WebhookConfig
        webhooks={[]}
        onAdd={unexpectedOperation}
        onUpdate={unexpectedOperation}
        onDelete={unexpectedOperation}
        onTest={unexpectedOperation}
      />
      <output aria-label="Notification operations">{operations()}</output>
    </main>
  );
}

render(() => <Fixture />, document.getElementById('root')!);
