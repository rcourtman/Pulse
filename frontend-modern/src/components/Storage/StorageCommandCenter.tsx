import { Accessor, Component, For, Show, createMemo, createResource, createSignal } from 'solid-js';
import { apiFetchJSON } from '@/utils/apiClient';
import { Card } from '@/components/shared/Card';

type StorageSummaryIncident = {
  resourceId: string;
  resourceType: string;
  name: string;
  parentName?: string;
  platform?: string;
  topology?: string;
  status: string;
  incidentCount: number;
  incidentCategory?: string;
  incidentLabel?: string;
  incidentSeverity?: string;
  incidentPriority?: number;
  incidentSummary?: string;
  incidentImpactSummary?: string;
  incidentUrgency?: string;
  incidentAction?: string;
  protectionReduced?: boolean;
  rebuildInProgress?: boolean;
  consumerCount?: number;
  protectedWorkloads?: number;
  affectedDatastores?: number;
};

type StorageSummaryResponse = {
  generatedAt: string;
  totalResources: number;
  riskyResources: number;
  criticalResources: number;
  warningResources: number;
  protectionReducedCount: number;
  rebuildInProgressCount: number;
  dependentResourceCount: number;
  protectedWorkloadCount: number;
  affectedDatastoreCount: number;
  byPlatform: Record<string, number>;
  byResourceType: Record<string, number>;
  byIncidentCategory: Record<string, number>;
  topIncidents: StorageSummaryIncident[];
};

type StorageIncidentSection = {
  category: string;
  label: string;
  resourceCount: number;
  criticalResources: number;
  warningResources: number;
  primaryUrgency?: string;
  resources: StorageSummaryIncident[];
};

type StorageIncidentsResponse = {
  generatedAt: string;
  totalResources: number;
  criticalResources: number;
  warningResources: number;
  byCategory: Record<string, number>;
  byUrgency: Record<string, number>;
  sections: StorageIncidentSection[];
};

type StorageCommandCenterProps = {
  lastUpdateToken: Accessor<string>;
  search: Accessor<string>;
  selectedNodeId: Accessor<string>;
  sourceFilter: Accessor<string>;
};

type StorageCommandCenterPayload = {
  summary: StorageSummaryResponse;
  incidents: StorageIncidentsResponse;
};

const emptyStorageSummaryResponse = (): StorageSummaryResponse => ({
  generatedAt: '',
  totalResources: 0,
  riskyResources: 0,
  criticalResources: 0,
  warningResources: 0,
  protectionReducedCount: 0,
  rebuildInProgressCount: 0,
  dependentResourceCount: 0,
  protectedWorkloadCount: 0,
  affectedDatastoreCount: 0,
  byPlatform: {},
  byResourceType: {},
  byIncidentCategory: {},
  topIncidents: [],
});

const emptyStorageIncidentsResponse = (): StorageIncidentsResponse => ({
  generatedAt: '',
  totalResources: 0,
  criticalResources: 0,
  warningResources: 0,
  byCategory: {},
  byUrgency: {},
  sections: [],
});

const sourceFilterToCanonicalSource = (value: string): string | null => {
  switch ((value || '').trim().toLowerCase()) {
    case '':
    case 'all':
      return null;
    case 'proxmox':
    case 'proxmox-pve':
      return 'proxmox';
    case 'proxmox-pbs':
    case 'pbs':
      return 'pbs';
    case 'proxmox-pmg':
    case 'pmg':
      return 'pmg';
    case 'agent':
      return 'agent';
    case 'truenas':
      return 'truenas';
    default:
      return null;
  }
};

const buildStorageCommandCenterQuery = (
  search: string,
  selectedNodeId: string,
  sourceFilter: string,
): string => {
  const params = new URLSearchParams();
  const trimmedSearch = search.trim();
  if (trimmedSearch) {
    params.set('q', trimmedSearch);
  }
  const trimmedParent = selectedNodeId.trim();
  if (trimmedParent && trimmedParent !== 'all') {
    params.set('parent', trimmedParent);
  }
  const canonicalSource = sourceFilterToCanonicalSource(sourceFilter);
  if (canonicalSource) {
    params.set('source', canonicalSource);
  }
  const query = params.toString();
  return query ? `?${query}` : '';
};

const severityPillClass = (severity: string | undefined): string => {
  switch ((severity || '').trim().toLowerCase()) {
    case 'critical':
      return 'bg-red-100 text-red-700 dark:bg-red-950/60 dark:text-red-300';
    case 'warning':
      return 'bg-amber-100 text-amber-800 dark:bg-amber-950/60 dark:text-amber-300';
    default:
      return 'bg-surface-hover text-muted';
  }
};

const urgencyPillClass = (urgency: string | undefined): string => {
  switch ((urgency || '').trim().toLowerCase()) {
    case 'now':
      return 'bg-red-600 text-white';
    case 'today':
      return 'bg-orange-100 text-orange-800 dark:bg-orange-950/60 dark:text-orange-300';
    case 'monitor':
      return 'bg-blue-100 text-blue-800 dark:bg-blue-950/60 dark:text-blue-300';
    default:
      return 'bg-surface-hover text-muted';
  }
};

const sectionAccentClass = (category: string): string => {
  switch ((category || '').trim().toLowerCase()) {
    case 'recoverability':
      return 'border-l-4 border-l-red-500';
    case 'protection':
      return 'border-l-4 border-l-orange-500';
    case 'rebuild':
      return 'border-l-4 border-l-blue-500';
    case 'capacity':
      return 'border-l-4 border-l-amber-500';
    case 'disk-health':
      return 'border-l-4 border-l-slate-500';
    default:
      return 'border-l-4 border-l-border';
  }
};

const compactNumber = (value: number): string =>
  new Intl.NumberFormat(undefined, { notation: 'compact', maximumFractionDigits: 1 }).format(
    value || 0,
  );

const titleize = (value: string | undefined): string =>
  (value || '')
    .split(/[\s_-]+/)
    .filter(Boolean)
    .map((part) => part.charAt(0).toUpperCase() + part.slice(1))
    .join(' ');

export const StorageCommandCenter: Component<StorageCommandCenterProps> = (props) => {
  const [loadError, setLoadError] = createSignal<unknown>(null);
  const requestKey = createMemo(() => ({
    lastUpdateToken: props.lastUpdateToken(),
    search: props.search(),
    selectedNodeId: props.selectedNodeId(),
    sourceFilter: props.sourceFilter(),
  }));

  const [payload] = createResource(
    requestKey,
    async (key): Promise<StorageCommandCenterPayload> => {
      const query = buildStorageCommandCenterQuery(
        key.search,
        key.selectedNodeId,
        key.sourceFilter,
      );
      try {
        const [summary, incidents] = await Promise.all([
          apiFetchJSON<StorageSummaryResponse>(`/api/resources/storage-summary${query}`, {
            cache: 'no-store',
          }),
          apiFetchJSON<StorageIncidentsResponse>(`/api/resources/storage-incidents${query}`, {
            cache: 'no-store',
          }),
        ]);
        setLoadError(null);
        return { summary, incidents };
      } catch (error) {
        setLoadError(error);
        return {
          summary: emptyStorageSummaryResponse(),
          incidents: emptyStorageIncidentsResponse(),
        };
      }
    },
  );

  const summary = createMemo(() => payload()?.summary ?? null);
  const incidents = createMemo(() => payload()?.incidents ?? null);
  const criticalRecoverabilityCount = createMemo(
    () => summary()?.byIncidentCategory?.recoverability ?? 0,
  );
  const visibleSections = createMemo(() => (incidents()?.sections || []).slice(0, 4));
  const hasActiveIncidents = createMemo(() => (incidents()?.totalResources || 0) > 0);
  const statCards = createMemo(() => [
    {
      label: 'Critical',
      value: summary()?.criticalResources || 0,
      tone: 'text-red-700 dark:text-red-300',
      helper: 'Need immediate action',
    },
    {
      label: 'Protection Reduced',
      value: summary()?.protectionReducedCount || 0,
      tone: 'text-orange-700 dark:text-orange-300',
      helper: 'Parity or redundancy weakened',
    },
    {
      label: 'Rebuild Active',
      value: summary()?.rebuildInProgressCount || 0,
      tone: 'text-blue-700 dark:text-blue-300',
      helper: 'Avoid risky storage changes',
    },
    {
      label: 'Backup Risk',
      value: criticalRecoverabilityCount(),
      tone: 'text-rose-700 dark:text-rose-300',
      helper: 'Recoverability exposure',
    },
  ]);

  return (
    <Card padding="md" tone="card" class="overflow-hidden">
      <div class="flex flex-col gap-5">
        <div class="flex flex-col gap-3 lg:flex-row lg:items-end lg:justify-between">
          <div class="space-y-1">
            <div class="text-[11px] font-semibold uppercase tracking-[0.18em] text-muted">
              Storage Command Center
            </div>
            <Show
              when={summary()}
              fallback={
                <div class="text-lg font-semibold text-base-content">
                  Building storage posture view…
                </div>
              }
            >
              <div class="text-lg font-semibold text-base-content">
                <Show when={hasActiveIncidents()} fallback="Storage posture looks stable">
                  {(active) =>
                    `${active()} active storage incidents across ${summary()?.totalResources || 0} tracked resources`
                  }
                </Show>
              </div>
              <div class="text-sm text-muted">
                <Show
                  when={hasActiveIncidents()}
                  fallback={`${compactNumber(summary()?.dependentResourceCount || 0)} dependent resources and ${compactNumber(summary()?.protectedWorkloadCount || 0)} protected workloads currently covered`}
                >
                  {`${compactNumber(summary()?.dependentResourceCount || 0)} dependent resources and ${compactNumber(summary()?.protectedWorkloadCount || 0)} protected workloads could be affected`}
                </Show>
              </div>
            </Show>
          </div>

          <div class="flex items-center gap-2 text-xs text-muted">
            <Show when={payload.loading}>
              <span class="inline-flex items-center rounded-full bg-surface-hover px-2 py-1 font-medium">
                Refreshing…
              </span>
            </Show>
            <Show when={summary()?.affectedDatastoreCount}>
              <span class="inline-flex items-center rounded-full bg-surface-hover px-2 py-1 font-medium text-base-content">
                {compactNumber(summary()?.affectedDatastoreCount || 0)} backup datastores affected
              </span>
            </Show>
          </div>
        </div>

        <Show
          when={!loadError()}
          fallback={
            <div class="text-sm text-muted">
              Storage posture is temporarily unavailable. Pool and disk details are still shown
              below.
            </div>
          }
        >
          <div class="grid gap-3 md:grid-cols-2 xl:grid-cols-4">
            <For each={statCards()}>
              {(card) => (
                <div class="rounded-xl border border-border bg-surface-alt px-4 py-3">
                  <div class="text-[11px] font-semibold uppercase tracking-wide text-muted">
                    {card.label}
                  </div>
                  <div class={`mt-2 text-2xl font-semibold ${card.tone}`}>
                    {compactNumber(card.value)}
                  </div>
                  <div class="mt-1 text-xs text-muted">{card.helper}</div>
                </div>
              )}
            </For>
          </div>

          <Show
            when={hasActiveIncidents()}
            fallback={
              <div class="rounded-xl border border-emerald-200 bg-emerald-50 px-4 py-4 text-sm text-emerald-900 dark:border-emerald-900/60 dark:bg-emerald-950/30 dark:text-emerald-200">
                No active storage incidents. Pulse is tracking{' '}
                {compactNumber(summary()?.totalResources || 0)} storage resources with no current
                protection, recoverability, rebuild, or disk-health incidents.
              </div>
            }
          >
            <div class="grid gap-4 xl:grid-cols-2">
              <For each={visibleSections()}>
                {(section) => (
                  <div
                    class={`rounded-xl border border-border bg-surface-alt p-4 ${sectionAccentClass(section.category)}`}
                  >
                    <div class="flex flex-wrap items-start justify-between gap-3">
                      <div class="space-y-1">
                        <div class="text-sm font-semibold text-base-content">{section.label}</div>
                        <div class="text-xs text-muted">
                          {section.resourceCount} resource{section.resourceCount === 1 ? '' : 's'}{' '}
                          in this lane
                        </div>
                      </div>
                      <div class="flex items-center gap-2">
                        <Show when={section.criticalResources > 0}>
                          <span class="rounded-full bg-red-100 px-2 py-1 text-[11px] font-semibold text-red-700 dark:bg-red-950/60 dark:text-red-300">
                            {section.criticalResources} critical
                          </span>
                        </Show>
                        <Show when={section.PrimaryUrgency}>
                          <span
                            class={`rounded-full px-2 py-1 text-[11px] font-semibold uppercase tracking-wide ${urgencyPillClass(section.PrimaryUrgency)}`}
                          >
                            {section.PrimaryUrgency}
                          </span>
                        </Show>
                      </div>
                    </div>

                    <div class="mt-3 space-y-3">
                      <For each={section.resources.slice(0, 3)}>
                        {(incident) => (
                          <div class="rounded-lg border border-border bg-surface px-3 py-3">
                            <div class="flex flex-wrap items-start justify-between gap-2">
                              <div class="min-w-0">
                                <div class="flex flex-wrap items-center gap-2">
                                  <div class="text-sm font-semibold text-base-content">
                                    {incident.name}
                                  </div>
                                  <Show when={incident.parentName}>
                                    <span class="text-xs text-muted">on {incident.parentName}</span>
                                  </Show>
                                </div>
                                <div class="mt-1 flex flex-wrap items-center gap-2 text-[11px] text-muted">
                                  <Show when={incident.platform}>
                                    <span>{titleize(incident.platform)}</span>
                                  </Show>
                                  <Show when={incident.topology}>
                                    <span>• {titleize(incident.topology)}</span>
                                  </Show>
                                  <span>• {incident.resourceType.replace(/_/g, ' ')}</span>
                                </div>
                              </div>

                              <div class="flex flex-wrap items-center gap-2">
                                <Show when={incident.incidentSeverity}>
                                  <span
                                    class={`rounded-full px-2 py-1 text-[11px] font-semibold uppercase tracking-wide ${severityPillClass(incident.incidentSeverity)}`}
                                  >
                                    {incident.incidentSeverity}
                                  </span>
                                </Show>
                                <Show when={incident.incidentUrgency}>
                                  <span
                                    class={`rounded-full px-2 py-1 text-[11px] font-semibold uppercase tracking-wide ${urgencyPillClass(incident.incidentUrgency)}`}
                                  >
                                    {incident.incidentUrgency}
                                  </span>
                                </Show>
                              </div>
                            </div>

                            <Show when={incident.incidentLabel}>
                              <div class="mt-2 text-xs font-semibold uppercase tracking-wide text-muted">
                                {incident.incidentLabel}
                              </div>
                            </Show>
                            <Show when={incident.incidentSummary}>
                              <div class="mt-1 text-sm text-base-content">
                                {incident.incidentSummary}
                              </div>
                            </Show>
                            <Show when={incident.incidentImpactSummary}>
                              <div class="mt-2 text-xs text-muted">
                                {incident.incidentImpactSummary}
                              </div>
                            </Show>

                            <div class="mt-3 flex flex-wrap items-center gap-2">
                              <Show when={incident.protectionReduced}>
                                <span class="rounded-full bg-orange-100 px-2 py-1 text-[11px] font-medium text-orange-800 dark:bg-orange-950/60 dark:text-orange-300">
                                  Protection reduced
                                </span>
                              </Show>
                              <Show when={incident.rebuildInProgress}>
                                <span class="rounded-full bg-blue-100 px-2 py-1 text-[11px] font-medium text-blue-800 dark:bg-blue-950/60 dark:text-blue-300">
                                  Rebuild in progress
                                </span>
                              </Show>
                              <Show when={(incident.consumerCount || 0) > 0}>
                                <span class="rounded-full bg-surface-hover px-2 py-1 text-[11px] font-medium text-base-content">
                                  {incident.consumerCount} dependent resources
                                </span>
                              </Show>
                              <Show when={(incident.protectedWorkloads || 0) > 0}>
                                <span class="rounded-full bg-surface-hover px-2 py-1 text-[11px] font-medium text-base-content">
                                  {incident.protectedWorkloads} protected workloads
                                </span>
                              </Show>
                            </div>

                            <Show when={incident.incidentAction}>
                              <div class="mt-3 rounded-md bg-base px-3 py-2 text-xs text-base-content">
                                <span class="font-semibold">Recommended action:</span>{' '}
                                {incident.incidentAction}
                              </div>
                            </Show>
                          </div>
                        )}
                      </For>
                    </div>
                  </div>
                )}
              </For>
            </div>
          </Show>
        </Show>
      </div>
    </Card>
  );
};

export default StorageCommandCenter;
