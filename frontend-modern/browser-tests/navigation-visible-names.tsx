// Actual shell, router and consuming pages. Only collected HTTP observations
// and the surrounding authenticated session are synthetic; no actions execute.
import { Route, Router } from '@solidjs/router';
import { createSignal, type ParentProps } from 'solid-js';
import { render } from 'solid-js/web';
import { AppLayout } from '../src/AppLayout';
import { SkipToContentLink } from '../src/components/shared/SkipToContentLink';
import Actions from '../src/pages/Actions';
import { AIIntelligence } from '../src/pages/AIIntelligence';
import { aiChatStore } from '../src/stores/aiChat';
import { actionInboxStore } from '../src/stores/actionInbox';
import { patrolAttentionStore } from '../src/stores/patrolAttention';
import type { State } from '../src/types/api';
import '../src/index.css';

const [alerts, setAlerts] = createSignal([
  { id: 'critical-1', level: 'critical', acknowledged: false },
  { id: 'critical-2', level: 'critical', acknowledged: false },
  { id: 'warning-1', level: 'warning', acknowledged: false },
  { id: 'warning-2', level: 'warning', acknowledged: false },
  { id: 'warning-3', level: 'warning', acknowledged: false },
]);
const refresh = async () => {
  await Promise.all([
    patrolAttentionStore.loadSummary(),
    actionInboxStore.loadPendingActionCount(),
  ]);
};
Object.assign(window, { __navigationProof: { refresh, setAlerts } });
aiChatStore.setEnabled(true);

const Shell = (props: ParentProps) => (
  <>
    <SkipToContentLink />
    <div class="app-scroll-shell isolate flex-1 min-w-0 overflow-y-scroll text-base-content font-sans py-1 sm:py-6">
      <AppLayout
        connectionStatus={() => ({
          kind: 'connected',
          label: 'Connected',
          detail: 'Synthetic collected stream is connected.',
          tone: 'healthy',
        })}
        lastUpdateText={() => ''}
        versionInfo={() =>
          ({ version: '6.5.0', channel: 'stable', isDevelopment: false, isDocker: false }) as never
        }
        hasAuth={() => true}
        needsAuth={() => false}
        proxyAuthInfo={() => null}
        handleLogout={() => {}}
        state={() => ({ activeAlerts: alerts(), resources: [] }) as unknown as State}
        tokenScopes={() => ['*']}
        organizations={() => []}
        activeOrgID={() => 'default'}
        orgsLoading={() => false}
        showOrgSwitcher={() => false}
        onSwitchOrg={() => {}}
      >
        {props.children}
      </AppLayout>
    </div>
  </>
);

render(
  () => (
    <Router root={Shell}>
      <Route path="/patrol" component={AIIntelligence} />
      <Route path="*" component={Actions} />
    </Router>
  ),
  document.getElementById('root')!,
);
void refresh();
