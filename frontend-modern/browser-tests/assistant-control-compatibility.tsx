// Actual existing Settings/Chat/Docs surfaces, not a shipped route. Synthetic
// API responses and local settings saves only; no model/action/backend calls.
import { render } from 'solid-js/web';
import { A, Route, Router } from '@solidjs/router';
import { AIAssistantSettings } from '../src/components/Settings/AISettings';
import { AIChat } from '../src/components/AI/Chat';
import Docs from '../src/pages/Docs';
import { aiChatStore } from '../src/stores/aiChat';
import '../src/index.css';
const params = new URLSearchParams(location.search);
window.history.replaceState(
  {},
  '',
  params.get('surface') === 'chat' ? '/proof-chat' : '/settings/ai/assistant',
);
aiChatStore.setEnabled(true);
(window as any).__assistantProof = {
  scoped: () =>
    aiChatStore.setContext({
      autonomousMode: false,
      briefing: {
        sourceLabel: 'Pulse Alerts',
        title: 'Synthetic alert investigation attached',
        subject: 'lab-vm',
      },
      handoffResources: [{ id: 'vm:lab:101', name: 'lab-vm', type: 'vm', node: 'lab' }],
      context: { alertIdentifier: 'synthetic-alert-1' },
    }),
  context: () => aiChatStore.context,
};
function SettingsFixture() {
  return (
    <main class="mx-auto max-w-4xl bg-surface text-base-content p-4">
      <AIAssistantSettings />
      <nav class="space-x-3 mt-4">
        <A href="/docs/AI#control-levels">Read Assistant modes</A>
        <A href="/docs/AI_AUTONOMY#assistant-control-levels">Read control guide</A>
      </nav>
    </main>
  );
}
function ChatFixture() {
  aiChatStore.open();
  return (
    <div class="mx-auto max-w-3xl h-screen bg-surface text-base-content">
      <AIChat onClose={() => aiChatStore.close()} />
    </div>
  );
}
render(
  () => (
    <Router>
      <Route path="/settings/ai/assistant" component={SettingsFixture} />
      <Route path="/proof-chat" component={ChatFixture} />
      <Route path="/docs/*docPath" component={Docs} />
    </Router>
  ),
  document.getElementById('root')!,
);
