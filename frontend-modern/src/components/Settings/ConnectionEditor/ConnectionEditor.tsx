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
import {
  INFRASTRUCTURE_AGENT_DISCOVERY_LABELS,
  INFRASTRUCTURE_ONBOARDING_PATHS,
  INFRASTRUCTURE_ONBOARDING_STEPS,
  getInfrastructureApiProductsByGovernanceState,
  getInfrastructureOnboardingProductPresentation,
  getInfrastructureSupportSummaryBadges,
} from '@/utils/infrastructureOnboardingPresentation';

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
}

const PLATFORM_TILE_META: Record<
  Exclude<ConnectionType, 'agent' | 'docker' | 'kubernetes'>,
  TileMeta
> = {
  pve: { icon: Server },
  pbs: { icon: Archive },
  pmg: { icon: Mail },
  vmware: { icon: ServerCog },
  truenas: { icon: Database },
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

  const supportedApiProducts = createMemo(() =>
    getInfrastructureApiProductsByGovernanceState('supported'),
  );
  const supportBadges = createMemo(() => getInfrastructureSupportSummaryBadges());

  const renderBadge = (label: string, tone: 'neutral' | 'accent' = 'neutral') => (
    <span
      class={
        tone === 'accent'
          ? 'inline-flex items-center rounded-full border border-blue-200 bg-blue-100 px-2 py-0.5 text-[11px] font-medium text-blue-800 dark:border-blue-900 dark:bg-blue-950/40 dark:text-blue-200'
          : 'inline-flex items-center rounded-full border border-border bg-surface px-2 py-0.5 text-[11px] font-medium text-base-content'
      }
    >
      {label}
    </span>
  );

  return (
    <div class="flex h-full flex-col">
      <Show
        when={showCredentialSlot()}
        fallback={
          <div class="space-y-6 p-4">
            <section class="rounded-xl border border-border bg-surface-alt p-4">
              <div class="space-y-4">
                <div class="space-y-1">
                  <div class="text-sm font-semibold text-base-content">
                    Choose how Pulse should connect
                  </div>
                  <p class="text-sm text-muted">
                    Pulse can monitor supported platforms through their management APIs, or install
                    Pulse Agent on a host for machine telemetry and local runtime discovery.
                  </p>
                </div>

                <div class="grid grid-cols-1 gap-3 xl:grid-cols-2">
                  <div class="rounded-lg border border-border bg-surface p-4">
                    <div class="space-y-3">
                      <div class="flex items-start justify-between gap-3">
                        <div class="space-y-1">
                          <div class="text-sm font-semibold text-base-content">
                            {INFRASTRUCTURE_ONBOARDING_PATHS.api.title}
                          </div>
                          <p class="text-xs text-muted">
                            {INFRASTRUCTURE_ONBOARDING_PATHS.api.bestFor}
                          </p>
                        </div>
                        <span class="rounded-full border border-border bg-surface-alt px-2 py-0.5 text-[10px] font-medium uppercase tracking-wide text-muted">
                          API path
                        </span>
                      </div>

                      <p class="text-xs text-muted">
                        {INFRASTRUCTURE_ONBOARDING_PATHS.api.description}
                      </p>

                      <div class="space-y-1.5">
                        <div class="text-[11px] font-medium uppercase tracking-wide text-muted">
                          Supported today
                        </div>
                        <div class="flex flex-wrap gap-1.5">
                          <For each={supportedApiProducts()}>
                            {(product) => renderBadge(product.label)}
                          </For>
                        </div>
                      </div>

                      <Show when={supportBadges().currentAdmissionPath.length > 0}>
                        <div class="space-y-1.5">
                          <div class="text-[11px] font-medium uppercase tracking-wide text-muted">
                            Current admission path
                          </div>
                          <div class="flex flex-wrap gap-1.5">
                            <For each={supportBadges().currentAdmissionPath}>
                              {(label) => renderBadge(label, 'accent')}
                            </For>
                          </div>
                        </div>
                      </Show>

                      <p class="text-xs text-muted">
                        {INFRASTRUCTURE_ONBOARDING_PATHS.api.coverage}
                      </p>
                    </div>
                  </div>

                  <div class="rounded-lg border border-emerald-200 bg-surface p-4 dark:border-emerald-900">
                    <div class="space-y-3">
                      <div class="flex items-start justify-between gap-3">
                        <div class="space-y-1">
                          <div class="text-sm font-semibold text-base-content">
                            {INFRASTRUCTURE_ONBOARDING_PATHS.agent.title}
                          </div>
                          <p class="text-xs text-muted">
                            {INFRASTRUCTURE_ONBOARDING_PATHS.agent.bestFor}
                          </p>
                        </div>
                        <span class="rounded-full border border-emerald-200 bg-emerald-100 px-2 py-0.5 text-[10px] font-medium uppercase tracking-wide text-emerald-800 dark:border-emerald-900 dark:bg-emerald-900/40 dark:text-emerald-200">
                          Agent path
                        </span>
                      </div>

                      <p class="text-xs text-muted">
                        {INFRASTRUCTURE_ONBOARDING_PATHS.agent.description}
                      </p>

                      <div class="flex flex-wrap gap-1.5">
                        <For each={supportBadges().installPath}>
                          {(label) => renderBadge(label)}
                        </For>
                      </div>

                      <p class="text-xs text-muted">
                        {INFRASTRUCTURE_ONBOARDING_PATHS.agent.coverage}
                      </p>
                    </div>
                  </div>
                </div>
              </div>
            </section>

            <section class="space-y-4 rounded-xl border border-border bg-surface p-4">
              <div class="space-y-1">
                <div class="text-sm font-semibold text-base-content">
                  Connect a supported platform
                </div>
                <p class="text-xs text-muted">
                  Paste an address to identify a supported platform automatically, or pick the
                  product you already know from the catalog.
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
                  <div class="space-y-3">
                    <div class="space-y-1">
                      <div class="text-sm font-semibold text-base-content">
                        Supported platform catalog
                      </div>
                      <p class="text-xs text-muted">
                        Choose the product path that matches your environment when you already know
                        the system type.
                      </p>
                    </div>

                    <div class="grid grid-cols-1 gap-3 sm:grid-cols-2 xl:grid-cols-3">
                      <For each={platformCatalogEntries()}>
                        {(entry) => {
                          if (entry.kind === 'type') {
                            const meta = PLATFORM_TILE_META[entry.type];
                            const Icon = meta.icon;
                            const product = getInfrastructureOnboardingProductPresentation(
                              entry.type,
                            );
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
                                  <div class="space-y-1">
                                    <div class="text-sm font-semibold text-base-content">
                                      {product.label}
                                    </div>
                                    <Show when={product.governanceState === 'admitted'}>
                                      <span class="inline-flex items-center rounded-full border border-blue-200 bg-blue-100 px-2 py-0.5 text-[10px] font-medium uppercase tracking-wide text-blue-800 dark:border-blue-900 dark:bg-blue-950/40 dark:text-blue-200">
                                        Current admission path
                                      </span>
                                    </Show>
                                  </div>
                                </div>
                                <div class="text-xs text-muted">{product.catalogDescription}</div>
                                <div class="text-[11px] text-muted">{product.bestFor}</div>
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
                            const product = getInfrastructureOnboardingProductPresentation(type);
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
                                  <div class="space-y-1">
                                    <div class="text-sm font-semibold text-base-content">
                                      {product.label}
                                    </div>
                                    <Show when={product.governanceState === 'admitted'}>
                                      <span class="inline-flex items-center rounded-full border border-blue-200 bg-blue-100 px-2 py-0.5 text-[10px] font-medium uppercase tracking-wide text-blue-800 dark:border-blue-900 dark:bg-blue-950/40 dark:text-blue-200">
                                        Current admission path
                                      </span>
                                    </Show>
                                  </div>
                                </div>
                                <div class="text-xs text-muted">{product.catalogDescription}</div>
                                <div class="text-[11px] text-muted">{product.bestFor}</div>
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

            <section class="space-y-3 rounded-xl border border-border bg-surface p-4">
              <div class="space-y-1">
                <div class="text-sm font-semibold text-base-content">Install Pulse Agent</div>
                <p class="text-xs text-muted">
                  Use the agent when you want machine telemetry, or when the system has no
                  management API Pulse can connect to directly.
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
                    Install on Linux, FreeBSD, or compatible hosts such as Unraid when you want CPU
                    temperature, disk SMART, system services, and network metrics from the host
                    itself.
                  </div>
                  <div class="text-xs text-muted">
                    Also detects local Docker, Kubernetes, and other supported services on the same
                    machine and connects them when available.
                  </div>
                  <div class="flex flex-wrap gap-1.5 pt-1">
                    <For each={INFRASTRUCTURE_AGENT_DISCOVERY_LABELS}>
                      {(label) => renderBadge(label)}
                    </For>
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

            <section class="rounded-xl border border-border bg-surface-alt p-4">
              <div class="space-y-3">
                <div class="space-y-1">
                  <div class="text-sm font-semibold text-base-content">What happens next</div>
                  <p class="text-xs text-muted">
                    Pulse guides each path through the same monitored-system admission flow before
                    the system lands in the shared infrastructure ledger.
                  </p>
                </div>

                <div class="grid grid-cols-1 gap-2 md:grid-cols-5">
                  <For each={INFRASTRUCTURE_ONBOARDING_STEPS}>
                    {(step, index) => (
                      <div class="rounded-lg border border-border bg-surface px-3 py-3">
                        <div class="text-[11px] font-medium uppercase tracking-wide text-muted">
                          Step {index() + 1}
                        </div>
                        <div class="mt-1 text-sm font-medium text-base-content">{step}</div>
                      </div>
                    )}
                  </For>
                </div>
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
