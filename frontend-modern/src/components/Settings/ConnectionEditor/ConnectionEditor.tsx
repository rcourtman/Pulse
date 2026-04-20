import { Component, For, type JSX, Show, createMemo, createSignal } from 'solid-js';
import { Archive, ArrowRight, Cpu, Database, Mail, Server, ServerCog } from 'lucide-solid';
import type { ConnectionType, ProbeCandidate } from '@/api/connections';
import { AddressProbeStep } from './AddressProbeStep';
import {
  CONNECTION_TYPE_LABELS,
  createConnectionEditorState,
  type ConnectionEditorState,
} from './useConnectionEditor';

export type ConnectionEditorMode = 'add' | 'edit';

export interface ConnectionEditorSlotContext {
  mode: ConnectionEditorMode;
  type: ConnectionType;
  candidate: ProbeCandidate | null;
  onCancel: () => void;
  onSaved: () => void;
}

export type CredentialSlotRenderer = (context: ConnectionEditorSlotContext) => JSX.Element;

export interface ConnectionEditorProps {
  mode?: ConnectionEditorMode;
  initialType?: ConnectionType;
  initialAddress?: string;
  renderCredentialSlot: CredentialSlotRenderer;
  manualTypeOptions?: ConnectionType[];
  onClose: () => void;
  onSaved?: () => void;
}

// Platform integrations — connect to a product's management API. Peers of
// each other. The agent is NOT in this list: it is a different kind of
// integration (see the dedicated section below the grid) and surfacing it
// as a tile alongside these hides what it actually adds.
const DEFAULT_PLATFORM_TYPES: ConnectionType[] = ['pve', 'pbs', 'pmg', 'vmware', 'truenas'];

interface TileMeta {
  icon: Component<{ class?: string }>;
  description: string;
}

const PLATFORM_TILE_META: Partial<Record<ConnectionType, TileMeta>> = {
  pve: { icon: Server, description: 'VMs, containers, storage, backups' },
  pbs: { icon: Archive, description: 'Backups, sync and verify jobs' },
  pmg: { icon: Mail, description: 'Mail stats, queues, quarantine' },
  vmware: { icon: ServerCog, description: 'vCenter or ESXi clusters' },
  truenas: { icon: Database, description: 'Pools, datasets, replications' },
};

export const ConnectionEditor: Component<ConnectionEditorProps> = (props) => {
  const state: ConnectionEditorState = createConnectionEditorState();
  if (props.initialAddress) {
    state.setAddress(props.initialAddress);
  }

  const [selectedType, setSelectedType] = createSignal<ConnectionType | null>(
    props.initialType ?? null,
  );
  const [selectedCandidate, setSelectedCandidate] = createSignal<ProbeCandidate | null>(null);

  const platformOptions = createMemo(() =>
    (props.manualTypeOptions ?? DEFAULT_PLATFORM_TYPES).filter((type) => type !== 'agent'),
  );

  const activeType = () => selectedType();
  const showCredentialSlot = () => activeType() !== null;

  const chooseCandidate = (candidate: ProbeCandidate) => {
    setSelectedCandidate(candidate);
    setSelectedType(candidate.type);
  };

  const chooseManualType = (type: ConnectionType) => {
    setSelectedCandidate(null);
    setSelectedType(type);
  };

  const reopenProbe = () => {
    state.reset();
    setSelectedCandidate(null);
    setSelectedType(null);
  };

  const handleSaved = () => {
    props.onSaved?.();
    props.onClose();
  };

  return (
    <div class="flex h-full flex-col">
      <Show
        when={showCredentialSlot()}
        fallback={
          <div class="space-y-8 p-4">
            <section class="space-y-3">
              <button
                type="button"
                onClick={() => chooseManualType('agent')}
                class="group flex w-full items-start gap-4 rounded-lg border-2 border-blue-300 bg-blue-50/40 p-4 text-left transition-colors hover:border-blue-500 hover:bg-blue-50 dark:border-blue-800 dark:bg-blue-950/20 dark:hover:border-blue-600 dark:hover:bg-blue-950/40"
              >
                <div
                  aria-hidden="true"
                  class="flex h-10 w-10 flex-none items-center justify-center rounded-md border border-blue-200 bg-blue-100 text-blue-700 dark:border-blue-900 dark:bg-blue-900/40 dark:text-blue-300"
                >
                  <Cpu class="h-5 w-5" />
                </div>
                <div class="flex-1 space-y-1.5">
                  <div class="flex flex-wrap items-center gap-2">
                    <div class="text-sm font-semibold text-base-content">Install Pulse Agent</div>
                    <div class="rounded-full border border-blue-300 bg-blue-100 px-2 py-0.5 text-[10px] font-semibold uppercase tracking-wide text-blue-800 dark:border-blue-700 dark:bg-blue-900/60 dark:text-blue-200">
                      Recommended
                    </div>
                    <div class="rounded-full border border-border bg-surface px-2 py-0.5 text-[10px] font-medium uppercase tracking-wide text-muted">
                      Runs on a host
                    </div>
                  </div>
                  <div class="text-xs text-muted">
                    <span class="font-medium text-base-content">
                      On a Proxmox host, this is the fastest path.
                    </span>{' '}
                    The installer auto-detects Proxmox on the machine, creates the needed API
                    token(s), and auto-registers any detected PVE / PBS services — no address or
                    credentials to paste.
                  </div>
                  <div class="text-xs text-muted">
                    It also reports CPU temperature, disk SMART, systemd services, and network
                    metrics from the host, and auto-detects Docker and Kubernetes on that machine.
                    Required for bare-metal Linux / Unraid / FreeBSD targets that have no platform
                    API to connect.
                  </div>
                </div>
                <div
                  aria-hidden="true"
                  class="flex-none self-center text-blue-600 transition-colors group-hover:text-blue-700 dark:text-blue-400 dark:group-hover:text-blue-300"
                >
                  <ArrowRight class="h-4 w-4" />
                </div>
              </button>
            </section>

            <section class="space-y-4">
              <div class="flex items-center gap-3 text-xs font-semibold uppercase tracking-wide text-muted">
                <span class="h-px flex-1 bg-border" aria-hidden="true" />
                Or connect a platform API directly
                <span class="h-px flex-1 bg-border" aria-hidden="true" />
              </div>
              <p class="text-xs text-muted">
                For VMware, TrueNAS, PMG, or a remote Proxmox you can't install the agent on. Paste
                the address and Pulse will auto-detect the product, or pick it below.
              </p>

              <AddressProbeStep
                state={state}
                onSelectCandidate={chooseCandidate}
                onInstallAgent={() => chooseManualType('agent')}
              />

              <div class="grid grid-cols-1 gap-3 sm:grid-cols-2 xl:grid-cols-3">
                <For each={platformOptions()}>
                  {(type) => {
                    const meta = PLATFORM_TILE_META[type];
                    const Icon = meta?.icon ?? Server;
                    const label = CONNECTION_TYPE_LABELS[type] ?? type;
                    return (
                      <button
                        type="button"
                        onClick={() => chooseManualType(type)}
                        class="group flex h-full flex-col gap-2 rounded-lg border border-border bg-surface p-4 text-left transition-colors hover:border-blue-500 hover:bg-blue-50/40 dark:hover:bg-blue-950/20"
                      >
                        <div class="flex items-center gap-2.5">
                          <div
                            aria-hidden="true"
                            class="flex h-8 w-8 flex-none items-center justify-center rounded-md border border-border bg-surface-alt text-base-content"
                          >
                            <Icon class="h-4 w-4" />
                          </div>
                          <div class="text-sm font-semibold text-base-content">{label}</div>
                        </div>
                        <Show when={meta?.description}>
                          <div class="text-xs text-muted">{meta!.description}</div>
                        </Show>
                      </button>
                    );
                  }}
                </For>
              </div>
            </section>
          </div>
        }
      >
        <div class="flex items-center justify-between border-b border-border px-4 py-2">
          <div class="text-sm">
            <span class="font-semibold text-base-content">
              {activeType() === 'agent'
                ? 'Install Pulse Agent'
                : (CONNECTION_TYPE_LABELS[activeType()!] ?? activeType())}
            </span>
            <Show when={selectedCandidate()}>
              <span class="ml-2 text-xs text-muted">{selectedCandidate()!.host}</span>
            </Show>
          </div>
          <Show when={(props.mode ?? 'add') === 'add'}>
            <button
              type="button"
              onClick={reopenProbe}
              class="inline-flex items-center rounded-md border border-border px-2.5 py-1 text-xs font-medium text-base-content transition-colors hover:bg-surface-hover"
            >
              ← Back to catalog
            </button>
          </Show>
        </div>

        <div class="flex-1 overflow-y-auto p-4">
          {props.renderCredentialSlot({
            mode: props.mode ?? 'add',
            type: activeType()!,
            candidate: selectedCandidate(),
            onCancel: props.onClose,
            onSaved: handleSaved,
          })}
        </div>
      </Show>
    </div>
  );
};
