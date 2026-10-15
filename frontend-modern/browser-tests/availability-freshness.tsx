// Production snapshot mapper, workload row and full drawer. Synthetic evidence
// only; replacing a snapshot performs no availability probe or guest operation.
import { createSignal, Show } from 'solid-js';
import { useWorkloads } from '../src/hooks/useWorkloads';
import { GuestRow } from '../src/components/Workloads/GuestRow';
import { GuestDrawer } from '../src/components/Workloads/GuestDrawer';
import { renderInAppRuntime } from './fixtureRuntime';
import type { Resource, ResourceAvailabilityMeta } from '../src/types/resource';
import '../src/index.css';

const certificate: ResourceAvailabilityMeta['certificate'] = {
  observedAt: '2026-10-15T19:00:00Z',
  notBefore: '2026-09-01T00:00:00Z',
  notAfter: '2027-09-01T00:00:00Z',
  fingerprintSha256: 'a'.repeat(64),
  trustStatus: 'trusted',
  chainValid: true,
  hostnameValid: true,
  selfSigned: false,
};
const initial: ResourceAvailabilityMeta = {
  targetId: 'fixture-service',
  address: 'service.example.test',
  protocol: 'https',
  port: 443,
  path: '/health',
  enabled: true,
  available: true,
  aggregateState: 'healthy',
  correlationState: 'attached',
  latencyMillis: 8,
  transportOutcome: 'reachable',
  applicationOutcome: 'passed',
  applicationStatusCode: 200,
  certificate,
  expectedLocations: 1,
  reportingLocations: 1,
  locations: [{ locationId: 'pulse:local', kind: 'pulse', outcome: 'reachable', latencyMillis: 8 }],
};
const resource = (availability: ResourceAvailabilityMeta): Resource => ({
  id: 'fixture:pve-a:101',
  name: 'Service guest',
  displayName: 'Service guest',
  type: 'vm',
  platformType: 'proxmox-pve',
  platformScopes: ['proxmox-pve'],
  sourceType: 'api',
  sources: ['proxmox'],
  status: 'online',
  cpu: { current: 10 },
  memory: { current: 25, used: 256 * 1024 ** 2, total: 1024 ** 3 },
  proxmox: { vmid: 101, nodeName: 'pve-a', runtimeStatus: 'running' },
  availability,
});

function Fixture() {
  const [snapshot, setSnapshot] = createSignal([resource(initial)]);
  const state = useWorkloads(() => true, { resourceSnapshot: snapshot });
  const guest = () => state.workloads()[0];
  const [expanded, setExpanded] = createSignal(false);
  Object.assign(window, {
    availabilityFixture: {
      replace: (availability: ResourceAvailabilityMeta) => setSnapshot([resource(availability)]),
      initial,
    },
  });
  return (
    <main class="mx-auto min-h-screen max-w-4xl space-y-3 bg-surface p-3 text-base-content pulse-wide-data-surface">
      <h1 class="text-base font-semibold">Workloads</h1>
      <p class="text-xs text-muted">Synthetic service observations. No check is sent.</p>
      <Show when={guest()}>
        {(value) => (
          <table class="w-full table-fixed">
            <colgroup>
              <col style={{ width: '55%' }} />
              <col style={{ width: '45%' }} />
            </colgroup>
            <thead>
              <tr>
                <th scope="col">Name</th>
                <th scope="col">Availability</th>
              </tr>
            </thead>
            <tbody>
              <GuestRow
                guest={value()}
                visibleColumnIds={['name', 'availability']}
                workloadTableLayoutMode={
                  matchMedia('(max-width: 639px)').matches ? 'phone' : 'wide'
                }
                isExpanded={expanded()}
                onClick={() => setExpanded(!expanded())}
              />
              <Show when={expanded()}>
                <tr>
                  <td colspan="2" class="bg-surface-alt p-2">
                    <GuestDrawer guest={value()} onClose={() => setExpanded(false)} />
                  </td>
                </tr>
              </Show>
            </tbody>
          </table>
        )}
      </Show>
    </main>
  );
}
renderInAppRuntime(Fixture);
