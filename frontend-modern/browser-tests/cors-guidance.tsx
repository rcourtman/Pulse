// Real settings section, Docs, router and CSS with local signal-only settings.
// No backend, credential, proxy, save or browser CORS request is exercised.
import { createSignal } from 'solid-js';
import { render } from 'solid-js/web';
import { A, Route, Router } from '@solidjs/router';
import { NetworkBoundarySettingsSection } from '../src/components/Settings/NetworkBoundarySettingsSection';
import Docs from '../src/pages/Docs';
import '../src/index.css';

const overridden = new URLSearchParams(window.location.search).get('override') === '1';
window.history.replaceState({}, '', `/settings/system/network${overridden ? '?override=1' : ''}`);

function SettingsFixture() {
  const [publicURL, setPublicURL] = createSignal('');
  const [allowedOrigins, setAllowedOrigins] = createSignal(
    overridden ? 'https://managed.example.com' : '',
  );
  const [allowEmbedding, setAllowEmbedding] = createSignal(false);
  const [allowedEmbedOrigins, setAllowedEmbedOrigins] = createSignal('');
  const [webhookAllowedPrivateCIDRs, setWebhookAllowedPrivateCIDRs] = createSignal('');
  const [dirty, setHasUnsavedChanges] = createSignal(false);
  return (
    <main class="max-w-3xl mx-auto p-4 bg-surface text-base-content">
      <h1 class="text-xl font-semibold">Network settings</h1>
      <NetworkBoundarySettingsSection
        publicURL={publicURL}
        setPublicURL={setPublicURL}
        allowedOrigins={allowedOrigins}
        setAllowedOrigins={setAllowedOrigins}
        allowEmbedding={allowEmbedding}
        setAllowEmbedding={setAllowEmbedding}
        allowedEmbedOrigins={allowedEmbedOrigins}
        setAllowedEmbedOrigins={setAllowedEmbedOrigins}
        webhookAllowedPrivateCIDRs={webhookAllowedPrivateCIDRs}
        setWebhookAllowedPrivateCIDRs={setWebhookAllowedPrivateCIDRs}
        envOverrides={() => ({ allowedOrigins: overridden })}
        setHasUnsavedChanges={setHasUnsavedChanges}
      />
      <output aria-label="Fixture changed">{dirty() ? 'changed' : 'unchanged'}</output>
      <nav>
        <A href="/docs/FAQ">Open FAQ</A>
      </nav>
    </main>
  );
}

render(
  () => (
    <Router>
      <Route path="/settings/system/network" component={SettingsFixture} />
      <Route path="/docs" component={Docs} />
      <Route path="/docs/*docPath" component={Docs} />
    </Router>
  ),
  document.getElementById('root')!,
);
