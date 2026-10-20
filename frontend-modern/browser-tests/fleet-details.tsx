// Real table, filters, history API client and detail drawer. Only the current
// snapshots and offline API responses are synthetic; no probe is executed.
import { createSignal } from 'solid-js';
import {
  AvailabilityChecksTable,
  type AvailabilityChecksView,
} from '../src/features/standalone/AvailabilityChecksTable';
import type { Resource } from '../src/types/resource';
import { renderInAppRuntime } from './fixtureRuntime';
import '../src/index.css';

const now = Date.now();
const initial: Resource[] = Array.from({ length: 50 }, (_, index) => {
  const n = index + 1;
  const down = n % 5 === 0;
  const unknown = n % 5 === 2;
  const name = `Probe ${String(n).padStart(2, '0')}`;
  return {
    id: `availability:target-${n}`,
    name,
    displayName: name,
    type: 'network-endpoint',
    platformId: `target-${n}`,
    platformType: 'availability',
    sourceType: 'api',
    sources: ['availability'],
    status: unknown ? 'unknown' : down ? 'offline' : 'online',
    lastSeen: now,
    availability: {
      targetId: `target-${n}`,
      protocol: n % 4 === 0 ? 'tcp' : n % 4 === 1 ? 'https' : n % 4 === 2 ? 'udp' : 'icmp',
      address: `service-${n}.example.test`,
      port: n % 4 === 0 ? 443 : undefined,
      path: n % 4 === 1 ? '/health' : undefined,
      enabled: true,
      available: unknown ? undefined : !down,
      outcome: unknown ? 'indeterminate' : down ? 'unreachable' : 'reachable',
      transportOutcome: unknown ? 'indeterminate' : down ? 'unreachable' : 'reachable',
      latencyMillis: down || unknown ? undefined : 10 + n,
      lastChecked: new Date(now - 1000).toISOString(),
      pollIntervalSeconds: 600,
      failureThreshold: 2,
      consecutiveFailures: down ? 3 : 0,
    },
  } as Resource;
});

function Fixture() {
  const [resources, setResources] = createSignal(initial);
  const [view, setView] = createSignal<AvailabilityChecksView>('fleet');
  Object.assign(window, {
    fleetFixture: {
      // Replace every object, as a real inventory refresh does. Recovering the
      // selected target changes attention ordering without changing its ID.
      recover: (id: string) =>
        setResources((current) =>
          current.map((resource): Resource => ({
            ...resource,
            ...(resource.id === id
              ? {
                  name: 'Probe 10 recovered',
                  displayName: 'Probe 10 recovered',
                  status: 'online',
                  availability: {
                    ...resource.availability!,
                    available: true,
                    outcome: 'reachable',
                    transportOutcome: 'reachable',
                    consecutiveFailures: 0,
                    latencyMillis: 7,
                    lastChecked: new Date().toISOString(),
                  },
                }
              : {}),
          })),
        ),
      refresh: () => setResources((current) => current.map((resource) => ({ ...resource }))),
      clear: () => setResources([]),
      remove: (id: string) => setResources((current) => current.filter((r) => r.id !== id)),
      restore: (id: string) =>
        setResources((current) => [...current, initial.find((r) => r.id === id)!]),
    },
  });
  return (
    <main class="min-h-screen space-y-3 bg-surface p-3 text-base-content">
      <h1 class="text-base font-semibold">Availability — 50 synthetic targets</h1>
      <p class="text-xs text-muted">Offline navigation proof. No network check is sent.</p>
      <AvailabilityChecksTable
        resources={resources()}
        emptyIcon={<span />}
        emptyTitle="No checks"
        emptyDescription="No fixtures"
        view={view()}
        onViewChange={setView}
      />
    </main>
  );
}

renderInAppRuntime(Fixture);
