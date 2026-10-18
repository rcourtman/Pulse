// Safe read-only transport through the real Protection reader. Docker rows
// contain synthetic metadata names only, never secret values or provider reads.
import { createSignal } from 'solid-js';
import { render } from 'solid-js/web';
import { DockerSecretsTable } from '../src/features/docker/DockerSecretsTable';
import { TrueNASProtectionTable } from '../src/features/truenas/TrueNASProtectionTable';
import { useRecoveryPoints } from '../src/hooks/useRecoveryPoints';
import type { Resource } from '../src/types/resource';
import '../src/index.css';

const secrets: Resource[] = ['bravo', 'alpha', 'charlie'].map((name, index) => ({
  id: `fixture-secret-${name}`,
  name,
  displayName: name,
  type: 'docker-secret',
  status: 'online',
  platformId: 'fixture-docker',
  platformType: 'docker',
  sourceType: 'agent',
  lastSeen: 1_792_310_400_000,
  docker: {
    hostname: 'fixture-docker',
    driver: 'local',
    objectCreatedAt: `2026-10-${16 + index}T05:00:00Z`,
    labels: { purpose: 'synthetic-metadata' },
  },
}));

function Fixture() {
  const recovery = useRecoveryPoints(() => ({ platform: 'truenas', page: 1, limit: 200 }));
  const [submits, setSubmits] = createSignal(0);
  return (
    <main class="mx-auto max-w-6xl space-y-6 p-3 text-base-content">
      <h1 class="text-xl font-semibold">Platform sorting</h1>
      <section aria-label="Protection activity" class="space-y-2">
        <button type="button" class="min-h-11 rounded-md border border-border px-3">
          Before Protection sorting
        </button>
        <TrueNASProtectionTable
          points={recovery.points()}
          loading={recovery.response.loading}
          error={recovery.response.error}
          emptyIcon={<span />}
          emptyTitle="No protection"
          emptyDescription="No protection"
          showToolbar={false}
        />
      </section>
      <form
        aria-label="Docker metadata"
        class="space-y-2"
        onSubmit={(event) => {
          event.preventDefault();
          setSubmits((value) => value + 1);
        }}
      >
        <button type="button" class="min-h-11 rounded-md border border-border px-3">
          Before Docker sorting
        </button>
        <DockerSecretsTable
          resources={secrets}
          emptyIcon={<span />}
          emptyTitle="No metadata"
          emptyDescription="No metadata"
          showToolbar={false}
        />
        <output data-testid="form-submits">{submits()}</output>
      </form>
    </main>
  );
}

render(() => <Fixture />, document.getElementById('root')!);
