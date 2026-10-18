import { For, Show, createResource, createSignal } from 'solid-js';
import { render } from 'solid-js/web';
import { NodeDrawer } from '../src/components/Workloads/NodeDrawer';
import { nodeFromResource } from '../src/utils/resourceStateAdapters';
import type { Resource } from '../src/types/resource';
import '../src/index.css';

const Fixture = () => {
  const [states] = createResource(async () => {
    const response = await fetch('/proof/states');
    if (!response.ok) throw new Error('Missing producer states');
    return (await response.json()) as Record<string, Resource>;
  });
  const [selected, setSelected] = createSignal('id_rsa');
  const node = () => {
    const resource = states()?.[selected()];
    return resource ? nodeFromResource(resource) : null;
  };
  return (
    <main class="min-h-screen space-y-4 bg-surface p-4 text-base-content">
      <h1>Cluster peer sensor proof</h1>
      <p>Invented local keys and readings; production resource adapter and node drawer.</p>
      <nav class="flex flex-wrap gap-2" aria-label="Producer state">
        <For each={['id_rsa', 'id_ed25519', 'denied', 'recovered', 'stale', 'foreign', 'local']}>
          {(name) => (
            <button
              class="min-h-11 border px-3"
              aria-pressed={selected() === name}
              onClick={() => setSelected(name)}
            >
              {name}
            </button>
          )}
        </For>
      </nav>
      <Show when={node()}>{(value) => <NodeDrawer node={value()} />}</Show>
    </main>
  );
};
render(() => <Fixture />, document.getElementById('root')!);
