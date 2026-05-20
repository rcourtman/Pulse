import type { Component } from 'solid-js';
import { For, Show, createMemo, createResource, createSignal } from 'solid-js';
import { apiFetchJSON } from '@/utils/apiClient';
import { Card } from '@/components/shared/Card';
import { SearchField } from '@/components/shared/SearchField';
import {
  Table,
  TableHeader,
  TableBody,
  TableRow,
  TableHead,
  TableCell,
} from '@/components/shared/Table';
import { EmptyState } from '@/components/shared/EmptyState';
import {
  PLATFORM_TABLE_BODY_CLASS,
  PLATFORM_TABLE_HEADER_ROW_CLASS,
  getPlatformTableCellClassForKind,
  getPlatformTableHeadClassForKind,
} from '@/features/platformPage/sharedPlatformPage';
import { formatBytes, formatRelativeTime } from '@/utils/format';
import { getServiceHealthPresentation } from '@/utils/serviceHealthPresentation';
import { asTrimmedString } from '@/utils/stringUtils';
import {
  getPMGDetailsDrawerPresentation,
  PMG_DETAILS_FAILURE_STATE_TITLE,
  PMG_DETAILS_LOADING_STATE_DESCRIPTION,
  PMG_DETAILS_LOADING_STATE_TITLE,
  PMG_DETAILS_EMPTY_STATE_DESCRIPTION,
  PMG_DETAILS_EMPTY_STATE_TITLE,
} from '@/utils/pmgPresentation';

type PMGNodeStatus = {
  name: string;
  status: string;
  role?: string;
  uptime?: number;
  loadAvg?: string;
  queueStatus?: {
    active?: number;
    deferred?: number;
    hold?: number;
    incoming?: number;
    total?: number;
  };
};

type PMGMailStats = {
  timeframe?: string;
  countIn?: number;
  countOut?: number;
  spamIn?: number;
  spamOut?: number;
  virusIn?: number;
  virusOut?: number;
  bouncesIn?: number;
  bouncesOut?: number;
  bytesIn?: number;
  bytesOut?: number;
  greylistCount?: number;
  rblRejects?: number;
  averageProcessTimeMs?: number;
};

type PMGQuarantine = {
  spam?: number;
  virus?: number;
  attachment?: number;
  blacklisted?: number;
};

type PMGSpamBucket = {
  bucket: string;
  count: number;
};

type PMGRelayDomain = {
  domain: string;
  comment?: string;
};

type PMGDomainStat = {
  domain: string;
  mailCount: number;
  spamCount: number;
  virusCount: number;
  bytes?: number;
};

type PMGData = {
  instanceId?: string;
  hostname?: string;
  version?: string;
  nodeCount?: number;
  uptimeSeconds?: number;
  queueActive?: number;
  queueDeferred?: number;
  queueHold?: number;
  queueIncoming?: number;
  queueTotal?: number;
  mailCountTotal?: number;
  spamIn?: number;
  virusIn?: number;
  connectionHealth?: string;
  lastUpdated?: string;

  nodes?: PMGNodeStatus[];
  mailStats?: PMGMailStats;
  quarantine?: PMGQuarantine;
  spamDistribution?: PMGSpamBucket[];
  relayDomains?: PMGRelayDomain[];
  domainStats?: PMGDomainStat[];
  domainStatsAsOf?: string;
};

type UnifiedResourceResponse = {
  id: string;
  type: string;
  name?: string;
  status?: string;
  lastSeen?: string;
  customUrl?: string;
  pmg?: PMGData;
};

type PMGInstanceDrawerProps = {
  resourceId: string;
  resourceName?: string;
};

const formatCompact = (value?: number | null): string => {
  if (value === undefined || value === null || Number.isNaN(value)) return '—';
  if (value >= 1_000_000) return `${(value / 1_000_000).toFixed(1)}M`;
  if (value >= 1_000) return `${(value / 1_000).toFixed(1)}K`;
  return Math.round(value).toLocaleString();
};

export const PMGInstanceDrawer: Component<PMGInstanceDrawerProps> = (props) => {
  const [searchRelay, setSearchRelay] = createSignal('');
  const [searchDomain, setSearchDomain] = createSignal('');
  const drawerPresentation = getPMGDetailsDrawerPresentation();

  const resourceId = createMemo(() => asTrimmedString(props.resourceId) ?? '');

  const [resource] = createResource(
    resourceId,
    async (id) => {
      if (!id) return null;
      return apiFetchJSON<UnifiedResourceResponse>(`/api/resources/${encodeURIComponent(id)}`, {
        cache: 'no-store',
      });
    },
    { initialValue: null },
  );

  const loadError = createMemo(() => {
    const err = resource.error;
    if (!err) return '';
    return (err as Error)?.message || 'Failed to fetch PMG details';
  });

  const pmg = createMemo(() => resource()?.pmg ?? null);

  const lastUpdatedRelative = createMemo(() => {
    const raw = pmg()?.lastUpdated;
    if (!raw) return '';
    const parsed = Date.parse(raw);
    if (!Number.isFinite(parsed)) return '';
    return formatRelativeTime(parsed);
  });

  const domainStatsAsOfRelative = createMemo(() => {
    const raw = pmg()?.domainStatsAsOf;
    if (!raw) return '';
    const parsed = Date.parse(raw);
    if (!Number.isFinite(parsed)) return '';
    return formatRelativeTime(parsed);
  });

  const relayDomains = createMemo(() => {
    const rows = pmg()?.relayDomains ?? [];
    const term = (asTrimmedString(searchRelay()) ?? '').toLowerCase();
    if (!term) return rows;
    return rows.filter(
      (row) =>
        row.domain.toLowerCase().includes(term) || (row.comment || '').toLowerCase().includes(term),
    );
  });

  const domainStats = createMemo(() => {
    const rows = pmg()?.domainStats ?? [];
    const term = (asTrimmedString(searchDomain()) ?? '').toLowerCase();
    const filtered = term ? rows.filter((row) => row.domain.toLowerCase().includes(term)) : rows;
    return [...filtered].sort((a, b) => (b.mailCount || 0) - (a.mailCount || 0));
  });

  const spamBuckets = createMemo(() => {
    const rows = pmg()?.spamDistribution ?? [];
    const parsed = rows
      .map((row) => ({ bucket: asTrimmedString(row.bucket) ?? '', count: Number(row.count || 0) }))
      .filter((row) => row.bucket.length > 0);
    return parsed.sort((a, b) => a.bucket.localeCompare(b.bucket));
  });

  const maxSpamBucketCount = createMemo(() =>
    Math.max(0, ...spamBuckets().map((row) => row.count)),
  );

  const headingId = () => `pmg-drawer-heading-${resourceId() || 'instance'}`;
  const drawerLabel = () => asTrimmedString(props.resourceName) || resourceId() || 'PMG instance';

  return (
    <section class="space-y-3" aria-labelledby={headingId()}>
      <h2 id={headingId()} class="sr-only">
        {drawerLabel()} details
      </h2>
      <Show when={loadError()}>
        <div role="alert" aria-live="assertive">
          <Card padding="lg" tone="danger">
            <EmptyState
              title={PMG_DETAILS_FAILURE_STATE_TITLE}
              description={loadError()}
              tone="danger"
            />
          </Card>
        </div>
      </Show>
      <Show
        when={!resource.loading}
        fallback={
          <Card padding="lg">
            <EmptyState
              title={PMG_DETAILS_LOADING_STATE_TITLE}
              description={PMG_DETAILS_LOADING_STATE_DESCRIPTION}
            />
          </Card>
        }
      >
        <Show
          when={pmg()}
          fallback={
            <Card padding="lg">
              <EmptyState
                title={PMG_DETAILS_EMPTY_STATE_TITLE}
                description={PMG_DETAILS_EMPTY_STATE_DESCRIPTION}
              />
            </Card>
          }
        >
          {(pmgData) => (
            <>
              <Card padding="lg">
                <div class="flex items-start justify-between gap-4">
                  <div class="min-w-0">
                    <div
                      class="text-sm font-semibold text-base-content truncate"
                      title={
                        asTrimmedString(props.resourceName) ||
                        resource()?.name ||
                        drawerPresentation.defaultResourceName
                      }
                    >
                      {asTrimmedString(props.resourceName) ||
                        resource()?.name ||
                        drawerPresentation.defaultResourceName}
                    </div>
                    <div class="mt-1 text-xs text-muted">
                      {pmgData().hostname || drawerPresentation.unknownHostLabel}
                      <Show when={pmgData().version}>
                        <span class="mx-2 text-muted">|</span>
                        <span>v{pmgData().version}</span>
                      </Show>
                    </div>
                    <Show when={lastUpdatedRelative()}>
                      <div class="mt-1 text-[11px] text-muted">
                        {drawerPresentation.updatedPrefix} {lastUpdatedRelative()}
                      </div>
                    </Show>
                  </div>

                  <div class="shrink-0 flex items-center gap-2">
                    {(() => {
                      const health = getServiceHealthPresentation(
                        resource()?.status,
                        pmgData().connectionHealth,
                      );
                      return (
                        <>
                          <span class={`inline-block h-2.5 w-2.5 rounded-full ${health.dot}`} />
                          <span class={`text-xs font-medium ${health.text}`}>{health.label}</span>
                        </>
                      );
                    })()}
                    <Show when={resource()?.customUrl}>
                      {(url) => (
                        <a
                          class="ml-2 text-xs font-medium text-blue-700 hover:text-blue-800 dark:text-blue-300 dark:hover:text-blue-200"
                          href={url()}
                          target="_blank"
                          rel="noreferrer"
                        >
                          Open
                        </a>
                      )}
                    </Show>
                  </div>
                </div>

                <div class="mt-4 grid grid-cols-2 gap-3 text-xs sm:grid-cols-4">
                  <div class="rounded border border-border bg-surface px-3 py-2">
                    <div class="text-[10px] uppercase tracking-wide text-muted">Queue Total</div>
                    <div class="mt-1 text-sm font-semibold text-base-content">
                      {formatCompact(pmgData().queueTotal ?? 0)}
                    </div>
                  </div>
                  <div class="rounded border border-border bg-surface px-3 py-2">
                    <div class="text-[10px] uppercase tracking-wide text-muted">Deferred</div>
                    <div class="mt-1 text-sm font-semibold text-base-content">
                      {formatCompact(pmgData().queueDeferred ?? 0)}
                    </div>
                  </div>
                  <div class="rounded border border-border bg-surface px-3 py-2">
                    <div class="text-[10px] uppercase tracking-wide text-muted">Mail</div>
                    <div class="mt-1 text-sm font-semibold text-base-content">
                      {formatCompact(pmgData().mailCountTotal ?? 0)}
                    </div>
                  </div>
                  <div class="rounded border border-border bg-surface px-3 py-2">
                    <div class="text-[10px] uppercase tracking-wide text-muted">Spam/Virus</div>
                    <div class="mt-1 text-sm font-semibold text-base-content">
                      {formatCompact(pmgData().spamIn ?? 0)} /{' '}
                      {formatCompact(pmgData().virusIn ?? 0)}
                    </div>
                  </div>
                </div>
              </Card>

              <Show when={(pmgData().nodes || []).length > 0}>
                <Card padding="lg">
                  <div class="text-xs font-semibold text-base-content">
                    {drawerPresentation.nodesSectionTitle}
                  </div>
                  <Table
                    wrapperClass="mt-2"
                    class="min-w-full table-fixed text-xs md:min-w-[560px]"
                  >
                    <TableHeader>
                      <TableRow class={PLATFORM_TABLE_HEADER_ROW_CLASS}>
                        <TableHead class={`${getPlatformTableHeadClassForKind('name')} md:w-[34%]`}>
                          {drawerPresentation.nodeColumnLabel}
                        </TableHead>
                        <TableHead class={`${getPlatformTableHeadClassForKind('text')} md:w-[22%]`}>
                          {drawerPresentation.roleColumnLabel}
                        </TableHead>
                        <TableHead class={`${getPlatformTableHeadClassForKind('text')} md:w-[22%]`}>
                          {drawerPresentation.statusColumnLabel}
                        </TableHead>
                        <TableHead
                          class={`${getPlatformTableHeadClassForKind('numeric-value')} md:w-[22%]`}
                        >
                          {drawerPresentation.queueColumnLabel}
                        </TableHead>
                      </TableRow>
                    </TableHeader>
                    <TableBody class={PLATFORM_TABLE_BODY_CLASS}>
                      <For each={pmgData().nodes || []}>
                        {(node) => (
                          <TableRow class="text-[11px] sm:text-xs">
                            <TableCell
                              class={`${getPlatformTableCellClassForKind('name')} text-base-content`}
                            >
                              <span class="block truncate" title={node.name}>
                                {node.name}
                              </span>
                            </TableCell>
                            <TableCell
                              class={`${getPlatformTableCellClassForKind('text')} text-muted`}
                            >
                              <span class="block truncate" title={node.role || '—'}>
                                {node.role || '—'}
                              </span>
                            </TableCell>
                            <TableCell
                              class={`${getPlatformTableCellClassForKind('text')} text-muted`}
                            >
                              <span class="block truncate" title={node.status || '—'}>
                                {node.status || '—'}
                              </span>
                            </TableCell>
                            <TableCell
                              class={`${getPlatformTableCellClassForKind('numeric-value')} text-muted tabular-nums`}
                            >
                              {formatCompact(node.queueStatus?.total ?? 0)}
                            </TableCell>
                          </TableRow>
                        )}
                      </For>
                    </TableBody>
                  </Table>
                </Card>
              </Show>

              <Show when={(pmgData().relayDomains || []).length > 0}>
                <Card padding="lg">
                  <div class="flex items-center justify-between gap-3">
                    <div class="text-xs font-semibold text-base-content">
                      {drawerPresentation.relayDomainsSectionTitle}
                    </div>
                    <SearchField
                      value={searchRelay()}
                      onChange={setSearchRelay}
                      placeholder={drawerPresentation.domainSearchPlaceholder}
                      class="w-56"
                      inputClass="py-1 text-xs"
                    />
                  </div>
                  <Table
                    wrapperClass="mt-2"
                    class="min-w-full table-fixed text-xs md:min-w-[520px]"
                  >
                    <TableHeader>
                      <TableRow class={PLATFORM_TABLE_HEADER_ROW_CLASS}>
                        <TableHead class={`${getPlatformTableHeadClassForKind('name')} md:w-[45%]`}>
                          {drawerPresentation.domainColumnLabel}
                        </TableHead>
                        <TableHead class={`${getPlatformTableHeadClassForKind('text')} md:w-[55%]`}>
                          {drawerPresentation.commentColumnLabel}
                        </TableHead>
                      </TableRow>
                    </TableHeader>
                    <TableBody class={PLATFORM_TABLE_BODY_CLASS}>
                      <For each={relayDomains()}>
                        {(row) => (
                          <TableRow class="text-[11px] sm:text-xs">
                            <TableCell
                              class={`${getPlatformTableCellClassForKind('name')} text-base-content`}
                            >
                              <span class="block truncate" title={row.domain}>
                                {row.domain}
                              </span>
                            </TableCell>
                            <TableCell
                              class={`${getPlatformTableCellClassForKind('text')} text-muted`}
                            >
                              <span class="block truncate" title={row.comment || '—'}>
                                {row.comment || '—'}
                              </span>
                            </TableCell>
                          </TableRow>
                        )}
                      </For>
                    </TableBody>
                  </Table>
                </Card>
              </Show>

              <Show when={(pmgData().domainStats || []).length > 0}>
                <Card padding="lg">
                  <div class="flex items-center justify-between gap-3">
                    <div class="min-w-0">
                      <div class="text-xs font-semibold text-base-content">
                        {drawerPresentation.domainStatsSectionTitle}
                      </div>
                      <Show when={domainStatsAsOfRelative()}>
                        <div class="mt-0.5 text-[11px] text-muted">
                          {drawerPresentation.asOfPrefix} {domainStatsAsOfRelative()}
                        </div>
                      </Show>
                    </div>
                    <SearchField
                      value={searchDomain()}
                      onChange={setSearchDomain}
                      placeholder={drawerPresentation.domainSearchPlaceholder}
                      class="w-56"
                      inputClass="py-1 text-xs"
                    />
                  </div>
                  <Table
                    wrapperClass="mt-2"
                    class="min-w-full table-fixed text-xs md:min-w-[720px]"
                  >
                    <TableHeader>
                      <TableRow class={PLATFORM_TABLE_HEADER_ROW_CLASS}>
                        <TableHead class={`${getPlatformTableHeadClassForKind('name')} md:w-[36%]`}>
                          {drawerPresentation.domainColumnLabel}
                        </TableHead>
                        <TableHead
                          class={`${getPlatformTableHeadClassForKind('numeric-value')} md:w-[16%]`}
                        >
                          {drawerPresentation.mailColumnLabel}
                        </TableHead>
                        <TableHead
                          class={`${getPlatformTableHeadClassForKind('numeric-value')} md:w-[16%]`}
                        >
                          {drawerPresentation.spamColumnLabel}
                        </TableHead>
                        <TableHead
                          class={`${getPlatformTableHeadClassForKind('numeric-value')} md:w-[16%]`}
                        >
                          {drawerPresentation.virusColumnLabel}
                        </TableHead>
                        <TableHead
                          class={`${getPlatformTableHeadClassForKind('numeric-value')} md:w-[16%]`}
                        >
                          {drawerPresentation.bytesColumnLabel}
                        </TableHead>
                      </TableRow>
                    </TableHeader>
                    <TableBody class={PLATFORM_TABLE_BODY_CLASS}>
                      <For each={domainStats()}>
                        {(row) => (
                          <TableRow class="text-[11px] sm:text-xs">
                            <TableCell
                              class={`${getPlatformTableCellClassForKind('name')} text-base-content`}
                            >
                              <span class="block truncate" title={row.domain}>
                                {row.domain}
                              </span>
                            </TableCell>
                            <TableCell
                              class={`${getPlatformTableCellClassForKind('numeric-value')} text-muted tabular-nums`}
                            >
                              {formatCompact(row.mailCount)}
                            </TableCell>
                            <TableCell
                              class={`${getPlatformTableCellClassForKind('numeric-value')} text-muted tabular-nums`}
                            >
                              {formatCompact(row.spamCount)}
                            </TableCell>
                            <TableCell
                              class={`${getPlatformTableCellClassForKind('numeric-value')} text-muted tabular-nums`}
                            >
                              {formatCompact(row.virusCount)}
                            </TableCell>
                            <TableCell
                              class={`${getPlatformTableCellClassForKind('numeric-value')} text-muted tabular-nums`}
                            >
                              {row.bytes ? formatBytes(row.bytes) : '—'}
                            </TableCell>
                          </TableRow>
                        )}
                      </For>
                    </TableBody>
                  </Table>
                </Card>
              </Show>

              <Show when={spamBuckets().length > 0}>
                <Card padding="lg">
                  <div class="text-xs font-semibold text-base-content">
                    {drawerPresentation.spamDistributionSectionTitle}
                  </div>
                  <div class="mt-3 space-y-2">
                    <For each={spamBuckets()}>
                      {(bucket) => (
                        <div class="flex items-center gap-3">
                          <div class="w-16 text-[11px] font-medium text-muted">{bucket.bucket}</div>
                          <div class="flex-1 h-2 rounded bg-surface-alt overflow-hidden">
                            <div
                              class="h-full bg-amber-500"
                              style={{
                                width: `${maxSpamBucketCount() > 0 ? Math.round((bucket.count / maxSpamBucketCount()) * 100) : 0}%`,
                              }}
                            />
                          </div>
                          <div class="w-14 text-right text-[11px] text-muted">
                            {formatCompact(bucket.count)}
                          </div>
                        </div>
                      )}
                    </For>
                  </div>
                </Card>
              </Show>
            </>
          )}
        </Show>
      </Show>
    </section>
  );
};

export default PMGInstanceDrawer;
