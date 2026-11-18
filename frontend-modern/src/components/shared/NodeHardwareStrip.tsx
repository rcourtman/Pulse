import { For, Show, createMemo, type Component } from 'solid-js';
import CpuIcon from 'lucide-solid/icons/cpu';
import EthernetPortIcon from 'lucide-solid/icons/ethernet-port';
import FanIcon from 'lucide-solid/icons/fan';
import MemoryStickIcon from 'lucide-solid/icons/memory-stick';
import WifiIcon from 'lucide-solid/icons/wifi';

import type { Node } from '@/types/api';
import { nodeHardwareModel } from '@/features/proxmox/nodeHardwareSummary';

interface NodeHardwareStripProps {
  node: Node;
  class?: string;
}

const ICON_CLASS = 'h-3.5 w-3.5 shrink-0 text-muted';

// Neutral sibling of the row's cluster and Agent badges: hardware reads as
// part of the node's identity, not as a reading.
const PILL_CLASS =
  'inline-flex items-center gap-1 rounded-sm bg-surface-alt px-1.5 py-0.5 text-[10px] font-medium whitespace-nowrap text-base-content';

/**
 * What a node is, as icons: its CPU, installed memory, one glyph per physical
 * port (with link speed when the agent reports it) and its fans. Inventory,
 * not live readings, so it sits on identity rows without adding row height.
 */
export const NodeHardwareStrip: Component<NodeHardwareStripProps> = (props) => {
  const hardware = createMemo(() => nodeHardwareModel(props.node));
  const hasAny = () => {
    const model = hardware();
    return Boolean(model.cpu || model.memory || model.fans || model.ports.length > 0);
  };

  return (
    <Show when={hasAny()}>
      <div
        data-testid="node-hardware-strip"
        class={`flex min-w-0 items-center gap-1.5 font-normal ${props.class ?? ''}`}
      >
        <Show when={hardware().cpu}>
          {(cpu) => (
            <span class={`${PILL_CLASS} min-w-0`} title={cpu().title}>
              <CpuIcon class={ICON_CLASS} aria-hidden="true" />
              <span class="truncate">{cpu().name}</span>
              <Show when={cpu().threads}>
                {(threads) => <span class="tabular-nums text-muted">×{threads()}</span>}
              </Show>
            </span>
          )}
        </Show>
        <Show when={hardware().memory}>
          {(memory) => (
            <span class={PILL_CLASS} title={memory().title}>
              <MemoryStickIcon class={ICON_CLASS} aria-hidden="true" />
              <span class="tabular-nums">{memory().label}</span>
            </span>
          )}
        </Show>
        <Show when={hardware().ports.length > 0}>
          <span
            class={`${PILL_CLASS} gap-1.5`}
            title={hardware()
              .ports.map((port) => port.title)
              .join('\n')}
          >
            <For each={hardware().ports}>
              {(port) => (
                <span class="inline-flex items-center gap-0.5">
                  <Show
                    when={port.wireless}
                    fallback={<EthernetPortIcon class={ICON_CLASS} aria-hidden="true" />}
                  >
                    <WifiIcon class={ICON_CLASS} aria-hidden="true" />
                  </Show>
                  <Show when={port.speedLabel}>
                    {(speed) => <span class="tabular-nums">{speed()}</span>}
                  </Show>
                </span>
              )}
            </For>
            <span class="sr-only">
              {`Network ports: ${hardware()
                .ports.map((port) => port.title)
                .join(', ')}`}
            </span>
          </span>
        </Show>
        <Show when={hardware().fans}>
          {(fans) => (
            <span class={PILL_CLASS} title={fans().title}>
              <FanIcon class={ICON_CLASS} aria-hidden="true" />
              <span>{fans().label}</span>
            </span>
          )}
        </Show>
      </div>
    </Show>
  );
};
