// Mount the production Patrol surface under a local Router; the browser script
// supplies bounded API responses for the manual-rule creation and reversal states.
import { Route, Router } from '@solidjs/router';
import { render } from 'solid-js/web';
import { PatrolIntelligenceSurface } from '../src/features/patrol/PatrolIntelligenceSurface';
import { eventBus } from '../src/stores/events';
import { apiFetch, setOrgID } from '../src/utils/apiClient';
import '../src/index.css';

setOrgID('fixture-tenant-a');
(window as any).__patrolRuleRemoval = {
  apiFetch,
  switchOrg: () => {
    setOrgID('fixture-tenant-b');
    eventBus.emit('org_switched', 'fixture-tenant-b');
  },
  revoke: () => eventBus.emit('organizations_changed'),
};

render(
  () => (
    <Router>
      <Route
        path="*"
        component={() => (
          <main class="mx-auto max-w-6xl p-4">
            <PatrolIntelligenceSurface />
          </main>
        )}
      />
    </Router>
  ),
  document.getElementById('root')!,
);
