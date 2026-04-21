import { Component, For, type JSX, Show, createMemo, createSignal } from 'solid-js';
import { Archive, ArrowRight, Cpu, Database, Mail, Server, ServerCog } from 'lucide-solid';
import type { ConnectionType, ProbeCandidate } from '@/api/connections';
import { AddressProbeStep } from './AddressProbeStep';
import {
  CONNECTION_TYPE_LABELS,
  DEFAULT_CONNECTION_EDITOR_PLATFORM_TYPES,
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
    (props.manualTypeOptions ?? DEFAULT_CONNECTION_EDITOR_PLATFORM_TYPES).filter(
      (type) => type !== 'agent',
    ),
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
            <section class="space-y-4">
              <div class="space-y-1">
                <div class="text-sm font-semibold text-base-content">Connect a platform</div>
                <p class="text-xs text-muted">
                  Use a management API when the product exposes one. Paste an address to auto-detect
                  it, or pick a supported platform from the catalog.
                </p>
              </div>

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

            <section class="space-y-3 border-t border-border pt-6">
              <div class="space-y-1">
                <div class="text-sm font-semibold text-base-content">Install on a host instead</div>
                <p class="text-xs text-muted">
                  Use the Pulse Agent when you want machine-level telemetry or the system has no
                  management API to connect.
                </p>
              </div>

              <button
                type="button"
                onClick={() => chooseManualType('agent')}
                class="group flex w-full items-start gap-4 rounded-lg border border-border bg-surface p-4 text-left transition-colors hover:border-blue-500 hover:bg-blue-50/40 dark:hover:bg-blue-950/20"
              >
                <div
                  aria-hidden="true"
                  class="flex h-10 w-10 flex-none items-center justify-center rounded-md border border-emerald-200 bg-emerald-100 text-emerald-700 dark:border-emerald-900 dark:bg-emerald-900/40 dark:text-emerald-300"
                >
                  <Cpu class="h-5 w-5" />
                </div>
                <div class="flex-1 space-y-1.5">
                  <div class="flex flex-wrap items-center gap-2">
                    <div class="text-sm font-semibold text-base-content">Install Pulse Agent</div>
                    <div class="rounded-full border border-border bg-surface px-2 py-0.5 text-[10px] font-medium uppercase tracking-wide text-muted">
                      Runs on a host
                    </div>
                  </div>
                  <div class="text-xs text-muted">
                    Install on bare-metal Linux, Unraid, FreeBSD, or any machine where you want CPU
                    temperature, disk SMART, systemd services, and network metrics from the host
                    itself.
                  </div>
                  <div class="text-xs text-muted">
                    On supported Proxmox hosts, the installer can also detect local PVE / PBS
                    services, create the needed API token(s), and register them automatically.
                    Docker and Kubernetes on that machine are detected too.
                  </div>
                </div>
                <div
                  aria-hidden="true"
                  class="flex-none self-center text-muted transition-colors group-hover:text-blue-700 dark:group-hover:text-blue-300"
                >
                  <ArrowRight class="h-4 w-4" />
                </div>
              </button>
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
