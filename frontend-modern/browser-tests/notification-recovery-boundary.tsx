// Full existing Notifications tab and its actual state/API owners. Only the
// offline server's responses are synthetic; no sender or real queue is used.
import { createSignal } from 'solid-js';
import { render } from 'solid-js/web';
import { Router, Route } from '@solidjs/router';
import { DestinationsTab } from '../src/features/alerts/tabs/DestinationsTab';
import { ToastContainer } from '../src/components/Toast/Toast';
import type { UIEmailConfig, UIAppriseConfig } from '../src/features/alerts/types';
import type { Webhook } from '../src/api/notifications';
import '../src/index.css';

function Fixture() {
  const sendTests = new URLSearchParams(location.search).has('send-tests');
  const [email, setEmail] = createSignal<UIEmailConfig>({
    enabled: sendTests,
    provider: 'smtp',
    server: sendTests ? 'smtp.example.test' : '',
    port: 587,
    from: sendTests ? 'pulse@example.test' : '',
    to: sendTests ? ['operator@example.test'] : [],
    username: '',
    password: '',
    rateLimit: 0,
    tls: false,
    startTLS: true,
  });
  const [apprise, setApprise] = createSignal<UIAppriseConfig>({
    enabled: sendTests,
    mode: 'cli',
    targetsText: sendTests ? 'mailto://operator@example.test' : '',
    cliPath: 'apprise',
    timeoutSeconds: 10,
    serverUrl: '',
    configKey: '',
    apiKey: '',
    apiKeyHeader: 'X-API-KEY',
    skipTlsVerify: false,
    hasApiKey: false,
  });
  const [webhooks, setWebhooks] = createSignal<Webhook[]>([
    {
      id: 'ops',
      name: 'Fixture operations',
      url: 'https://notifications.invalid/fixture',
      method: 'POST',
      enabled: true,
      headers: {},
    },
  ]);
  const [pingUrl, setPingUrl] = createSignal('');
  const [severity, setSeverity] = createSignal<'all' | 'critical'>('all');
  return (
    <main class="mx-auto min-h-screen max-w-4xl space-y-4 bg-surface p-3 text-base-content sm:p-6">
      <h1 class="text-lg font-semibold">Alert notifications</h1>
      <p class="text-sm text-muted">
        Synthetic notification activity. No real destination or queue is contacted.
      </p>
      <DestinationsTab
        emailConfig={email}
        setEmailConfig={setEmail}
        appriseConfig={apprise}
        setAppriseConfig={setApprise}
        webhooks={webhooks}
        setWebhooks={setWebhooks}
        configLoadError={() => null}
        isRetrying={() => false}
        isLoadingDestinations={() => false}
        onRetryLoad={() => {}}
        setHasUnsavedChanges={() => {}}
        deadManPingUrl={pingUrl}
        setDeadManPingUrl={setPingUrl}
        pushMinimumSeverity={severity}
        setPushMinimumSeverity={setSeverity}
      />
      <ToastContainer />
    </main>
  );
}
render(
  () => (
    <Router>
      <Route path="*" component={Fixture} />
    </Router>
  ),
  document.getElementById('root')!,
);
