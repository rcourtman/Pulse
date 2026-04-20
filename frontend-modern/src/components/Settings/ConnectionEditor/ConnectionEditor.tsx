import { Component, For, type JSX, Show, createMemo, createSignal } from 'solid-js';
import { Archive, Cpu, Database, Mail, Server, ServerCog } from 'lucide-solid';
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

const DEFAULT_MANUAL_TYPES: ConnectionType[] = ['pve', 'pbs', 'pmg', 'vmware', 'truenas', 'agent'];

interface TileMeta {
  icon: Component<{ class?: string }>;
  description: string;
}

const TILE_META: Partial<Record<ConnectionType, TileMeta>> = {
  pve: { icon: Server, description: 'VMs, containers, storage, backups' },
  pbs: { icon: Archive, description: 'Backups, sync and verify jobs' },
  pmg: { icon: Mail, description: 'Mail stats, queues, quarantine' },
  vmware: { icon: ServerCog, description: 'vCenter or ESXi clusters' },
  truenas: { icon: Database, description: 'Pools, datasets, replications' },
  agent: {
    icon: Cpu,
    description: 'Host metrics, or bare-metal Linux / Unraid / FreeBSD',
  },
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

  const manualOptions = createMemo(() => props.manualTypeOptions ?? DEFAULT_MANUAL_TYPES);

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
          <div class="space-y-6 p-4">
            <AddressProbeStep
              state={state}
              onSelectCandidate={chooseCandidate}
              onInstallAgent={() => chooseManualType('agent')}
            />

            <div class="flex items-center gap-3 text-xs font-semibold uppercase tracking-wide text-muted">
              <span class="h-px flex-1 bg-border" aria-hidden="true" />
              Or pick your system directly
              <span class="h-px flex-1 bg-border" aria-hidden="true" />
            </div>

            <div class="grid grid-cols-1 gap-3 sm:grid-cols-2 xl:grid-cols-3">
              <For each={manualOptions()}>
                {(type) => {
                  const meta = TILE_META[type];
                  const Icon = meta?.icon ?? Server;
                  const label = CONNECTION_TYPE_LABELS[type] ?? type;
                  const isAgent = type === 'agent';
                  return (
                    <button
                      type="button"
                      onClick={() => chooseManualType(type)}
                      class="group flex h-full flex-col gap-2 rounded-lg border border-border bg-surface p-4 text-left transition-colors hover:border-blue-500 hover:bg-blue-50/40 dark:hover:bg-blue-950/20"
                    >
                      <div class="flex items-center gap-2.5">
                        <div
                          aria-hidden="true"
                          class={
                            isAgent
                              ? 'flex h-8 w-8 flex-none items-center justify-center rounded-md border border-blue-200 bg-blue-100 text-blue-700 dark:border-blue-900 dark:bg-blue-900/40 dark:text-blue-300'
                              : 'flex h-8 w-8 flex-none items-center justify-center rounded-md border border-border bg-surface-alt text-base-content'
                          }
                        >
                          <Icon class="h-4 w-4" />
                        </div>
                        <div class="text-sm font-semibold text-base-content">
                          {isAgent ? 'Install Pulse Agent' : label}
                        </div>
                      </div>
                      <Show when={meta?.description}>
                        <div class="text-xs text-muted">{meta!.description}</div>
                      </Show>
                    </button>
                  );
                }}
              </For>
            </div>
          </div>
        }
      >
        <div class="flex items-center justify-between border-b border-border px-4 py-2">
          <div class="text-sm">
            <span class="font-semibold text-base-content">
              {activeType() === 'agent'
                ? 'Install Pulse Agent'
                : CONNECTION_TYPE_LABELS[activeType()!] ?? activeType()}
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
