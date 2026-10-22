// Production resource drawers, resource normalizer, state producer and shipped
// CSS. Synthetic observations only: no provider, guest, backup or mail action.
import { render } from 'solid-js/web';
import { createStore, produce, reconcile } from 'solid-js/store';
import { ResourceDetailDrawer } from '../src/components/Infrastructure/ResourceDetailDrawer';
import { DarkModeContext, WebSocketContext } from '../src/contexts/appRuntime';
import { canonicalizeRealtimeResource } from '../src/utils/resourceStateAdapters';
import type { Resource } from '../src/types/resource';
import {
  PBS_JOB_EVIDENCE_FIXTURE,
  pbsJobEvidenceFixture,
} from '../src/components/Infrastructure/__fixtures__/pbsJobEvidence';
import '../src/index.css';

type Service = 'pbs' | 'pmg';

function observation(
  kind: Service,
  status: Resource['status'],
  connectionHealth: string,
): Resource {
  const facet = {
    ...(kind === 'pbs' ? pbsJobEvidenceFixture() : {}),
    hostname: `${kind}.example.invalid`,
    connectionHealth,
  };
  const canonical = new URLSearchParams(location.search).get('transport') === 'canonical';
  return canonicalizeRealtimeResource({
    id: `${kind}-connection`,
    name: `${kind}-connection`,
    displayName: `${kind.toUpperCase()} connection`,
    type: kind,
    platformType: kind === 'pbs' ? 'proxmox-pbs' : 'proxmox-pmg',
    platformId: `${kind}-connection`,
    sourceType: 'api',
    status,
    lastSeen: Date.parse('2026-10-22T05:36:31Z'),
    platformData: canonical ? { sources: [kind], [kind]: facet } : undefined,
    ...(!canonical ? { [kind]: facet } : {}),
  });
}

function Fixture() {
  const [resources, setResources] = createStore({
    pbs: observation('pbs', 'online', 'healthy'),
    pmg: observation('pmg', 'online', 'healthy'),
  });
  Object.assign(window, {
    __serviceConnection: {
      fixture: PBS_JOB_EVIDENCE_FIXTURE,
      replace: (kind: Service, status: Resource['status'], health: string) =>
        setResources(kind, reconcile(observation(kind, status, health))),
      status: (kind: Service, status: Resource['status']) => setResources(kind, 'status', status),
      jobs: (error: string) =>
        setResources(
          'pbs',
          produce((resource) => {
            const pbs = resource.platformData!.pbs!;
            pbs.backupJobs![0].error = error;
            pbs.jobHealthEvidence![0].error = error;
          }),
        ),
      health: (kind: Service, health: string) =>
        setResources(
          kind,
          produce((resource) => {
            (resource.platformData![kind] as { connectionHealth: string }).connectionHealth =
              health;
          }),
        ),
    },
  });
  return (
    <main class="mx-auto max-w-5xl space-y-5 p-3 sm:p-6">
      <h1 class="text-base font-semibold text-base-content">Service connection observations</h1>
      <p class="text-xs text-muted">Read-only synthetic observations in the production drawers.</p>
      <section data-service-fixture="pbs" class="min-w-0 rounded-lg border border-border p-3">
        <ResourceDetailDrawer resource={resources.pbs} presentation="table-row" />
      </section>
      <section data-service-fixture="pmg" class="min-w-0 rounded-lg border border-border p-3">
        <ResourceDetailDrawer resource={resources.pmg} presentation="table-row" />
      </section>
    </main>
  );
}

render(
  () => (
    <WebSocketContext.Provider
      value={
        {
          state: { pmg: [] },
          activeAlerts: {},
          connected: () => true,
          initialDataReceived: () => true,
          reconnecting: () => false,
        } as any
      }
    >
      <DarkModeContext.Provider value={() => document.documentElement.classList.contains('dark')}>
        <Fixture />
      </DarkModeContext.Provider>
    </WebSocketContext.Provider>
  ),
  document.getElementById('root')!,
);
