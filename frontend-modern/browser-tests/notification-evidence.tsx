// The production Notifications tab and API readers with synthetic read responses.
// No notification is sent, retried or dismissed by this fixture.
import { createSignal } from 'solid-js';
import { render } from 'solid-js/web';
import { Router, Route } from '@solidjs/router';
import { DestinationsTab } from '../src/features/alerts/tabs/DestinationsTab';
import type { UIEmailConfig, UIAppriseConfig } from '../src/features/alerts/types';
import '../src/index.css';

function Fixture() {
  const [emailConfig, setEmailConfig] = createSignal<UIEmailConfig>({
    enabled: false,
    provider: 'smtp',
    server: '',
    port: 587,
    username: '',
    password: '',
    from: '',
    to: [],
    tls: true,
    startTLS: true,
    replyTo: '',
    maxRetries: 3,
    retryDelay: 5,
    rateLimit: 60,
  });
  const [appriseConfig, setAppriseConfig] = createSignal<UIAppriseConfig>({
    enabled: false,
    mode: 'cli',
    targetsText: '',
    cliPath: '',
    timeoutSeconds: 20,
    serverUrl: '',
    configKey: '',
    apiKey: '',
    apiKeyHeader: 'X-API-KEY',
    skipTlsVerify: false,
    hasApiKey: false,
  });
  return (
    <main class="mx-auto max-w-4xl p-3 text-base-content">
      <h1 class="mb-4 text-xl font-semibold">Notifications</h1>
      <DestinationsTab
        emailConfig={emailConfig}
        setEmailConfig={setEmailConfig}
        appriseConfig={appriseConfig}
        setAppriseConfig={setAppriseConfig}
        setHasUnsavedChanges={() => {}}
        configLoadError={() => null}
        isRetrying={() => false}
        isLoadingDestinations={() => false}
        onRetryLoad={() => {}}
        webhooks={() => []}
        deadManPingUrl={() => ''}
        setDeadManPingUrl={() => {}}
        pushMinimumSeverity={() => 'all'}
        setPushMinimumSeverity={() => {}}
      />
    </main>
  );
}
render(
  () => (
    <Router>
      <Route path="/*" component={Fixture} />
    </Router>
  ),
  document.getElementById('root')!,
);
