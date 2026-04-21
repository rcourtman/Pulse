import { Component, For, type JSX, Show, createMemo, createSignal } from 'solid-js';
import { Archive, ArrowRight, Cpu, Database, Mail, Server, ServerCog } from 'lucide-solid';
import type { ConnectionType, ProbeCandidate } from '@/api/connections';
import { AddressProbeStep } from './AddressProbeStep';
import {
  buildConnectionEditorCatalogEntries,
  CONNECTION_TYPE_LABELS,
  type ConnectionEditorCatalogFamilyEntry,
  type ConnectionEditorCatalogFamilyId,
  type ConnectionEditorCatalogEntry,
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

const PLATFORM_TILE_META: Record<
  Exclude<ConnectionType, 'agent' | 'docker' | 'kubernetes'>,
  TileMeta
> = {
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
  const [selectedFamilyId, setSelectedFamilyId] =
    createSignal<ConnectionEditorCatalogFamilyId | null>(null);

  const platformCatalogEntries = createMemo<ConnectionEditorCatalogEntry[]>(() =>
    buildConnectionEditorCatalogEntries(props.manualTypeOptions),
  );
  const activeFamily = createMemo<ConnectionEditorCatalogFamilyEntry | null>(() => {
    const familyId = selectedFamilyId();
    if (!familyId) return null;
    const family = platformCatalogEntries().find(
      (entry): entry is ConnectionEditorCatalogFamilyEntry =>
        entry.kind === 'family' && entry.id === familyId,
    );
    return family ?? null;
  });

  const activeType = () => selectedType();
  const showCredentialSlot = () => activeType() !== null;

  const chooseCandidate = (candidate: ProbeCandidate) => {
    setSelectedCandidate(candidate);
    setSelectedFamilyId(null);
    setSelectedType(candidate.type);
  };

  const chooseManualType = (type: ConnectionType) => {
    setSelectedCandidate(null);
    setSelectedFamilyId(null);
    setSelectedType(type);
  };

  const chooseFamily = (familyId: ConnectionEditorCatalogFamilyId) => {
    setSelectedCandidate(null);
    setSelectedFamilyId(familyId);
  };

  const reopenFamilies = () => {
    setSelectedFamilyId(null);
  };

  const reopenProbe = () => {
    state.reset();
    setSelectedCandidate(null);
    setSelectedFamilyId(null);
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

              <Show
                when={activeFamily()}
                fallback={
                  <div class="grid grid-cols-1 gap-3 sm:grid-cols-2 xl:grid-cols-3">
                    <For each={platformCatalogEntries()}>
                      {(entry) => {
                        if (entry.kind === 'type') {
                          const meta = PLATFORM_TILE_META[entry.type];
                          const Icon = meta.icon;
                          const label = CONNECTION_TYPE_LABELS[entry.type];
                          return (
                            <button
                              type="button"
                              onClick={() => chooseManualType(entry.type)}
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
                              <div class="text-xs text-muted">{meta.description}</div>
                            </button>
                          );
                        }

                        const familyLeadType = entry.childTypes[0];
                        const Icon = PLATFORM_TILE_META[familyLeadType]?.icon ?? Server;
                        return (
                          <button
                            type="button"
                            onClick={() => chooseFamily(entry.id)}
                            class="group flex h-full flex-col gap-2 rounded-lg border border-border bg-surface p-4 text-left transition-colors hover:border-blue-500 hover:bg-blue-50/40 dark:hover:bg-blue-950/20"
                          >
                            <div class="flex items-center gap-2.5">
                              <div
                                aria-hidden="true"
                                class="flex h-8 w-8 flex-none items-center justify-center rounded-md border border-border bg-surface-alt text-base-content"
                              >
                                <Icon class="h-4 w-4" />
                              </div>
                              <div class="text-sm font-semibold text-base-content">
                                {entry.label}
                              </div>
                            </div>
                            <div class="text-xs text-muted">{entry.description}</div>
                          </button>
                        );
                      }}
                    </For>
                  </div>
                }
              >
                {(familyAccessor) => {
                  const family = familyAccessor();
                  return (
                    <div class="space-y-4 rounded-lg border border-border bg-surface p-4">
                      <div class="flex flex-wrap items-start justify-between gap-3">
                        <div class="space-y-1">
                          <div class="text-sm font-semibold text-base-content">
                            Choose a {family.label} product
                          </div>
                          <p class="text-xs text-muted">
                            Pick the {family.label} product you want to connect. Pulse will open the
                            matching credential flow next.
                          </p>
                        </div>
                        <button
                          type="button"
                          onClick={reopenFamilies}
                          class="inline-flex items-center rounded-md border border-border px-2.5 py-1 text-xs font-medium text-base-content transition-colors hover:bg-surface-hover"
                        >
                          ← Back to platforms
                        </button>
                      </div>

                      <div class="grid grid-cols-1 gap-3 sm:grid-cols-2 xl:grid-cols-3">
                        <For each={family.childTypes}>
                          {(type) => {
                            const meta = PLATFORM_TILE_META[type];
                            const Icon = meta.icon;
                            return (
                              <button
                                type="button"
                                onClick={() => chooseManualType(type)}
                                class="group flex h-full flex-col gap-2 rounded-lg border border-border bg-surface-alt p-4 text-left transition-colors hover:border-blue-500 hover:bg-blue-50/40 dark:hover:bg-blue-950/20"
                              >
                                <div class="flex items-center gap-2.5">
                                  <div
                                    aria-hidden="true"
                                    class="flex h-8 w-8 flex-none items-center justify-center rounded-md border border-border bg-surface text-base-content"
                                  >
                                    <Icon class="h-4 w-4" />
                                  </div>
                                  <div class="text-sm font-semibold text-base-content">
                                    {CONNECTION_TYPE_LABELS[type]}
                                  </div>
                                </div>
                                <div class="text-xs text-muted">{meta.description}</div>
                              </button>
                            );
                          }}
                        </For>
                      </div>
                    </div>
                  );
                }}
              </Show>
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
